package base

import (
	"cosmossdk.io/store/prefix"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k *BaseKeeper) GetDerivativeMarketByID(ctx sdk.Context, marketID common.Hash) *v2.DerivativeMarket {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	market := k.GetDerivativeMarket(ctx, marketID, true)
	if market != nil {
		return market
	}

	return k.GetDerivativeMarket(ctx, marketID, false)
}

// IsDerivativesExchangeEnabled returns true if Derivatives Exchange is enabled
func (k *BaseKeeper) IsDerivativesExchangeEnabled(ctx sdk.Context) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	return store.Has(types.DerivativeExchangeEnabledKey)
}

// SetDerivativesExchangeEnabled sets the indicator to enable derivatives exchange
func (k *BaseKeeper) SetDerivativesExchangeEnabled(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Set(types.DerivativeExchangeEnabledKey, []byte{1})
}

// HasDerivativeMarket returns true the if the derivative market exists in the store.
func (k *BaseKeeper) HasDerivativeMarket(ctx sdk.Context, marketID common.Hash, isEnabled bool) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	marketStore := prefix.NewStore(store, types.GetDerivativeMarketPrefix(isEnabled))
	return marketStore.Has(marketID.Bytes())
}

// GetDerivativeMarket fetches the Derivative Market from the store by marketID.
func (k *BaseKeeper) GetDerivativeMarket(ctx sdk.Context, marketID common.Hash, isEnabled bool) *v2.DerivativeMarket {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	marketStore := prefix.NewStore(store, types.GetDerivativeMarketPrefix(isEnabled))

	bz := marketStore.Get(marketID.Bytes())
	if bz == nil {
		return nil
	}

	var market v2.DerivativeMarket
	k.cdc.MustUnmarshal(bz, &market)

	return &market
}

// SetDerivativeMarket saves derivative market in keeper.
func (k *BaseKeeper) SetDerivativeMarket(ctx sdk.Context, market *v2.DerivativeMarket) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	isEnabled := market.IsActive()
	marketID := market.MarketID()
	preExistingMarketOfOpposingStatus := k.HasDerivativeMarket(ctx, marketID, !isEnabled)

	if preExistingMarketOfOpposingStatus {
		k.DeleteDerivativeMarket(ctx, marketID, !isEnabled)
	}

	marketStore := prefix.NewStore(store, types.GetDerivativeMarketPrefix(isEnabled))
	bz := k.cdc.MustMarshal(market)
	marketStore.Set(marketID.Bytes(), bz)
}

// DeleteDerivativeMarket deletes DerivativeMarket from the markets store (needed for moving to another hash).
func (k *BaseKeeper) DeleteDerivativeMarket(ctx sdk.Context, marketID common.Hash, isEnabled bool) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	marketStore := prefix.NewStore(store, types.GetDerivativeMarketPrefix(isEnabled))
	bz := marketStore.Get(marketID.Bytes())
	if bz == nil {
		return
	}

	marketStore.Delete(marketID.Bytes())
}

// IterateDerivativeMarkets iterates over derivative markets calling process on each market.
func (k *BaseKeeper) IterateDerivativeMarkets(ctx sdk.Context, isEnabled *bool, process func(*v2.DerivativeMarket) (stop bool)) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	var marketStore prefix.Store
	if isEnabled != nil {
		marketStore = prefix.NewStore(store, types.GetDerivativeMarketPrefix(*isEnabled))
	} else {
		marketStore = prefix.NewStore(store, types.DerivativeMarketPrefix)
	}

	iterateSafe(marketStore.Iterator(nil, nil), func(_, value []byte) bool {
		var market v2.DerivativeMarket
		k.cdc.MustUnmarshal(value, &market)
		return process(&market)
	})
}

func (k *BaseKeeper) ScheduleDerivativeMarketParamUpdate(ctx sdk.Context, p *v2.DerivativeMarketParamUpdateProposal) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	marketID := common.HexToHash(p.MarketId)
	paramUpdateStore := prefix.NewStore(store, types.DerivativeMarketParamUpdateScheduleKey)
	bz := k.cdc.MustMarshal(p)
	paramUpdateStore.Set(marketID.Bytes(), bz)
}

