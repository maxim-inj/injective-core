package keeper

import (
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

// GetAllBandRelayers fetches all band price relayers.
func (k *Keeper) GetAllBandRelayers(ctx sdk.Context) []string {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	bandRelayers := make([]string, 0)
	store := ctx.KVStore(k.storeKey)
	bandRelayerStore := prefix.NewStore(store, types.BandRelayerKey)

	iterator := bandRelayerStore.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		relayer := sdk.AccAddress(iterator.Key())
		bandRelayers = append(bandRelayers, relayer.String())
	}

	return bandRelayers
}

// GetBandPriceState reads the stored price state.
func (k *Keeper) GetBandPriceState(ctx sdk.Context, symbol string) *types.BandPriceState {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var priceState types.BandPriceState
	bz := k.getStore(ctx).Get(types.GetBandPriceStoreKey(symbol))
	if bz == nil {
		return nil
	}

	k.cdc.MustUnmarshal(bz, &priceState)
	return &priceState
}

// GetBandReferencePrice fetches prices for a given pair in math.LegacyDec
func (k *Keeper) GetBandReferencePrice(ctx sdk.Context, base, quote string) *math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()
	// query ref by using GetBandPriceState
	basePriceState := k.GetBandPriceState(ctx, base)
	if basePriceState == nil {
		return nil
	}

	if quote == types.QuoteUSD {
		return &basePriceState.PriceState.Price
	}

	quotePriceState := k.GetBandPriceState(ctx, quote)

	if quotePriceState == nil {
		return nil
	}

	baseRate := basePriceState.Rate.ToLegacyDec()
	quoteRate := quotePriceState.Rate.ToLegacyDec()

	if baseRate.IsNil() || quoteRate.IsNil() || !baseRate.IsPositive() || !quoteRate.IsPositive() {
		return nil
	}

	price := baseRate.Quo(quoteRate)
	return &price
}

// GetAllBandPriceStates reads all stored band price states.
func (k *Keeper) GetAllBandPriceStates(ctx sdk.Context) []*types.BandPriceState {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	priceStates := make([]*types.BandPriceState, 0)
	store := ctx.KVStore(k.storeKey)
	bandPriceStore := prefix.NewStore(store, types.BandPriceKey)

	iterator := bandPriceStore.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var bandPriceState types.BandPriceState
		k.cdc.MustUnmarshal(iterator.Value(), &bandPriceState)
		priceStates = append(priceStates, &bandPriceState)
	}

	return priceStates
}
