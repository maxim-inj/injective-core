package oracle

import (
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/keeper"
)

type BlockHandler struct {
	k keeper.Keeper

	svcTags metrics.Tags
}

func NewBlockHandler(k keeper.Keeper) *BlockHandler {
	return &BlockHandler{
		k: k,

		svcTags: metrics.Tags{
			"svc": "oracle_b",
		},
	}
}

func (h *BlockHandler) BeginBlocker(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, h.svcTags)
	defer doneFn()

	if ctx.BlockHeight()%100000 == 0 {
		h.k.CleanupHistoricalPriceRecords(ctx)
	}
}
