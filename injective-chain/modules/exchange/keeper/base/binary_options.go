package base

import (
	"cosmossdk.io/store/prefix"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k *BaseKeeper) GetBinaryOptionsMarketByID(ctx sdk.Context, marketID common.Hash) *v2.BinaryOptionsMarket {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	market := k.GetBinaryOptionsMarket(ctx, marketID, true)
	if market != nil {
		return market
	}

	return k.GetBinaryOptionsMarket(ctx, marketID, false)
}

func (k *BaseKeeper) IterateBinaryOptionsMarketExpiryTimestamps(
	ctx sdk.Context,
	endTimestampLimit uint64,
	process func(marketID common.Hash) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	expirationStore := prefix.NewStore(k.getStore(ctx), types.BinaryOptionsMarketExpiryTimestampPrefix)
	endTimestampLimitBytes := sdk.Uint64ToBigEndian(endTimestampLimit)

	iterateSafe(expirationStore.Iterator(nil, endTimestampLimitBytes), func(_, value []byte) bool {
		marketID := common.BytesToHash(value)
		return process(marketID)
	})
}

func (k *BaseKeeper) IterateBinaryOptionsMarketSettlementTimestamps(
	ctx sdk.Context,
	endTimestampLimit uint64,
	process func(marketIDBytes []byte) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	settlementStore := prefix.NewStore(k.getStore(ctx), types.BinaryOptionsMarketSettlementTimestampPrefix)
	endTimestampLimitBytes := sdk.Uint64ToBigEndian(endTimestampLimit)

	iterateSafe(settlementStore.Iterator(nil, endTimestampLimitBytes), func(_, value []byte) bool {
		return process(value)
	})
}

// HasBinaryOptionsMarket returns true the if the binary options market exists in the store.
func (k *BaseKeeper) HasBinaryOptionsMarket(ctx sdk.Context, marketID common.Hash, isEnabled bool) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetBinaryOptionsMarketKey(isEnabled, marketID)
	return store.Has(key)
}

// GetBinaryOptionsMarket fetches the binary options Market from the store by marketID.
func (k *BaseKeeper) GetBinaryOptionsMarket(ctx sdk.Context, marketID common.Hash, isEnabled bool) *v2.BinaryOptionsMarket {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	marketStore := prefix.NewStore(store, types.GetBinaryOptionsMarketPrefix(isEnabled))

	bz := marketStore.Get(marketID.Bytes())
	if bz == nil {
		return nil
	}

	var market v2.BinaryOptionsMarket
	k.cdc.MustUnmarshal(bz, &market)

	return &market
}

func (k *BaseKeeper) SetBinaryOptionsMarket(ctx sdk.Context, market *v2.BinaryOptionsMarket) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	isEnabled := market.IsActive()
	marketID := market.MarketID()

	marketStore := prefix.NewStore(store, types.GetBinaryOptionsMarketPrefix(isEnabled))
	bz := k.cdc.MustMarshal(market)
	marketStore.Set(marketID.Bytes(), bz)
}

// DeleteBinaryOptionsMarket deletes Binary Options Market from the markets store (needed for moving to another hash).
func (k *BaseKeeper) DeleteBinaryOptionsMarket(ctx sdk.Context, market *v2.BinaryOptionsMarket, isEnabled bool) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	marketID := market.MarketID()

	k.DeleteBinaryOptionsMarketExpiryTimestampIndex(ctx, marketID, market.ExpirationTimestamp)
	k.DeleteBinaryOptionsMarketSettlementTimestampIndex(ctx, marketID, market.SettlementTimestamp)

	marketStore := prefix.NewStore(store, types.GetBinaryOptionsMarketPrefix(isEnabled))

	marketStore.Delete(marketID.Bytes())
}

// DeleteBinaryOptionsMarketExpiryTimestampIndex deletes the binary options market's market id index from the keeper.
func (k *BaseKeeper) DeleteBinaryOptionsMarketExpiryTimestampIndex(ctx sdk.Context, marketID common.Hash, expirationTimestamp int64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetBinaryOptionsMarketExpiryTimestampKey(expirationTimestamp, marketID)
	store.Delete(key)
}

