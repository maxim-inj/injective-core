package base

import (
	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// GetParams returns the total set of exchange parameters.
func (k *BaseKeeper) GetParams(ctx sdk.Context) v2.Params {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.ParamsKey)
	if bz == nil {
		return v2.Params{}
	}

	var params v2.Params
	k.cdc.MustUnmarshal(bz, &params)

	return params
}

// SetParams set the params
func (k *BaseKeeper) SetParams(ctx sdk.Context, params v2.Params) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Set(types.ParamsKey, k.cdc.MustMarshal(&params))
}

func (k *BaseKeeper) IsPostOnlyMode(ctx sdk.Context) bool {
	return k.GetParams(ctx).PostOnlyModeHeightThreshold > ctx.BlockHeight()
}

func (k *BaseKeeper) GetMinimalProtocolFeeRate(ctx sdk.Context, market v2.MarketI) math.LegacyDec {
	if market.GetDisabledMinimalProtocolFee() {
		return math.LegacyZeroDec()
	}

	return k.GetParams(ctx).MinimalProtocolFeeRate
}
