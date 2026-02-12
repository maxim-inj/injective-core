package derivative

import (
	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/events"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

//nolint:revive //ok
func (k DerivativeKeeper) GetDerivativeMarketOrderExecutionData(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	marketOrderTradeFeeRate math.LegacyDec,
	markPrice math.LegacyDec,
	funding *v2.PerpetualMarketFunding,
	marketBuyOrders, marketSellOrders []*v2.DerivativeMarketOrder,
	positionStates map[common.Hash]*v2.PositionState,
	positionCache map[common.Hash]*v2.Position,
	feeDiscountConfig *v2.FeeDiscountConfig,
	isLiquidation bool,
	currentOpenNotional math.LegacyDec,
	openNotionalCap v2.OpenNotionalCap,
) *v2.DerivativeMarketOrderExpansionData {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	derivativeMarketOrderExecutionData := &v2.DerivativeMarketOrderExpansionData{
		OpenInterestDelta: math.LegacyZeroDec(),
	}

	var (
		marketBuyOrderbook = newDerivativeMarketOrderbook(
			k,
			true,
			isLiquidation,
			marketBuyOrders,
			market,
			markPrice,
			funding,
			currentOpenNotional,
			openNotionalCap,
			positionStates,
			positionCache,
		)
		limitSellOrderbook = newLimitOrderbook(
			k,
			ctx,
			false,
			nil,
			market,
			markPrice,
			funding,
			currentOpenNotional,
			openNotionalCap,
			positionStates,
			positionCache,
		)

		marketSellOrderbook = newDerivativeMarketOrderbook(
			k,
			false,
			isLiquidation,
			marketSellOrders,
			market,
			markPrice,
			funding,
			currentOpenNotional,
			openNotionalCap,
			positionStates,
			positionCache,
		)

		limitBuyOrderbook = newLimitOrderbook(
			k,
			ctx,
			true,
			nil,
			market,
			markPrice,
			funding,
			currentOpenNotional,
			openNotionalCap,
			positionStates,
			positionCache,
		)
	)

	if limitBuyOrderbook != nil && marketSellOrderbook != nil {
		limitBuyOrderbook.SetOppositeSideDerivativeOrderbook(marketSellOrderbook)
		marketSellOrderbook.SetOppositeSideDerivativeOrderbook(limitBuyOrderbook)
	}

	if limitSellOrderbook != nil && marketBuyOrderbook != nil {
		limitSellOrderbook.SetOppositeSideDerivativeOrderbook(marketBuyOrderbook)
		marketBuyOrderbook.SetOppositeSideDerivativeOrderbook(limitSellOrderbook)
	}

	if limitBuyOrderbook != nil {
		defer limitBuyOrderbook.Close()
	}

	if limitSellOrderbook != nil {
		defer limitSellOrderbook.Close()
	}

	matchingOrderbooks := newMarketExecutionOrderbooks(
		limitBuyOrderbook,
		limitSellOrderbook,
		marketBuyOrderbook,
		marketSellOrderbook,
	)

	tradeRewardsMultiplierConfig := k.GetEffectiveTradingRewardsMarketPointsMultiplierConfig(ctx, market.MarketID())

	for idx := range matchingOrderbooks {
		m := matchingOrderbooks[idx]

		if m.marketOrderbook == nil {
			continue
		}

		k.executeDerivativeMarketOrders(ctx, m)

		var marketOrderClearingPrice math.LegacyDec
		if !m.marketOrderbook.totalQuantity.IsZero() {
			marketOrderClearingPrice = m.limitOrderbook.GetNotional().Quo(m.marketOrderbook.GetTotalQuantityFilled())
		}

		if isLiquidation {
			marketOrderTradeFeeRate = math.LegacyZeroDec() // no trading fees for liquidations
		}

		marketOrderStateExpansions, marketOrderCancels := k.processDerivativeMarketOrderbookMatchingResults(
			ctx,
			market,
			funding,
			m.marketOrderbook.orders,
			m.marketOrderbook.GetOrderbookFillQuantities(),
			positionStates,
			marketOrderClearingPrice,
			marketOrderTradeFeeRate,
			tradeRewardsMultiplierConfig.TakerPointsMultiplier,
			feeDiscountConfig,
		)

		derivativeMarketOrderExecutionData.OpenInterestDelta = derivativeMarketOrderExecutionData.OpenInterestDelta.Add(
			m.marketOrderbook.GetOpenInterestDelta(),
		)

		var restingLimitOrderStateExpansions []*v2.DerivativeOrderStateExpansion
		var restingLimitOrderCancels []*v2.DerivativeLimitOrder
		if m.limitOrderbook != nil {
			restingOrderFills := m.limitOrderbook.GetRestingOrderbookFills()
			limitOrderClearingPrice := math.LegacyDec{} // no clearing price for limit orders when executed against market orders
			restingLimitOrderStateExpansions = k.processRestingDerivativeLimitOrderbookFills(
				ctx,
				market,
				funding,
				restingOrderFills,
				!m.isMarketBuy,
				positionStates,
				limitOrderClearingPrice,
				tradeRewardsMultiplierConfig,
				feeDiscountConfig,
				isLiquidation,
			)
			restingLimitOrderCancels = m.limitOrderbook.GetRestingOrderbookCancels()

			derivativeMarketOrderExecutionData.OpenInterestDelta = derivativeMarketOrderExecutionData.OpenInterestDelta.Add(
				m.limitOrderbook.GetOpenInterestDelta(),
			)
		}

		if m.isMarketBuy {
			derivativeMarketOrderExecutionData.SetBuyExecutionData(
				marketOrderClearingPrice,
				m.marketOrderbook.totalQuantity,
				restingLimitOrderCancels,
				marketOrderStateExpansions,
				restingLimitOrderStateExpansions,
				marketOrderCancels,
			)
		} else {
			derivativeMarketOrderExecutionData.SetSellExecutionData(
				marketOrderClearingPrice,
				m.marketOrderbook.totalQuantity,
				restingLimitOrderCancels,
				marketOrderStateExpansions,
				restingLimitOrderStateExpansions,
				marketOrderCancels,
			)
		}
	}

	return derivativeMarketOrderExecutionData
}

//nolint:revive //ok
func (k DerivativeKeeper) PersistSingleDerivativeMarketOrderExecution(
	ctx sdk.Context,
	execution *v2.DerivativeBatchExecutionData,
	derivativeVwapData v2.DerivativeVwapInfo,
	tradingRewardPoints types.TradingRewardPoints,
	modifiedPositionCache v2.ModifiedPositionCache,
	isLiquidation bool,
) (points types.TradingRewardPoints, isMarketSolvent bool) {
	if execution == nil {
		return tradingRewardPoints, true
	}

	marketID := execution.Market.MarketID()
	isMarketSolvent = k.EnsureMarketSolvency(ctx, execution.Market, execution.MarketBalanceDelta, true)

	if !isMarketSolvent {
		return tradingRewardPoints, isMarketSolvent
	}

	k.ApplyOpenInterestDeltaForMarket(
		ctx,
		marketID,
		execution.OpenInterestDelta,
	)

	hasValidMarkPrice := execution.Market.GetMarketType() == types.MarketType_BinaryOption || !execution.MarkPrice.IsNil() && execution.MarkPrice.IsPositive()

	if execution.VwapData != nil && !execution.VwapData.Price.IsZero() && !execution.VwapData.Quantity.IsZero() && hasValidMarkPrice {
		derivativeVwapData.ApplyVwap(marketID, &execution.MarkPrice, execution.VwapData, execution.Market.GetMarketType())
	}

	for _, subaccountID := range execution.DepositSubaccountIDs {
		if isLiquidation {
			// in liquidations beyond bankruptcy we shall not charge from bank to avoid rugging from bank balances
			k.subaccount.UpdateDepositWithDeltaWithoutBankCharge(
				ctx,
				subaccountID,
				execution.Market.GetQuoteDenom(),
				execution.DepositDeltas[subaccountID],
			)
		} else {
			k.subaccount.UpdateDepositWithDelta(
				ctx,
				subaccountID,
				execution.Market.GetQuoteDenom(),
				execution.DepositDeltas[subaccountID],
			)
		}
	}

	k.UpdateDerivativeLimitOrdersFromFilledDeltas(ctx, marketID, true, execution.RestingLimitOrderFilledDeltas, nil)
	k.UpdateDerivativeLimitOrdersFromFilledDeltas(ctx, marketID, true, execution.RestingLimitOrderCancelledDeltas, nil)

	for idx, subaccountID := range execution.PositionSubaccountIDs {
		k.SavePosition(ctx, marketID, subaccountID, execution.Positions[idx])

		if modifiedPositionCache != nil {
			modifiedPositionCache.SetPosition(marketID, subaccountID, execution.Positions[idx])
		}
	}

	if execution.MarketBuyOrderExecutionEvent != nil {
		events.Emit(ctx, k.BaseKeeper, execution.MarketBuyOrderExecutionEvent)
		events.Emit(ctx, k.BaseKeeper, execution.RestingLimitSellOrderExecutionEvent)
	}
	if execution.MarketSellOrderExecutionEvent != nil {
		events.Emit(ctx, k.BaseKeeper, execution.MarketSellOrderExecutionEvent)
		events.Emit(ctx, k.BaseKeeper, execution.RestingLimitBuyOrderExecutionEvent)
	}

	for idx := range execution.CancelLimitOrderEvents {
		events.Emit(ctx, k.BaseKeeper, execution.CancelLimitOrderEvents[idx])
	}
	for idx := range execution.CancelMarketOrderEvents {
		events.Emit(ctx, k.BaseKeeper, execution.CancelMarketOrderEvents[idx])
	}

	if len(execution.TradingRewards) > 0 {
		tradingRewardPoints = types.MergeTradingRewardPoints(tradingRewardPoints, execution.TradingRewards)
	}

	return tradingRewardPoints, isMarketSolvent
}

func (k DerivativeKeeper) ExecuteDerivativeMarketOrderMatching(
	ctx sdk.Context,
	matchedMarketDirection *types.MatchedMarketDirection,
	stakingInfo *v2.FeeDiscountStakingInfo,
) *v2.DerivativeBatchExecutionData {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := matchedMarketDirection.MarketId

	market, markPrice := k.GetDerivativeOrBinaryOptionsMarketWithMarkPrice(ctx, marketID, true)

	if market == nil {
		return nil
	}

	feeDiscountConfig := k.feeDiscounts.GetFeeDiscountConfigForMarket(ctx, marketID, stakingInfo)

	var funding *v2.PerpetualMarketFunding
	if market.GetIsPerpetual() {
		funding = k.GetPerpetualMarketFunding(ctx, marketID)
	}

	// Step 0: Obtain the market buy and sell orders from the transient store for convenience
	// Step 0: Obtain the market buy and sell orders from the transient store for convenience
	marketBuyOrders := k.GetAllTransientDerivativeMarketOrdersByMarketDirection(ctx, marketID, true)
	marketSellOrders := k.GetAllTransientDerivativeMarketOrdersByMarketDirection(ctx, marketID, false)

	positionStates := v2.NewPositionStates()
	positionCache := make(map[common.Hash]*v2.Position)

	currentOpenNotional := k.GetOpenNotionalForMarket(ctx, marketID, markPrice)
	openNotionalCap := market.GetOpenNotionalCap()

	isLiquidation := false
	derivativeMarketOrderExecution := k.GetDerivativeMarketOrderExecutionData(
		ctx,
		market,
		market.GetTakerFeeRate(),
		markPrice,
		funding,
		marketBuyOrders,
		marketSellOrders,
		positionStates,
		positionCache,
		feeDiscountConfig,
		isLiquidation,
		currentOpenNotional,
		openNotionalCap,
	)

	batchExecutionData := derivativeMarketOrderExecution.GetMarketDerivativeBatchExecutionData(
		market,
		markPrice,
		funding,
		positionStates,
		isLiquidation,
	)

	return batchExecutionData
}

func (k DerivativeKeeper) executeDerivativeMarketOrders(ctx sdk.Context, matchingOrderbook *marketExecutionOrderbook) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var (
		isMarketBuy     = matchingOrderbook.isMarketBuy
		marketOrderbook = matchingOrderbook.marketOrderbook
		limitOrderbook  = matchingOrderbook.limitOrderbook
	)

	if marketOrderbook == nil || limitOrderbook == nil {
		return
	}

	for {
		var buyOrder, sellOrder *v2.PriceLevel

		if isMarketBuy {
			buyOrder = marketOrderbook.Peek(ctx)
			sellOrder = limitOrderbook.Peek(ctx)
		} else {
			sellOrder = marketOrderbook.Peek(ctx)
			buyOrder = limitOrderbook.Peek(ctx)
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

		marketOrderbook.Fill(ctx, matchQuantityIncrement)
		limitOrderbook.Fill(matchQuantityIncrement)
	}
}

// processRestingDerivativeLimitOrderbookFills processes the resting derivative limit order execution.
// NOTE: clearingPrice may be Nil
//
//nolint:revive //ok
func (k DerivativeKeeper) processRestingDerivativeLimitOrderbookFills(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	funding *v2.PerpetualMarketFunding,
	fills *orderbookFills,
	isBuy bool,
	positionStates map[common.Hash]*v2.PositionState,
	clearingPrice math.LegacyDec,
	tradeRewardsMultiplierConfig v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
	isLiquidation bool,
) []*v2.DerivativeOrderStateExpansion {
	stateExpansions := make([]*v2.DerivativeOrderStateExpansion, len(fills.Orders))

	for idx := range fills.Orders {
		stateExpansions[idx] = k.applyPositionDeltaAndGetDerivativeLimitOrderStateExpansion(
			ctx,
			market,
			funding,
			isBuy,
			false,
			fills.Orders[idx],
			positionStates,
			fills.FillQuantities[idx],
			clearingPrice,
			tradeRewardsMultiplierConfig,
			feeDiscountConfig,
			isLiquidation,
		)
	}

	return stateExpansions
}

// NOTE: clearingPrice can be nil
//
//nolint:revive //ok
func (k DerivativeKeeper) applyPositionDeltaAndGetDerivativeLimitOrderStateExpansion(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	funding *v2.PerpetualMarketFunding,
	isBuy bool,
	isTransient bool,
	order *v2.DerivativeLimitOrder,
	positionStates map[common.Hash]*v2.PositionState,
	fillQuantity, clearingPrice math.LegacyDec,
	tradeRewardMultiplierConfig v2.PointsMultiplier,
	feeDiscountConfig *v2.FeeDiscountConfig,
	isLiquidation bool,
) *v2.DerivativeOrderStateExpansion {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var executionPrice math.LegacyDec
	if clearingPrice.IsNil() {
		executionPrice = order.OrderInfo.Price
	} else {
		executionPrice = clearingPrice
	}

	var tradeFeeRate, tradeRewardMultiplier math.LegacyDec
	if isTransient {
		tradeFeeRate = market.GetTakerFeeRate()
		tradeRewardMultiplier = tradeRewardMultiplierConfig.TakerPointsMultiplier
	} else {
		tradeFeeRate = market.GetMakerFeeRate()
		tradeRewardMultiplier = tradeRewardMultiplierConfig.MakerPointsMultiplier
	}

	if tradeFeeRate.IsNegative() && isLiquidation {
		// liquidated position is closed with zero trading fee, so no taker fee to pay the negative maker fee
		tradeFeeRate = math.LegacyZeroDec()
	}

	isMaker := !isTransient
	feeData := k.trading.GetTradeDataAndIncrementVolumeContribution(
		ctx,
		order.SubaccountID(),
		market.MarketID(),
		fillQuantity,
		executionPrice,
		tradeFeeRate,
		market.GetRelayerFeeShareRate(),
		tradeRewardMultiplier,
		feeDiscountConfig,
		isMaker,
	)

	k.fillPositionStateCache(ctx, market.MarketID(), funding, order.SubaccountID(), order.IsBuy(), positionStates)
	position := positionStates[order.SubaccountID()].Position

	var (
		positionDelta               *v2.PositionDelta
		unusedExecutionMarginRefund = math.LegacyZeroDec()
	)

	if fillQuantity.IsPositive() {
		marginFillProportion := order.Margin.Mul(fillQuantity).Quo(order.OrderInfo.Quantity)

		var executionMargin math.LegacyDec
		if market.GetMarketType() != types.MarketType_BinaryOption {
			executionMargin = marginFillProportion
		} else {
			executionMargin = types.GetRequiredBinaryOptionsOrderMargin(
				executionPrice,
				fillQuantity,
				market.GetOracleScaleFactor(),
				order.IsBuy(),
				order.IsReduceOnly(),
			)

			if marginFillProportion.GT(executionMargin) {
				unusedExecutionMarginRefund = marginFillProportion.Sub(executionMargin)
			}
		}

		positionDelta = &v2.PositionDelta{
			IsLong:            isBuy,
			ExecutionQuantity: fillQuantity,
			ExecutionMargin:   executionMargin,
			ExecutionPrice:    executionPrice,
		}
	}

	payout, closeExecutionMargin, collateralizationMargin, pnl := position.ApplyPositionDelta(positionDelta, feeData.TraderFee)

	unmatchedFeeRefundRate := math.LegacyZeroDec()
	if isTransient {
		positiveMakerFeeRatePart := math.LegacyMaxDec(math.LegacyZeroDec(), market.GetMakerFeeRate())
		unmatchedFeeRefundRate = market.GetTakerFeeRate().Sub(positiveMakerFeeRatePart)
	}

	unmatchedFeeRefund, matchedFeeRefundOrCharge, feeCharge := getDerivativeOrderFeesAndRefunds(
		order.Fillable,
		order.Price(),
		order.IsReduceOnly(),
		fillQuantity,
		executionPrice,
		tradeFeeRate,
		unmatchedFeeRefundRate,
		feeData,
	)

	order.Fillable = order.Fillable.Sub(fillQuantity)

	totalBalanceChange := payout.Sub(collateralizationMargin.Add(feeCharge))
	availableBalanceChange := payout.Add(closeExecutionMargin).Add(matchedFeeRefundOrCharge).Add(unmatchedFeeRefund).Add(unusedExecutionMarginRefund)

	hasTradingFeeInPayout := order.IsReduceOnly()
	isFeeRebateForAvailableBalanceRequired := feeData.TraderFee.IsNegative() && !hasTradingFeeInPayout

	if isFeeRebateForAvailableBalanceRequired {
		availableBalanceChange = availableBalanceChange.Add(feeData.TraderFee.Abs())
	}

	availableBalanceChange, totalBalanceChange = k.adjustPositionMarginIfNecessary(
		ctx,
		market,
		order.SubaccountID(),
		position,
		availableBalanceChange,
		totalBalanceChange,
	)

	marketBalanceDelta := v2.GetMarketBalanceDelta(payout, collateralizationMargin, feeData.TraderFee, order.IsReduceOnly())
	stateExpansion := v2.DerivativeOrderStateExpansion{
		SubaccountID:          order.SubaccountID(),
		PositionDelta:         positionDelta,
		Payout:                payout,
		Pnl:                   pnl,
		MarketBalanceDelta:    marketBalanceDelta,
		TotalBalanceDelta:     totalBalanceChange,
		AvailableBalanceDelta: availableBalanceChange,
		AuctionFeeReward:      feeData.AuctionFeeReward,
		TradingRewardPoints:   feeData.TradingRewardPoints,
		FeeRecipientReward:    feeData.FeeRecipientReward,
		FeeRecipient:          order.FeeRecipient(),
		LimitOrderFilledDelta: &v2.DerivativeLimitOrderDelta{
			Order:          order,
			FillQuantity:   fillQuantity,
			CancelQuantity: math.LegacyZeroDec(),
		},
		OrderHash: order.Hash(),
		Cid:       order.Cid(),
	}

	return &stateExpansion
}

func (k DerivativeKeeper) fillPositionStateCache(
	ctx sdk.Context,
	marketID common.Hash,
	funding *v2.PerpetualMarketFunding,
	orderSubaccountID common.Hash,
	isOrderBuy bool,
	positionStates map[common.Hash]*v2.PositionState,
) {
	positionState := positionStates[orderSubaccountID]
	if positionState != nil {
		return
	}

	position := k.GetPosition(ctx, marketID, orderSubaccountID)

	if position == nil {
		var cumulativeFundingEntry math.LegacyDec
		if funding != nil {
			cumulativeFundingEntry = funding.CumulativeFunding
		}
		position = v2.NewPosition(isOrderBuy, cumulativeFundingEntry)
	}

	positionStates[orderSubaccountID] = &v2.PositionState{
		Position: position,
	}
}

// NOTE: clearingPrice may be Nil
//
//nolint:revive //ok
func (k DerivativeKeeper) processDerivativeMarketOrderbookMatchingResults(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	funding *v2.PerpetualMarketFunding,
	marketOrders []*v2.DerivativeMarketOrder,
	marketFillQuantities []math.LegacyDec,
	positionStates map[common.Hash]*v2.PositionState,
	clearingPrice math.LegacyDec,
	tradeFeeRate math.LegacyDec,
	tradeRewardsMultiplier math.LegacyDec,
	feeDiscountConfig *v2.FeeDiscountConfig,
) ([]*v2.DerivativeOrderStateExpansion, []*v2.DerivativeMarketOrderCancel) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	stateExpansions := make([]*v2.DerivativeOrderStateExpansion, len(marketOrders))
	ordersToCancel := make([]*v2.DerivativeMarketOrderCancel, 0, len(marketOrders))

	for idx := range marketOrders {
		o := marketOrders[idx]
		unfilledQuantity := o.OrderInfo.Quantity.Sub(marketFillQuantities[idx])

		if clearingPrice.IsNil() {
			stateExpansions[idx] = &v2.DerivativeOrderStateExpansion{
				SubaccountID:          o.SubaccountID(),
				PositionDelta:         nil,
				Payout:                math.LegacyZeroDec(),
				Pnl:                   math.LegacyZeroDec(),
				MarketBalanceDelta:    math.LegacyZeroDec(),
				TotalBalanceDelta:     math.LegacyZeroDec(),
				AvailableBalanceDelta: o.MarginHold,
				AuctionFeeReward:      math.LegacyZeroDec(),
				TradingRewardPoints:   math.LegacyZeroDec(),
				FeeRecipientReward:    math.LegacyZeroDec(),
				FeeRecipient:          o.FeeRecipient(),
				LimitOrderFilledDelta: nil,
				MarketOrderFilledDelta: &v2.DerivativeMarketOrderDelta{
					Order:        o,
					FillQuantity: math.LegacyZeroDec(),
				},
				OrderHash: o.Hash(),
				Cid:       o.Cid(),
			}
		} else {
			stateExpansions[idx] = k.applyPositionDeltaAndGetDerivativeMarketOrderStateExpansion(
				ctx,
				market,
				funding,
				marketOrders[idx],
				positionStates,
				marketFillQuantities[idx],
				clearingPrice,
				tradeFeeRate,
				market.GetRelayerFeeShareRate(),
				tradeRewardsMultiplier,
				feeDiscountConfig,
			)
		}

		if !unfilledQuantity.IsZero() {
			ordersToCancel = append(ordersToCancel, &v2.DerivativeMarketOrderCancel{
				MarketOrder:    o,
				CancelQuantity: unfilledQuantity,
			})
		}
	}

	return stateExpansions, ordersToCancel
}