// SetBinaryOptionsMarketExpiryTimestampIndex saves the binary options market id keyed by expiration timestamp
func (k *BaseKeeper) SetBinaryOptionsMarketExpiryTimestampIndex(ctx sdk.Context, marketID common.Hash, expirationTimestamp int64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetBinaryOptionsMarketExpiryTimestampKey(expirationTimestamp, marketID)
	store.Set(key, marketID.Bytes())
}

// DeleteBinaryOptionsMarketSettlementTimestampIndex deletes the binary options market's market id index from the keeper.
func (k *BaseKeeper) DeleteBinaryOptionsMarketSettlementTimestampIndex(ctx sdk.Context, marketID common.Hash, settlementTimestamp int64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetBinaryOptionsMarketSettlementTimestampKey(settlementTimestamp, marketID)
	store.Delete(key)
}

// SetBinaryOptionsMarketSettlementTimestampIndex saves the binary options market id keyed by settlement timestamp
func (k *BaseKeeper) SetBinaryOptionsMarketSettlementTimestampIndex(ctx sdk.Context, marketID common.Hash, settlementTimestamp int64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetBinaryOptionsMarketSettlementTimestampKey(settlementTimestamp, marketID)
	store.Set(key, marketID.Bytes())
}

// ScheduleBinaryOptionsMarketForSettlement saves the Binary Options market ID into the keeper to be settled later in the next BeginBlocker
func (k *BaseKeeper) ScheduleBinaryOptionsMarketForSettlement(ctx sdk.Context, marketID common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	settlementStore := prefix.NewStore(store, types.BinaryOptionsMarketSettlementSchedulePrefix)
	settlementStore.Set(marketID.Bytes(), marketID.Bytes())
}

// IterateScheduledBinaryOptionsMarketSettlements iterates over binary options markets ready to be settled, calling process on each one.
func (k *BaseKeeper) IterateScheduledBinaryOptionsMarketSettlements(ctx sdk.Context, process func(marketID common.Hash) (stop bool)) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	settlementStore := prefix.NewStore(store, types.BinaryOptionsMarketSettlementSchedulePrefix)

	iterateSafe(settlementStore.Iterator(nil, nil), func(_, value []byte) bool {
		marketID := common.BytesToHash(value)
		return process(marketID)
	})
}

// RemoveScheduledSettlementOfBinaryOptionsMarket removes scheduled market id from the store
func (k *BaseKeeper) RemoveScheduledSettlementOfBinaryOptionsMarket(ctx sdk.Context, marketID common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	settlementStore := prefix.NewStore(store, types.BinaryOptionsMarketSettlementSchedulePrefix)
	settlementStore.Delete(marketID.Bytes())
}

// IterateBinaryOptionsMarkets iterates over binary options markets calling process on each market.
func (k *BaseKeeper) IterateBinaryOptionsMarkets(
	ctx sdk.Context,
	isEnabled *bool,
	process func(market *v2.BinaryOptionsMarket) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	var marketStore prefix.Store
	if isEnabled != nil {
		marketStore = prefix.NewStore(store, types.GetBinaryOptionsMarketPrefix(*isEnabled))
	} else {
		marketStore = prefix.NewStore(store, types.BinaryOptionsMarketPrefix)
	}

	iterateSafe(marketStore.Iterator(nil, nil), func(_, value []byte) bool {
		var market v2.BinaryOptionsMarket
		k.cdc.MustUnmarshal(value, &market)
		return process(&market)
	})
}

func (k *BaseKeeper) ScheduleBinaryOptionsMarketParamUpdate(ctx sdk.Context, p *v2.BinaryOptionsMarketParamUpdateProposal) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	marketID := common.HexToHash(p.MarketId)
	paramUpdateStore := prefix.NewStore(store, types.BinaryOptionsMarketParamUpdateSchedulePrefix)
	bz := k.cdc.MustMarshal(p)
	paramUpdateStore.Set(marketID.Bytes(), bz)
	return nil
}

