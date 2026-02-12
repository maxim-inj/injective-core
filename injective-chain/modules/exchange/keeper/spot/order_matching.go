package spot

import (
	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/events"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k SpotKeeper) ExecuteAtomicSpotMarketOrder(
	ctx sdk.Context,
	market *v2.SpotMarket,
	marketOrder *v2.SpotMarketOrder,
	feeRate math.LegacyDec,
) *v2.SpotMarketOrderResults {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := market.MarketID()

	stakingInfo, feeDiscountConfig := k.feeDiscounts.GetFeeDiscountConfigAndStakingInfoForMarket(ctx, marketID)
	tradingRewards := types.NewTradingRewardPoints()
	spotVwapInfo := &v2.SpotVwapInfo{}
	tradeRewardsMultiplierConfig := k.GetEffectiveTradingRewardsMarketPointsMultiplierConfig(ctx, market.MarketID())

	isMarketBuy := marketOrder.IsBuy()

	spotLimitOrderStateExpansions, spotMarketOrderStateExpansions, clearingPrice, clearingQuantity :=
		k.getMarketOrderStateExpansionsAndClearingPrice(
			ctx, market, isMarketBuy, []*v2.SpotMarketOrder{marketOrder}, tradeRewardsMultiplierConfig, feeDiscountConfig, feeRate,
		)
	batchExecutionData := GetSpotMarketOrderBatchExecutionData(
		isMarketBuy, market, spotLimitOrderStateExpansions, spotMarketOrderStateExpansions, clearingPrice, clearingQuantity,
	)

	modifiedPositionCache := v2.NewModifiedPositionCache()

	tradingRewards = k.PersistSingleSpotMarketOrderExecution(ctx, marketID, batchExecutionData, *spotVwapInfo, tradingRewards)

	sortedSubaccountIDs := modifiedPositionCache.GetSortedSubaccountIDsByMarket(marketID)
	k.AppendModifiedSubaccountsByMarket(ctx, marketID, sortedSubaccountIDs)

	k.tradingRewards.PersistTradingRewardPoints(ctx, tradingRewards)
	k.feeDiscounts.PersistFeeDiscountStakingInfoUpdates(ctx, stakingInfo)
	k.tradingRewards.PersistVwapInfo(ctx, spotVwapInfo, nil)

	// a trade will always occur since there must exist at least one spot limit order that will cross
	marketOrderTrade := batchExecutionData.MarketOrderExecutionEvent.Trades[0]

	return &v2.SpotMarketOrderResults{
		Quantity: marketOrderTrade.Quantity,
		Price:    marketOrderTrade.Price,
		Fee:      marketOrderTrade.Fee,
	}
}

func GetSpotMarketOrderBatchExecutionData(
	isMarketBuy bool,
	market *v2.SpotMarket,
	spotLimitOrderStateExpansions, spotMarketOrderStateExpansions []*v2.SpotOrderStateExpansion,
	clearingPrice, clearingQuantity math.LegacyDec,
) *v2.SpotBatchExecutionData {
	baseDenomDepositDeltas := types.NewDepositDeltas()
	quoteDenomDepositDeltas := types.NewDepositDeltas()

	// Step 3a: Process market order events
	marketOrderBatchEvent := &v2.EventBatchSpotExecution{
		MarketId:      market.MarketID().Hex(),
		IsBuy:         isMarketBuy,
		ExecutionType: v2.ExecutionType_Market,
	}

	trades := make([]*v2.TradeLog, len(spotMarketOrderStateExpansions))

	marketOrderTradingRewardPoints := types.NewTradingRewardPoints()

	for idx := range spotMarketOrderStateExpansions {
		expansion := spotMarketOrderStateExpansions[idx]
		expansion.UpdateFromDepositDeltas(market, baseDenomDepositDeltas, quoteDenomDepositDeltas)

		realizedTradeFee := expansion.AuctionFeeReward

		isSelfRelayedTrade := expansion.FeeRecipient == types.SubaccountIDToEthAddress(expansion.SubaccountID)
		if !isSelfRelayedTrade {
			realizedTradeFee = realizedTradeFee.Add(expansion.FeeRecipientReward)
		}

		trades[idx] = &v2.TradeLog{
			Quantity:            expansion.BaseChangeAmount.Abs(),
			Price:               expansion.TradePrice,
			SubaccountId:        expansion.SubaccountID.Bytes(),
			Fee:                 realizedTradeFee,
			OrderHash:           expansion.OrderHash.Bytes(),
			FeeRecipientAddress: expansion.FeeRecipient.Bytes(),
			Cid:                 expansion.Cid,
		}
		marketOrderTradingRewardPoints.AddPointsForAddress(expansion.TraderAddress, expansion.TradingRewardPoints)
	}
	marketOrderBatchEvent.Trades = trades

	if len(trades) == 0 {
		marketOrderBatchEvent = nil
	}

	// Stage 3b: Process limit order events
	limitOrderBatchEvent, filledDeltas, limitOrderTradingRewardPoints := v2.GetBatchExecutionEventsFromSpotLimitOrderStateExpansions(
		!isMarketBuy,
		market,
		v2.ExecutionType_LimitFill,
		spotLimitOrderStateExpansions,
		baseDenomDepositDeltas, quoteDenomDepositDeltas,
	)

	limitOrderExecutionEvent := make([]*v2.EventBatchSpotExecution, 0)
	if limitOrderBatchEvent != nil {
		limitOrderExecutionEvent = append(limitOrderExecutionEvent, limitOrderBatchEvent)
	}

	vwapData := v2.NewSpotVwapData()
	vwapData = vwapData.ApplyExecution(clearingPrice, clearingQuantity)

	tradingRewardPoints := types.MergeTradingRewardPoints(marketOrderTradingRewardPoints, limitOrderTradingRewardPoints)

	// Final Step: Store the SpotBatchExecutionData for future reduction/processing
	batch := &v2.SpotBatchExecutionData{
		Market:                         market,
		BaseDenomDepositDeltas:         baseDenomDepositDeltas,
		QuoteDenomDepositDeltas:        quoteDenomDepositDeltas,
		BaseDenomDepositSubaccountIDs:  baseDenomDepositDeltas.GetSortedSubaccountKeys(),
		QuoteDenomDepositSubaccountIDs: quoteDenomDepositDeltas.GetSortedSubaccountKeys(),
		LimitOrderFilledDeltas:         filledDeltas,
		MarketOrderExecutionEvent:      marketOrderBatchEvent,
		LimitOrderExecutionEvent:       limitOrderExecutionEvent,
		TradingRewardPoints:            tradingRewardPoints,
		VwapData:                       vwapData,
	}
	return batch
}