//nolint:revive //ok
func (k DerivativeKeeper) applyPositionDeltaAndGetDerivativeMarketOrderStateExpansion(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	funding *v2.PerpetualMarketFunding,
	order *v2.DerivativeMarketOrder,
	positionStates map[common.Hash]*v2.PositionState,
	fillQuantity, clearingPrice math.LegacyDec,
	takerFeeRate, relayerFeeShareRate math.LegacyDec,
	tradeRewardMultiplier math.LegacyDec,
	feeDiscountConfig *v2.FeeDiscountConfig,
) *v2.DerivativeOrderStateExpansion {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if fillQuantity.IsNil() {
		fillQuantity = math.LegacyZeroDec()
	}

	isMaker := false
	feeData := k.trading.GetTradeDataAndIncrementVolumeContribution(
		ctx,
		order.SubaccountID(),
		market.MarketID(),
		fillQuantity,
		clearingPrice,
		takerFeeRate,
		relayerFeeShareRate,
		tradeRewardMultiplier,
		feeDiscountConfig,
		isMaker,
	)
	k.fillPositionStateCache(ctx, market.MarketID(), funding, order.SubaccountID(), order.IsBuy(), positionStates)
	position := positionStates[order.SubaccountID()].Position

	var executionMargin math.LegacyDec
	if market.GetMarketType() == types.MarketType_BinaryOption {
		executionMargin = types.GetRequiredBinaryOptionsOrderMargin(
			clearingPrice,
			fillQuantity,
			market.GetOracleScaleFactor(),
			order.IsBuy(),
			order.IsReduceOnly(),
		)
	} else {
		executionMargin = order.Margin.Mul(fillQuantity).Quo(order.Quantity())
	}
	unusedExecutionMarginRefund := order.Margin.Sub(executionMargin)

	var positionDelta *v2.PositionDelta

	if fillQuantity.IsPositive() {
		positionDelta = &v2.PositionDelta{
			IsLong:            order.IsBuy(),
			ExecutionQuantity: fillQuantity,
			ExecutionMargin:   executionMargin,
			ExecutionPrice:    clearingPrice,
		}
	}

	payout, closeExecutionMargin, collateralizationMargin, pnl := position.ApplyPositionDelta(positionDelta, feeData.TraderFee)

	unmatchedFeeRefundRate := takerFeeRate
	unmatchedFeeRefund, matchedFeeRefundOrCharge, feeCharge := getDerivativeOrderFeesAndRefunds(
		order.Quantity(),
		order.Price(),
		order.IsReduceOnly(),
		fillQuantity,
		clearingPrice,
		takerFeeRate,
		unmatchedFeeRefundRate,
		feeData,
	)

	totalBalanceChange := payout.Sub(collateralizationMargin.Add(feeCharge))
	availableBalanceChange := payout.Add(closeExecutionMargin).
		Add(unusedExecutionMarginRefund).
		Add(matchedFeeRefundOrCharge).
		Add(unmatchedFeeRefund)

	availableBalanceChange, totalBalanceChange = k.adjustPositionMarginIfNecessary(
		ctx,
		market,
		order.SubaccountID(),
		position,
		availableBalanceChange,
		totalBalanceChange,
	)

	marketBalanceDelta := v2.GetMarketBalanceDelta(payout, collateralizationMargin, feeData.TraderFee, order.IsReduceOnly())
	stateExpansion := v2.DerivativeOrderStateExpansion{
		SubaccountID:          order.SubaccountID(),
		PositionDelta:         positionDelta,
		Payout:                payout,
		Pnl:                   pnl,
		MarketBalanceDelta:    marketBalanceDelta,
		TotalBalanceDelta:     totalBalanceChange,
		AvailableBalanceDelta: availableBalanceChange,
		AuctionFeeReward:      feeData.AuctionFeeReward,
		TradingRewardPoints:   feeData.TradingRewardPoints,
		FeeRecipientReward:    feeData.FeeRecipientReward,
		FeeRecipient:          order.FeeRecipient(),
		MarketOrderFilledDelta: &v2.DerivativeMarketOrderDelta{
			Order:        order,
			FillQuantity: fillQuantity,
		},
		OrderHash: order.Hash(),
		Cid:       order.Cid(),
	}

	return &stateExpansion
}

