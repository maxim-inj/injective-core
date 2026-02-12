package base

import (
	"cosmossdk.io/store/prefix"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// GetAuctionExchangeTransferDenomDecimals returns the decimals of the given denom.
func (k *BaseKeeper) GetAuctionExchangeTransferDenomDecimals(ctx sdk.Context, denom string) uint64 {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetDenomDecimalsKey(denom))

	if bz == nil && types.IsPeggyToken(denom) {
		return 18
	}

	if bz == nil && types.IsIBCDenom(denom) {
		return 6
	}

	if bz == nil {
		return 0
	}

	decimals := sdk.BigEndianToUint64(bz)
	return decimals
}

// SetAuctionExchangeTransferDenomDecimals saves the decimals of the given denom.
func (k *BaseKeeper) SetAuctionExchangeTransferDenomDecimals(ctx sdk.Context, denom string, decimals uint64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Set(types.GetDenomDecimalsKey(denom), sdk.Uint64ToBigEndian(decimals))
}

// DeleteAuctionExchangeTransferDenomDecimals delete the decimals of the given denom.
func (k *BaseKeeper) DeleteAuctionExchangeTransferDenomDecimals(ctx sdk.Context, denom string) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.GetDenomDecimalsKey(denom))
}

// GetAllAuctionExchangeTransferDenomDecimals returns all denom decimals
func (k *BaseKeeper) GetAllAuctionExchangeTransferDenomDecimals(ctx sdk.Context) []v2.DenomDecimals {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	denomDecimals := make([]v2.DenomDecimals, 0)
	k.IterateAuctionExchangeTransferDenomDecimals(ctx, func(p v2.DenomDecimals) (stop bool) {
		denomDecimals = append(denomDecimals, p)
		return false
	})

	return denomDecimals
}

// IterateAuctionExchangeTransferDenomDecimals iterates over denom decimals calling process on each denom decimal.
func (k *BaseKeeper) IterateAuctionExchangeTransferDenomDecimals(ctx sdk.Context, process func(denomDecimal v2.DenomDecimals) (stop bool)) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	denomDecimalStore := prefix.NewStore(store, types.DenomDecimalsPrefix)

	iterateSafe(denomDecimalStore.Iterator(nil, nil), func(key, value []byte) bool {
		denom := string(key)
		decimals := sdk.BigEndianToUint64(value)
		return process(v2.DenomDecimals{
			Denom:    denom,
			Decimals: decimals,
		})
	})
}