//nolint:revive // ok
func (k SpotKeeper) getMarketOrderStateExpansionsAndClearingPrice(
	ctx sdk.Context,
	market *v2.SpotMarket,
	isMarketBuy bool,
	marketOrders []*v2.SpotMarketOrder,
	pointsMultiplier v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
	takerFeeRate math.LegacyDec,
) (spotLimitOrderStateExpansions, spotMarketOrderStateExpansions []*v2.SpotOrderStateExpansion, clearingPrice, clearingQuantity math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	isLimitBuy := !isMarketBuy
	limitOrdersIterator := k.SpotLimitOrderbookIterator(ctx, market.MarketID(), isLimitBuy)
	limitOrderbook := NewSpotLimitOrderbook(k, limitOrdersIterator, nil, isLimitBuy)

	if limitOrderbook != nil {
		defer limitOrderbook.Close()
	} else {
		spotMarketOrderStateExpansions = k.processSpotMarketOrderStateExpansions(
			ctx,
			market.MarketID(),
			isMarketBuy,
			marketOrders,
			make([]math.LegacyDec, len(marketOrders)),
			math.LegacyDec{},
			takerFeeRate,
			market.RelayerFeeShareRate,
			pointsMultiplier,
			feeDiscountConfig,
		)

		return
	}

	marketOrderbook := NewSpotMarketOrderbook(marketOrders)

	// Determine matchable market orders and limit orders
	for {
		var buyOrder, sellOrder *v2.PriceLevel

		if isMarketBuy {
			buyOrder = marketOrderbook.Peek()
			sellOrder = limitOrderbook.Peek()
		} else {
			sellOrder = marketOrderbook.Peek()
			buyOrder = limitOrderbook.Peek()
		}

		// Base Case: Iterated over all the orders!
		if buyOrder == nil || sellOrder == nil {
			break
		}

		unitSpread := sellOrder.Price.Sub(buyOrder.Price)
		matchQuantityIncrement := math.LegacyMinDec(buyOrder.Quantity, sellOrder.Quantity)

		// Exit if no more matchable orders
		if unitSpread.IsPositive() || matchQuantityIncrement.IsZero() {
			break
		}

		if err := marketOrderbook.Fill(matchQuantityIncrement); err != nil {
			k.Logger(ctx).Error("Fill marketOrderbook failed during getMarketOrderStateExpansionsAndClearingPrice:", err)
		}
		if err := limitOrderbook.Fill(matchQuantityIncrement); err != nil {
			k.Logger(ctx).Error("Fill limitOrderbook failed during getMarketOrderStateExpansionsAndClearingPrice:", err)
		}
	}

	clearingQuantity = limitOrderbook.GetTotalQuantityFilled()

	if clearingQuantity.IsPositive() {
		// Clearing Price equals limit orderbook side average weighted price
		clearingPrice = limitOrderbook.GetNotional().Quo(clearingQuantity)
	}

	spotLimitOrderStateExpansions = k.processRestingSpotLimitOrderExpansions(
		ctx,
		market.MarketID(),
		limitOrderbook.GetRestingOrderbookFills(),
		!isMarketBuy,
		math.LegacyDec{},
		market.MakerFeeRate,
		market.RelayerFeeShareRate,
		pointsMultiplier,
		feeDiscountConfig,
	)

	spotMarketOrderStateExpansions = k.processSpotMarketOrderStateExpansions(
		ctx,
		market.MarketID(),
		isMarketBuy,
		marketOrders,
		marketOrderbook.GetOrderbookFillQuantities(),
		clearingPrice,
		takerFeeRate,
		market.RelayerFeeShareRate,
		pointsMultiplier,
		feeDiscountConfig,
	)

	return
}

// processSpotMarketOrderStateExpansions processes the spot market order state expansions.
// NOTE: clearingPrice may be Nil
//
//nolint:revive // ok
func (k SpotKeeper) processSpotMarketOrderStateExpansions(
	ctx sdk.Context,
	marketID common.Hash,
	isMarketBuy bool,
	marketOrders []*v2.SpotMarketOrder,
	marketFillQuantities []math.LegacyDec,
	clearingPrice math.LegacyDec,
	tradeFeeRate, relayerFeeShareRate math.LegacyDec,
	pointsMultiplier v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
) []*v2.SpotOrderStateExpansion {
	stateExpansions := make([]*v2.SpotOrderStateExpansion, len(marketOrders))

	for idx := range marketOrders {
		stateExpansions[idx] = k.getSpotMarketOrderStateExpansion(
			ctx,
			marketID,
			marketOrders[idx],
			isMarketBuy,
			marketFillQuantities[idx],
			clearingPrice,
			tradeFeeRate,
			relayerFeeShareRate,
			pointsMultiplier,
			feeDiscountConfig,
		)
	}
	return stateExpansions
}

//nolint:revive // ok
func (k SpotKeeper) getSpotMarketOrderStateExpansion(
	ctx sdk.Context,
	marketID common.Hash,
	order *v2.SpotMarketOrder,
	isMarketBuy bool,
	fillQuantity, clearingPrice math.LegacyDec,
	takerFeeRate, relayerFeeShareRate math.LegacyDec,
	pointsMultiplier v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
) *v2.SpotOrderStateExpansion {
	var baseChangeAmount, quoteChangeAmount math.LegacyDec

	if fillQuantity.IsNil() {
		fillQuantity = math.LegacyZeroDec()
	}
	orderNotional := math.LegacyZeroDec()
	if !clearingPrice.IsNil() {
		orderNotional = fillQuantity.Mul(clearingPrice)
	}

	isMaker := false

	feeData := k.tradingRewards.GetTradeDataAndIncrementVolumeContribution(
		ctx,
		order.SubaccountID(),
		marketID,
		fillQuantity,
		clearingPrice,
		takerFeeRate,
		relayerFeeShareRate,
		pointsMultiplier.TakerPointsMultiplier,
		feeDiscountConfig,
		isMaker,
	)

	baseRefundAmount, quoteRefundAmount, quoteChangeAmount := math.LegacyZeroDec(), math.LegacyZeroDec(), math.LegacyZeroDec()

	if isMarketBuy {
		// market buys are credited with the order fill quantity in base denom
		baseChangeAmount = fillQuantity
		// market buys are debited with (fillQuantity * clearingPrice) * (1 + takerFee) in quote denom
		if !clearingPrice.IsNil() {
			quoteChangeAmount = fillQuantity.Mul(clearingPrice).Add(feeData.TotalTradeFee).Neg()
		}
		quoteRefundAmount = order.BalanceHold.Add(quoteChangeAmount)
	} else {
		// market sells are debited by fillQuantity in base denom
		baseChangeAmount = fillQuantity.Neg()
		// market sells are credited with the (fillQuantity * clearingPrice) * (1 - TakerFee) in quote denom
		if !clearingPrice.IsNil() {
			quoteChangeAmount = orderNotional.Sub(feeData.TotalTradeFee)
		}
		// base denom refund unfilled market order quantity
		if fillQuantity.LT(order.OrderInfo.Quantity) {
			baseRefundAmount = order.OrderInfo.Quantity.Sub(fillQuantity)
		}
	}

	tradePrice := clearingPrice
	if tradePrice.IsNil() {
		tradePrice = math.LegacyZeroDec()
	}

	stateExpansion := v2.SpotOrderStateExpansion{
		BaseChangeAmount:        baseChangeAmount,
		BaseRefundAmount:        baseRefundAmount,
		QuoteChangeAmount:       quoteChangeAmount,
		QuoteRefundAmount:       quoteRefundAmount,
		TradePrice:              tradePrice,
		FeeRecipient:            order.FeeRecipient(),
		FeeRecipientReward:      feeData.FeeRecipientReward,
		AuctionFeeReward:        feeData.AuctionFeeReward,
		TraderFeeReward:         math.LegacyZeroDec(),
		TradingRewardPoints:     feeData.TradingRewardPoints,
		MarketOrder:             order,
		MarketOrderFillQuantity: fillQuantity,
		OrderHash:               common.BytesToHash(order.OrderHash),
		OrderPrice:              order.OrderInfo.Price,
		SubaccountID:            order.SubaccountID(),
		TraderAddress:           order.SdkAccAddress().String(),
		Cid:                     order.Cid(),
	}
	return &stateExpansion
}