// NOTE: unmatchedFeeRefundRate is:
//
//	0 for resting limit orders
//	γ_taker - max(γ_maker, 0) for transient limit orders
//	γ_taker for market orders
//
//nolint:revive //ok
func getDerivativeOrderFeesAndRefunds(
	orderFillableQuantity,
	orderPrice math.LegacyDec,
	isOrderReduceOnly bool,
	fillQuantity,
	executionPrice,
	tradeFeeRate,
	unmatchedFeeRefundRate math.LegacyDec,
	feeData *v2.TradeFeeData,
) (unmatchedFeeRefund, matchedFeeRefundOrCharge, feeCharge math.LegacyDec) {
	if isOrderReduceOnly {
		return math.LegacyZeroDec(), math.LegacyZeroDec(), math.LegacyZeroDec()
	}

	// the amount of trading fees the trader will pay
	feeCharge = feeData.TraderFee

	var (
		positiveTradeFeeRatePart      = math.LegacyMaxDec(math.LegacyZeroDec(), tradeFeeRate)
		positiveDiscountedFeeRatePart = math.LegacyMaxDec(math.LegacyZeroDec(), feeData.DiscountedTradeFeeRate)
		unfilledQuantity              = orderFillableQuantity.Sub(fillQuantity)
		// nolint:all
		// ΔPrice = OrderPrice - ExecutionPrice
		priceDelta = orderPrice.Sub(executionPrice)
	)

	// the fee refund for the unfilled order quantity
	unmatchedFeeRefund = unfilledQuantity.Mul(orderPrice).Mul(unmatchedFeeRefundRate)

	// for a buy order, priceDelta >= 0, so get a fee refund for the matching, since the margin assumed a higher price
	// for a sell order, priceDelta <= 0, so pay extra trading fee

	// matched fee refund or charge = FillQuantity * ΔPrice * Rate
	// this is the fee refund or charge resulting from the order being executed at a better price
	matchedFeePriceDeltaRefundOrCharge := fillQuantity.Mul(priceDelta).Mul(positiveDiscountedFeeRatePart)

	feeRateDelta := positiveTradeFeeRatePart.Sub(positiveDiscountedFeeRatePart)
	matchedFeeDiscountRefund := fillQuantity.Mul(orderPrice).Mul(feeRateDelta)

	matchedFeeRefundOrCharge = matchedFeePriceDeltaRefundOrCharge.Add(matchedFeeDiscountRefund)

	// Example for matchedFeeRefundOrCharge for market buy order:
	// paid originally takerFee * orderQuantity * orderPrice   = 0.001  * 12 * 1.7 = 0.0204
	// paid now discountedTakerFee * fillQuantity * executionPrice = 0.0007 * 12 * 1.6 = 0.01344
	//
	// discount refund = (takerFeeRate - discountedTradeFeeRate) * fillQuantity * orderPrice = (0.001-0.0007) * 12 * 1.7 = 0.00612
	// price delta refund or charge = discounted fee * fill quantity * ΔPrice =  0.0007 * 12 * 0.1 = 0.00084
	//
	// paid originally == paid now + discount refund + price delta refund
	// 0.0204 == 0.01344 + 0.00612 + 0.00084 ✅

	// Example for matchedFeeRefundOrCharge for market sell order:
	// paid originally takerFee * orderQuantity * orderPrice   = 0.001  * 12 * 1.7 = 0.0204
	// paid now discountedTakerFee * fillQuantity * executionPrice = 0.0007 * 12 * 1.8 = 0.01512
	//
	// discount refund = (takerFeeRate - discountedTakerFeeRate) * fillQuantity * orderPrice = (0.001-0.0007) * 12 * 1.7 = 0.00612
	// price delta refund or charge = discounted fee * fill quantity * ΔPrice =  0.0007 * 12 * -0.1 = -0.00084
	//
	// paid originally == paid now + discount refund + price delta refund
	// 0.0204 == 0.01512 + 0.00612 - 0.00084 ✅

	return unmatchedFeeRefund, matchedFeeRefundOrCharge, feeCharge
}

