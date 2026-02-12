package websocket

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"strings"
	"sync"

	sdklog "cosmossdk.io/log"
	tmlog "github.com/cometbft/cometbft/libs/log"
	rpcserver "github.com/cometbft/cometbft/rpc/jsonrpc/server"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"

	chainstreamserver "github.com/InjectiveLabs/injective-core/injective-chain/stream/server"
	"github.com/InjectiveLabs/injective-core/injective-chain/stream/types/v2"
)

const ResponseSuccess = "success"

// SubscribeRequest represents a subscription request with a client-provided ID.
type SubscribeRequest struct {
	// SubscriptionID is a client-provided unique identifier for this subscription.
	// The client must use this ID to unsubscribe later.
	SubscriptionID string `json:"subscription_id"`
	// Filter contains the stream filters for the subscription.
	Filter *v2.StreamRequest `json:"filter"`
}

// UnsubscribeRequest represents an unsubscribe request.
type UnsubscribeRequest struct {
	// SubscriptionID is the client-provided identifier used when subscribing.
	SubscriptionID string `json:"subscription_id"`
}

type Server struct {
	streamSvr     *chainstreamserver.StreamServer
	manager       *rpcserver.WebsocketManager
	subscriptions map[string]map[string]*WsStream
	mux           *sync.RWMutex
	logger        sdklog.Logger
	tmLogger      tmlog.Logger
	rpcConfig     *rpcserver.Config
	listener      net.Listener
}

type cometLoggerAdapter struct {
	logger sdklog.Logger
}

func newCometLogger(logger sdklog.Logger) tmlog.Logger {
	if logger == nil {
		return tmlog.NewNopLogger()
	}
	return &cometLoggerAdapter{logger: logger}
}

func (c *cometLoggerAdapter) Debug(msg string, keyvals ...any) {
	if c.logger == nil {
		return
	}
	c.logger.Debug(msg, keyvals...)
}

func (c *cometLoggerAdapter) Info(msg string, keyvals ...any) {
	if c.logger == nil {
		return
	}
	c.logger.Info(msg, keyvals...)
}

func (c *cometLoggerAdapter) Error(msg string, keyvals ...any) {
	if c.logger == nil {
		return
	}
	c.logger.Error(msg, keyvals...)
}

func (c *cometLoggerAdapter) With(keyvals ...any) tmlog.Logger {
	if c.logger == nil {
		return tmlog.NewNopLogger()
	}
	return &cometLoggerAdapter{logger: c.logger.With(keyvals...)}
}

func NewServer(streamSvr *chainstreamserver.StreamServer, logger sdklog.Logger) *Server {
	var moduleLogger sdklog.Logger
	if logger != nil {
		moduleLogger = logger.With("module", "websocket")
	}
	tmLogger := newCometLogger(moduleLogger)
	s := &Server{
		streamSvr:     streamSvr,
		subscriptions: map[string]map[string]*WsStream{},
		mux:           new(sync.RWMutex),
		logger:        moduleLogger,
		tmLogger:      tmLogger,
		rpcConfig:     rpcserver.DefaultConfig(),
	}
	fnMap := map[string]*rpcserver.RPCFunc{
		"subscribe":   rpcserver.NewWSRPCFunc(s.subscribe, "req"),
		"unsubscribe": rpcserver.NewWSRPCFunc(s.unsubscribe, "req"),
	}
	s.manager = rpcserver.NewWebsocketManager(
		fnMap,
		rpcserver.OnDisconnect(s.onDisconnect),
	)
	s.manager.SetLogger(tmLogger)
	return s
}

func (s *Server) WithMaxOpenConnections(maxOpenConnections int) {
	s.rpcConfig.MaxOpenConnections = maxOpenConnections
}

func (s *Server) WithRPCConfig(update func(cfg *rpcserver.Config)) {
	if update == nil {
		return
	}
	update(s.rpcConfig)
}

func (s *Server) HasSubscriber(subscriber string) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()
	_, exist := s.subscriptions[subscriber]
	return exist
}

func (s *Server) GetAllSubscriptions(subscriber string) (map[string]*WsStream, bool) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	subscriptions, exist := s.subscriptions[subscriber]
	if !exist {
		return nil, false
	}

	// Return a shallow copy to avoid race conditions when caller iterates
	// without holding the lock
	return maps.Clone(subscriptions), true
}

func (s *Server) GetSubscription(subscriber, subscriptionID string) (*WsStream, bool) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	subscriptions, exist := s.subscriptions[subscriber]
	if !exist {
		return nil, false
	}

	ws, ok := subscriptions[subscriptionID]
	return ws, ok
}

func (s *Server) SetSubscription(subscriber, subscriptionID string, ws *WsStream) {
	s.mux.Lock()
	defer s.mux.Unlock()
	_, exist := s.subscriptions[subscriber]
	if !exist {
		s.subscriptions[subscriber] = map[string]*WsStream{}
	}

	s.subscriptions[subscriber][subscriptionID] = ws
}

