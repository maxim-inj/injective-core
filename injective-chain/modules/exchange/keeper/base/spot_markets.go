package base

import (
	"cosmossdk.io/store/prefix"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k *BaseKeeper) GetSpotMarketByID(ctx sdk.Context, marketID common.Hash) *v2.SpotMarket {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	market := k.GetSpotMarket(ctx, marketID, true)
	if market != nil {
		return market
	}

	return k.GetSpotMarket(ctx, marketID, false)
}

// IsSpotExchangeEnabled returns true if Spot Exchange is enabled
func (k *BaseKeeper) IsSpotExchangeEnabled(ctx sdk.Context) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	return store.Has(types.SpotExchangeEnabledKey)
}

// SetSpotExchangeEnabled sets the indicator to enable spot exchange
func (k *BaseKeeper) SetSpotExchangeEnabled(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Set(types.SpotExchangeEnabledKey, []byte{1})
}

// HasSpotMarket returns true if SpotMarket exists by ID.
func (k *BaseKeeper) HasSpotMarket(ctx sdk.Context, marketID common.Hash, isEnabled bool) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	marketStore := prefix.NewStore(store, types.GetSpotMarketKey(isEnabled))
	return marketStore.Has(marketID.Bytes())
}

// GetSpotMarket returns Spot Market from marketID.
func (k *BaseKeeper) GetSpotMarket(ctx sdk.Context, marketID common.Hash, isEnabled bool) *v2.SpotMarket {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	marketStore := prefix.NewStore(store, types.GetSpotMarketKey(isEnabled))

	bz := marketStore.Get(marketID.Bytes())
	if bz == nil {
		return nil
	}

	var market v2.SpotMarket
	k.cdc.MustUnmarshal(bz, &market)

	return &market
}

func (k *BaseKeeper) ScheduleSpotMarketParamUpdate(ctx sdk.Context, p *v2.SpotMarketParamUpdateProposal) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	marketID := common.HexToHash(p.MarketId)

	paramUpdateStore := prefix.NewStore(store, types.SpotMarketParamUpdateScheduleKey)
	bz := k.cdc.MustMarshal(p)
	paramUpdateStore.Set(marketID.Bytes(), bz)
}

// IterateSpotMarketParamUpdates iterates over SpotMarketParamUpdates calling process on each pair.
func (k *BaseKeeper) IterateSpotMarketParamUpdates(ctx sdk.Context, process func(*v2.SpotMarketParamUpdateProposal) (stop bool)) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	paramUpdateStore := prefix.NewStore(store, types.SpotMarketParamUpdateScheduleKey)
	proposals := []*v2.SpotMarketParamUpdateProposal{}

	iterateSafe(paramUpdateStore.Iterator(nil, nil), func(_, v []byte) bool {
		var proposal v2.SpotMarketParamUpdateProposal
		k.cdc.MustUnmarshal(v, &proposal)
		proposals = append(proposals, &proposal)
		return false
	})

	for _, proposal := range proposals {
		if process(proposal) {
			return
		}
	}
}

// SetSpotMarket sets SpotMarket in keeper.
func (k *BaseKeeper) SetSpotMarket(ctx sdk.Context, spotMarket *v2.SpotMarket) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	marketID := common.HexToHash(spotMarket.MarketId)
	isEnabled := spotMarket.Status == v2.MarketStatus_Active
	marketStore := prefix.NewStore(store, types.GetSpotMarketKey(isEnabled))
	bz := k.cdc.MustMarshal(spotMarket)
	marketStore.Set(marketID.Bytes(), bz)
}

// DeleteSpotMarket deletes SpotMarket from keeper (needed for moving to another hash).
func (k *BaseKeeper) DeleteSpotMarket(ctx sdk.Context, marketID common.Hash, isEnabled bool) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	marketStore := prefix.NewStore(store, types.GetSpotMarketKey(isEnabled))
	bz := marketStore.Get(marketID.Bytes())
	if bz == nil {
		return
	}

	marketStore.Delete(marketID.Bytes())
}

// IterateSpotMarkets iterates over SpotMarkets calling process on each pair.
func (k *BaseKeeper) IterateSpotMarkets(ctx sdk.Context, isEnabled *bool, process func(*v2.SpotMarket) (stop bool)) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	var marketStore prefix.Store
	if isEnabled != nil {
		marketStore = prefix.NewStore(store, types.GetSpotMarketKey(*isEnabled))
	} else {
		marketStore = prefix.NewStore(store, types.SpotMarketsPrefix)
	}

	iterateSafe(marketStore.Iterator(nil, nil), func(_, value []byte) bool {
		var market v2.SpotMarket
		k.cdc.MustUnmarshal(value, &market)
		return process(&market)
	})
}

// IterateForceCloseSpotMarkets iterates over Spot market settlement infos calling process on each info.
func (k *BaseKeeper) IterateForceCloseSpotMarkets(ctx sdk.Context, process func(common.Hash) (stop bool)) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	marketStore := prefix.NewStore(store, types.SpotMarketForceCloseInfoKey)

	iterateSafe(marketStore.Iterator(nil, nil), func(_, value []byte) bool {
		marketIDResult := common.BytesToHash(value)
		return process(marketIDResult)
	})
}

// GetSpotMarketForceCloseInfo gets the SpotMarketForceCloseInfo from the keeper.
func (k *BaseKeeper) GetSpotMarketForceCloseInfo(ctx sdk.Context, marketID common.Hash) *common.Hash {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	settlementStore := prefix.NewStore(store, types.SpotMarketForceCloseInfoKey)

	bz := settlementStore.Get(marketID.Bytes())
	if bz == nil {
		return nil
	}

	marketIDResult := common.BytesToHash(bz)
	return &marketIDResult
}

// SetSpotMarketForceCloseInfo saves the SpotMarketSettlementInfo to the keeper.
func (k *BaseKeeper) SetSpotMarketForceCloseInfo(ctx sdk.Context, marketID common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	settlementStore := prefix.NewStore(store, types.SpotMarketForceCloseInfoKey)
	settlementStore.Set(marketID.Bytes(), marketID.Bytes())
}

// DeleteSpotMarketForceCloseInfo deletes the SpotMarketForceCloseInfo from the keeper.
func (k *BaseKeeper) DeleteSpotMarketForceCloseInfo(ctx sdk.Context, marketID common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	settlementStore := prefix.NewStore(store, types.SpotMarketForceCloseInfoKey)

	bz := settlementStore.Get(marketID.Bytes())
	if bz == nil {
		return
	}

	settlementStore.Delete(marketID.Bytes())
}

func (k *BaseKeeper) GetAllSpotMarkets(ctx sdk.Context) []*v2.SpotMarket {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	spotMarkets := make([]*v2.SpotMarket, 0)
	k.IterateSpotMarkets(ctx, nil, func(m *v2.SpotMarket) (stop bool) {
		spotMarkets = append(spotMarkets, m)
		return false
	})

	return spotMarkets
}