// Can happen if sell order is matched at better price incurring a higher trading fee that needs to be charged to trader. Function is implemented
// in a more general way to also handle other unknown cases as defensive programming.
//
//nolint:revive //ok
func (k DerivativeKeeper) adjustPositionMarginIfNecessary(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	subaccountID common.Hash,
	position *v2.Position,
	availableBalanceChange, totalBalanceChange math.LegacyDec,
) (math.LegacyDec, math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// if available balance delta is negative, it means sell order was matched at better price implying a higher fee
	// we need to charge trader for the higher fee
	hasPositiveAvailableBalanceDelta := !availableBalanceChange.IsNegative()
	if hasPositiveAvailableBalanceDelta {
		return availableBalanceChange, totalBalanceChange
	}

	// for binary options:
	// 	we can safely reduce from balances, because his margin was adjusted meaning he has enough balance to cover it
	// 	and we shouldn't adjust the margin anyways
	isBinaryOptions := market.GetMarketType().IsBinaryOptions()
	if isBinaryOptions {
		return availableBalanceChange, totalBalanceChange
	}

	// check if position has sufficient margin to deduct from, may not be the case during liquidations beyond bankruptcy
	//
	// NOTE that one may think that a reduce-only order could result in a case where a trader has insufficient balance and no position margin.
	// This would require a reduce-only order of the full position size at exactly bankruptcy price, leading to zero total payout.
	// -> user would have zero balance and zero margin and we could not charge him.
	// However, this is not exploitable, because a user would first need to create an order at a price even worse than bankruptcy price
	// to create a non-zero matched fee charge (trading fee of matched vs. order price). Fortunately we always check if an order closes
	// a position beyond bankruptcy price (`CheckValidPositionToReduce`), even during FBA matching and we use the original order price for this check.

	hasSufficientMarginToCharge := position.Margin.GT(availableBalanceChange.Abs())
	if !hasSufficientMarginToCharge {
		return availableBalanceChange, totalBalanceChange
	}

	chainFormattedAvailableBalanceChange := market.NotionalToChainFormat(availableBalanceChange)
	spendableFunds := k.subaccount.GetSpendableFunds(ctx, subaccountID, market.GetQuoteDenom())
	isTraderMissingFunds := spendableFunds.Add(chainFormattedAvailableBalanceChange).IsNegative()

	if !isTraderMissingFunds {
		return availableBalanceChange, totalBalanceChange
	}

	// trader has **not** have enough funds to cover additional fee
	// for derivatives: we can instead safely reduce his position margin
	position.Margin = position.Margin.Add(availableBalanceChange)
	k.ApplyMarketBalanceDelta(ctx, market.MarketID(), chainFormattedAvailableBalanceChange)

	// charging from margin, so give back to available and total balance
	modifiedTotalBalanceChange := totalBalanceChange.Sub(availableBalanceChange)
	modifiedAvailableBalanceChange := math.LegacyZeroDec() // available - available becomes 0

	return modifiedAvailableBalanceChange, modifiedTotalBalanceChange
}