func (s *Server) SetSubscriptionIfNotExists(subscriber, subscriptionID string, ws *WsStream) *WsStream {
	s.mux.Lock()
	defer s.mux.Unlock()

	subscriptions, exist := s.subscriptions[subscriber]
	if exist {
		if existing, ok := subscriptions[subscriptionID]; ok {
			return existing
		}
	} else {
		s.subscriptions[subscriber] = map[string]*WsStream{}
	}

	s.subscriptions[subscriber][subscriptionID] = ws
	return nil
}

func (s *Server) DeleteSubscription(subscriber, subscriptionID string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	_, exist := s.subscriptions[subscriber]
	if !exist {
		return
	}

	delete(s.subscriptions[subscriber], subscriptionID)
}

func (s *Server) DeleteSubscriber(subscriber string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	_, exist := s.subscriptions[subscriber]
	if !exist {
		return
	}

	delete(s.subscriptions, subscriber)
}

func (s *Server) subscribe(ctx *rpctypes.Context, req *SubscribeRequest) (string, error) {
	requestID, ok := ctx.JSONReq.ID.(rpctypes.JSONRPCIntID)
	if !ok {
		return "", errors.New("invalid request: expected non-negative int as id")
	}

	if req.SubscriptionID == "" {
		return "", errors.New("subscription_id is required")
	}

	if req.Filter == nil {
		return "", errors.New("filter is required")
	}

	if err := req.Filter.Validate(); err != nil {
		return "", fmt.Errorf("invalid filter: %w", err)
	}

	subscriber := ctx.RemoteAddr()
	subscriptionID := req.SubscriptionID
	cancelCtx, cancelFn := context.WithCancel(context.Background())

	ws := &WsStream{
		ctx:      cancelCtx,
		id:       requestID,
		cancelFn: cancelFn,
		wsConn:   ctx.WSConn,
	}

	if existingStream := s.SetSubscriptionIfNotExists(subscriber, subscriptionID, ws); existingStream != nil {
		cancelFn() // Clean up the unused context
		return "", fmt.Errorf("subscription_id already exists: %s", subscriptionID)
	}

	go func() {
		defer func() {
			cancelFn() // Ensure context is cancelled when goroutine exits
			s.DeleteSubscription(subscriber, subscriptionID)
		}()
		if err := s.streamSvr.StreamV2(req.Filter, ws); err != nil {
			_ = ctx.WSConn.WriteRPCResponse(ws.ctx, rpctypes.NewRPCErrorResponse(requestID, 1, "stream error", ""))
		}
	}()

	return ResponseSuccess, nil
}

func (s *Server) unsubscribe(ctx *rpctypes.Context, req *UnsubscribeRequest) (string, error) {
	_, ok := ctx.JSONReq.ID.(rpctypes.JSONRPCIntID)
	if !ok {
		return "", errors.New("invalid request: expected non-negative int as id")
	}

	if req.SubscriptionID == "" {
		return "", errors.New("subscription_id is required")
	}

	subscriber := ctx.RemoteAddr()
	ws, subscriptionExist := s.GetSubscription(subscriber, req.SubscriptionID)
	if subscriptionExist {
		ws.cancelFn()
		s.DeleteSubscription(subscriber, req.SubscriptionID)
		return ResponseSuccess, nil
	}
	return "", fmt.Errorf("subscription not found: %s", req.SubscriptionID)
}

func (s *Server) onDisconnect(subscriber string) {
	subscriptions, exist := s.GetAllSubscriptions(subscriber)
	if !exist {
		return
	}

	for _, r := range subscriptions {
		r.cancelFn()
	}
	s.DeleteSubscriber(subscriber)
}

func (s *Server) Serve(addr string) error {
	listenAddr := ensureAddressScheme(addr)
	mux := http.NewServeMux()
	mux.HandleFunc("/injstream-ws", s.manager.WebsocketHandler)

	config := s.rpcConfig

	listener, err := rpcserver.Listen(listenAddr, config.MaxOpenConnections)
	if err != nil {
		return err
	}

	s.listener = listener

	go func() {
		if s.logger != nil {
			s.logger.Info("websocket server started", "address", listenAddr)
		}
		if err := rpcserver.Serve(listener, mux, s.tmLogger, config); err != nil {
			if s.logger != nil {
				s.logger.Error("websocket server stopped", "error", err)
			}
		}
	}()

	return nil
}

// Stop gracefully shuts down the websocket server by closing the listener
// and cancelling all active subscriptions.
func (s *Server) Stop() {
	if s.logger != nil {
		s.logger.Info("stopping websocket server")
	}

	// Close the listener to stop accepting new connections
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			if s.logger != nil {
				s.logger.Error("failed to close websocket listener", "error", err)
			}
		}
	}

	// Cancel all active subscriptions
	s.mux.Lock()
	for subscriber, subscriptions := range s.subscriptions {
		for _, ws := range subscriptions {
			ws.cancelFn()
		}
		delete(s.subscriptions, subscriber)
	}
	s.mux.Unlock()
}

func ensureAddressScheme(addr string) string {
	if strings.Contains(addr, "://") {
		return addr
	}

	return fmt.Sprintf("tcp://%s", addr)
}

func DefaultRPCConfig() *rpcserver.Config {
	cfg := rpcserver.DefaultConfig()
	return cfg
}