// IterateBinaryOptionsMarketParamUpdates iterates over DerivativeMarketParamUpdates calling process on each pair.
func (k *BaseKeeper) IterateBinaryOptionsMarketParamUpdates(
	ctx sdk.Context, process func(*v2.BinaryOptionsMarketParamUpdateProposal,
	) (stop bool)) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	paramUpdateStore := prefix.NewStore(store, types.BinaryOptionsMarketParamUpdateSchedulePrefix)

	proposals := []*v2.BinaryOptionsMarketParamUpdateProposal{}

	iterateSafe(paramUpdateStore.Iterator(nil, nil), func(_, v []byte) bool {
		var proposal v2.BinaryOptionsMarketParamUpdateProposal
		k.cdc.MustUnmarshal(v, &proposal)
		proposals = append(proposals, &proposal)
		return false
	})

	for _, p := range proposals {
		if process(p) {
			return
		}
	}
}

// GetAllBinaryOptionsMarkets returns all binary options markets.
func (k *BaseKeeper) GetAllBinaryOptionsMarkets(ctx sdk.Context) []*v2.BinaryOptionsMarket {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	markets := make([]*v2.BinaryOptionsMarket, 0)
	k.IterateBinaryOptionsMarkets(ctx, nil, func(p *v2.BinaryOptionsMarket) (stop bool) {
		markets = append(markets, p)

		return false
	})

	return markets
}

// GetAllActiveBinaryOptionsMarkets returns all active binary options markets.
func (k *BaseKeeper) GetAllActiveBinaryOptionsMarkets(ctx sdk.Context) []*v2.BinaryOptionsMarket {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	isEnabled := true
	markets := make([]*v2.BinaryOptionsMarket, 0)
	k.IterateBinaryOptionsMarkets(ctx, &isEnabled, func(p *v2.BinaryOptionsMarket) (stop bool) {
		if p.Status == v2.MarketStatus_Active {
			markets = append(markets, p)
		}

		return false
	})

	return markets
}

// the ugly truth behind this method being here is because
// bo and derv keepers are designed so that bo is a superset of derv
// as bo is really derv under the hood. However, the original SetBinaryOptionsMarket
// was doing more that just writing to the store. Currently, it's impossible to put this
// in bo keeper as that would introduce a circular dependency, derv would need bo and vice versa.
// DemolishOrPauseGenericMarket - it belongs to derivative keeper but the logic is also applicable to a bo market
//
// todo(dusan):
//nolint:revive // ok
// 1. (easy): keep this ugly piece here but move event emission outside on every call site (as that was the case previously) - done
// 2. (harder): the exact business logic that's indifferent to derv and bo markets should reside in its own package (market settlement, eg. DemolishOrPauseGenericMarket)

// SaveBinaryOptionsMarket saves the binary options market in keeper.
func (k *BaseKeeper) SaveBinaryOptionsMarket(ctx sdk.Context, market *v2.BinaryOptionsMarket) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	isEnabled := market.IsActive()
	marketID := market.MarketID()

	if k.HasBinaryOptionsMarket(ctx, marketID, !isEnabled) {
		k.DeleteBinaryOptionsMarket(ctx, market, !isEnabled)
	}

	k.SetBinaryOptionsMarket(ctx, market)

	switch market.Status {
	case v2.MarketStatus_Active:
		k.SetBinaryOptionsMarketExpiryTimestampIndex(ctx, marketID, market.ExpirationTimestamp)
		k.SetBinaryOptionsMarketSettlementTimestampIndex(ctx, marketID, market.SettlementTimestamp)
	case v2.MarketStatus_Expired:
		// delete the expiry timestamp index (if any), since the market is expired
		k.DeleteBinaryOptionsMarketExpiryTimestampIndex(ctx, marketID, market.ExpirationTimestamp)
		k.SetBinaryOptionsMarketSettlementTimestampIndex(ctx, marketID, market.SettlementTimestamp)
	case v2.MarketStatus_Demolished:
		// delete the expiry and settlement timestamp index (if any), since the market is demolished
		k.DeleteBinaryOptionsMarketExpiryTimestampIndex(ctx, marketID, market.ExpirationTimestamp)
		k.DeleteBinaryOptionsMarketSettlementTimestampIndex(ctx, marketID, market.SettlementTimestamp)
		k.RemoveScheduledSettlementOfBinaryOptionsMarket(ctx, marketID)
	default:
	}
}
