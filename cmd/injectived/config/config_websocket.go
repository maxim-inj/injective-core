package config

import (
	"time"

	injwebsocket "github.com/InjectiveLabs/injective-core/injective-chain/websocket"
)

// WebsocketConfig defines configuration for the chainstream websocket server.
type WebsocketConfig struct {
	Address string `mapstructure:"address"`

	MaxOpenConnections  int           `mapstructure:"max-open-connections"`
	ReadTimeout         time.Duration `mapstructure:"read-timeout"`
	WriteTimeout        time.Duration `mapstructure:"write-timeout"`
	MaxBodyBytes        int64         `mapstructure:"max-body-bytes"`
	MaxHeaderBytes      int           `mapstructure:"max-header-bytes"`
	MaxRequestBatchSize int           `mapstructure:"max-request-batch-size"`
}

// DefaultWebsocketConfig returns the default websocket configuration.
func DefaultWebsocketConfig() *WebsocketConfig {
	rpcCfg := injwebsocket.DefaultRPCConfig()
	return &WebsocketConfig{
		Address:             "",
		MaxOpenConnections:  rpcCfg.MaxOpenConnections,
		ReadTimeout:         rpcCfg.ReadTimeout,
		WriteTimeout:        rpcCfg.WriteTimeout,
		MaxBodyBytes:        rpcCfg.MaxBodyBytes,
		MaxHeaderBytes:      rpcCfg.MaxHeaderBytes,
		MaxRequestBatchSize: rpcCfg.MaxRequestBatchSize,
	}
}
