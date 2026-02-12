package base

import (
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// GetPerpetualMarketFunding gets the perpetual market funding state from the keeper
func (k *BaseKeeper) GetPerpetualMarketFunding(ctx sdk.Context, marketID common.Hash) *v2.PerpetualMarketFunding {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	fundingStore := prefix.NewStore(store, types.PerpetualMarketFundingPrefix)

	bz := fundingStore.Get(marketID.Bytes())
	if bz == nil {
		return nil
	}

	var funding v2.PerpetualMarketFunding
	k.cdc.MustUnmarshal(bz, &funding)

	return &funding
}

// SetPerpetualMarketFunding saves the perpetual market funding to the keeper
func (k *BaseKeeper) SetPerpetualMarketFunding(ctx sdk.Context, marketID common.Hash, funding *v2.PerpetualMarketFunding) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	fundingStore := prefix.NewStore(store, types.PerpetualMarketFundingPrefix)
	key := marketID.Bytes()
	bz := k.cdc.MustMarshal(funding)
	fundingStore.Set(key, bz)
}

// IteratePerpetualMarketFundings iterates over perpetual market funding state calling process on each funding state
func (k *BaseKeeper) IteratePerpetualMarketFundings(ctx sdk.Context, process func(*v2.PerpetualMarketFunding, common.Hash) (stop bool)) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	fundingStore := prefix.NewStore(store, types.PerpetualMarketFundingPrefix)

	iterateSafe(fundingStore.Iterator(nil, nil), func(key, value []byte) bool {
		marketID := common.BytesToHash(key)
		var funding v2.PerpetualMarketFunding
		k.cdc.MustUnmarshal(value, &funding)
		return process(&funding, marketID)
	})
}

func (k *BaseKeeper) GetPerpetualMarketInfo(ctx sdk.Context, marketID common.Hash) *v2.PerpetualMarketInfo {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	perpetualMarketInfoStore := prefix.NewStore(store, types.PerpetualMarketInfoPrefix)

	bz := perpetualMarketInfoStore.Get(marketID.Bytes())
	if bz == nil {
		return nil
	}

	var marketInfo v2.PerpetualMarketInfo
	k.cdc.MustUnmarshal(bz, &marketInfo)

	return &marketInfo
}

// SetPerpetualMarketInfo saves the perpetual market's market info to the keeper
func (k *BaseKeeper) SetPerpetualMarketInfo(ctx sdk.Context, marketID common.Hash, marketInfo *v2.PerpetualMarketInfo) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	perpetualMarketInfoStore := prefix.NewStore(store, types.PerpetualMarketInfoPrefix)
	key := marketID.Bytes()
	bz := k.cdc.MustMarshal(marketInfo)
	perpetualMarketInfoStore.Set(key, bz)
}

// IteratePerpetualMarketInfos iterates over perpetual market's market info calling process on each market info
func (k *BaseKeeper) IteratePerpetualMarketInfos(ctx sdk.Context, process func(*v2.PerpetualMarketInfo, common.Hash) (stop bool)) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	perpetualMarketInfoStore := prefix.NewStore(store, types.PerpetualMarketInfoPrefix)

	iterateSafe(perpetualMarketInfoStore.Iterator(nil, nil), func(key, value []byte) bool {
		var marketInfo v2.PerpetualMarketInfo
		k.cdc.MustUnmarshal(value, &marketInfo)
		return process(&marketInfo, common.BytesToHash(key))
	})
}

// GetAllActiveDerivativeMarkets returns all active derivative markets.
func (k *BaseKeeper) GetAllActiveDerivativeMarkets(ctx sdk.Context) []*v2.DerivativeMarket {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	isEnabled := true
	markets := make([]*v2.DerivativeMarket, 0)
	k.IterateDerivativeMarkets(ctx, &isEnabled, func(p *v2.DerivativeMarket) (stop bool) {
		if p.Status == v2.MarketStatus_Active {
			markets = append(markets, p)
		}

		return false
	})

	return markets
}

