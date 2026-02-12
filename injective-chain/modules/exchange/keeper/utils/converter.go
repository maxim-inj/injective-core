package utils //nolint:revive // ok

import (
	"slices"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

type ChainValuesConverter interface {
	PriceFromChainFormat(price math.LegacyDec) math.LegacyDec
	QuantityFromChainFormat(quantity math.LegacyDec) math.LegacyDec
	NotionalFromChainFormat(notional math.LegacyDec) math.LegacyDec
	PriceToChainFormat(humanReadableValue math.LegacyDec) math.LegacyDec
	QuantityToChainFormat(humanReadableValue math.LegacyDec) math.LegacyDec
	NotionalToChainFormat(humanReadableValue math.LegacyDec) math.LegacyDec
}

const (
	MainnetChainID = "injective-1"
	TestnetChainID = "injective-888"
	DevnetChainID  = "injective-777"
)

var (
	HumanReadableUpgradeBlockHeightPerChainID = map[string]int64{
		MainnetChainID: 127250000, // Human readable upgrade height taken from https://injhub.com/proposal/541
		TestnetChainID: 76832674,  // Human readable upgrade height taken from https://testnet.hub.injective.network/proposal/609
		DevnetChainID:  490,       // This value has to be changed when testing on devnet
	}

	_ ChainValuesConverter = PreHumanReadableUpgradeValuesConverter{}
	_ ChainValuesConverter = PostHumanReadableUpgradeValuesConverter{}
)

func NewChainValuesConverter(ctx sdk.Context, market v2.MarketI) ChainValuesConverter {
	var converter ChainValuesConverter

	if isPreHumanReadableUpgradeBlock(ctx) {
		converter = PreHumanReadableUpgradeValuesConverter{market}
	} else {
		converter = PostHumanReadableUpgradeValuesConverter{market}
	}
	return converter
}

func ConditionalNotionalToChainFormat(ctx sdk.Context, value math.LegacyDec, decimals uint32) math.LegacyDec {
	var notional math.LegacyDec
	if isPreHumanReadableUpgradeBlock(ctx) {
		notional = value
	} else {
		notional = types.NotionalToChainFormat(value, decimals)
	}
	return notional
}

func isPreHumanReadableUpgradeBlock(ctx sdk.Context) bool {
	chainID := ctx.ChainID()
	humanReadableUpgradeBlockHeight, found := HumanReadableUpgradeBlockHeightPerChainID[chainID]
	if !found {
		humanReadableUpgradeBlockHeight = 0
	}
	return ctx.BlockHeight() < humanReadableUpgradeBlockHeight
}

type PreHumanReadableUpgradeValuesConverter struct {
	market v2.MarketI
}

func (PreHumanReadableUpgradeValuesConverter) PriceFromChainFormat(price math.LegacyDec) math.LegacyDec {
	// In blocks before the v1.16.0 upgrade, prices were stored in chain format already by the keeper
	return price
}

func (PreHumanReadableUpgradeValuesConverter) QuantityFromChainFormat(quantity math.LegacyDec) math.LegacyDec {
	// In blocks before the v1.16.0 upgrade, quantities were stored in chain format already by the keeper
	return quantity
}

func (PreHumanReadableUpgradeValuesConverter) NotionalFromChainFormat(notional math.LegacyDec) math.LegacyDec {
	// In blocks before the v1.16.0 upgrade, notional values were stored in chain format already by the keeper
	return notional
}

func (PreHumanReadableUpgradeValuesConverter) PriceToChainFormat(humanReadableValue math.LegacyDec) math.LegacyDec {
	// In blocks before the v1.16.0 upgrade, prices were stored in chain format already by the keeper
	return humanReadableValue
}

func (PreHumanReadableUpgradeValuesConverter) QuantityToChainFormat(humanReadableValue math.LegacyDec) math.LegacyDec {
	// In blocks before the v1.16.0 upgrade, quantities were stored in chain format already by the keeper
	return humanReadableValue
}

func (PreHumanReadableUpgradeValuesConverter) NotionalToChainFormat(humanReadableValue math.LegacyDec) math.LegacyDec {
	// In blocks before the v1.16.0 upgrade, notional values were stored in chain format already by the keeper
	return humanReadableValue
}

type PostHumanReadableUpgradeValuesConverter struct {
	market v2.MarketI
}

func (c PostHumanReadableUpgradeValuesConverter) PriceFromChainFormat(price math.LegacyDec) math.LegacyDec {
	return c.market.PriceFromChainFormat(price)
}

func (c PostHumanReadableUpgradeValuesConverter) QuantityFromChainFormat(quantity math.LegacyDec) math.LegacyDec {
	return c.market.QuantityFromChainFormat(quantity)
}

func (c PostHumanReadableUpgradeValuesConverter) NotionalFromChainFormat(notional math.LegacyDec) math.LegacyDec {
	return c.market.NotionalFromChainFormat(notional)
}

func (c PostHumanReadableUpgradeValuesConverter) PriceToChainFormat(humanReadableValue math.LegacyDec) math.LegacyDec {
	return c.market.PriceToChainFormat(humanReadableValue)
}

func (c PostHumanReadableUpgradeValuesConverter) QuantityToChainFormat(humanReadableValue math.LegacyDec) math.LegacyDec {
	return c.market.QuantityToChainFormat(humanReadableValue)
}

func (c PostHumanReadableUpgradeValuesConverter) NotionalToChainFormat(humanReadableValue math.LegacyDec) math.LegacyDec {
	return c.market.NotionalToChainFormat(humanReadableValue)
}

func NewV1MarketVolumeFromV2(valuesConverter ChainValuesConverter, v2MarketVolume v2.MarketVolume) types.MarketVolume {
	return types.MarketVolume{
		MarketId: v2MarketVolume.MarketId,
		Volume:   NewV1VolumeRecordFromV2(valuesConverter, v2MarketVolume.Volume),
	}
}

func NewV1VolumeRecordFromV2(valuesConverter ChainValuesConverter, v2VolumeRecord v2.VolumeRecord) types.VolumeRecord {
	chainFormatMakerVolume := valuesConverter.NotionalToChainFormat(v2VolumeRecord.MakerVolume)
	chainFormatTakerVolume := valuesConverter.NotionalToChainFormat(v2VolumeRecord.TakerVolume)
	return types.VolumeRecord{
		MakerVolume: chainFormatMakerVolume,
		TakerVolume: chainFormatTakerVolume,
	}
}

func NewV1SpotMarketFromV2(valuesConverter ChainValuesConverter, spotMarket v2.SpotMarket) types.SpotMarket {
	chainFormattedMinPriceTickSize := valuesConverter.PriceToChainFormat(spotMarket.MinPriceTickSize)
	chainFormattedMinQuantityTickSize := valuesConverter.QuantityToChainFormat(spotMarket.MinQuantityTickSize)
	chainFormattedMinNotional := valuesConverter.NotionalToChainFormat(spotMarket.MinNotional)
	return types.SpotMarket{
		Ticker:              spotMarket.Ticker,
		BaseDenom:           spotMarket.BaseDenom,
		QuoteDenom:          spotMarket.QuoteDenom,
		MakerFeeRate:        spotMarket.MakerFeeRate,
		TakerFeeRate:        spotMarket.TakerFeeRate,
		RelayerFeeShareRate: spotMarket.RelayerFeeShareRate,
		MarketId:            spotMarket.MarketId,
		Status:              types.MarketStatus(spotMarket.Status),
		MinPriceTickSize:    chainFormattedMinPriceTickSize,
		MinQuantityTickSize: chainFormattedMinQuantityTickSize,
		MinNotional:         chainFormattedMinNotional,
		Admin:               spotMarket.Admin,
		AdminPermissions:    spotMarket.AdminPermissions,
		BaseDecimals:        spotMarket.BaseDecimals,
		QuoteDecimals:       spotMarket.QuoteDecimals,
	}
}

func NewV1DerivativeMarketOrderFromV2(valuesConverter ChainValuesConverter, order v2.DerivativeMarketOrder) types.DerivativeMarketOrder {
	v1OrderInfo := NewV1OrderInfoFromV2(valuesConverter, order.OrderInfo)
	v1Order := types.DerivativeMarketOrder{
		OrderInfo:  v1OrderInfo,
		OrderType:  types.OrderType(order.OrderType),
		Margin:     valuesConverter.NotionalToChainFormat(order.Margin),
		MarginHold: valuesConverter.NotionalToChainFormat(order.MarginHold),
		OrderHash:  order.OrderHash,
	}

	if order.TriggerPrice != nil {
		chainFormatTriggerPrice := valuesConverter.PriceToChainFormat(*order.TriggerPrice)
		v1Order.TriggerPrice = &chainFormatTriggerPrice
	}

	return v1Order
}

func NewV1DerivativeLimitOrderFromV2(valuesConverter ChainValuesConverter, order v2.DerivativeLimitOrder) types.DerivativeLimitOrder {
	v1OrderInfo := NewV1OrderInfoFromV2(valuesConverter, order.OrderInfo)
	v1Order := types.DerivativeLimitOrder{
		OrderInfo: v1OrderInfo,
		OrderType: types.OrderType(order.OrderType),
		Margin:    valuesConverter.NotionalToChainFormat(order.Margin),
		Fillable:  valuesConverter.QuantityToChainFormat(order.Fillable),
		OrderHash: order.OrderHash,
	}

	if order.TriggerPrice != nil {
		chainFormatTriggerPrice := valuesConverter.PriceToChainFormat(*order.TriggerPrice)
		v1Order.TriggerPrice = &chainFormatTriggerPrice
	}

	return v1Order
}

func NewV1ExpiryFuturesMarketInfoStateFromV2(
	valuesConverter ChainValuesConverter, marketInfoState v2.ExpiryFuturesMarketInfoState,
) types.ExpiryFuturesMarketInfoState {
	v1State := types.ExpiryFuturesMarketInfoState{
		MarketId: marketInfoState.MarketId,
	}

	if marketInfoState.MarketInfo != nil {
		v1MarketInfo := NewV1ExpiryFuturesMarketInfoFromV2(valuesConverter, *marketInfoState.MarketInfo)
		v1State.MarketInfo = &v1MarketInfo
	}

	return v1State
}

func NewV1ExpiryFuturesMarketInfoFromV2(
	valuesConverter ChainValuesConverter,
	marketInfo v2.ExpiryFuturesMarketInfo,
) types.ExpiryFuturesMarketInfo {
	v1MarketInfo := types.ExpiryFuturesMarketInfo{
		MarketId:            marketInfo.MarketId,
		ExpirationTimestamp: marketInfo.ExpirationTimestamp,
		TwapStartTimestamp:  marketInfo.TwapStartTimestamp,
	}

	if !marketInfo.ExpirationTwapStartPriceCumulative.IsNil() {
		v1MarketInfo.ExpirationTwapStartPriceCumulative = valuesConverter.PriceToChainFormat(marketInfo.ExpirationTwapStartPriceCumulative)
	}

	if !marketInfo.SettlementPrice.IsNil() {
		v1MarketInfo.SettlementPrice = valuesConverter.PriceToChainFormat(marketInfo.SettlementPrice)
	}

	return v1MarketInfo
}

func NewV1DerivativePositonFromV2(valuesConverter ChainValuesConverter, position v2.DerivativePosition) types.DerivativePosition {
	v1DerivativePosition := types.DerivativePosition{
		SubaccountId: position.SubaccountId,
		MarketId:     position.MarketId,
	}

	if position.Position != nil {
		v1Position := NewV1PositionFromV2(valuesConverter, *position.Position)
		v1DerivativePosition.Position = &v1Position
	}

	return v1DerivativePosition
}

func NewV1PositionFromV2(valuesConverter ChainValuesConverter, position v2.Position) types.Position {
	return types.Position{
		IsLong:                 position.IsLong,
		Quantity:               valuesConverter.QuantityToChainFormat(position.Quantity),
		EntryPrice:             valuesConverter.PriceToChainFormat(position.EntryPrice),
		Margin:                 valuesConverter.NotionalToChainFormat(position.Margin),
		CumulativeFundingEntry: valuesConverter.NotionalToChainFormat(position.CumulativeFundingEntry),
	}
}

func NewV1DerivativeMarketFromV2(valuesConverter ChainValuesConverter, derivativeMarket v2.DerivativeMarket) types.DerivativeMarket {
	return types.DerivativeMarket{
		Ticker:                 derivativeMarket.Ticker,
		OracleBase:             derivativeMarket.OracleBase,
		OracleQuote:            derivativeMarket.OracleQuote,
		OracleType:             derivativeMarket.OracleType,
		OracleScaleFactor:      derivativeMarket.OracleScaleFactor + derivativeMarket.QuoteDecimals,
		QuoteDenom:             derivativeMarket.QuoteDenom,
		MarketId:               derivativeMarket.MarketId,
		InitialMarginRatio:     derivativeMarket.InitialMarginRatio,
		MaintenanceMarginRatio: derivativeMarket.MaintenanceMarginRatio,
		MakerFeeRate:           derivativeMarket.MakerFeeRate,
		TakerFeeRate:           derivativeMarket.TakerFeeRate,
		RelayerFeeShareRate:    derivativeMarket.RelayerFeeShareRate,
		IsPerpetual:            derivativeMarket.IsPerpetual,
		Status:                 types.MarketStatus(derivativeMarket.Status),
		MinPriceTickSize:       valuesConverter.PriceToChainFormat(derivativeMarket.MinPriceTickSize),
		MinQuantityTickSize:    valuesConverter.QuantityToChainFormat(derivativeMarket.MinQuantityTickSize),
		MinNotional:            valuesConverter.NotionalToChainFormat(derivativeMarket.MinNotional),
		Admin:                  derivativeMarket.Admin,
		AdminPermissions:       derivativeMarket.AdminPermissions,
		QuoteDecimals:          derivativeMarket.QuoteDecimals,
		ReduceMarginRatio:      derivativeMarket.ReduceMarginRatio,
		OpenNotionalCap:        convertOpenNotionalCapV2ToV1(derivativeMarket.OpenNotionalCap),
	}
}

func NewV1PerpetualMarketFundingStateFromV2(
	valuesConverter ChainValuesConverter, fundingState v2.PerpetualMarketFundingState,
) types.PerpetualMarketFundingState {
	v1State := types.PerpetualMarketFundingState{
		MarketId: fundingState.MarketId,
	}

	if fundingState.Funding != nil {
		v1Funding := NewV1PerpetualMarketFundingFromV2(valuesConverter, *fundingState.Funding)
		v1State.Funding = &v1Funding
	}

	return v1State
}

func NewV1PerpetualMarketFundingFromV2(
	valuesConverter ChainValuesConverter,
	funding v2.PerpetualMarketFunding,
) types.PerpetualMarketFunding {
	return types.PerpetualMarketFunding{
		CumulativeFunding: valuesConverter.NotionalToChainFormat(funding.CumulativeFunding),
		// cumulative_price defines the running time-integral of the perp premium
		// ((VWAP - mark_price) / mark_price) i.e., sum(premium * seconds)
		// used to compute the interval’s average premium for funding
		// NO CONVERSION REQUIRED
		CumulativePrice: funding.CumulativePrice,
		LastTimestamp:   funding.LastTimestamp,
	}
}

func NewV1DerivativeMarketSettlementInfoFromV2(
	valuesConverter ChainValuesConverter, settlementInfo v2.DerivativeMarketSettlementInfo,
) types.DerivativeMarketSettlementInfo {
	return types.DerivativeMarketSettlementInfo{
		MarketId:        settlementInfo.MarketId,
		SettlementPrice: valuesConverter.PriceToChainFormat(settlementInfo.SettlementPrice),
	}
}

func NewV1SpotLimitOrderFromV2(valuesConverter ChainValuesConverter, order v2.SpotLimitOrder) types.SpotLimitOrder {
	v1OrderInfo := NewV1OrderInfoFromV2(valuesConverter, order.OrderInfo)
	v1Order := types.SpotLimitOrder{
		OrderInfo: v1OrderInfo,
		OrderType: types.OrderType(order.OrderType),
		Fillable:  valuesConverter.QuantityToChainFormat(order.Fillable),
		OrderHash: order.OrderHash,
	}

	if order.TriggerPrice != nil {
		chainFormatTriggerPrice := valuesConverter.PriceToChainFormat(*order.TriggerPrice)
		v1Order.TriggerPrice = &chainFormatTriggerPrice
	}

	return v1Order
}

func NewV2SpotOrderFromV1(market v2.MarketI, order types.SpotOrder) *v2.SpotOrder {
	v2OrderInfo := NewV2OrderInfoFromV1(market, order.OrderInfo)
	v2Order := v2.SpotOrder{
		MarketId:  order.MarketId,
		OrderInfo: *v2OrderInfo,
		OrderType: v2.OrderType(order.OrderType),
	}

	if order.TriggerPrice != nil && !order.TriggerPrice.IsNil() {
		humanPrice := market.PriceFromChainFormat(*order.TriggerPrice)
		v2Order.TriggerPrice = &humanPrice
	}

	return &v2Order
}

func NewV1FullSpotMarketFromV2(valuesConverter ChainValuesConverter, fullSpotMarket v2.FullSpotMarket) types.FullSpotMarket {
	newFullSpotMarket := types.FullSpotMarket{}

	if fullSpotMarket.Market != nil {
		v1SpotMarket := NewV1SpotMarketFromV2(valuesConverter, *fullSpotMarket.Market)
		newFullSpotMarket.Market = &v1SpotMarket
	}

	if fullSpotMarket.MidPriceAndTob != nil {
		v1MidPriceAndTOB := NewV1MidPriceAndTOBFromV2(valuesConverter, *fullSpotMarket.MidPriceAndTob)
		newFullSpotMarket.MidPriceAndTob = &v1MidPriceAndTOB
	}

	return newFullSpotMarket
}

func NewV1FullDerivativeMarketFromV2(
	valuesConverter ChainValuesConverter,
	fullDerivativeMarket v2.FullDerivativeMarket,
) types.FullDerivativeMarket {
	v1FullMarket := types.FullDerivativeMarket{}

	switch info := fullDerivativeMarket.Info.(type) {
	case *v2.FullDerivativeMarket_FuturesInfo:
		v1FuturesInfo := NewV1FuturesInfoFromV2(valuesConverter, *info)
		v1FullMarket.Info = &v1FuturesInfo
	case *v2.FullDerivativeMarket_PerpetualInfo:
		v1PerpetualInfo := NewV1PerpetualInfoFromV2(valuesConverter, *info)
		v1FullMarket.Info = &v1PerpetualInfo
	}

	if fullDerivativeMarket.Market != nil {
		v1DerivativeMarket := NewV1DerivativeMarketFromV2(valuesConverter, *fullDerivativeMarket.Market)
		v1FullMarket.Market = &v1DerivativeMarket

		v1FullMarket.MarkPrice = valuesConverter.PriceToChainFormat(fullDerivativeMarket.MarkPrice)
	}

	if fullDerivativeMarket.MidPriceAndTob != nil {
		v1MidPriceAndTOB := NewV1MidPriceAndTOBFromV2(valuesConverter, *fullDerivativeMarket.MidPriceAndTob)
		v1FullMarket.MidPriceAndTob = &v1MidPriceAndTOB
	}

	return v1FullMarket
}

func NewV1MidPriceAndTOBFromV2(valuesConverter ChainValuesConverter, midPriceAndTOB v2.MidPriceAndTOB) types.MidPriceAndTOB {
	var v1MidPrice, v1BestBuyPrice, v1BestSellPrice *math.LegacyDec
	if midPriceAndTOB.MidPrice != nil {
		chainFormatMidPrice := valuesConverter.PriceToChainFormat(*midPriceAndTOB.MidPrice)
		v1MidPrice = &chainFormatMidPrice
	}
	if midPriceAndTOB.BestBuyPrice != nil {
		chainFormatBestBuyPrice := valuesConverter.PriceToChainFormat(*midPriceAndTOB.BestBuyPrice)
		v1BestBuyPrice = &chainFormatBestBuyPrice
	}
	if midPriceAndTOB.BestSellPrice != nil {
		chainFormatBestSellPrice := valuesConverter.PriceToChainFormat(*midPriceAndTOB.BestSellPrice)
		v1BestSellPrice = &chainFormatBestSellPrice
	}
	return types.MidPriceAndTOB{
		MidPrice:      v1MidPrice,
		BestBuyPrice:  v1BestBuyPrice,
		BestSellPrice: v1BestSellPrice,
	}
}

func NewV1FuturesInfoFromV2(
	valuesConverter ChainValuesConverter,
	info v2.FullDerivativeMarket_FuturesInfo,
) types.FullDerivativeMarket_FuturesInfo {
	v1FullFuturesInfo := types.FullDerivativeMarket_FuturesInfo{}

	if info.FuturesInfo != nil {
		v1FuturesInfo := NewV1ExpiryFuturesMarketInfoFromV2(valuesConverter, *info.FuturesInfo)
		v1FullFuturesInfo.FuturesInfo = &v1FuturesInfo
	}
	return v1FullFuturesInfo
}

func NewV1PerpetualInfoFromV2(
	valuesConverter ChainValuesConverter, perpetualInfo v2.FullDerivativeMarket_PerpetualInfo,
) types.FullDerivativeMarket_PerpetualInfo {
	v1PerpetualInfo := types.FullDerivativeMarket_PerpetualInfo{}

	if perpetualInfo.PerpetualInfo != nil {
		v1PerpetualMarketState := types.PerpetualMarketState{}
		if perpetualInfo.PerpetualInfo.MarketInfo != nil {
			v1PerpetualMarketInfo := NewV1PerpetualMarketInfoFromV2(*perpetualInfo.PerpetualInfo.MarketInfo)
			v1PerpetualMarketState.MarketInfo = &v1PerpetualMarketInfo
		}
		if perpetualInfo.PerpetualInfo.FundingInfo != nil {
			v1FundingInfo := NewV1PerpetualMarketFundingFromV2(valuesConverter, *perpetualInfo.PerpetualInfo.FundingInfo)
			v1PerpetualMarketState.FundingInfo = &v1FundingInfo
		}
		v1PerpetualInfo.PerpetualInfo = &v1PerpetualMarketState
	}

	return v1PerpetualInfo
}

func NewV1PerpetualMarketInfoFromV2(perpetualMarketInfo v2.PerpetualMarketInfo) types.PerpetualMarketInfo {
	return types.PerpetualMarketInfo{
		MarketId:             perpetualMarketInfo.MarketId,
		HourlyFundingRateCap: perpetualMarketInfo.HourlyFundingRateCap,
		HourlyInterestRate:   perpetualMarketInfo.HourlyInterestRate,
		NextFundingTimestamp: perpetualMarketInfo.NextFundingTimestamp,
		FundingInterval:      perpetualMarketInfo.FundingInterval,
	}
}

func NewV1TrimmedDerivativeLimitOrderFromV2(
	valuesConverter ChainValuesConverter, trimmedOrder v2.TrimmedDerivativeLimitOrder,
) types.TrimmedDerivativeLimitOrder {
	chainFormatPrice := valuesConverter.PriceToChainFormat(trimmedOrder.Price)
	chainFormatQuantity := valuesConverter.QuantityToChainFormat(trimmedOrder.Quantity)
	chainFormatMargin := valuesConverter.NotionalToChainFormat(trimmedOrder.Margin)
	chainFormatFillable := valuesConverter.QuantityToChainFormat(trimmedOrder.Fillable)
	return types.TrimmedDerivativeLimitOrder{
		Price:     chainFormatPrice,
		Quantity:  chainFormatQuantity,
		Margin:    chainFormatMargin,
		Fillable:  chainFormatFillable,
		IsBuy:     trimmedOrder.IsBuy,
		OrderHash: trimmedOrder.OrderHash,
		Cid:       trimmedOrder.Cid,
	}
}

func NewV1TrimmedSpotLimitOrderFromV2(
	valuesConverter ChainValuesConverter,
	trimmedOrder *v2.TrimmedSpotLimitOrder,
) *types.TrimmedSpotLimitOrder {
	return &types.TrimmedSpotLimitOrder{
		Price:     valuesConverter.PriceToChainFormat(trimmedOrder.Price),
		Quantity:  valuesConverter.QuantityToChainFormat(trimmedOrder.Quantity),
		Fillable:  valuesConverter.QuantityToChainFormat(trimmedOrder.Fillable),
		IsBuy:     trimmedOrder.IsBuy,
		OrderHash: trimmedOrder.OrderHash,
		Cid:       trimmedOrder.Cid,
	}
}

func NewV1BinaryOptionsMarketFromV2(valuesConverter ChainValuesConverter, market v2.BinaryOptionsMarket) types.BinaryOptionsMarket {
	v1Market := types.BinaryOptionsMarket{
		Ticker:              market.Ticker,
		OracleSymbol:        market.OracleSymbol,
		OracleProvider:      market.OracleProvider,
		OracleType:          market.OracleType,
		OracleScaleFactor:   market.OracleScaleFactor + market.QuoteDecimals,
		ExpirationTimestamp: market.ExpirationTimestamp,
		SettlementTimestamp: market.SettlementTimestamp,
		Admin:               market.Admin,
		QuoteDenom:          market.QuoteDenom,
		MarketId:            market.MarketId,
		MakerFeeRate:        market.MakerFeeRate,
		TakerFeeRate:        market.TakerFeeRate,
		RelayerFeeShareRate: market.RelayerFeeShareRate,
		Status:              types.MarketStatus(market.Status),
		MinPriceTickSize:    valuesConverter.PriceToChainFormat(market.MinPriceTickSize),
		MinQuantityTickSize: valuesConverter.QuantityToChainFormat(market.MinQuantityTickSize),
		MinNotional:         valuesConverter.NotionalToChainFormat(market.MinNotional),
		AdminPermissions:    market.AdminPermissions,
		QuoteDecimals:       market.QuoteDecimals,
	}

	if market.SettlementPrice != nil {
		chainFormatSettlementPrice := valuesConverter.PriceToChainFormat(*market.SettlementPrice)
		v1Market.SettlementPrice = &chainFormatSettlementPrice
	}

	return v1Market
}

func NewV1ExchangeParamsFromV2(params v2.Params) types.Params {
	return types.Params{
		SpotMarketInstantListingFee:                  params.SpotMarketInstantListingFee,
		DerivativeMarketInstantListingFee:            params.DerivativeMarketInstantListingFee,
		DefaultSpotMakerFeeRate:                      params.DefaultSpotMakerFeeRate,
		DefaultSpotTakerFeeRate:                      params.DefaultSpotTakerFeeRate,
		DefaultDerivativeMakerFeeRate:                params.DefaultDerivativeMakerFeeRate,
		DefaultDerivativeTakerFeeRate:                params.DefaultDerivativeTakerFeeRate,
		DefaultInitialMarginRatio:                    params.DefaultInitialMarginRatio,
		DefaultMaintenanceMarginRatio:                params.DefaultMaintenanceMarginRatio,
		DefaultFundingInterval:                       params.DefaultFundingInterval,
		FundingMultiple:                              params.FundingMultiple,
		RelayerFeeShareRate:                          params.RelayerFeeShareRate,
		DefaultHourlyFundingRateCap:                  params.DefaultHourlyFundingRateCap,
		DefaultHourlyInterestRate:                    params.DefaultHourlyInterestRate,
		MaxDerivativeOrderSideCount:                  params.MaxDerivativeOrderSideCount,
		InjRewardStakedRequirementThreshold:          params.InjRewardStakedRequirementThreshold,
		TradingRewardsVestingDuration:                params.TradingRewardsVestingDuration,
		LiquidatorRewardShareRate:                    params.LiquidatorRewardShareRate,
		BinaryOptionsMarketInstantListingFee:         params.BinaryOptionsMarketInstantListingFee,
		AtomicMarketOrderAccessLevel:                 types.AtomicMarketOrderAccessLevel(params.AtomicMarketOrderAccessLevel),
		SpotAtomicMarketOrderFeeMultiplier:           params.SpotAtomicMarketOrderFeeMultiplier,
		DerivativeAtomicMarketOrderFeeMultiplier:     params.DerivativeAtomicMarketOrderFeeMultiplier,
		BinaryOptionsAtomicMarketOrderFeeMultiplier:  params.BinaryOptionsAtomicMarketOrderFeeMultiplier,
		MinimalProtocolFeeRate:                       params.MinimalProtocolFeeRate,
		IsInstantDerivativeMarketLaunchEnabled:       params.IsInstantDerivativeMarketLaunchEnabled,
		PostOnlyModeHeightThreshold:                  params.PostOnlyModeHeightThreshold,
		MarginDecreasePriceTimestampThresholdSeconds: params.MarginDecreasePriceTimestampThresholdSeconds,
		ExchangeAdmins:                               params.ExchangeAdmins,
		InjAuctionMaxCap:                             params.InjAuctionMaxCap,
		FixedGasEnabled:                              params.FixedGasEnabled,
	}
}

func NewV1OrderInfoFromV2(valuesConverter ChainValuesConverter, orderInfo v2.OrderInfo) types.OrderInfo {
	return types.OrderInfo{
		SubaccountId: orderInfo.SubaccountId,
		FeeRecipient: orderInfo.FeeRecipient,
		Price:        valuesConverter.PriceToChainFormat(orderInfo.Price),
		Quantity:     valuesConverter.QuantityToChainFormat(orderInfo.Quantity),
		Cid:          orderInfo.Cid,
	}
}

func NewV1SubaccountOrderDataFromV2(valuesConverter ChainValuesConverter, orderData *v2.SubaccountOrderData) *types.SubaccountOrderData {
	v1OrderData := types.SubaccountOrderData{
		OrderHash: orderData.OrderHash,
	}

	if orderData.Order != nil {
		v1Order := &types.SubaccountOrder{
			Price:        valuesConverter.PriceToChainFormat(orderData.Order.Price),
			Quantity:     valuesConverter.QuantityToChainFormat(orderData.Order.Quantity),
			IsReduceOnly: orderData.Order.IsReduceOnly,
			Cid:          orderData.Order.Cid,
		}
		v1OrderData.Order = v1Order
	}
	return &v1OrderData
}

func NewV1LevelFromV2(valuesConverter ChainValuesConverter, level *v2.Level) *types.Level {
	return &types.Level{
		P: valuesConverter.PriceToChainFormat(level.P),
		Q: valuesConverter.QuantityToChainFormat(level.Q),
	}
}

func NewV1TradeRecordsFromV2(valuesConverter ChainValuesConverter, tradeRecords v2.TradeRecords) types.TradeRecords {
	v1TradeRecords := types.TradeRecords{
		MarketId:           tradeRecords.MarketId,
		LatestTradeRecords: make([]*types.TradeRecord, 0, len(tradeRecords.LatestTradeRecords)),
	}

	for _, tradeRecord := range tradeRecords.LatestTradeRecords {
		v1TradeRecord := NewV1TradeRecordFromV2(valuesConverter, *tradeRecord)
		v1TradeRecords.LatestTradeRecords = append(v1TradeRecords.LatestTradeRecords, &v1TradeRecord)
	}

	return v1TradeRecords
}

func NewV1TradeRecordFromV2(valuesConverter ChainValuesConverter, record v2.TradeRecord) types.TradeRecord {
	v1TradeRecord := types.TradeRecord{
		Timestamp: record.Timestamp,
		Price:     valuesConverter.PriceToChainFormat(record.Price),
		Quantity:  valuesConverter.QuantityToChainFormat(record.Quantity),
	}

	return v1TradeRecord
}

func NewV2OrderInfoFromV1(market v2.MarketI, orderInfo types.OrderInfo) *v2.OrderInfo {
	humanPrice := market.PriceFromChainFormat(orderInfo.Price)
	humanQuantity := market.QuantityFromChainFormat(orderInfo.Quantity)

	return &v2.OrderInfo{
		SubaccountId: orderInfo.SubaccountId,
		FeeRecipient: orderInfo.FeeRecipient,
		Price:        humanPrice,
		Quantity:     humanQuantity,
		Cid:          orderInfo.Cid,
	}
}

func NewV2DerivativeOrderFromV1(market v2.MarketI, order types.DerivativeOrder) *v2.DerivativeOrder {
	humanMargin := market.NotionalFromChainFormat(order.Margin)
	v2OrderInfo := NewV2OrderInfoFromV1(market, order.OrderInfo)
	v2Order := v2.DerivativeOrder{
		MarketId:  order.MarketId,
		OrderInfo: *v2OrderInfo,
		OrderType: v2.OrderType(order.OrderType),
		Margin:    humanMargin,
	}

	if order.TriggerPrice != nil && !order.TriggerPrice.IsNil() {
		humanPrice := market.PriceFromChainFormat(*order.TriggerPrice)
		v2Order.TriggerPrice = &humanPrice
	}

	return &v2Order
}

func CheckIfExceedDecimals(dec math.LegacyDec, maxDecimals uint32) bool {
	powered := dec.Mul(math.LegacyNewDec(10).Power(uint64(maxDecimals)))
	return !powered.Equal(powered.Ceil())
}

func ProposalAlreadyExists(prop v1.Proposal, marketID common.Hash, proposalTypes ...string) bool {
	msgs, err := tx.GetMsgs(prop.Messages, "proposal")
	if err != nil {
		return false
	}

	for _, msg := range msgs {
		legacyProposalExists := checkLegacyProposalExists(msg, marketID, proposalTypes...)
		if legacyProposalExists {
			return true
		}
	}

	return false
}

func checkLegacyProposalExists(msg sdk.Msg, marketID common.Hash, proposalTypes ...string) bool {
	if legacyMsg, ok := msg.(*v1.MsgExecLegacyContent); ok { // nolint:gocritic
		//	1. msg is legacy
		content, err := v1.LegacyContentFromMessage(legacyMsg)
		if err != nil {
			return false
		}
		isMatchingProposalType := slices.Contains(proposalTypes, content.ProposalType())
		if isMatchingProposalType {
			return checkProposalTypeAndMarketID(content, marketID)
		}
	}

	return false
}

//revive:disable:cyclomatic // the function is clear the way it is
//revive:disable:cognitive-complexity // the function is clear the way it is
func checkProposalTypeAndMarketID(content govtypes.Content, marketID common.Hash) bool {
	switch content.ProposalType() {
	case types.ProposalTypeExpiryFuturesMarketLaunch:
		p, ok := content.(*types.ExpiryFuturesMarketLaunchProposal)
		return ok && marketID == types.NewExpiryFuturesMarketID(
			p.Ticker, p.QuoteDenom, p.OracleBase, p.OracleQuote, p.OracleType, p.Expiry,
		)
	case types.ProposalTypePerpetualMarketLaunch:
		p, ok := content.(*types.PerpetualMarketLaunchProposal)
		return ok && marketID == types.NewPerpetualMarketID(p.Ticker, p.QuoteDenom, p.OracleBase, p.OracleQuote, p.OracleType)
	case types.ProposalTypeBinaryOptionsMarketLaunch:
		p, ok := content.(*types.BinaryOptionsMarketLaunchProposal)
		return ok && marketID == types.NewBinaryOptionsMarketID(
			p.Ticker, p.QuoteDenom, p.OracleSymbol, p.OracleProvider, p.OracleType,
		)
	case types.ProposalTypeSpotMarketLaunch:
		p, ok := content.(*types.SpotMarketLaunchProposal)
		return ok && marketID == types.NewSpotMarketID(p.BaseDenom, p.QuoteDenom)
	case v2.ProposalTypeExpiryFuturesMarketLaunch:
		p, ok := content.(*v2.ExpiryFuturesMarketLaunchProposal)
		return ok && marketID == types.NewExpiryFuturesMarketID(
			p.Ticker, p.QuoteDenom, p.OracleBase, p.OracleQuote, p.OracleType, p.Expiry,
		)
	case v2.ProposalTypePerpetualMarketLaunch:
		p, ok := content.(*v2.PerpetualMarketLaunchProposal)
		return ok && marketID == types.NewPerpetualMarketID(p.Ticker, p.QuoteDenom, p.OracleBase, p.OracleQuote, p.OracleType)
	case v2.ProposalTypeBinaryOptionsMarketLaunch:
		p, ok := content.(*v2.BinaryOptionsMarketLaunchProposal)
		return ok && marketID == types.NewBinaryOptionsMarketID(
			p.Ticker, p.QuoteDenom, p.OracleSymbol, p.OracleProvider, p.OracleType,
		)
	case v2.ProposalTypeSpotMarketLaunch:
		p, ok := content.(*v2.SpotMarketLaunchProposal)
		return ok && marketID == types.NewSpotMarketID(p.BaseDenom, p.QuoteDenom)
	}
	return false
}

func NewV1DerivativePositionFromV2(valuesConverter ChainValuesConverter, position v2.DerivativePosition) types.DerivativePosition {
	v1DerivativePosition := types.DerivativePosition{
		SubaccountId: position.SubaccountId,
		MarketId:     position.MarketId,
	}

	if position.Position != nil {
		v1Position := NewV1PositionFromV2(valuesConverter, *position.Position)
		v1DerivativePosition.Position = &v1Position
	}

	return v1DerivativePosition
}

func convertOpenNotionalCapV2ToV1(openNotionalCap v2.OpenNotionalCap) types.OpenNotionalCap {
	switch {
	case openNotionalCap.GetCapped() != nil:
		return types.OpenNotionalCap{
			Cap: &types.OpenNotionalCap_Capped{
				Capped: &types.OpenNotionalCapCapped{
					Value: openNotionalCap.GetCapped().Value,
				},
			},
		}

	case openNotionalCap.GetUncapped() != nil:
		return types.OpenNotionalCap{
			Cap: &types.OpenNotionalCap_Uncapped{
				Uncapped: &types.OpenNotionalCapUncapped{},
			},
		}

	default: // should not happen
		return types.OpenNotionalCap{
			Cap: &types.OpenNotionalCap_Uncapped{
				Uncapped: &types.OpenNotionalCapUncapped{},
			},
		}
	}
}

func NewV1TradingRewardCampaignInfoFromV2(campaignInfo *v2.TradingRewardCampaignInfo) *types.TradingRewardCampaignInfo {
	v1CampaignInfo := &types.TradingRewardCampaignInfo{
		CampaignDurationSeconds: campaignInfo.CampaignDurationSeconds,
		QuoteDenoms:             campaignInfo.QuoteDenoms,
		DisqualifiedMarketIds:   campaignInfo.DisqualifiedMarketIds,
	}

	if campaignInfo.TradingRewardBoostInfo != nil {
		v1TradingRewardBoostInfo := &types.TradingRewardCampaignBoostInfo{
			BoostedSpotMarketIds: campaignInfo.TradingRewardBoostInfo.BoostedSpotMarketIds,
			SpotMarketMultipliers: make(
				[]types.PointsMultiplier,
				0,
				len(campaignInfo.TradingRewardBoostInfo.SpotMarketMultipliers),
			),
			BoostedDerivativeMarketIds: campaignInfo.TradingRewardBoostInfo.BoostedDerivativeMarketIds,
			DerivativeMarketMultipliers: make(
				[]types.PointsMultiplier,
				0,
				len(campaignInfo.TradingRewardBoostInfo.DerivativeMarketMultipliers),
			),
		}
		for _, multiplier := range campaignInfo.TradingRewardBoostInfo.SpotMarketMultipliers {
			v1TradingRewardBoostInfo.SpotMarketMultipliers = append(
				v1TradingRewardBoostInfo.SpotMarketMultipliers,
				types.PointsMultiplier{
					MakerPointsMultiplier: multiplier.MakerPointsMultiplier,
					TakerPointsMultiplier: multiplier.TakerPointsMultiplier,
				},
			)
		}

		for _, multiplier := range campaignInfo.TradingRewardBoostInfo.DerivativeMarketMultipliers {
			v1TradingRewardBoostInfo.DerivativeMarketMultipliers = append(
				v1TradingRewardBoostInfo.DerivativeMarketMultipliers,
				types.PointsMultiplier{
					MakerPointsMultiplier: multiplier.MakerPointsMultiplier,
					TakerPointsMultiplier: multiplier.TakerPointsMultiplier,
				},
			)
		}
		v1CampaignInfo.TradingRewardBoostInfo = v1TradingRewardBoostInfo
	}

	return v1CampaignInfo
}