// limitOrderbookExpansionSide holds an orderbook side (buy or sell) and the expansionData
// callbacks used when processing fills, so both sides can be handled in a single loop.
type limitOrderbookExpansionSide struct {
	orderbook                     *limitOrderbook
	isBuy                         bool
	addNewRestingOrder            func(*v2.DerivativeLimitOrder)
	setRestingLimitOrderCancels   func([]*v2.DerivativeLimitOrder)
	setTransientLimitOrderCancels func([]*v2.DerivativeLimitOrder)
}

//nolint:revive //ok
func (k DerivativeKeeper) GetDerivativeMatchingExecutionData(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	markPrice math.LegacyDec,
	funding *v2.PerpetualMarketFunding,
	transientBuyOrders, transientSellOrders []*v2.DerivativeLimitOrder,
	positionStates map[common.Hash]*v2.PositionState,
	positionCache map[common.Hash]*v2.Position,
	feeDiscountConfig *v2.FeeDiscountConfig,
	currentOpenNotional math.LegacyDec,
	openNotionalCap v2.OpenNotionalCap,
) *v2.DerivativeMatchingExpansionData {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var (
		buyOrderbook = newLimitOrderbook(
			k,
			ctx,
			true,
			transientBuyOrders,
			market,
			markPrice,
			funding,
			currentOpenNotional,
			openNotionalCap,
			positionStates,
			positionCache,
		)
		sellOrderbook = newLimitOrderbook(
			k,
			ctx,
			false,
			transientSellOrders,
			market,
			markPrice,
			funding,
			currentOpenNotional,
			openNotionalCap,
			positionStates,
			positionCache,
		)
	)

	if buyOrderbook != nil && sellOrderbook != nil {
		buyOrderbook.SetOppositeSideDerivativeOrderbook(sellOrderbook)
		sellOrderbook.SetOppositeSideDerivativeOrderbook(buyOrderbook)
	}

	if buyOrderbook != nil {
		defer buyOrderbook.Close()
	}

	if sellOrderbook != nil {
		defer sellOrderbook.Close()
	}

	var clearingQuantity, clearingPrice math.LegacyDec

	if buyOrderbook != nil && sellOrderbook != nil {
		var (
			lastBuyPrice  math.LegacyDec
			lastSellPrice math.LegacyDec
		)

		for {
			buyOrder := buyOrderbook.Peek(ctx)
			sellOrder := sellOrderbook.Peek(ctx)

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

			lastBuyPrice = buyOrder.Price
			lastSellPrice = sellOrder.Price

			buyOrderbook.Fill(matchQuantityIncrement)
			sellOrderbook.Fill(matchQuantityIncrement)
		}

		clearingQuantity = buyOrderbook.GetTotalQuantityFilled()

		if clearingQuantity.IsPositive() {
			midMarketPrice := k.GetDerivativeMidPriceOrBestPrice(ctx, market.MarketID())
			clearingPrice = k.GetClearingPriceFromMatching(
				lastBuyPrice,
				lastSellPrice,
				markPrice,
				clearingQuantity,
				midMarketPrice,
				buyOrderbook,
				sellOrderbook,
			)
		}
	}

	tradeRewardsMultiplierConfig := k.GetEffectiveTradingRewardsMarketPointsMultiplierConfig(ctx, market.MarketID())
	expansionData := v2.NewDerivativeMatchingExpansionData(clearingPrice, clearingQuantity)

	var sides []limitOrderbookExpansionSide
	if buyOrderbook != nil {
		sides = append(sides, limitOrderbookExpansionSide{
			orderbook:                     buyOrderbook,
			isBuy:                         true,
			addNewRestingOrder:            expansionData.AddNewBuyRestingLimitOrder,
			setRestingLimitOrderCancels:   expansionData.SetRestingLimitBuyOrderCancels,
			setTransientLimitOrderCancels: expansionData.SetTransientLimitBuyOrderCancels,
		})
	}
	if sellOrderbook != nil {
		sides = append(sides, limitOrderbookExpansionSide{
			orderbook:                     sellOrderbook,
			isBuy:                         false,
			addNewRestingOrder:            expansionData.AddNewSellRestingLimitOrder,
			setRestingLimitOrderCancels:   expansionData.SetRestingLimitSellOrderCancels,
			setTransientLimitOrderCancels: expansionData.SetTransientLimitSellOrderCancels,
		})
	}

	for _, side := range sides {
		for hash := range side.orderbook.GetPartialCancelOrders() {
			expansionData.PartialCancelOrders[hash] = struct{}{}
		}

		mergedOrderbookFills := newMergedDerivativeOrderbookFills(
			side.isBuy,
			side.orderbook.GetTransientOrderbookFills(),
			side.orderbook.GetRestingOrderbookFills(),
		)

		for {
			fill := mergedOrderbookFills.Next()

			if fill == nil {
				break
			}

			expansion := k.applyPositionDeltaAndGetDerivativeLimitOrderStateExpansion(
				ctx,
				market,
				funding,
				side.isBuy,
				fill.IsTransient,
				fill.Order,
				positionStates,
				fill.FillQuantity,
				clearingPrice,
				tradeRewardsMultiplierConfig,
				feeDiscountConfig,
				false,
			)

			expansionData.AddExpansion(side.isBuy, fill.IsTransient, expansion)

			_, isPartialCancel := expansionData.PartialCancelOrders[fill.Order.Hash()]
			if fill.IsTransient && expansion.LimitOrderFilledDelta.FillableQuantity().IsPositive() && !isPartialCancel {
				side.addNewRestingOrder(fill.Order)
			}
		}

		side.setRestingLimitOrderCancels(side.orderbook.GetRestingOrderbookCancels())
		side.setTransientLimitOrderCancels(side.orderbook.GetTransientOrderbookCancels())
		expansionData.OpenInterestDelta = expansionData.OpenInterestDelta.Add(side.orderbook.GetOpenInterestDelta())
	}

	return expansionData
}

