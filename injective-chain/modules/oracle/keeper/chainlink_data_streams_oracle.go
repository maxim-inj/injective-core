package keeper

import (
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/metrics"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

// ChainlinkDataStreamsKeeper defines the interface for Chainlink Data Streams operations.
type ChainlinkDataStreamsKeeper interface {
	GetChainlinkDataStreamsPrice(ctx sdk.Context, base, quote string) *math.LegacyDec
	SetChainlinkDataStreamsPriceState(ctx sdk.Context, priceState *types.ChainlinkDataStreamsPriceState)
	GetChainlinkDataStreamsPriceState(ctx sdk.Context, feedID string) *types.ChainlinkDataStreamsPriceState
	GetAllChainlinkDataStreamsPriceStates(ctx sdk.Context) []*types.ChainlinkDataStreamsPriceState
}

// GetChainlinkDataStreamsPrice gets price for a given base/quote pair.
func (k *Keeper) GetChainlinkDataStreamsPrice(ctx sdk.Context, base, quote string) *math.LegacyDec {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	basePriceState := k.GetChainlinkDataStreamsPriceState(ctx, base)
	if basePriceState == nil {
		return nil
	}

	if quote == types.QuoteUSD {
		return &basePriceState.PriceState.Price
	}

	quotePriceState := k.GetChainlinkDataStreamsPriceState(ctx, quote)
	if quotePriceState == nil {
		return nil
	}

	basePrice := basePriceState.PriceState.Price
	quotePrice := quotePriceState.PriceState.Price

	if basePrice.IsNil() || quotePrice.IsNil() || !basePrice.IsPositive() || !quotePrice.IsPositive() {
		return nil
	}

	price := basePrice.Quo(quotePrice)
	return &price
}

// SetChainlinkDataStreamsPriceState stores a given Chainlink Data Streams price state.
func (k *Keeper) SetChainlinkDataStreamsPriceState(ctx sdk.Context, priceState *types.ChainlinkDataStreamsPriceState) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	priceKey := types.GetChainlinkDataStreamsPriceStoreKey(priceState.FeedId)
	bz := k.cdc.MustMarshal(priceState)

	k.getStore(ctx).Set(priceKey, bz)

	k.AppendPriceRecord(ctx, types.OracleType_ChainlinkDataStreams, priceState.FeedId, &types.PriceRecord{
		Timestamp: priceState.PriceState.Timestamp,
		Price:     priceState.PriceState.Price,
	})
}

// GetChainlinkDataStreamsPriceState retrieves the Chainlink Data Streams price state for a given feed ID.
func (k *Keeper) GetChainlinkDataStreamsPriceState(ctx sdk.Context, feedID string) *types.ChainlinkDataStreamsPriceState {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	var priceState types.ChainlinkDataStreamsPriceState
	bz := k.getStore(ctx).Get(types.GetChainlinkDataStreamsPriceStoreKey(feedID))
	if bz == nil {
		return nil
	}

	k.cdc.MustUnmarshal(bz, &priceState)
	return &priceState
}

// GetAllChainlinkDataStreamsPriceStates fetches all Chainlink Data Streams price states.
func (k *Keeper) GetAllChainlinkDataStreamsPriceStates(ctx sdk.Context) []*types.ChainlinkDataStreamsPriceState {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	priceStates := make([]*types.ChainlinkDataStreamsPriceState, 0)
	store := ctx.KVStore(k.storeKey)

	priceStore := prefix.NewStore(store, types.ChainlinkDataStreamsPriceKey)

	iter := priceStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var priceState types.ChainlinkDataStreamsPriceState
		k.cdc.MustUnmarshal(iter.Value(), &priceState)
		priceStates = append(priceStates, &priceState)
	}

	return priceStates
}

// ProcessChainlinkDataStreamsReport processes a Chainlink Data Streams report and updates the price state.
func (k *Keeper) ProcessChainlinkDataStreamsReport(
	ctx sdk.Context,
	feedID string,
	reportPrice math.Int,
	validFromTimestamp uint64,
	observationsTimestamp uint64,
	price math.LegacyDec,
) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	priceState := k.GetChainlinkDataStreamsPriceState(ctx, feedID)
	blockTime := ctx.BlockTime().Unix()

	if priceState == nil {
		priceState = types.NewChainlinkDataStreamsPriceState(
			feedID,
			reportPrice,
			validFromTimestamp,
			observationsTimestamp,
			price,
			blockTime,
		)
	} else {
		// don't update prices with an older observation timestamp
		if priceState.ObservationsTimestamp >= observationsTimestamp {
			return
		}

		// skip price update if the price changes beyond 100x or less than 1% of the last price
		if types.CheckPriceFeedThreshold(priceState.PriceState.Price, price) {
			return
		}
		priceState.Update(reportPrice, validFromTimestamp, observationsTimestamp, price, blockTime)
	}

	k.SetChainlinkDataStreamsPriceState(ctx, priceState)

	// nolint:errcheck //ignored on purpose
	ctx.EventManager().EmitTypedEvent(&types.EventSetChainlinkDataStreamsPrices{
		Prices: []*types.ChainlinkDataStreamsPriceState{priceState},
	})
}