//nolint:revive // ok
func (k SpotKeeper) processRestingSpotLimitOrderExpansions(
	ctx sdk.Context,
	marketID common.Hash,
	fills *v2.OrderbookFills,
	isLimitBuy bool,
	clearingPrice math.LegacyDec,
	makerFeeRate, relayerFeeShareRate math.LegacyDec,
	pointsMultiplier v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
) []*v2.SpotOrderStateExpansion {
	stateExpansions := make([]*v2.SpotOrderStateExpansion, len(fills.Orders))
	for idx, order := range fills.Orders {
		fillQuantity, fillPrice := fills.FillQuantities[idx], order.OrderInfo.Price
		if !clearingPrice.IsNil() {
			fillPrice = clearingPrice
		}

		if isLimitBuy {
			stateExpansions[idx] = k.getRestingSpotLimitBuyStateExpansion(
				ctx,
				marketID,
				order,
				order.Hash(),
				fillQuantity,
				fillPrice,
				makerFeeRate,
				relayerFeeShareRate,
				pointsMultiplier,
				feeDiscountConfig,
			)
		} else {
			stateExpansions[idx] = k.getSpotLimitSellStateExpansion(
				ctx,
				marketID,
				order,
				true,
				fillQuantity,
				fillPrice,
				makerFeeRate,
				relayerFeeShareRate,
				pointsMultiplier,
				feeDiscountConfig,
			)
		}
	}
	return stateExpansions
}

//nolint:revive // ok
func (k SpotKeeper) getRestingSpotLimitBuyStateExpansion(
	ctx sdk.Context,
	marketID common.Hash,
	order *v2.SpotLimitOrder,
	orderHash common.Hash,
	fillQuantity, fillPrice, makerFeeRate, relayerFeeShareRate math.LegacyDec,
	pointsMultiplier v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
) *v2.SpotOrderStateExpansion {
	var baseChangeAmount, quoteChangeAmount math.LegacyDec

	isMaker := true
	feeData := k.tradingRewards.GetTradeDataAndIncrementVolumeContribution(
		ctx,
		order.SubaccountID(),
		marketID,
		fillQuantity,
		fillPrice,
		makerFeeRate,
		relayerFeeShareRate,
		pointsMultiplier.MakerPointsMultiplier,
		feeDiscountConfig,
		isMaker,
	)

	orderNotional := fillQuantity.Mul(fillPrice)

	// limit buys are credited with the order fill quantity in base denom
	baseChangeAmount = fillQuantity
	quoteRefund := math.LegacyZeroDec()

	// limit buys are debited with (fillQuantity * Price) * (1 + makerFee) in quote denom
	if feeData.TotalTradeFee.IsNegative() {
		quoteChangeAmount = orderNotional.Neg().Add(feeData.TraderFee.Abs())
		quoteRefund = feeData.TraderFee.Abs()
	} else {
		quoteChangeAmount = orderNotional.Add(feeData.TotalTradeFee).Neg()
	}

	positiveDiscountedFeeRatePart := math.LegacyMaxDec(math.LegacyZeroDec(), feeData.DiscountedTradeFeeRate)

	if !fillPrice.Equal(order.OrderInfo.Price) {
		// nolint:all
		// priceDelta = price - fill price
		priceDelta := order.OrderInfo.Price.Sub(fillPrice)
		// nolint:all
		// clearingRefund = fillQuantity * priceDelta
		clearingRefund := fillQuantity.Mul(priceDelta)

		// nolint:all
		// matchedFeeRefund = max(discountedMakerFeeRate, 0) * fillQuantity * priceDelta
		matchedFeeRefund := positiveDiscountedFeeRatePart.Mul(fillQuantity.Mul(priceDelta))

		// nolint:all
		// quoteRefund += (1 + max(makerFeeRate, 0)) * fillQuantity * priceDelta
		quoteRefund = quoteRefund.Add(clearingRefund.Add(matchedFeeRefund))
	}

	if feeData.TotalTradeFee.IsPositive() {
		positiveMakerFeeRatePart := math.LegacyMaxDec(makerFeeRate, math.LegacyZeroDec())
		makerFeeRateDelta := positiveMakerFeeRatePart.Sub(feeData.DiscountedTradeFeeRate)
		matchedFeeDiscountRefund := fillQuantity.Mul(order.OrderInfo.Price).Mul(makerFeeRateDelta)
		quoteRefund = quoteRefund.Add(matchedFeeDiscountRefund)
	}

	order.Fillable = order.Fillable.Sub(fillQuantity)

	stateExpansion := v2.SpotOrderStateExpansion{
		BaseChangeAmount:       baseChangeAmount,
		BaseRefundAmount:       math.LegacyZeroDec(),
		QuoteChangeAmount:      quoteChangeAmount,
		QuoteRefundAmount:      quoteRefund,
		TradePrice:             fillPrice,
		FeeRecipient:           order.FeeRecipient(),
		FeeRecipientReward:     feeData.FeeRecipientReward,
		AuctionFeeReward:       feeData.AuctionFeeReward,
		TraderFeeReward:        feeData.TraderFee,
		TradingRewardPoints:    feeData.TradingRewardPoints,
		LimitOrder:             order,
		LimitOrderFillQuantity: fillQuantity,
		OrderPrice:             order.OrderInfo.Price,
		OrderHash:              orderHash,
		SubaccountID:           order.SubaccountID(),
		TraderAddress:          order.SdkAccAddress().String(),
		Cid:                    order.Cid(),
	}
	return &stateExpansion
}