//nolint:revive //ok
func (k DerivativeKeeper) GetClearingPriceFromMatching(
	lastBuyPrice,
	lastSellPrice,
	markPrice,
	_ math.LegacyDec,
	midMarketPrice *math.LegacyDec,
	buyOrderbook,
	sellOrderbook *limitOrderbook,
) math.LegacyDec {
	hasEmptyRestingOrderbookAndMarkPrice := midMarketPrice == nil && markPrice.IsNil()
	if hasEmptyRestingOrderbookAndMarkPrice {
		// rare edge case, no other choice than using matched orders
		return GetFullFallBackClearingPrice(lastBuyPrice, lastSellPrice)
	}

	if midMarketPrice == nil {
		return GetOracleFallBackClearingPrice(lastBuyPrice, lastSellPrice, markPrice)
	}

	return GetRegularClearingPrice(lastBuyPrice, lastSellPrice, markPrice, midMarketPrice)
}

func GetOracleFallBackClearingPrice(lastBuyPrice, lastSellPrice, markPrice math.LegacyDec) math.LegacyDec {
	if lastBuyPrice.LTE(markPrice) {
		return lastBuyPrice
	}

	if lastSellPrice.GTE(markPrice) {
		return lastSellPrice
	}

	return markPrice
}

func GetFullFallBackClearingPrice(lastBuyPrice, lastSellPrice math.LegacyDec) math.LegacyDec {
	// clearing price = (lastBuyPrice + lastSellPrice) / 2
	return lastBuyPrice.Add(lastSellPrice).Quo(math.LegacyNewDec(2))
}

func GetRegularClearingPrice(lastBuyPrice, lastSellPrice, markPrice math.LegacyDec, midMarketPrice *math.LegacyDec) math.LegacyDec {
	if lastBuyPrice.LTE(*midMarketPrice) {
		return lastBuyPrice
	}

	if lastSellPrice.GTE(*midMarketPrice) {
		return lastSellPrice
	}

	if !markPrice.IsNil() {
		return GetOracleFallBackClearingPrice(lastBuyPrice, lastSellPrice, markPrice)
	}

	return *midMarketPrice
}

//nolint:revive //ok
func (k DerivativeKeeper) PersistDerivativeMatchingExecution(
	ctx sdk.Context,
	batchDerivativeMatchingExecutionData []*v2.DerivativeBatchExecutionData,
	derivativeVwapData v2.DerivativeVwapInfo,
	tradingRewardPoints types.TradingRewardPoints,
) types.TradingRewardPoints {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	for batchIdx := range batchDerivativeMatchingExecutionData {
		execution := batchDerivativeMatchingExecutionData[batchIdx]
		if execution == nil {
			continue
		}

		marketID := execution.Market.MarketID()

		// market orders are matched in previous step but still existing transiently, cancelling would lead to double counting
		shouldCancelMarketOrders := false
		isMarketSolvent := k.EnsureMarketSolvency(ctx, execution.Market, execution.MarketBalanceDelta, shouldCancelMarketOrders)

		if !isMarketSolvent {
			continue
		}

		k.ApplyOpenInterestDeltaForMarket(
			ctx,
			marketID,
			execution.OpenInterestDelta,
		)

		if execution.VwapData != nil && !execution.VwapData.Price.IsZero() && !execution.VwapData.Quantity.IsZero() {
			vwapMarkPrice := execution.MarkPrice
			if vwapMarkPrice.IsNil() || vwapMarkPrice.IsNegative() {
				// hack to make this work with binary options
				vwapMarkPrice = math.LegacyZeroDec()
			}
			derivativeVwapData.ApplyVwap(marketID, &vwapMarkPrice, execution.VwapData, execution.Market.GetMarketType())
		}

		for _, subaccountID := range execution.DepositSubaccountIDs {
			k.subaccount.UpdateDepositWithDelta(ctx, subaccountID, execution.Market.GetQuoteDenom(), execution.DepositDeltas[subaccountID])
		}

		for idx, subaccountID := range execution.PositionSubaccountIDs {
			k.SavePosition(ctx, marketID, subaccountID, execution.Positions[idx])
		}

		k.UpdateDerivativeLimitOrdersFromFilledDeltas(ctx, marketID, true, execution.RestingLimitOrderFilledDeltas, nil)
		k.UpdateDerivativeLimitOrdersFromFilledDeltas(ctx, marketID, false, execution.TransientLimitOrderFilledDeltas, execution.PartialCancelOrders)
		k.UpdateDerivativeLimitOrdersFromFilledDeltas(ctx, marketID, true, execution.RestingLimitOrderCancelledDeltas, nil)
		k.UpdateDerivativeLimitOrdersFromFilledDeltas(ctx, marketID, false, execution.TransientLimitOrderCancelledDeltas, execution.PartialCancelOrders)

		if execution.NewOrdersEvent != nil {
			events.Emit(ctx, k.BaseKeeper, execution.NewOrdersEvent)
		}

		if execution.RestingLimitBuyOrderExecutionEvent != nil {
			events.Emit(ctx, k.BaseKeeper, execution.RestingLimitBuyOrderExecutionEvent)
		}

		if execution.RestingLimitSellOrderExecutionEvent != nil {
			events.Emit(ctx, k.BaseKeeper, execution.RestingLimitSellOrderExecutionEvent)
		}

		if execution.TransientLimitBuyOrderExecutionEvent != nil {
			events.Emit(ctx, k.BaseKeeper, execution.TransientLimitBuyOrderExecutionEvent)
		}

		if execution.TransientLimitSellOrderExecutionEvent != nil {
			events.Emit(ctx, k.BaseKeeper, execution.TransientLimitSellOrderExecutionEvent)
		}

		for idx := range execution.CancelLimitOrderEvents {
			events.Emit(ctx, k.BaseKeeper, execution.CancelLimitOrderEvents[idx])
		}

		if len(execution.TradingRewards) > 0 {
			tradingRewardPoints = types.MergeTradingRewardPoints(tradingRewardPoints, execution.TradingRewards)
		}
	}

	return tradingRewardPoints
}