// AccumulateAtomicPerpetualVwap accumulates VWAP data from atomic orders in transient storage
// so it can be merged with regular trades in EndBlocker.
func (k *BaseKeeper) AccumulateAtomicPerpetualVwap(ctx sdk.Context, marketID common.Hash, markPrice, vwapPrice, vwapQuantity math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	key := types.GetTransientAtomicPerpetualVwapKey(marketID)

	var existingMarkPrice, existingPrice, existingQuantity math.LegacyDec

	if bz := store.Get(key); bz != nil {
		existingMarkPrice, existingPrice, existingQuantity = decodeTransientVwapData(bz)
	} else {
		existingMarkPrice = markPrice
		existingPrice = math.LegacyZeroDec()
		existingQuantity = math.LegacyZeroDec()
	}

	// Merge VWAP data: newVwap = (oldPrice * oldQty + newPrice * newQty) / (oldQty + newQty)
	newQuantity := existingQuantity.Add(vwapQuantity)
	var newPrice math.LegacyDec
	if newQuantity.IsZero() {
		newPrice = math.LegacyZeroDec()
	} else {
		newPrice = existingPrice.Mul(existingQuantity).Add(vwapPrice.Mul(vwapQuantity)).Quo(newQuantity)
	}

	bz := encodeTransientVwapData(existingMarkPrice, newPrice, newQuantity)
	store.Set(key, bz)
}

// GetAllAtomicPerpetualVwap retrieves all accumulated atomic VWAP data from transient storage.
// Returns a map of marketID -> VwapInfo.
func (k *BaseKeeper) GetAllAtomicPerpetualVwap(ctx sdk.Context) map[common.Hash]*v2.VwapInfo {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	vwapStore := prefix.NewStore(store, types.TransientAtomicPerpetualVwapPrefix)

	result := make(map[common.Hash]*v2.VwapInfo)

	iterateSafe(vwapStore.Iterator(nil, nil), func(key, value []byte) bool {
		marketID := common.BytesToHash(key)
		markPrice, vwapPrice, vwapQuantity := decodeTransientVwapData(value)
		result[marketID] = &v2.VwapInfo{
			MarkPrice: &markPrice,
			VwapData: &v2.VwapData{
				Price:    vwapPrice,
				Quantity: vwapQuantity,
			},
		}
		return false
	})

	return result
}

// encodeTransientVwapData encodes mark price, VWAP price and quantity to bytes.
// Format: [len][markPrice][len][price][len][quantity] where len is 2 bytes (big-endian uint16)
func encodeTransientVwapData(markPrice, price, quantity math.LegacyDec) []byte {
	markPriceBz := types.UnsignedDecToUnsignedDecBytes(markPrice)
	priceBz := types.UnsignedDecToUnsignedDecBytes(price)
	quantityBz := types.UnsignedDecToUnsignedDecBytes(quantity)

	buf := make([]byte, 0, 6+len(markPriceBz)+len(priceBz)+len(quantityBz))
	buf = appendLengthPrefixed(buf, markPriceBz)
	buf = appendLengthPrefixed(buf, priceBz)
	buf = appendLengthPrefixed(buf, quantityBz)

	return buf
}

// decodeTransientVwapData decodes bytes back to mark price, VWAP price and quantity.
func decodeTransientVwapData(bz []byte) (markPrice, price, quantity math.LegacyDec) {
	var data []byte

	data, bz = readLengthPrefixed(bz)
	markPrice = types.UnsignedDecBytesToDec(data)

	data, bz = readLengthPrefixed(bz)
	price = types.UnsignedDecBytesToDec(data)

	data, _ = readLengthPrefixed(bz)
	quantity = types.UnsignedDecBytesToDec(data)

	return markPrice, price, quantity
}

// appendLengthPrefixed appends data with a 2-byte big-endian length prefix.
func appendLengthPrefixed(buf, data []byte) []byte {
	buf = append(buf, byte(len(data)>>8), byte(len(data)))
	return append(buf, data...)
}

// readLengthPrefixed reads a length-prefixed value and returns the remaining bytes.
func readLengthPrefixed(bz []byte) (data, remaining []byte) {
	length := int(bz[0])<<8 | int(bz[1])
	return bz[2 : 2+length], bz[2+length:]
}