//nolint:revive // ok
func (k SpotKeeper) getSpotLimitSellStateExpansion(
	ctx sdk.Context,
	marketID common.Hash,
	order *v2.SpotLimitOrder,
	isMaker bool,
	fillQuantity, fillPrice, tradeFeeRate, relayerFeeShareRate math.LegacyDec,
	pointsMultiplier v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
) *v2.SpotOrderStateExpansion {
	orderNotional := fillQuantity.Mul(fillPrice)

	var tradeRewardMultiplier math.LegacyDec
	if isMaker {
		tradeRewardMultiplier = pointsMultiplier.MakerPointsMultiplier
	} else {
		tradeRewardMultiplier = pointsMultiplier.TakerPointsMultiplier
	}
	feeData := k.tradingRewards.GetTradeDataAndIncrementVolumeContribution(
		ctx,
		order.SubaccountID(),
		marketID,
		fillQuantity,
		fillPrice,
		tradeFeeRate,
		relayerFeeShareRate,
		tradeRewardMultiplier,
		feeDiscountConfig,
		isMaker,
	)

	// limit sells are credited with the (fillQuantity * price) * traderFee in quote denom
	// traderFee can be positive or negative
	quoteChangeAmount := orderNotional.Sub(feeData.TraderFee)
	order.Fillable = order.Fillable.Sub(fillQuantity)

	stateExpansion := v2.SpotOrderStateExpansion{
		// limit sells are debited by fillQuantity in base denom
		BaseChangeAmount:       fillQuantity.Neg(),
		BaseRefundAmount:       math.LegacyZeroDec(),
		QuoteChangeAmount:      quoteChangeAmount,
		QuoteRefundAmount:      math.LegacyZeroDec(),
		TradePrice:             fillPrice,
		FeeRecipient:           order.FeeRecipient(),
		FeeRecipientReward:     feeData.FeeRecipientReward,
		AuctionFeeReward:       feeData.AuctionFeeReward,
		TraderFeeReward:        feeData.TraderFee,
		TradingRewardPoints:    feeData.TradingRewardPoints,
		LimitOrder:             order,
		LimitOrderFillQuantity: fillQuantity,
		OrderPrice:             order.OrderInfo.Price,
		OrderHash:              order.Hash(),
		SubaccountID:           order.SubaccountID(),
		TraderAddress:          order.SdkAccAddress().String(),
		Cid:                    order.Cid(),
	}
	return &stateExpansion
}

//nolint:revive // ok
func (k SpotKeeper) PersistSingleSpotMarketOrderExecution(
	ctx sdk.Context,
	marketID common.Hash,
	execution *v2.SpotBatchExecutionData,
	spotVwapData v2.SpotVwapInfo,
	tradingRewardPoints types.TradingRewardPoints,
) types.TradingRewardPoints {
	if execution == nil {
		return tradingRewardPoints
	}

	if execution.VwapData != nil && !execution.VwapData.Price.IsZero() && !execution.VwapData.Quantity.IsZero() {
		spotVwapData.ApplyVwap(marketID, execution.VwapData)
	}
	baseDenom, quoteDenom := execution.Market.BaseDenom, execution.Market.QuoteDenom

	for _, subaccountID := range execution.BaseDenomDepositSubaccountIDs {
		k.subaccount.UpdateDepositWithDelta(
			ctx,
			subaccountID,
			baseDenom,
			execution.BaseDenomDepositDeltas[subaccountID],
		)
	}
	for _, subaccountID := range execution.QuoteDenomDepositSubaccountIDs {
		k.subaccount.UpdateDepositWithDelta(
			ctx,
			subaccountID,
			quoteDenom,
			execution.QuoteDenomDepositDeltas[subaccountID],
		)
	}

	for _, limitOrderDelta := range execution.LimitOrderFilledDeltas {
		k.UpdateSpotLimitOrder(ctx, marketID, limitOrderDelta)
	}

	// only get first index since only one limit order side that gets filled
	if execution.MarketOrderExecutionEvent != nil {
		events.Emit(ctx, k.BaseKeeper, execution.MarketOrderExecutionEvent)
	}

	if len(execution.LimitOrderExecutionEvent) > 0 {
		events.Emit(ctx, k.BaseKeeper, execution.LimitOrderExecutionEvent[0])
	}

	if len(execution.TradingRewardPoints) > 0 {
		tradingRewardPoints = types.MergeTradingRewardPoints(tradingRewardPoints, execution.TradingRewardPoints)
	}

	return tradingRewardPoints
}

func (k SpotKeeper) ExecuteSpotMarketOrders(
	ctx sdk.Context,
	marketOrderIndicator *v2.MarketOrderIndicator,
	stakingInfo *v2.FeeDiscountStakingInfo,
) *v2.SpotBatchExecutionData {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var (
		marketID                     = common.HexToHash(marketOrderIndicator.MarketId)
		isMarketBuy                  = marketOrderIndicator.IsBuy
		market                       = k.GetSpotMarket(ctx, marketID, true)
		tradeRewardsMultiplierConfig = k.GetEffectiveTradingRewardsMarketPointsMultiplierConfig(ctx, market.MarketID())
		feeDiscountConfig            = k.feeDiscounts.GetFeeDiscountConfigForMarket(ctx, marketID, stakingInfo)
	)

	if market == nil {
		return nil
	}

	// Step 1: Obtain the clearing price, clearing quantity, spot limit & spot market state expansions
	marketOrders := k.GetAllTransientSpotMarketOrders(ctx, marketID, isMarketBuy)
	spotLimitOrderStateExpansions,
		spotMarketOrderStateExpansions,
		clearingPrice,
		clearingQuantity := k.getMarketOrderStateExpansionsAndClearingPrice(
		ctx,
		market,
		isMarketBuy,
		marketOrders,
		tradeRewardsMultiplierConfig,
		feeDiscountConfig,
		market.TakerFeeRate,
	)

	batchExecutionData := GetSpotMarketOrderBatchExecutionData(
		isMarketBuy,
		market,
		spotLimitOrderStateExpansions,
		spotMarketOrderStateExpansions,
		clearingPrice,
		clearingQuantity,
	)

	return batchExecutionData
}

