package base

import (
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// GetOrderbookPriceLevelQuantity gets the aggregate quantity of the orders for a given market at a given price
//
//nolint:revive // ok
func (k *BaseKeeper) GetOrderbookPriceLevelQuantity(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy,
	isSpot bool,
	price math.LegacyDec,
) math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var key []byte
	if isSpot {
		key = types.GetSpotOrderbookLevelsForPriceKey(marketID, isBuy, price)
	} else {
		key = types.GetDerivativeOrderbookLevelsForPriceKey(marketID, isBuy, price)
	}

	// check transient store first
	tStore := k.getTransientStore(ctx)
	bz := tStore.Get(key)

	if bz != nil {
		return types.UnsignedDecBytesToDec(bz)
	}

	store := k.getStore(ctx)
	bz = store.Get(key)

	if bz == nil {
		return math.LegacyZeroDec()
	}

	return types.UnsignedDecBytesToDec(bz)
}

// SetOrderbookPriceLevelQuantity sets the orderbook price level.
//
//nolint:revive // ok
func (k *BaseKeeper) SetOrderbookPriceLevelQuantity(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy,
	isSpot bool,
	price,
	quantity math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var key []byte
	if isSpot {
		key = types.GetSpotOrderbookLevelsForPriceKey(marketID, isBuy, price)
	} else {
		key = types.GetDerivativeOrderbookLevelsForPriceKey(marketID, isBuy, price)
	}
	bz := types.UnsignedDecToUnsignedDecBytes(quantity)

	store := k.getStore(ctx)
	if quantity.IsZero() {
		store.Delete(key)
	} else {
		store.Set(key, bz)
	}

	//// set transient store value to 0 in order to emit this info in the event
	tStore := k.getTransientStore(ctx)
	tStore.Set(key, bz)
}

// IterateTransientOrderbookPriceLevels iterates over the transient orderbook price levels (so it cointains only price levels changed in this block), calling process on each level.
//
//nolint:revive // ok
func (k *BaseKeeper) IterateTransientOrderbookPriceLevels(
	ctx sdk.Context,
	isSpot bool,
	process func(marketID common.Hash, isBuy bool, priceLevel *v2.Level) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	var priceLevelStore prefix.Store
	if isSpot {
		priceLevelStore = prefix.NewStore(store, types.SpotOrderbookLevelsPrefix)
	} else {
		priceLevelStore = prefix.NewStore(store, types.DerivativeOrderbookLevelsPrefix)
	}

	keys := [][]byte{}
	values := [][]byte{}

	iterateSafe(priceLevelStore.Iterator(nil, nil), func(k, v []byte) bool {
		keys = append(keys, k)
		values = append(values, v)
		return false
	})

	for idx, key := range keys {
		marketID := common.BytesToHash(key[:common.HashLength])
		isBuy := types.IsTrueByte(key[common.HashLength : common.HashLength+1])
		price := types.GetPriceFromPaddedPrice(string(key[common.HashLength+1:]))
		quantity := types.UnsignedDecBytesToDec(values[idx])

		if process(marketID, isBuy, v2.NewLevel(price, quantity)) {
			return
		}
	}
}