func (k DerivativeKeeper) PersistDerivativeMarketOrderExecution(
	ctx sdk.Context,
	batchDerivativeExecutionData []*v2.DerivativeBatchExecutionData,
	derivativeVwapData v2.DerivativeVwapInfo,
	tradingRewardPoints types.TradingRewardPoints,
	modifiedPositionCache v2.ModifiedPositionCache,
) types.TradingRewardPoints {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	for _, derivativeExecutionData := range batchDerivativeExecutionData {
		tradingRewardPoints, _ = k.PersistSingleDerivativeMarketOrderExecution(
			ctx,
			derivativeExecutionData,
			derivativeVwapData,
			tradingRewardPoints,
			modifiedPositionCache,
			false,
		)
	}

	return tradingRewardPoints
}

// ExecuteDerivativeMarketOrderImmediately executes market order immediately (without waiting for end-blocker). Used for atomic orders execution by smart contract, and for liquidations
//
//nolint:revive //ok
func (k DerivativeKeeper) ExecuteDerivativeMarketOrderImmediately(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	markPrice math.LegacyDec,
	funding *v2.PerpetualMarketFunding,
	marketOrder *v2.DerivativeMarketOrder,
	positionStates map[common.Hash]*v2.PositionState,
	positionCache map[common.Hash]*v2.Position,
	isLiquidation bool,
) (*v2.DerivativeMarketOrderResults, bool, error) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketBuyOrders := make([]*v2.DerivativeMarketOrder, 0)
	marketSellOrders := make([]*v2.DerivativeMarketOrder, 0)

	if marketOrder.IsBuy() {
		marketBuyOrders = append(marketBuyOrders, marketOrder)
	} else {
		marketSellOrders = append(marketSellOrders, marketOrder)
	}

	marketID := market.MarketID()

	stakingInfo, feeDiscountConfig := k.feeDiscounts.GetFeeDiscountConfigAndStakingInfoForMarket(ctx, marketID)

	takerFeeRate := market.GetTakerFeeRate()
	if marketOrder.OrderType.IsAtomic() {
		multiplier := k.GetMarketAtomicExecutionFeeMultiplier(ctx, marketID, market.GetMarketType())
		takerFeeRate = takerFeeRate.Mul(multiplier)
	}

	currentOpenNotional := k.GetOpenNotionalForMarket(ctx, marketID, markPrice)
	openNotionalCap := market.GetOpenNotionalCap()

	derivativeMarketOrderExecution := k.GetDerivativeMarketOrderExecutionData(
		ctx,
		market,
		takerFeeRate,
		markPrice,
		funding,
		marketBuyOrders,
		marketSellOrders,
		positionStates,
		positionCache,
		feeDiscountConfig,
		isLiquidation,
		currentOpenNotional,
		openNotionalCap,
	)

	if isLiquidation {
		if marketOrder.IsBuy() && derivativeMarketOrderExecution.MarketBuyClearingQuantity.IsZero() {
			metrics.ReportFuncError(k.svcTags)
			return nil, true, types.ErrNoLiquidity
		}

		if !marketOrder.IsBuy() && derivativeMarketOrderExecution.MarketSellClearingQuantity.IsZero() {
			metrics.ReportFuncError(k.svcTags)
			return nil, true, types.ErrNoLiquidity
		}
	}

	batchExecutionData := derivativeMarketOrderExecution.GetMarketDerivativeBatchExecutionData(
		market,
		markPrice,
		funding,
		positionStates,
		isLiquidation,
	)

	modifiedPositionCache := v2.NewModifiedPositionCache()
	derivativeVwapData := v2.NewDerivativeVwapInfo()
	tradingRewards, isMarketSolvent := k.PersistSingleDerivativeMarketOrderExecution(
		ctx,
		batchExecutionData,
		derivativeVwapData,
		types.NewTradingRewardPoints(),
		modifiedPositionCache,
		isLiquidation,
	)

	sortedSubaccountIDs := modifiedPositionCache.GetSortedSubaccountIDsByMarket(marketID)
	k.AppendModifiedSubaccountsByMarket(ctx, marketID, sortedSubaccountIDs)

	k.trading.PersistTradingRewardPoints(ctx, tradingRewards)
	k.feeDiscounts.PersistFeeDiscountStakingInfoUpdates(ctx, stakingInfo)
	k.trading.PersistVwapInfo(ctx, nil, &derivativeVwapData)

	if market.GetIsPerpetual() {
		vwapInfo := derivativeVwapData.PerpetualVwapInfo[marketID]
		if vwapInfo != nil && vwapInfo.MarkPrice != nil && !vwapInfo.MarkPrice.IsZero() &&
			vwapInfo.VwapData != nil && !vwapInfo.VwapData.Quantity.IsZero() {
			k.AccumulateAtomicPerpetualVwap(ctx, marketID, *vwapInfo.MarkPrice, vwapInfo.VwapData.Price, vwapInfo.VwapData.Quantity)
		}
	}

	results := batchExecutionData.GetAtomicDerivativeMarketOrderResults()
	return results, isMarketSolvent, nil
}

func (k DerivativeKeeper) ExecuteDerivativeLimitOrderMatching(
	ctx sdk.Context,
	matchedMarketDirection *types.MatchedMarketDirection,
	stakingInfo *v2.FeeDiscountStakingInfo,
	modifiedPositionCache v2.ModifiedPositionCache,
) *v2.DerivativeBatchExecutionData {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := matchedMarketDirection.MarketId

	market, markPrice := k.GetDerivativeOrBinaryOptionsMarketWithMarkPrice(ctx, marketID, true)
	if market == nil {
		return nil
	}

	feeDiscountConfig := k.feeDiscounts.GetFeeDiscountConfigForMarket(ctx, marketID, stakingInfo)

	var funding *v2.PerpetualMarketFunding
	if market.GetIsPerpetual() {
		funding = k.GetPerpetualMarketFunding(ctx, marketID)
	}

	positionStates := v2.NewPositionStates()
	positionCache := make(map[common.Hash]*v2.Position)

	currentOpenNotional := k.GetOpenNotionalForMarket(ctx, marketID, markPrice)
	openNotionalCap := market.GetOpenNotionalCap()

	// Step 0: Obtain the limit buy and sell orders from the transient store for convenience

	filteredResults := k.getFilteredTransientOrdersAndOrdersToCancel(ctx, marketID, modifiedPositionCache)
	derivativeLimitOrderExecutionData := k.GetDerivativeMatchingExecutionData(
		ctx,
		market,
		markPrice,
		funding,
		filteredResults.transientLimitBuyOrders,
		filteredResults.transientLimitSellOrders,
		positionStates,
		positionCache,
		feeDiscountConfig,
		currentOpenNotional,
		openNotionalCap,
	)

	derivativeLimitOrderExecutionData.TransientLimitBuyOrderCancels = append(
		derivativeLimitOrderExecutionData.TransientLimitBuyOrderCancels,
		filteredResults.transientLimitBuyOrdersToCancel...,
	)

	derivativeLimitOrderExecutionData.TransientLimitSellOrderCancels = append(
		derivativeLimitOrderExecutionData.TransientLimitSellOrderCancels,
		filteredResults.transientLimitSellOrdersToCancel...,
	)

	batchExecutionData := derivativeLimitOrderExecutionData.GetLimitMatchingDerivativeBatchExecutionData(
		market,
		markPrice,
		funding,
		positionStates,
	)

	return batchExecutionData
}