func (k SpotKeeper) PersistSpotMarketOrderExecution(
	ctx sdk.Context,
	batchSpotExecutionData []*v2.SpotBatchExecutionData,
	spotVwapData v2.SpotVwapInfo,
) types.TradingRewardPoints {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	tradingRewardPoints := types.NewTradingRewardPoints()
	for batchIdx := range batchSpotExecutionData {
		execution := batchSpotExecutionData[batchIdx]
		marketID := execution.Market.MarketID()

		tradingRewardPoints = k.PersistSingleSpotMarketOrderExecution(ctx, marketID, execution, spotVwapData, tradingRewardPoints)
	}
	return tradingRewardPoints
}

func (k SpotKeeper) ExecuteSpotLimitOrderMatching(
	ctx sdk.Context,
	matchedMarketDirection *types.MatchedMarketDirection,
	stakingInfo *v2.FeeDiscountStakingInfo,
) *v2.SpotBatchExecutionData {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := matchedMarketDirection.MarketId
	market := k.GetSpotMarket(ctx, marketID, true)
	if market == nil {
		return nil
	}

	tradeRewardsMultiplierConfig := k.GetEffectiveTradingRewardsMarketPointsMultiplierConfig(ctx, market.MarketID())
	feeDiscountConfig := k.feeDiscounts.GetFeeDiscountConfigForMarket(ctx, marketID, stakingInfo)

	// Step 0: Obtain the new buy and sell limit orders from the transient store for convenience
	newBuyOrders := k.GetAllTransientSpotLimitOrdersByMarketDirection(ctx, marketID, true)
	newSellOrders := k.GetAllTransientSpotLimitOrdersByMarketDirection(ctx, marketID, false)

	// Step 1: Obtain the buy and sell orderbooks with updated fill quantities and the clearing price from matching
	matchingResults := k.getMatchedSpotLimitOrderClearingResults(ctx, marketID, newBuyOrders, newSellOrders)

	clearingPrice := matchingResults.ClearingPrice
	batchExecutionData := k.GetSpotLimitMatchingBatchExecutionData(
		ctx,
		market,
		matchingResults,
		clearingPrice,
		tradeRewardsMultiplierConfig,
		feeDiscountConfig,
	)

	return batchExecutionData
}

// getMatchedSpotLimitOrderClearingResults returns the SpotOrderbookMatchingResults.
//
//nolint:revive // ok
func (k SpotKeeper) getMatchedSpotLimitOrderClearingResults(
	ctx sdk.Context,
	marketID common.Hash,
	transientBuyOrders []*v2.SpotLimitOrder,
	transientSellOrders []*v2.SpotLimitOrder,
) *v2.SpotOrderbookMatchingResults {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	buyOrdersIterator := k.SpotLimitOrderbookIterator(ctx, marketID, true)
	sellOrdersIterator := k.SpotLimitOrderbookIterator(ctx, marketID, false)
	buyOrderbook := NewSpotLimitOrderbook(k, buyOrdersIterator, transientBuyOrders, true)
	sellOrderbook := NewSpotLimitOrderbook(k, sellOrdersIterator, transientSellOrders, false)

	if buyOrderbook != nil {
		defer buyOrderbook.Close()
	}

	if sellOrderbook != nil {
		defer sellOrderbook.Close()
	}

	orderbookResults := NewSpotOrderbookMatchingResults(transientBuyOrders, transientSellOrders)
	if buyOrderbook == nil || sellOrderbook == nil {
		return orderbookResults
	}

	var (
		lastBuyPrice  math.LegacyDec
		lastSellPrice math.LegacyDec
	)

	for {
		buyOrder := buyOrderbook.Peek()
		sellOrder := sellOrderbook.Peek()

		// Base Case: Finished iterating over all the orders
		if buyOrder == nil || sellOrder == nil {
			break
		}

		unitSpread := sellOrder.Price.Sub(buyOrder.Price)
		hasNoMatchableOrdersLeft := unitSpread.IsPositive()

		if hasNoMatchableOrdersLeft {
			break
		}

		lastBuyPrice = buyOrder.Price
		lastSellPrice = sellOrder.Price

		matchQuantityIncrement := math.LegacyMinDec(buyOrder.Quantity, sellOrder.Quantity)

		if err := buyOrderbook.Fill(matchQuantityIncrement); err != nil {
			k.Logger(ctx).Error("Fill buyOrderbook failed during getMatchedSpotLimitOrderClearingResults:", err)
		}
		if err := sellOrderbook.Fill(matchQuantityIncrement); err != nil {
			k.Logger(ctx).Error("Fill sellOrderbook failed during getMatchedSpotLimitOrderClearingResults:", err)
		}
	}

	var clearingPrice math.LegacyDec
	clearingQuantity := sellOrderbook.GetTotalQuantityFilled()

	if clearingQuantity.IsPositive() {
		midMarketPrice := k.GetSpotMidPriceOrBestPrice(ctx, marketID)
		switch {
		case midMarketPrice != nil && lastBuyPrice.LTE(*midMarketPrice):
			// default case when a resting orderbook exists beforehand
			clearingPrice = lastBuyPrice
		case midMarketPrice != nil && lastSellPrice.GTE(*midMarketPrice):
			clearingPrice = lastSellPrice
		case midMarketPrice != nil:
			clearingPrice = *midMarketPrice
		default:
			// edge case when a resting orderbook does not exist, so no other choice
			// clearing price = (lastBuyPrice + lastSellPrice) / 2
			validClearingPrice := lastBuyPrice.Add(lastSellPrice).Quo(math.LegacyNewDec(2))
			clearingPrice = validClearingPrice
		}
	}

	orderbookResults.ClearingPrice = clearingPrice
	orderbookResults.ClearingQuantity = clearingQuantity
	orderbookResults.TransientBuyOrderbookFills = buyOrderbook.GetTransientOrderbookFills()
	orderbookResults.RestingBuyOrderbookFills = buyOrderbook.GetRestingOrderbookFills()
	orderbookResults.TransientSellOrderbookFills = sellOrderbook.GetTransientOrderbookFills()
	orderbookResults.RestingSellOrderbookFills = sellOrderbook.GetRestingOrderbookFills()

	return orderbookResults
}