// IterateDerivativeMarketParamUpdates iterates over DerivativeMarketParamUpdates calling process on each pair.
func (k *BaseKeeper) IterateDerivativeMarketParamUpdates(
	ctx sdk.Context,
	process func(*v2.DerivativeMarketParamUpdateProposal,
	) (stop bool)) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	paramUpdateStore := prefix.NewStore(store, types.DerivativeMarketParamUpdateScheduleKey)
	proposals := []*v2.DerivativeMarketParamUpdateProposal{}

	iterateSafe(paramUpdateStore.Iterator(nil, nil), func(_, v []byte) bool {
		var proposal v2.DerivativeMarketParamUpdateProposal
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

// IterateScheduledSettlementDerivativeMarkets iterates over derivative market settlement infos calling
// process on each info.
func (k *BaseKeeper) IterateScheduledSettlementDerivativeMarkets(
	ctx sdk.Context,
	process func(v2.DerivativeMarketSettlementInfo) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	marketStore := prefix.NewStore(store, types.DerivativeMarketScheduledSettlementInfo)

	iterateSafe(marketStore.Iterator(nil, nil), func(_, value []byte) bool {
		var marketSettlementInfo v2.DerivativeMarketSettlementInfo
		k.cdc.MustUnmarshal(value, &marketSettlementInfo)
		return process(marketSettlementInfo)
	})
}

// GetDerivativesMarketScheduledSettlementInfo gets the DerivativeMarketSettlementInfo from the keeper.
func (k *BaseKeeper) GetDerivativesMarketScheduledSettlementInfo(ctx sdk.Context, marketID common.Hash) *v2.DerivativeMarketSettlementInfo {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	settlementStore := prefix.NewStore(store, types.DerivativeMarketScheduledSettlementInfo)

	bz := settlementStore.Get(marketID.Bytes())
	if bz == nil {
		return nil
	}

	var derivativeMarketSettlementInfo v2.DerivativeMarketSettlementInfo
	k.cdc.MustUnmarshal(bz, &derivativeMarketSettlementInfo)
	return &derivativeMarketSettlementInfo
}

// SetDerivativesMarketScheduledSettlementInfo saves the DerivativeMarketSettlementInfo to the keeper.
func (k *BaseKeeper) SetDerivativesMarketScheduledSettlementInfo(ctx sdk.Context, settlementInfo *v2.DerivativeMarketSettlementInfo) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	marketID := common.HexToHash(settlementInfo.MarketId)
	settlementStore := prefix.NewStore(store, types.DerivativeMarketScheduledSettlementInfo)
	bz := k.cdc.MustMarshal(settlementInfo)

	settlementStore.Set(marketID.Bytes(), bz)
}

// DeleteDerivativesMarketScheduledSettlementInfo deletes the DerivativeMarketSettlementInfo from the keeper.
func (k *BaseKeeper) DeleteDerivativesMarketScheduledSettlementInfo(ctx sdk.Context, marketID common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	settlementStore := prefix.NewStore(store, types.DerivativeMarketScheduledSettlementInfo)

	bz := settlementStore.Get(marketID.Bytes())
	if bz == nil {
		return
	}

	settlementStore.Delete(marketID.Bytes())
}

func (k *BaseKeeper) GetAllDerivativeMarkets(ctx sdk.Context) []*v2.DerivativeMarket {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	markets := make([]*v2.DerivativeMarket, 0)
	k.IterateDerivativeMarkets(ctx, nil, func(p *v2.DerivativeMarket) (stop bool) {
		markets = append(markets, p)
		return false
	})

	return markets
}

// GetAllActiveDerivativeAndBinaryOptionsMarkets returns all active derivative markets and binary options markets.
func (k *BaseKeeper) GetAllActiveDerivativeAndBinaryOptionsMarkets(ctx sdk.Context) []v2.DerivativeMarketI {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	derivativeMarkets := k.GetAllActiveDerivativeMarkets(ctx)
	binaryOptionsMarkets := k.GetAllActiveBinaryOptionsMarkets(ctx)

	derivativeMarketsI := make([]v2.DerivativeMarketI, 0, len(derivativeMarkets)+len(binaryOptionsMarkets))

	for _, market := range derivativeMarkets {
		derivativeMarketsI = append(derivativeMarketsI, market)
	}

	for _, market := range binaryOptionsMarkets {
		derivativeMarketsI = append(derivativeMarketsI, market)
	}

	return derivativeMarketsI
}