// GetOrderbookPriceLevels returns the orderbook in price-sorted order (descending for buys, ascending for sells) - using persistent store.
//
//nolint:revive // ok
func (k *BaseKeeper) GetOrderbookPriceLevels(
	ctx sdk.Context,
	isSpot bool,
	marketID common.Hash,
	isBuy bool,
	limit *uint64,
	limitCumulativeNotional, // optionally retrieve only top positions up to this cumulative notional value (useful when calc. worst price for BUY)
	limitCumulativeQuantity *math.LegacyDec, // optionally retrieve only top positions up to this cumulative quantity value (useful when calc. worst price for SELL)
) []*v2.Level {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var storeKey []byte
	if isSpot {
		storeKey = types.GetSpotOrderbookLevelsKey(marketID, isBuy)
	} else {
		storeKey = types.GetDerivativeOrderbookLevelsKey(marketID, isBuy)
	}

	store := k.getStore(ctx)
	priceLevelStore := prefix.NewStore(store, storeKey)
	var iter storetypes.Iterator

	if isBuy {
		iter = priceLevelStore.ReverseIterator(nil, nil)
	} else {
		iter = priceLevelStore.Iterator(nil, nil)
	}

	levels := make([]*v2.Level, 0)
	cumulativeNotional := math.LegacyZeroDec()
	cumulativeQuantity := math.LegacyZeroDec()

	iterateSafe(iter, func(key, value []byte) bool {
		if limit != nil && uint64(len(levels)) == *limit {
			return true
		}
		if limitCumulativeNotional != nil && cumulativeNotional.GTE(*limitCumulativeNotional) {
			return true
		}
		if limitCumulativeQuantity != nil && cumulativeQuantity.GTE(*limitCumulativeQuantity) {
			return true
		}

		price := types.GetPriceFromPaddedPrice(string(key))
		quantity := types.UnsignedDecBytesToDec(value)
		levels = append(levels, v2.NewLevel(price, quantity))
		if limitCumulativeNotional != nil {
			cumulativeNotional = cumulativeNotional.Add(quantity.Mul(price))
		}
		if limitCumulativeQuantity != nil {
			cumulativeQuantity = cumulativeQuantity.Add(quantity)
		}
		return false
	})

	return levels
}

// GetOrderbookSequence gets the orderbook sequence for a given marketID.
func (k *BaseKeeper) GetOrderbookSequence(ctx sdk.Context, marketID common.Hash) uint64 {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	sequenceStore := prefix.NewStore(store, types.OrderbookSequencePrefix)
	bz := sequenceStore.Get(marketID.Bytes())
	if bz == nil {
		return 0
	}

	return sdk.BigEndianToUint64(bz)
}

// GetAllOrderbookSequences gets all the orderbook sequences.
func (k *BaseKeeper) GetAllOrderbookSequences(ctx sdk.Context) []*v2.OrderbookSequence {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	sequenceStore := prefix.NewStore(store, types.OrderbookSequencePrefix)

	orderbookSequences := make([]*v2.OrderbookSequence, 0)

	iterateSafe(sequenceStore.Iterator(nil, nil), func(key, value []byte) bool {
		marketID := common.BytesToHash(key)
		sequence := sdk.BigEndianToUint64(value)
		orderbookSequences = append(orderbookSequences, &v2.OrderbookSequence{
			Sequence: sequence,
			MarketId: marketID.Hex(),
		})
		return false
	})

	return orderbookSequences
}

// SetOrderbookSequence sets the orderbook sequence for a given marketID.
func (k *BaseKeeper) SetOrderbookSequence(ctx sdk.Context, marketID common.Hash, sequence uint64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	sequenceStore := prefix.NewStore(store, types.OrderbookSequencePrefix)
	sequenceStore.Set(marketID.Bytes(), sdk.Uint64ToBigEndian(sequence))
}

func (k *BaseKeeper) IncrementOrderbookPriceLevelQuantity(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy,
	isSpot bool,
	price,
	quantity math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if quantity.IsZero() {
		return
	}

	oldQuantity := k.GetOrderbookPriceLevelQuantity(ctx, marketID, isBuy, isSpot, price)
	newQuantity := oldQuantity.Add(quantity)

	k.SetOrderbookPriceLevelQuantity(ctx, marketID, isBuy, isSpot, price, newQuantity)
}

// DecrementOrderbookPriceLevelQuantity decrements the orderbook price level.
func (k *BaseKeeper) DecrementOrderbookPriceLevelQuantity(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy,
	isSpot bool,
	price,
	quantity math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if quantity.IsZero() {
		return
	}

	oldQuantity := k.GetOrderbookPriceLevelQuantity(ctx, marketID, isBuy, isSpot, price)
	newQuantity := oldQuantity.Sub(quantity)

	k.SetOrderbookPriceLevelQuantity(ctx, marketID, isBuy, isSpot, price, newQuantity)
}

// IncrementOrderbookSequence increments the orderbook sequence and returns the new sequence
func (k *BaseKeeper) IncrementOrderbookSequence(
	ctx sdk.Context,
	marketID common.Hash,
) uint64 {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	sequence := k.GetOrderbookSequence(ctx, marketID)
	sequence++
	k.SetOrderbookSequence(ctx, marketID, sequence)
	return sequence
}