func (k SpotKeeper) GetSpotLimitMatchingBatchExecutionData( //nolint:revive // ok
	ctx sdk.Context,
	market *v2.SpotMarket,
	orderbookResults *v2.SpotOrderbookMatchingResults,
	clearingPrice math.LegacyDec,
	pointsMultiplier v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
) *v2.SpotBatchExecutionData {
	// Initialize map DepositKey subaccountID => Deposit Delta (availableBalanceDelta, totalDepositsDelta)
	baseDenomDepositDeltas := types.NewDepositDeltas()
	quoteDenomDepositDeltas := types.NewDepositDeltas()

	limitBuyRestingOrderBatchEvent,
		limitSellRestingOrderBatchEvent,
		filledDeltas,
		restingTradingRewards := k.processBothRestingSpotLimitOrderbookMatchingResults(
		ctx,
		orderbookResults,
		market,
		clearingPrice,
		market.MakerFeeRate,
		market.RelayerFeeShareRate,
		baseDenomDepositDeltas,
		quoteDenomDepositDeltas,
		pointsMultiplier,
		feeDiscountConfig,
	)

	// filled deltas are handled implicitly with the new resting spot limit orders
	limitBuyNewOrderBatchEvent,
		limitSellNewOrderBatchEvent,
		newRestingBuySpotLimitOrders,
		newRestingSellSpotLimitOrders,
		transientTradingRewards := k.processBothTransientSpotLimitOrderbookMatchingResults(
		ctx,
		orderbookResults,
		market,
		clearingPrice,
		market.MakerFeeRate,
		market.TakerFeeRate,
		market.RelayerFeeShareRate,
		baseDenomDepositDeltas,
		quoteDenomDepositDeltas,
		pointsMultiplier,
		feeDiscountConfig,
	)

	eventBatchSpotExecution := make([]*v2.EventBatchSpotExecution, 0)

	if limitBuyRestingOrderBatchEvent != nil {
		eventBatchSpotExecution = append(eventBatchSpotExecution, limitBuyRestingOrderBatchEvent)
	}

	if limitSellRestingOrderBatchEvent != nil {
		eventBatchSpotExecution = append(eventBatchSpotExecution, limitSellRestingOrderBatchEvent)
	}

	if limitBuyNewOrderBatchEvent != nil {
		eventBatchSpotExecution = append(eventBatchSpotExecution, limitBuyNewOrderBatchEvent)
	}

	if limitSellNewOrderBatchEvent != nil {
		eventBatchSpotExecution = append(eventBatchSpotExecution, limitSellNewOrderBatchEvent)
	}

	vwapData := v2.NewSpotVwapData()
	vwapData = vwapData.ApplyExecution(orderbookResults.ClearingPrice, orderbookResults.ClearingQuantity)

	tradingRewards := types.MergeTradingRewardPoints(restingTradingRewards, transientTradingRewards)

	// Final Step: Store the SpotBatchExecutionData for future reduction/processing
	batch := &v2.SpotBatchExecutionData{
		Market:                         market,
		BaseDenomDepositDeltas:         baseDenomDepositDeltas,
		QuoteDenomDepositDeltas:        quoteDenomDepositDeltas,
		BaseDenomDepositSubaccountIDs:  baseDenomDepositDeltas.GetSortedSubaccountKeys(),
		QuoteDenomDepositSubaccountIDs: quoteDenomDepositDeltas.GetSortedSubaccountKeys(),
		LimitOrderFilledDeltas:         filledDeltas,
		LimitOrderExecutionEvent:       eventBatchSpotExecution,
		TradingRewardPoints:            tradingRewards,
		VwapData:                       vwapData,
	}

	if len(newRestingBuySpotLimitOrders) > 0 || len(newRestingSellSpotLimitOrders) > 0 {
		batch.NewOrdersEvent = &v2.EventNewSpotOrders{
			MarketId:   market.MarketId,
			BuyOrders:  newRestingBuySpotLimitOrders,
			SellOrders: newRestingSellSpotLimitOrders,
		}
	}
	return batch
}

// processBothRestingSpotLimitOrderbookMatchingResults processes both the orderbook matching results to produce the spot execution batch events and filledDelta.
// Note: clearingPrice should be set to math.LegacyDec{} for normal fills
//
//nolint:revive // ok
func (k SpotKeeper) processBothRestingSpotLimitOrderbookMatchingResults(
	ctx sdk.Context,
	o *v2.SpotOrderbookMatchingResults,
	market *v2.SpotMarket,
	clearingPrice math.LegacyDec,
	tradeFeeRate, relayerFeeShareRate math.LegacyDec,
	baseDenomDepositDeltas types.DepositDeltas,
	quoteDenomDepositDeltas types.DepositDeltas,
	pointsMultiplier v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
) (
	limitBuyRestingOrderBatchEvent *v2.EventBatchSpotExecution,
	limitSellRestingOrderBatchEvent *v2.EventBatchSpotExecution,
	filledDeltas []*v2.SpotLimitOrderDelta,
	tradingRewardPoints types.TradingRewardPoints,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := market.MarketID()

	var spotLimitBuyOrderStateExpansions, spotLimitSellOrderStateExpansions []*v2.SpotOrderStateExpansion
	var buyTradingRewards, sellTradingRewards types.TradingRewardPoints
	var currFilledDeltas []*v2.SpotLimitOrderDelta

	filledDeltas = make([]*v2.SpotLimitOrderDelta, 0)

	if o.RestingBuyOrderbookFills != nil {
		orderbookFills := o.GetOrderbookFills(v2.RestingLimitBuy)
		spotLimitBuyOrderStateExpansions = k.processRestingSpotLimitOrderExpansions(
			ctx,
			marketID,
			orderbookFills,
			true,
			clearingPrice,
			tradeFeeRate,
			relayerFeeShareRate,
			pointsMultiplier,
			feeDiscountConfig,
		)

		// Process limit order events and filledDeltas
		limitBuyRestingOrderBatchEvent, currFilledDeltas, buyTradingRewards = v2.GetBatchExecutionEventsFromSpotLimitOrderStateExpansions(
			true,
			market,
			v2.ExecutionType_LimitMatchRestingOrder,
			spotLimitBuyOrderStateExpansions,
			baseDenomDepositDeltas, quoteDenomDepositDeltas,
		)

		filledDeltas = append(filledDeltas, currFilledDeltas...)
	}

	if o.RestingSellOrderbookFills != nil {
		orderbookFills := o.GetOrderbookFills(v2.RestingLimitSell)
		spotLimitSellOrderStateExpansions = k.processRestingSpotLimitOrderExpansions(
			ctx,
			marketID,
			orderbookFills,
			false,
			clearingPrice,
			tradeFeeRate,
			relayerFeeShareRate,
			pointsMultiplier,
			feeDiscountConfig,
		)

		// Process limit order events and filledDeltas
		limitSellRestingOrderBatchEvent, currFilledDeltas, sellTradingRewards = v2.GetBatchExecutionEventsFromSpotLimitOrderStateExpansions(
			false,
			market,
			v2.ExecutionType_LimitMatchRestingOrder,
			spotLimitSellOrderStateExpansions,
			baseDenomDepositDeltas, quoteDenomDepositDeltas,
		)
		filledDeltas = append(filledDeltas, currFilledDeltas...)
	}

	tradingRewardPoints = types.MergeTradingRewardPoints(buyTradingRewards, sellTradingRewards)

	return
}

