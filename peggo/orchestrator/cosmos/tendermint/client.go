package tendermint

import (
	"context"

	"github.com/InjectiveLabs/coretracer"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	comettypes "github.com/cometbft/cometbft/rpc/core/types"
	log "github.com/xlab/suplog"
)

type Client interface {
	GetBlock(ctx context.Context, height int64) (*comettypes.ResultBlock, error)
}

type tmClient struct {
	rpcClient rpcclient.Client
	svcTags   coretracer.Tags
}

func NewRPCClient(rpcNodeAddr string) Client {
	rpcClient, err := rpchttp.NewWithTimeout(rpcNodeAddr, 10)
	if err != nil {
		log.WithError(err).Fatalln("failed to init rpcClient")
	}

	return &tmClient{
		rpcClient: rpcClient,
		svcTags:   coretracer.NewTag("svc", "tendermint"),
	}
}

// GetBlock queries for a block by height
func (c *tmClient) GetBlock(ctx context.Context, height int64) (*comettypes.ResultBlock, error) {
	defer coretracer.Trace(&ctx, c.svcTags)()

	return c.rpcClient.Block(ctx, &height)
}