func (k DerivativeKeeper) PersistPerpetualFundingInfo(ctx sdk.Context, perpetualVwapInfo v2.DerivativeVwapInfo) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketIDs := perpetualVwapInfo.GetSortedPerpetualMarketIDs()
	blockTime := ctx.BlockTime().Unix()

	for _, marketID := range marketIDs {
		markPrice := perpetualVwapInfo.PerpetualVwapInfo[marketID].MarkPrice
		if markPrice == nil || markPrice.IsNil() || markPrice.IsZero() {
			continue
		}

		syntheticVwapUnitDelta := perpetualVwapInfo.ComputeSyntheticVwapUnitDelta(marketID)

		funding := k.GetPerpetualMarketFunding(ctx, marketID)
		timeElapsed := math.LegacyNewDec(blockTime - funding.LastTimestamp)

		// newCumulativePrice = oldCumulativePrice + ∆t * price
		newCumulativePrice := funding.CumulativePrice.Add(timeElapsed.Mul(syntheticVwapUnitDelta))
		funding.CumulativePrice = newCumulativePrice
		funding.LastTimestamp = blockTime

		k.SetPerpetualMarketFunding(ctx, marketID, funding)
		events.Emit(ctx, k.BaseKeeper, &v2.EventPerpetualMarketFundingUpdate{
			MarketId:        marketID.Hex(),
			Funding:         *funding,
			IsHourlyFunding: false,
			FundingRate:     nil,
			MarkPrice:       nil,
		})
	}
}

//nolint:revive // ok
func (k DerivativeKeeper) getFilteredTransientOrdersAndOrdersToCancel(
	ctx sdk.Context,
	marketID common.Hash,
	modifiedPositionCache v2.ModifiedPositionCache,
) *filteredTransientOrderResults {
	// get orders while also obtaining the subaccountIDs corresponding to positions that have been modified by a market order earlier this block
	transientLimitBuyOrders, buyROTracker := k.GetAllTransientDerivativeLimitOrdersWithPotentiallyConflictingReduceOnlyOrders(ctx, marketID, true, modifiedPositionCache)
	transientLimitSellOrders, sellROTracker := k.GetAllTransientDerivativeLimitOrdersWithPotentiallyConflictingReduceOnlyOrders(ctx, marketID, false, modifiedPositionCache)

	transientOrderHashesToCancel := make(map[common.Hash]struct{})

	k.updateTransientOrderHashesToCancel(ctx, transientOrderHashesToCancel, marketID, true, buyROTracker, modifiedPositionCache)
	k.updateTransientOrderHashesToCancel(ctx, transientOrderHashesToCancel, marketID, false, sellROTracker, modifiedPositionCache)

	results := &filteredTransientOrderResults{
		transientLimitBuyOrders:          make([]*v2.DerivativeLimitOrder, 0, len(transientLimitBuyOrders)),
		transientLimitSellOrders:         make([]*v2.DerivativeLimitOrder, 0, len(transientLimitSellOrders)),
		transientLimitBuyOrdersToCancel:  make([]*v2.DerivativeLimitOrder, 0, len(transientOrderHashesToCancel)),
		transientLimitSellOrdersToCancel: make([]*v2.DerivativeLimitOrder, 0, len(transientOrderHashesToCancel)),
	}

	for _, order := range transientLimitBuyOrders {
		if _, found := transientOrderHashesToCancel[order.Hash()]; found {
			results.transientLimitBuyOrdersToCancel = append(results.transientLimitBuyOrdersToCancel, order)
		} else {
			results.transientLimitBuyOrders = append(results.transientLimitBuyOrders, order)
		}
	}

	for _, order := range transientLimitSellOrders {
		if _, found := transientOrderHashesToCancel[order.Hash()]; found {
			results.transientLimitSellOrdersToCancel = append(results.transientLimitSellOrdersToCancel, order)
		} else {
			results.transientLimitSellOrders = append(results.transientLimitSellOrders, order)
		}
	}

	return results
}

func (k DerivativeKeeper) updateTransientOrderHashesToCancel(
	ctx sdk.Context,
	transientOrderHashesToCancel map[common.Hash]struct{},
	marketID common.Hash,
	isBuy bool,
	roTracker v2.ReduceOnlyOrdersTracker,
	modifiedPositionCache v2.ModifiedPositionCache,
) {
	for _, subaccountID := range roTracker.GetSortedSubaccountIDs() {
		position := modifiedPositionCache.GetPosition(marketID, subaccountID)
		if position == nil {
			position = k.GetPosition(ctx, marketID, subaccountID)
		}

		isNotValidPositionToReduce := position == nil || position.Quantity.IsZero() || position.IsLong == isBuy
		if isNotValidPositionToReduce {
			addAllTransientRoOrdersForSubaccountToCancellation(transientOrderHashesToCancel, roTracker, subaccountID)
			continue
		}

		metadata := k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, isBuy)

		// For an opposing position, if position.quantity < AggregateReduceOnlyQuantity + AggregateVanillaQuantity
		// the new order might invalidate some existing reduce-only orders or itself be invalid (if it's reduce-only).
		cumulativeOrderSideQuantity := metadata.AggregateReduceOnlyQuantity.Add(metadata.AggregateVanillaQuantity)

		roQuantityToCancel := cumulativeOrderSideQuantity.Sub(position.Quantity)

		if !roQuantityToCancel.IsPositive() {
			continue
		}

		// simple, but overly restrictive implementation for now, just cancel all transient RO orders by quantity
		// more permissive will require more complex logic incl cancelling resting limit orders
		for i := len(roTracker[subaccountID]) - 1; i >= 0; i-- {
			roOrderToCancel := roTracker[subaccountID][i]
			transientOrderHashesToCancel[common.BytesToHash(roOrderToCancel.OrderHash)] = struct{}{}
			roQuantityToCancel = roQuantityToCancel.Sub(roOrderToCancel.GetQuantity())

			if roQuantityToCancel.LTE(math.LegacyZeroDec()) {
				break
			}
		}
	}
}

func (k DerivativeKeeper) GetOpenNotionalForMarket(ctx sdk.Context, marketID common.Hash, markPrice math.LegacyDec) math.LegacyDec {
	openInterest := k.GetOpenInterestForMarket(ctx, marketID)
	if markPrice.IsNil() {
		return math.LegacyZeroDec()
	}

	openNotional := openInterest.Mul(markPrice)
	return openNotional
}

func addAllTransientRoOrdersForSubaccountToCancellation(
	transientOrderHashesToCancel map[common.Hash]struct{},
	roTracker v2.ReduceOnlyOrdersTracker,
	subaccountID common.Hash,
) {
	for _, order := range roTracker[subaccountID] {
		transientOrderHashesToCancel[order.Hash()] = struct{}{}
	}
}

type filteredTransientOrderResults struct {
	transientLimitBuyOrders          []*v2.DerivativeLimitOrder
	transientLimitSellOrders         []*v2.DerivativeLimitOrder
	transientLimitBuyOrdersToCancel  []*v2.DerivativeLimitOrder
	transientLimitSellOrdersToCancel []*v2.DerivativeLimitOrder
}