// processBothTransientSpotLimitOrderbookMatchingResults processes the transient spot limit orderbook matching results.
// Note: clearingPrice should be set to math.LegacyDec{} for normal fills
//
//nolint:revive // ok
func (k SpotKeeper) processBothTransientSpotLimitOrderbookMatchingResults(
	ctx sdk.Context,
	o *v2.SpotOrderbookMatchingResults,
	market *v2.SpotMarket,
	clearingPrice math.LegacyDec,
	makerFeeRate, takerFeeRate, relayerFeeShareRate math.LegacyDec,
	baseDenomDepositDeltas, quoteDenomDepositDeltas types.DepositDeltas,
	pointsMultiplier v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
) (
	limitBuyNewOrderBatchEvent *v2.EventBatchSpotExecution,
	limitSellNewOrderBatchEvent *v2.EventBatchSpotExecution,
	newRestingBuySpotLimitOrders []*v2.SpotLimitOrder,
	newRestingSellSpotLimitOrders []*v2.SpotLimitOrder,
	tradingRewardPoints types.TradingRewardPoints,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var expansions []*v2.SpotOrderStateExpansion
	var buyTradingRewards types.TradingRewardPoints
	var sellTradingRewards types.TradingRewardPoints

	if o.TransientBuyOrderbookFills != nil {
		expansions, newRestingBuySpotLimitOrders = k.processTransientSpotLimitBuyOrderbookMatchingResults(
			ctx,
			market.MarketID(),
			o,
			clearingPrice,
			makerFeeRate,
			takerFeeRate,
			relayerFeeShareRate,
			pointsMultiplier,
			feeDiscountConfig,
		)

		limitBuyNewOrderBatchEvent, _, buyTradingRewards = v2.GetBatchExecutionEventsFromSpotLimitOrderStateExpansions(
			true,
			market,
			v2.ExecutionType_LimitMatchNewOrder,
			expansions,
			baseDenomDepositDeltas, quoteDenomDepositDeltas,
		)
	}

	if o.TransientSellOrderbookFills != nil {
		expansions, newRestingSellSpotLimitOrders = k.processTransientSpotLimitSellOrderbookMatchingResults(
			ctx,
			market.MarketID(),
			o,
			clearingPrice,
			takerFeeRate,
			relayerFeeShareRate,
			pointsMultiplier,
			feeDiscountConfig,
		)

		limitSellNewOrderBatchEvent, _, sellTradingRewards = v2.GetBatchExecutionEventsFromSpotLimitOrderStateExpansions(
			false,
			market,
			v2.ExecutionType_LimitMatchNewOrder,
			expansions,
			baseDenomDepositDeltas, quoteDenomDepositDeltas,
		)
	}
	tradingRewardPoints = types.MergeTradingRewardPoints(buyTradingRewards, sellTradingRewards)
	return
}

// TODO: refactor to merge processTransientSpotLimitBuyOrderbookMatchingResults and processTransientSpotLimitSellOrderbookMatchingResults
//
//nolint:revive // ok
func (k SpotKeeper) processTransientSpotLimitBuyOrderbookMatchingResults(
	ctx sdk.Context,
	marketID common.Hash,
	o *v2.SpotOrderbookMatchingResults,
	clearingPrice math.LegacyDec,
	makerFeeRate, takerFeeRate, relayerFeeShare math.LegacyDec,
	pointsMultiplier v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
) ([]*v2.SpotOrderStateExpansion, []*v2.SpotLimitOrder) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orderbookFills := o.TransientBuyOrderbookFills
	stateExpansions := make([]*v2.SpotOrderStateExpansion, len(orderbookFills.Orders))
	newRestingOrders := make([]*v2.SpotLimitOrder, 0, len(orderbookFills.Orders))

	for idx, order := range orderbookFills.Orders {
		fillQuantity := math.LegacyZeroDec()
		if orderbookFills.FillQuantities != nil {
			fillQuantity = orderbookFills.FillQuantities[idx]
		}
		stateExpansions[idx] = k.getTransientSpotLimitBuyStateExpansion(
			ctx,
			marketID,
			order,
			common.BytesToHash(order.OrderHash),
			clearingPrice, fillQuantity,
			makerFeeRate, takerFeeRate, relayerFeeShare,
			pointsMultiplier,
			feeDiscountConfig,
		)

		if order.Fillable.IsPositive() {
			newRestingOrders = append(newRestingOrders, order)
		}
	}
	return stateExpansions, newRestingOrders
}

