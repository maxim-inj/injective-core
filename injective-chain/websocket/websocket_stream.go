package websocket

import (
	"context"

	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/InjectiveLabs/injective-core/injective-chain/stream/types/v2"
)

// This is a compile-time assertion to ensure that WsStream satisfies the grpc.ServerStream interface
var _ grpc.ServerStream = &WsStream{}

type WsStream struct {
	ctx      context.Context
	id       rpctypes.JSONRPCIntID
	cancelFn func()
	wsConn   rpctypes.WSRPCConnection
}

// NewWsStream creates a new WsStream instance
func NewWsStream(ctx context.Context, id rpctypes.JSONRPCIntID, cancelFn func(), wsConn rpctypes.WSRPCConnection) *WsStream {
	return &WsStream{
		ctx:      ctx,
		id:       id,
		cancelFn: cancelFn,
		wsConn:   wsConn,
	}
}

func (ws *WsStream) Send(sr *v2.StreamResponse) error {
	return ws.wsConn.WriteRPCResponse(ws.ctx, rpctypes.NewRPCSuccessResponse(ws.id, *sr))
}

func (ws *WsStream) Context() context.Context {
	return ws.ctx
}

// The following methods satisfy grpc.ServerStream so WsStream can be passed to
// StreamV2 without relying on a nil-embedded ServerStream. They return
// unimplemented errors because the websocket adapter does not support direct
// gRPC stream plumbing.
func (*WsStream) SetHeader(_ metadata.MD) error {
	return status.Error(codes.Unimplemented, "SetHeader not supported for websocket stream")
}

func (*WsStream) SendHeader(_ metadata.MD) error {
	return status.Error(codes.Unimplemented, "SendHeader not supported for websocket stream")
}

func (*WsStream) SetTrailer(_ metadata.MD) {}

func (*WsStream) SendMsg(_ any) error {
	return status.Error(codes.Unimplemented, "SendMsg not supported for websocket stream")
}

func (*WsStream) RecvMsg(_ any) error {
	return status.Error(codes.Unimplemented, "RecvMsg not supported for websocket stream")
}