func (k SpotKeeper) getTransientSpotLimitBuyStateExpansion( //nolint:revive // ok
	ctx sdk.Context,
	marketID common.Hash,
	order *v2.SpotLimitOrder,
	orderHash common.Hash,
	clearingPrice, fillQuantity,
	makerFeeRate, takerFeeRate, relayerFeeShareRate math.LegacyDec,
	pointsMultiplier v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
) *v2.SpotOrderStateExpansion {
	orderNotional, clearingChargeOrRefund, matchedFeeRefund := math.LegacyZeroDec(), math.LegacyZeroDec(), math.LegacyZeroDec()

	isMaker := false
	feeData := k.tradingRewards.GetTradeDataAndIncrementVolumeContribution(
		ctx,
		order.SubaccountID(),
		marketID,
		fillQuantity,
		clearingPrice,
		takerFeeRate,
		relayerFeeShareRate,
		pointsMultiplier.TakerPointsMultiplier,
		feeDiscountConfig,
		isMaker,
	)

	if !fillQuantity.IsZero() {
		orderNotional = fillQuantity.Mul(clearingPrice)
		priceDelta := order.OrderInfo.Price.Sub(clearingPrice)
		// Clearing Refund = FillQuantity * (Price - ClearingPrice)
		clearingChargeOrRefund = fillQuantity.Mul(priceDelta)
		// Matched Fee Refund = FillQuantity * TakerFeeRate * (Price - ClearingPrice)
		matchedFeeRefund = fillQuantity.Mul(feeData.DiscountedTradeFeeRate).Mul(priceDelta)
	}

	// limit buys are credited with the order fill quantity in base denom
	baseChangeAmount := fillQuantity
	// limit buys are debited with (fillQuantity * Price) * (1 + makerFee) in quote denom
	quoteChangeAmount := orderNotional.Add(feeData.TotalTradeFee).Neg()
	// Unmatched Fee Refund = (Quantity - FillQuantity) * Price * (TakerFeeRate - MakerFeeRate)
	positiveMakerFeePart := math.LegacyMaxDec(math.LegacyZeroDec(), makerFeeRate)

	unfilledQuantity := order.OrderInfo.Quantity.Sub(fillQuantity)
	unmatchedFeeRefund := unfilledQuantity.Mul(order.OrderInfo.Price).Mul(takerFeeRate.Sub(positiveMakerFeePart))
	// Fee Refund = Matched Fee Refund + Unmatched Fee Refund
	feeRefund := matchedFeeRefund.Add(unmatchedFeeRefund)
	// refund amount = clearing charge or refund + matched fee refund + unmatched fee refund
	quoteRefundAmount := clearingChargeOrRefund.Add(feeRefund)
	order.Fillable = order.Fillable.Sub(fillQuantity)

	takerFeeRateDelta := takerFeeRate.Sub(feeData.DiscountedTradeFeeRate)
	matchedFeeDiscountRefund := fillQuantity.Mul(order.OrderInfo.Price).Mul(takerFeeRateDelta)
	quoteRefundAmount = quoteRefundAmount.Add(matchedFeeDiscountRefund)

	stateExpansion := v2.SpotOrderStateExpansion{
		BaseChangeAmount:       baseChangeAmount,
		BaseRefundAmount:       math.LegacyZeroDec(),
		QuoteChangeAmount:      quoteChangeAmount,
		QuoteRefundAmount:      quoteRefundAmount,
		TradePrice:             clearingPrice,
		FeeRecipient:           order.FeeRecipient(),
		FeeRecipientReward:     feeData.FeeRecipientReward,
		AuctionFeeReward:       feeData.AuctionFeeReward,
		TraderFeeReward:        math.LegacyZeroDec(),
		TradingRewardPoints:    feeData.TradingRewardPoints,
		LimitOrder:             order,
		LimitOrderFillQuantity: fillQuantity,
		OrderPrice:             order.OrderInfo.Price,
		OrderHash:              orderHash,
		SubaccountID:           order.SubaccountID(),
		TraderAddress:          order.SdkAccAddress().String(),
		Cid:                    order.Cid(),
	}
	return &stateExpansion
}

// processTransientSpotLimitSellOrderbookMatchingResults processes.
// Note: clearingPrice should be set to math.LegacyDec{} for normal fills
func (k SpotKeeper) processTransientSpotLimitSellOrderbookMatchingResults(
	ctx sdk.Context,
	marketID common.Hash,
	o *v2.SpotOrderbookMatchingResults,
	clearingPrice math.LegacyDec,
	takerFeeRate, relayerFeeShare math.LegacyDec,
	pointsMultiplier v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
) ([]*v2.SpotOrderStateExpansion, []*v2.SpotLimitOrder) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orderbookFills := o.TransientSellOrderbookFills

	stateExpansions := make([]*v2.SpotOrderStateExpansion, len(orderbookFills.Orders))
	newRestingOrders := make([]*v2.SpotLimitOrder, 0, len(orderbookFills.Orders))

	for idx, order := range orderbookFills.Orders {
		fillQuantity, fillPrice := orderbookFills.FillQuantities[idx], order.OrderInfo.Price
		if !clearingPrice.IsNil() {
			fillPrice = clearingPrice
		}
		stateExpansions[idx] = k.getSpotLimitSellStateExpansion(
			ctx,
			marketID,
			order,
			false,
			fillQuantity,
			fillPrice,
			takerFeeRate,
			relayerFeeShare,
			pointsMultiplier,
			feeDiscountConfig,
		)
		if order.Fillable.IsPositive() {
			newRestingOrders = append(newRestingOrders, order)
		}
	}
	return stateExpansions, newRestingOrders
}

func (k SpotKeeper) PersistSpotMatchingExecution( //nolint:revive // ok
	ctx sdk.Context,
	batchSpotMatchingExecutionData []*v2.SpotBatchExecutionData,
	spotVwapData v2.SpotVwapInfo,
	tradingRewardPoints types.TradingRewardPoints,
) types.TradingRewardPoints {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// Persist Spot Matching execution data
	for batchIdx := range batchSpotMatchingExecutionData {
		execution := batchSpotMatchingExecutionData[batchIdx]
		if execution == nil {
			continue
		}

		marketID := execution.Market.MarketID()
		baseDenom, quoteDenom := execution.Market.BaseDenom, execution.Market.QuoteDenom

		if execution.VwapData != nil && !execution.VwapData.Price.IsZero() && !execution.VwapData.Quantity.IsZero() {
			spotVwapData.ApplyVwap(marketID, execution.VwapData)
		}

		for _, subaccountID := range execution.BaseDenomDepositSubaccountIDs {
			k.subaccount.UpdateDepositWithDelta(ctx, subaccountID, baseDenom, execution.BaseDenomDepositDeltas[subaccountID])
		}

		for _, subaccountID := range execution.QuoteDenomDepositSubaccountIDs {
			k.subaccount.UpdateDepositWithDelta(ctx, subaccountID, quoteDenom, execution.QuoteDenomDepositDeltas[subaccountID])
		}

		if execution.NewOrdersEvent != nil {
			for idx := range execution.NewOrdersEvent.BuyOrders {
				k.SaveNewSpotLimitOrder(ctx,
					execution.NewOrdersEvent.BuyOrders[idx],
					marketID, true,
					execution.NewOrdersEvent.BuyOrders[idx].Hash(),
				)
			}

			for idx := range execution.NewOrdersEvent.SellOrders {
				k.SaveNewSpotLimitOrder(ctx,
					execution.NewOrdersEvent.SellOrders[idx],
					marketID, false,
					execution.NewOrdersEvent.SellOrders[idx].Hash(),
				)
			}

			events.Emit(ctx, k.BaseKeeper, execution.NewOrdersEvent)
		}

		for _, limitOrderDelta := range execution.LimitOrderFilledDeltas {
			k.UpdateSpotLimitOrder(ctx, marketID, limitOrderDelta)
		}

		for idx := range execution.LimitOrderExecutionEvent {
			if execution.LimitOrderExecutionEvent[idx] != nil {
				events.Emit(ctx, k.BaseKeeper, execution.LimitOrderExecutionEvent[idx])
			}
		}

		if len(execution.TradingRewardPoints) > 0 {
			tradingRewardPoints = types.MergeTradingRewardPoints(tradingRewardPoints, execution.TradingRewardPoints)
		}
	}
	return tradingRewardPoints
}
