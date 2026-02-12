package derivative

import (
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

type OrderBookI interface {
	GetAddedOpenNotional() math.LegacyDec
}

type marketExecutionOrderbook struct {
	isMarketBuy     bool
	limitOrderbook  *limitOrderbook
	marketOrderbook *marketOrderbook
}

func newMarketExecutionOrderbook(
	isMarketBuy bool,
	limitOrderbook *limitOrderbook,
	marketOrderbook *marketOrderbook,
) *marketExecutionOrderbook {
	return &marketExecutionOrderbook{
		isMarketBuy:     isMarketBuy,
		limitOrderbook:  limitOrderbook,
		marketOrderbook: marketOrderbook,
	}
}

func newMarketExecutionOrderbooks(
	limitBuyOrderbook, limitSellOrderbook *limitOrderbook,
	marketBuyOrderbook, marketSellOrderbook *marketOrderbook,
) []*marketExecutionOrderbook {
	return []*marketExecutionOrderbook{
		newMarketExecutionOrderbook(false, limitBuyOrderbook, marketSellOrderbook),
		newMarketExecutionOrderbook(true, limitSellOrderbook, marketBuyOrderbook),
	}
}

type marketOrderbook struct {
	k              DerivativeKeeper
	isBuy          bool
	isLiquidation  bool
	notional       math.LegacyDec
	totalQuantity  math.LegacyDec
	orders         []*v2.DerivativeMarketOrder
	fillQuantities []math.LegacyDec
	orderIdx       int
	market         v2.DerivativeMarketI
	markPrice      math.LegacyDec
	marketID       common.Hash
	funding        *v2.PerpetualMarketFunding

	positionStates          map[common.Hash]*v2.PositionState
	positionCache           map[common.Hash]*v2.Position
	addedOpenNotional       math.LegacyDec
	cachedAddedOpenNotional math.LegacyDec
	currentOpenNotional     math.LegacyDec
	openInterestDelta       math.LegacyDec
	openNotionalCap         v2.OpenNotionalCap

	oppositeSideDerivativeOrderbook OrderBookI
}

//nolint:revive //ok
func newDerivativeMarketOrderbook(
	k DerivativeKeeper,
	isBuy bool,
	isLiquidation bool,
	derivativeMarketOrders []*v2.DerivativeMarketOrder,
	market v2.DerivativeMarketI,
	markPrice math.LegacyDec,
	funding *v2.PerpetualMarketFunding,
	currentOpenNotional math.LegacyDec,
	openNotionalCap v2.OpenNotionalCap,
	positionStates map[common.Hash]*v2.PositionState,
	positionCache map[common.Hash]*v2.Position,
) *marketOrderbook {
	if len(derivativeMarketOrders) == 0 {
		return nil
	}

	fillQuantities := make([]math.LegacyDec, len(derivativeMarketOrders))
	for idx := range derivativeMarketOrders {
		fillQuantities[idx] = math.LegacyZeroDec()
	}

	if markPrice.IsNil() {
		// allow all matching by using a mark price of zero leading to zero open notional
		markPrice = math.LegacyZeroDec()
	}

	orderGroup := marketOrderbook{
		k:             k,
		isBuy:         isBuy,
		isLiquidation: isLiquidation,
		notional:      math.LegacyZeroDec(),
		totalQuantity: math.LegacyZeroDec(),

		orders:         derivativeMarketOrders,
		fillQuantities: fillQuantities,
		orderIdx:       0,

		market:         market,
		markPrice:      markPrice,
		marketID:       market.MarketID(),
		funding:        funding,
		positionStates: positionStates,
		positionCache:  positionCache,

		addedOpenNotional:       math.LegacyZeroDec(),
		cachedAddedOpenNotional: math.LegacyZeroDec(),
		currentOpenNotional:     currentOpenNotional,
		openNotionalCap:         openNotionalCap,
		openInterestDelta:       math.LegacyZeroDec(),
	}
	return &orderGroup
}

func (b *marketOrderbook) GetNotional() math.LegacyDec { return b.notional }

func (b *marketOrderbook) GetTotalQuantityFilled() math.LegacyDec { return b.totalQuantity }

func (b *marketOrderbook) GetOrderbookFillQuantities() []math.LegacyDec {
	return b.fillQuantities
}

func (b *marketOrderbook) Peek(ctx sdk.Context) *v2.PriceLevel {
	// finished iterating
	if b.orderIdx == len(b.orders) {
		return nil
	}

	order := b.orders[b.orderIdx]

	// Process order and check if it should be skipped
	if b.shouldSkipOrder(ctx, order) {
		b.orderIdx++
		return b.Peek(ctx)
	}

	remainingFillableOrderQuantity := b.getCurrOrderFillableQuantity()

	// fully filled
	if remainingFillableOrderQuantity.IsZero() {
		b.orderIdx++
		return b.Peek(ctx)
	}

	return &v2.PriceLevel{
		Price:    order.OrderInfo.Price,
		Quantity: remainingFillableOrderQuantity,
	}
}

func (b *marketOrderbook) shouldSkipOrder(ctx sdk.Context, order *v2.DerivativeMarketOrder) bool {
	b.initializedPositionState(ctx, order.SubaccountID())

	if b.shouldSkipForClosingPosition(ctx, order) {
		return true
	}
	if b.shouldSkipForMarginRequirement(order) {
		return true
	}

	result := b.shouldSkipForOpenNotionalCapAndUpdateState(order)

	return result
}

func (b *marketOrderbook) shouldSkipForClosingPosition(ctx sdk.Context, order *v2.DerivativeMarketOrder) bool {
	subaccountID := order.SubaccountID()
	position := b.getInitializedPositionState(ctx, subaccountID)

	if b.isLiquidation {
		return false
	}

	// defensive programming check
	if order.IsReduceOnly() && !isValidReduceOnlyOrder(position, order.IsBuy(), b.getCurrOrderFillableQuantity()) {
		return true
	}

	isClosingPosition := position != nil && order.IsBuy() != position.IsLong && position.Quantity.IsPositive()
	if !isClosingPosition {
		return false
	}

	fillableQuantity := b.getCurrOrderFillableQuantity()
	closingQuantity := math.LegacyMinDec(fillableQuantity, position.Quantity)
	closeExecutionMargin := order.Margin.Mul(closingQuantity).Quo(order.OrderInfo.Quantity)

	takerFeeRate := b.getTradeFeeRate(ctx, order)
	err := position.CheckValidPositionToReduce(
		b.market.GetMarketType(),
		order.OrderInfo.Price,
		order.IsBuy(),
		takerFeeRate,
		b.funding,
		closeExecutionMargin,
	)

	return err != nil
}

func (b *marketOrderbook) getTradeFeeRate(ctx sdk.Context, order *v2.DerivativeMarketOrder) math.LegacyDec {
	takerFeeRate := b.market.GetTakerFeeRate()
	if order.OrderType.IsAtomic() {
		multiplier := b.k.GetMarketAtomicExecutionFeeMultiplier(ctx, b.marketID, b.market.GetMarketType())
		takerFeeRate = takerFeeRate.Mul(multiplier)
	}

	return takerFeeRate
}

func (b *marketOrderbook) shouldSkipForMarginRequirement(order *v2.DerivativeMarketOrder) bool {
	if !order.IsVanilla() || b.market.GetMarketType() == types.MarketType_BinaryOption {
		return false
	}

	err := order.CheckInitialMarginRequirementMarkPriceThreshold(b.market.GetInitialMarginRatio(), b.markPrice)
	return err != nil
}

func (b *marketOrderbook) incrementCurrFillQuantities(incrQuantity math.LegacyDec) {
	b.fillQuantities[b.orderIdx] = b.fillQuantities[b.orderIdx].Add(incrQuantity)
}

func (b *marketOrderbook) getCurrOrderFillableQuantity() math.LegacyDec {
	return b.orders[b.orderIdx].OrderInfo.Quantity.Sub(b.fillQuantities[b.orderIdx])
}

func (b *marketOrderbook) IsPerpetual() bool {
	return b.funding != nil
}

func (b *marketOrderbook) getInitializedPositionState(
	ctx sdk.Context,
	subaccountID common.Hash,
) *v2.Position {
	if b.positionStates[subaccountID] == nil {
		position := b.k.GetPosition(ctx, b.marketID, subaccountID)

		if position == nil {
			var cumulativeFundingEntry math.LegacyDec

			if b.IsPerpetual() {
				cumulativeFundingEntry = b.funding.CumulativeFunding
			}

			position = v2.NewPosition(b.isBuy, cumulativeFundingEntry)
			positionState := &v2.PositionState{
				Position: position,
			}
			b.positionStates[subaccountID] = positionState
		}

		b.positionStates[subaccountID] = v2.ApplyFundingAndGetUpdatedPositionState(position, b.funding)
	}

	if b.positionCache[subaccountID] == nil {
		b.positionCache[subaccountID] = b.positionStates[subaccountID].Position.Copy()
	}

	return b.positionCache[subaccountID]
}

func (b *marketOrderbook) doesBreachOpenNotionalCapForMarketOrderbook(currOrder *v2.DerivativeMarketOrder) bool {
	doesBreachCap, notionalDelta := DoesBreachOpenNotionalCap(
		currOrder.OrderType,
		currOrder.OrderInfo.Quantity,
		b.markPrice,
		b.getTotalOpenNotional(),
		getSignedPositionQuantity(b.positionCache[currOrder.SubaccountID()]),
		b.openNotionalCap,
	)

	if !doesBreachCap {
		// cache notional delta for opposite side
		b.cachedAddedOpenNotional = notionalDelta
	} else {
		b.cachedAddedOpenNotional = math.LegacyZeroDec()
	}

	return doesBreachCap
}

func isValidReduceOnlyOrder(
	position *v2.Position,
	//revive:disable:flag-parameter
	isBuy bool,
	remainingFillable math.LegacyDec,
) bool {
	if position == nil {
		return false
	}

	if isBuy == position.IsLong {
		return false
	}

	if remainingFillable.GT(position.Quantity) {
		return false
	}

	return true
}

func (b *marketOrderbook) updateNotionalCapValuesAfterFill(ctx sdk.Context, currOrder *v2.DerivativeMarketOrder, fillQuantity math.LegacyDec) {
	notionalDelta, quantityDelta, _ := GetValuesForNotionalCapChecks(
		currOrder.OrderType,
		fillQuantity,
		b.markPrice,
		getSignedPositionQuantity(b.positionCache[currOrder.SubaccountID()]),
	)

	b.openInterestDelta = b.openInterestDelta.Add(quantityDelta)
	b.addedOpenNotional = b.addedOpenNotional.Add(notionalDelta)

	if pos := b.positionCache[currOrder.SubaccountID()]; pos != nil {
		executionMargin := currOrder.Margin.Mul(fillQuantity).Quo(currOrder.OrderInfo.Quantity)
		delta := &v2.PositionDelta{
			IsLong:            currOrder.IsBuy(),
			ExecutionQuantity: fillQuantity,
			ExecutionMargin:   executionMargin,
			ExecutionPrice:    currOrder.OrderInfo.Price, // using order price as worst case since FBA clearing price is unknown here
		}
		pos.ApplyPositionDelta(delta, b.getTradeFeeRate(ctx, currOrder))
	}

	b.cachedAddedOpenNotional = math.LegacyZeroDec()
}

func (b *marketOrderbook) shouldSkipForOpenNotionalCapAndUpdateState(
	currOrder *v2.DerivativeMarketOrder,
) bool {
	return b.doesBreachOpenNotionalCapForMarketOrderbook(currOrder)
}

func (b *marketOrderbook) SetOppositeSideDerivativeOrderbook(opposite OrderBookI) {
	b.oppositeSideDerivativeOrderbook = opposite
}

func (b *marketOrderbook) GetAddedOpenNotional() math.LegacyDec {
	return b.addedOpenNotional.Add(b.cachedAddedOpenNotional)
}

func (b *marketOrderbook) GetOpenInterestDelta() math.LegacyDec {
	return b.openInterestDelta
}

func (b *marketOrderbook) getTotalOpenNotional() math.LegacyDec {
	return b.currentOpenNotional.Add(b.addedOpenNotional).Add(b.oppositeSideDerivativeOrderbook.GetAddedOpenNotional())
}

func (b *marketOrderbook) initializedPositionState(
	ctx sdk.Context,
	subaccountID common.Hash,
) {
	if b.positionStates[subaccountID] != nil {
		return
	}

	position := b.k.GetPosition(ctx, b.marketID, subaccountID)

	if position == nil {
		var cumulativeFundingEntry math.LegacyDec

		if b.IsPerpetual() {
			cumulativeFundingEntry = b.funding.CumulativeFunding
		}

		position = v2.NewPosition(b.isBuy, cumulativeFundingEntry)
		positionState := &v2.PositionState{
			Position: position,
		}
		b.positionStates[subaccountID] = positionState
	}

	positionStates := v2.ApplyFundingAndGetUpdatedPositionState(position, b.funding)
	b.positionStates[subaccountID] = positionStates

	if b.positionCache[subaccountID] == nil {
		b.positionCache[subaccountID] = b.positionStates[subaccountID].Position.Copy()
	}
}

func (b *marketOrderbook) Fill(ctx sdk.Context, fillQuantity math.LegacyDec) {
	order := b.orders[b.orderIdx]

	b.incrementCurrFillQuantities(fillQuantity)
	b.notional = b.notional.Add(fillQuantity.Mul(order.OrderInfo.Price))
	b.totalQuantity = b.totalQuantity.Add(fillQuantity)

	b.updateNotionalCapValuesAfterFill(ctx, order, fillQuantity)
}

type limitOrderbook struct {
	k DerivativeKeeper

	isBuy    bool
	notional math.LegacyDec

	totalQuantity           math.LegacyDec
	transientOrderbookFills *orderbookFills

	transientOrderIdx     int
	restingOrderbookFills *orderbookFills

	restingOrderIterator    storetypes.Iterator
	orderCancelHashes       map[common.Hash]struct{}
	partialCancelOrders     map[common.Hash]struct{}
	restingOrdersToCancel   []*v2.DerivativeLimitOrder
	transientOrdersToCancel []*v2.DerivativeLimitOrder

	// pointers to the current OrderbookFills
	currState                       *orderbookFills
	market                          v2.DerivativeMarketI
	markPrice                       math.LegacyDec
	marketID                        common.Hash
	funding                         *v2.PerpetualMarketFunding
	positionStates                  map[common.Hash]*v2.PositionState
	positionCache                   map[common.Hash]*v2.Position
	addedOpenNotional               math.LegacyDec
	cachedAddedOpenNotional         math.LegacyDec
	currentOpenNotional             math.LegacyDec
	openInterestDelta               math.LegacyDec
	openNotionalCap                 v2.OpenNotionalCap
	oppositeSideDerivativeOrderbook OrderBookI
}

//nolint:revive //ok
func newLimitOrderbook(
	k DerivativeKeeper,
	ctx sdk.Context,
	isBuy bool,
	transientOrders []*v2.DerivativeLimitOrder,
	market v2.DerivativeMarketI,
	markPrice math.LegacyDec,
	funding *v2.PerpetualMarketFunding,
	currentOpenNotional math.LegacyDec,
	openNotionalCap v2.OpenNotionalCap,
	positionStates map[common.Hash]*v2.PositionState,
	positionCache map[common.Hash]*v2.Position,
) *limitOrderbook {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	iterator := k.DerivativeLimitOrdersIterator(ctx, market.MarketID(), isBuy)
	// return early if there are no limit orders in this direction

	if len(transientOrders) == 0 && !iterator.Valid() {
		iterator.Close()
		return nil
	}

	var transientOrderbookState *orderbookFills
	if len(transientOrders) != 0 {
		transientOrderFillQuantities := make([]math.LegacyDec, len(transientOrders))
		// pre-initialize to zero dec for convenience
		for idx := range transientOrderFillQuantities {
			transientOrderFillQuantities[idx] = math.LegacyZeroDec()
		}
		transientOrderbookState = &orderbookFills{
			Orders:         transientOrders,
			FillQuantities: transientOrderFillQuantities,
		}
	}

	var restingOrderbookState *orderbookFills

	if iterator.Valid() {
		restingOrderbookState = &orderbookFills{
			Orders:         make([]*v2.DerivativeLimitOrder, 0),
			FillQuantities: make([]math.LegacyDec, 0),
		}
	}

	if markPrice.IsNil() {
		// allow all matching by using a mark price of zero leading to zero open notional
		markPrice = math.LegacyZeroDec()
	}

	orderbook := limitOrderbook{
		k:             k,
		isBuy:         isBuy,
		notional:      math.LegacyZeroDec(),
		totalQuantity: math.LegacyZeroDec(),

		transientOrderbookFills: transientOrderbookState,
		transientOrderIdx:       0,
		restingOrderbookFills:   restingOrderbookState,
		restingOrderIterator:    iterator,

		orderCancelHashes:       make(map[common.Hash]struct{}),
		restingOrdersToCancel:   make([]*v2.DerivativeLimitOrder, 0),
		transientOrdersToCancel: make([]*v2.DerivativeLimitOrder, 0),
		partialCancelOrders:     make(map[common.Hash]struct{}),

		currState:      nil,
		market:         market,
		markPrice:      markPrice,
		marketID:       market.MarketID(),
		funding:        funding,
		positionStates: positionStates,
		positionCache:  positionCache,

		addedOpenNotional:       math.LegacyZeroDec(),
		cachedAddedOpenNotional: math.LegacyZeroDec(),
		currentOpenNotional:     currentOpenNotional,
		openNotionalCap:         openNotionalCap,
		openInterestDelta:       math.LegacyZeroDec(),

		oppositeSideDerivativeOrderbook: nil,
	}

	return &orderbook
}

func (b *limitOrderbook) GetNotional() math.LegacyDec { return b.notional }

func (b *limitOrderbook) GetTotalQuantityFilled() math.LegacyDec { return b.totalQuantity }

func (b *limitOrderbook) GetTransientOrderbookFills() *orderbookFills {
	if len(b.transientOrdersToCancel) == 0 {
		return b.transientOrderbookFills
	}

	capacity := len(b.transientOrderbookFills.Orders) - len(b.transientOrdersToCancel)
	filteredFills := &orderbookFills{
		Orders:         make([]*v2.DerivativeLimitOrder, 0, capacity),
		FillQuantities: make([]math.LegacyDec, 0, capacity),
	}
	for idx := range b.transientOrderbookFills.Orders {
		order := b.transientOrderbookFills.Orders[idx]
		if _, found := b.orderCancelHashes[order.Hash()]; !found {
			filteredFills.Orders = append(filteredFills.Orders, order)
			filteredFills.FillQuantities = append(filteredFills.FillQuantities, b.transientOrderbookFills.FillQuantities[idx])
		}
	}
	return filteredFills
}

func (b *limitOrderbook) GetRestingOrderbookFills() *orderbookFills {
	if len(b.restingOrdersToCancel) == 0 {
		return b.restingOrderbookFills
	}

	capacity := len(b.restingOrderbookFills.Orders) - len(b.restingOrdersToCancel)

	filteredFills := &orderbookFills{
		Orders:         make([]*v2.DerivativeLimitOrder, 0, capacity),
		FillQuantities: make([]math.LegacyDec, 0, capacity),
	}

	for idx := range b.restingOrderbookFills.Orders {
		order := b.restingOrderbookFills.Orders[idx]
		if _, found := b.orderCancelHashes[order.Hash()]; !found {
			filteredFills.Orders = append(filteredFills.Orders, order)
			filteredFills.FillQuantities = append(filteredFills.FillQuantities, b.restingOrderbookFills.FillQuantities[idx])
		}
	}
	return filteredFills
}

func (b *limitOrderbook) GetRestingOrderbookCancels() []*v2.DerivativeLimitOrder {
	return b.restingOrdersToCancel
}

func (b *limitOrderbook) GetTransientOrderbookCancels() []*v2.DerivativeLimitOrder {
	return b.transientOrdersToCancel
}

func (b *limitOrderbook) GetPartialCancelOrders() map[common.Hash]struct{} {
	return b.partialCancelOrders
}

func (b *limitOrderbook) IsPerpetual() bool {
	return b.funding != nil
}

func (b *limitOrderbook) checkAndInitializePosition(
	ctx sdk.Context,
	subaccountID common.Hash,
) *v2.Position {
	if b.positionStates[subaccountID] == nil {
		position := b.k.GetPosition(ctx, b.marketID, subaccountID)

		if position == nil {
			var cumulativeFundingEntry math.LegacyDec

			if b.IsPerpetual() {
				cumulativeFundingEntry = b.funding.CumulativeFunding
			}

			position = v2.NewPosition(b.isBuy, cumulativeFundingEntry)
			positionState := &v2.PositionState{
				Position: position,
			}
			b.positionStates[subaccountID] = positionState
		}

		b.positionStates[subaccountID] = v2.ApplyFundingAndGetUpdatedPositionState(position, b.funding)
	}

	if b.positionCache[subaccountID] == nil {
		b.positionCache[subaccountID] = b.positionStates[subaccountID].Position.Copy()
	}

	return b.positionCache[subaccountID]
}

func (b *limitOrderbook) getCurrOrderAndInitializeCurrState() *v2.DerivativeLimitOrder {
	restingOrder := b.getRestingOrder()
	transientOrder := b.getTransientOrder()

	var currOrder *v2.DerivativeLimitOrder

	// if iterating over both orderbooks, find the orderbook with the best priced order to use next
	switch {
	case restingOrder != nil && transientOrder != nil:
		// buy orders with higher prices or sell orders with lower prices are prioritized
		if (b.isBuy && restingOrder.OrderInfo.Price.LT(transientOrder.OrderInfo.Price)) ||
			(!b.isBuy && restingOrder.OrderInfo.Price.GT(transientOrder.OrderInfo.Price)) {
			b.currState = b.transientOrderbookFills
			currOrder = transientOrder
		} else {
			b.currState = b.restingOrderbookFills
			currOrder = restingOrder
		}
	case restingOrder != nil:
		b.currState = b.restingOrderbookFills
		currOrder = restingOrder
	case transientOrder != nil:
		b.currState = b.transientOrderbookFills
		currOrder = transientOrder
	default:
		b.currState = nil
		return nil
	}

	return currOrder
}

func (b *limitOrderbook) addInvalidOrderToCancelsAndAdvanceToNextOrder(ctx sdk.Context, currOrder *v2.DerivativeLimitOrder) {
	// Check if this order already has fills
	// This can happen when an order passes validation initially, receives fills during matching,
	// but then fails validation on a subsequent Peek() due to changed position state.
	var existingFill math.LegacyDec
	switch b.currState {
	case b.transientOrderbookFills:
		idx := b.transientOrderIdx
		if idx < len(b.transientOrderbookFills.FillQuantities) {
			existingFill = b.transientOrderbookFills.FillQuantities[idx]
		}
	case b.restingOrderbookFills:
		idx := len(b.restingOrderbookFills.Orders) - 1
		if idx >= 0 && idx < len(b.restingOrderbookFills.FillQuantities) {
			existingFill = b.restingOrderbookFills.FillQuantities[idx]
		}
	}

	if existingFill.IsPositive() {
		// Order has fills - mark for partial cancellation
		// DO NOT add to orderCancelHashes (so fills are preserved in Get*OrderbookFills)
		b.partialCancelOrders[currOrder.Hash()] = struct{}{}
	} else {
		// No fills - add to orderCancelHashes to filter out from fills
		b.orderCancelHashes[currOrder.Hash()] = struct{}{}
	}

	// Add to cancel lists for refund processing in both cases
	if b.isCurrOrderResting() {
		b.restingOrdersToCancel = append(b.restingOrdersToCancel, currOrder)
	} else {
		b.transientOrdersToCancel = append(b.transientOrdersToCancel, currOrder)
		b.transientOrderIdx++
	}

	b.currState = nil
	b.advanceNewOrder(ctx)
}

func (b *limitOrderbook) advanceNewOrder(ctx sdk.Context) {
	currOrder := b.getCurrOrderAndInitializeCurrState()

	if b.currState == nil {
		return
	}

	subaccountID := currOrder.SubaccountID()
	position := b.checkAndInitializePosition(ctx, subaccountID)

	// defensive programming check
	if currOrder.IsReduceOnly() && !isValidReduceOnlyOrder(position, currOrder.IsBuy(), b.getCurrFillableQuantity()) {
		b.addInvalidOrderToCancelsAndAdvanceToNextOrder(ctx, currOrder)
		return
	}

	isClosingPosition := position != nil && currOrder.IsBuy() != position.IsLong && position.Quantity.IsPositive()

	if isClosingPosition {
		tradeFeeRate := b.getCurrOrderTradeFeeRate()
		remainingFillable := b.getCurrFillableQuantity()
		closingQuantity := math.LegacyMinDec(remainingFillable, position.Quantity)
		closeExecutionMargin := currOrder.Margin.Mul(closingQuantity).Quo(currOrder.OrderInfo.Quantity)

		if err := position.CheckValidPositionToReduce(
			b.market.GetMarketType(),
			// NOTE: must be order price, not clearing price !!!
			// due to security reasons related to margin adjustment case after increased trading fee
			// see `adjustPositionMarginIfNecessary` for more details
			currOrder.OrderInfo.Price,
			b.isBuy,
			tradeFeeRate,
			b.funding,
			closeExecutionMargin,
		); err != nil {
			b.addInvalidOrderToCancelsAndAdvanceToNextOrder(ctx, currOrder)
			return
		}
	}

	if currOrder.IsVanilla() && b.market.GetMarketType() != types.MarketType_BinaryOption {
		err := currOrder.CheckInitialMarginRequirementMarkPriceThreshold(b.market.GetInitialMarginRatio(), b.markPrice)

		if err != nil {
			b.addInvalidOrderToCancelsAndAdvanceToNextOrder(ctx, currOrder)
		}
	}

	if b.doesBreachOpenNotionalCapForLimitOrderbook(currOrder) {
		b.addInvalidOrderToCancelsAndAdvanceToNextOrder(ctx, currOrder)
		return
	}
}

func getSignedPositionQuantity(position *v2.Position) math.LegacyDec {
	if position == nil {
		return math.LegacyZeroDec()
	}

	if !position.IsLong {
		return position.Quantity.Neg()
	}

	return position.Quantity
}

func (b *limitOrderbook) doesBreachOpenNotionalCapForLimitOrderbook(currOrder *v2.DerivativeLimitOrder) bool {
	doesBreachCap, notionalDelta := DoesBreachOpenNotionalCap(
		currOrder.OrderType,
		currOrder.OrderInfo.Quantity,
		b.markPrice,
		b.getTotalOpenNotional(),
		getSignedPositionQuantity(b.positionCache[currOrder.SubaccountID()]),
		b.openNotionalCap,
	)

	if !doesBreachCap {
		// cache notional delta for opposite side
		b.cachedAddedOpenNotional = notionalDelta
	} else {
		b.cachedAddedOpenNotional = math.LegacyZeroDec()
	}

	return doesBreachCap
}

func (b *limitOrderbook) Peek(ctx sdk.Context) *v2.PriceLevel {
	// Sets currState to the orderbook (transientOrderbook or restingOrderbook) with the next best priced order
	b.advanceNewOrder(ctx)

	if b.currState == nil {
		return nil
	}

	priceLevel := &v2.PriceLevel{
		Price:    b.getCurrPrice(),
		Quantity: b.getCurrFillableQuantity(),
	}

	return priceLevel
}

// NOTE: b.currState must NOT be nil!
func (b *limitOrderbook) getCurrIndex() int {
	var idx int
	// obtain index according to the currState
	if b.currState == b.restingOrderbookFills {
		idx = len(b.restingOrderbookFills.Orders) - 1
	} else {
		idx = b.transientOrderIdx
	}
	return idx
}

func (b *limitOrderbook) Fill(fillQuantity math.LegacyDec) {
	idx := b.getCurrIndex()

	orderCumulativeFillQuantity := b.currState.FillQuantities[idx].Add(fillQuantity)

	b.currState.FillQuantities[idx] = orderCumulativeFillQuantity

	order := b.currState.Orders[idx]

	fillNotional := fillQuantity.Mul(order.OrderInfo.Price)

	b.notional = b.notional.Add(fillNotional)
	b.totalQuantity = b.totalQuantity.Add(fillQuantity)

	b.updateNotionalCapValuesAfterFill(order, fillQuantity)

	// if currState is fully filled, set to nil
	if orderCumulativeFillQuantity.Equal(b.currState.Orders[idx].Fillable) {
		b.currState = nil
	}
}

func (b *limitOrderbook) Close() {
	b.restingOrderIterator.Close()
}

func (b *limitOrderbook) isCurrOrderResting() bool {
	return b.currState == b.restingOrderbookFills
}

func (b *limitOrderbook) isCurrRestingOrderCancelled() bool {
	idx := len(b.restingOrdersToCancel) - 1
	if idx == -1 {
		return false
	}

	return b.restingOrderbookFills.Orders[len(b.restingOrderbookFills.Orders)-1] == b.restingOrdersToCancel[idx]
}

func (b *limitOrderbook) getRestingFillableQuantity() math.LegacyDec {
	idx := len(b.restingOrderbookFills.Orders) - 1
	if idx == -1 || b.isCurrRestingOrderCancelled() {
		return math.LegacyZeroDec()
	}

	return b.restingOrderbookFills.Orders[idx].Fillable.Sub(b.restingOrderbookFills.FillQuantities[idx])
}

func (b *limitOrderbook) getTransientFillableQuantity() math.LegacyDec {
	idx := b.transientOrderIdx
	return b.transientOrderbookFills.Orders[idx].Fillable.Sub(b.transientOrderbookFills.FillQuantities[idx])
}

func (b *limitOrderbook) getCurrOrderTradeFeeRate() (tradeFeeRate math.LegacyDec) {
	if b.isCurrOrderResting() {
		tradeFeeRate = b.market.GetMakerFeeRate()
	} else {
		tradeFeeRate = b.market.GetTakerFeeRate()
	}

	return tradeFeeRate
}

func (b *limitOrderbook) getCurrFillableQuantity() math.LegacyDec {
	idx := b.getCurrIndex()
	return b.currState.Orders[idx].Fillable.Sub(b.currState.FillQuantities[idx])
}

func (b *limitOrderbook) getCurrPrice() math.LegacyDec {
	idx := b.getCurrIndex()
	return b.currState.Orders[idx].OrderInfo.Price
}

func (b *limitOrderbook) getRestingOrder() *v2.DerivativeLimitOrder {
	// if no more orders to iterate + fully filled, return nil
	if !b.restingOrderIterator.Valid() && (b.restingOrderbookFills == nil || b.getRestingFillableQuantity().IsZero()) {
		return nil
	}

	idx := len(b.restingOrderbookFills.Orders) - 1

	// if the current resting order state is fully filled, advance the iterator
	if b.getRestingFillableQuantity().IsZero() {
		order := b.k.UnmarshalDerivativeLimitOrder(b.restingOrderIterator.Value())

		b.restingOrderIterator.Next()
		b.restingOrderbookFills.Orders = append(b.restingOrderbookFills.Orders, &order)
		b.restingOrderbookFills.FillQuantities = append(b.restingOrderbookFills.FillQuantities, math.LegacyZeroDec())

		return &order
	}
	return b.restingOrderbookFills.Orders[idx]
}

func (b *limitOrderbook) getTransientOrder() *v2.DerivativeLimitOrder {
	if b.transientOrderbookFills == nil {
		return nil
	}
	if len(b.transientOrderbookFills.Orders) == b.transientOrderIdx {
		return nil
	}
	if b.getTransientFillableQuantity().IsZero() {
		b.transientOrderIdx++
		// apply recursion to obtain the new current New Order
		return b.getTransientOrder()
	}

	return b.transientOrderbookFills.Orders[b.transientOrderIdx]
}

func (b *limitOrderbook) SetOppositeSideDerivativeOrderbook(opposite OrderBookI) {
	b.oppositeSideDerivativeOrderbook = opposite
}

func (b *limitOrderbook) GetAddedOpenNotional() math.LegacyDec {
	return b.addedOpenNotional.Add(b.cachedAddedOpenNotional)
}

func (b *limitOrderbook) GetOpenInterestDelta() math.LegacyDec {
	return b.openInterestDelta
}

func (b *limitOrderbook) getTotalOpenNotional() math.LegacyDec {
	return b.currentOpenNotional.Add(b.addedOpenNotional).Add(b.oppositeSideDerivativeOrderbook.GetAddedOpenNotional())
}

func (b *limitOrderbook) updateNotionalCapValuesAfterFill(currOrder *v2.DerivativeLimitOrder, fillQuantity math.LegacyDec) {
	notionalDelta, quantityDelta, _ := GetValuesForNotionalCapChecks(
		currOrder.OrderType,
		fillQuantity,
		b.markPrice,
		getSignedPositionQuantity(b.positionCache[currOrder.SubaccountID()]),
	)

	b.openInterestDelta = b.openInterestDelta.Add(quantityDelta)
	b.addedOpenNotional = b.addedOpenNotional.Add(notionalDelta)

	if pos := b.positionCache[currOrder.SubaccountID()]; pos != nil {
		executionMargin := currOrder.Margin.Mul(fillQuantity).Quo(currOrder.OrderInfo.Quantity)
		delta := &v2.PositionDelta{
			IsLong:            currOrder.IsBuy(),
			ExecutionQuantity: fillQuantity,
			ExecutionMargin:   executionMargin,
			ExecutionPrice:    currOrder.OrderInfo.Price, // using order price as worst case since FBA clearing price is unknown here
		}
		pos.ApplyPositionDelta(delta, b.getCurrOrderTradeFeeRate())
	}

	b.cachedAddedOpenNotional = math.LegacyZeroDec()
}

type orderbookFills struct {
	Orders         []*v2.DerivativeLimitOrder
	FillQuantities []math.LegacyDec
}

type orderbookFill struct {
	Order        *v2.DerivativeLimitOrder
	FillQuantity math.LegacyDec
	IsTransient  bool
}

func (f *orderbookFill) GetPrice() math.LegacyDec {
	return f.Order.OrderInfo.Price
}

type mergedOrderbookFills struct {
	IsBuy          bool
	TransientFills *orderbookFills
	RestingFills   *orderbookFills

	transientIdx int
	restingIdx   int
}

// CONTRACT: orderbook fills must be sorted by price descending for buys and ascending for sells
func newMergedDerivativeOrderbookFills(isBuy bool, transientFills, restingFills *orderbookFills) *mergedOrderbookFills {
	return &mergedOrderbookFills{
		IsBuy:          isBuy,
		TransientFills: transientFills,
		RestingFills:   restingFills,
		transientIdx:   0,
		restingIdx:     0,
	}
}

func (m *mergedOrderbookFills) GetTransientFillsLength() int {
	if m.TransientFills == nil {
		return 0
	}

	return len(m.TransientFills.Orders)
}

func (m *mergedOrderbookFills) GetRestingFillsLength() int {
	if m.RestingFills == nil {
		return 0
	}

	return len(m.RestingFills.Orders)
}

// Done returns true if there are no more transient or resting fills to iterate over.
func (m *mergedOrderbookFills) Done() bool {
	return m.transientIdx == m.GetTransientFillsLength() && m.restingIdx == m.GetRestingFillsLength()
}

func (m *mergedOrderbookFills) Peek() *orderbookFill {
	currTransientFill := m.getTransientFillAtIndex(m.transientIdx)
	currRestingFill := m.getRestingFillAtIndex(m.restingIdx)

	switch {
	case currTransientFill == nil && currRestingFill == nil:
		return nil
	case currTransientFill == nil:
		return currRestingFill
	case currRestingFill == nil:
		return currTransientFill
	}

	// for buys, return the higher priced fill and for sells, return the lower priced fill since the matching algorithm
	// should process orders closest to TOB first
	if (m.IsBuy && currRestingFill.GetPrice().GTE(currTransientFill.GetPrice())) ||
		(!m.IsBuy && currRestingFill.GetPrice().LTE(currTransientFill.GetPrice())) {
		return currRestingFill
	}
	return currTransientFill
}

func (m *mergedOrderbookFills) Next() *orderbookFill {
	if m.Done() {
		return nil
	}

	fill := m.Peek()
	if fill == nil {
		return nil
	}

	if fill.IsTransient {
		m.transientIdx++
	} else {
		m.restingIdx++
	}

	return fill
}

func (m *mergedOrderbookFills) getTransientFillAtIndex(idx int) *orderbookFill {
	if m.TransientFills == nil || idx > len(m.TransientFills.Orders)-1 {
		return nil
	}

	return &orderbookFill{
		Order:        m.TransientFills.Orders[idx],
		FillQuantity: m.TransientFills.FillQuantities[idx],
		IsTransient:  true,
	}
}

func (m *mergedOrderbookFills) getRestingFillAtIndex(idx int) *orderbookFill {
	if m.RestingFills == nil || idx > len(m.RestingFills.Orders)-1 {
		return nil
	}

	return &orderbookFill{
		Order:        m.RestingFills.Orders[idx],
		FillQuantity: m.RestingFills.FillQuantities[idx],
		IsTransient:  false,
	}
}

func DoesBreachOpenNotionalCap(
	orderType v2.OrderType,
	orderQuantity,
	markPrice, totalOpenNotional math.LegacyDec,
	positionQuantity math.LegacyDec,
	openNotionalCap v2.OpenNotionalCap,

) (bool, math.LegacyDec) {
	if openNotionalCap.GetUncapped() != nil {
		return false, math.LegacyZeroDec()
	}

	notionalDelta, _, _ := GetValuesForNotionalCapChecks(
		orderType,
		orderQuantity,
		markPrice,
		positionQuantity,
	)

	// always accept orders reducing open interest
	if notionalDelta.IsNegative() {
		return false, notionalDelta
	}

	return totalOpenNotional.Add(notionalDelta).GT(openNotionalCap.GetCapped().Value), notionalDelta
}

func GetValuesForNotionalCapChecks(
	orderType v2.OrderType,
	orderQuantity, markPrice math.LegacyDec,
	positionQuantity math.LegacyDec,
) (notionalDelta, quantityDelta, newPositionQuantity math.LegacyDec) {
	isClosingPosition := !positionQuantity.IsNil() && !positionQuantity.IsZero() && orderType.IsBuy() == positionQuantity.IsNegative()

	if orderType.IsBuy() {
		newPositionQuantity = positionQuantity.Add(orderQuantity)
	} else {
		newPositionQuantity = positionQuantity.Sub(orderQuantity)
	}

	switch {
	case isClosingPosition:
		positionQuantityAbs := positionQuantity.Abs()
		isFlippingPosition := orderQuantity.GT(positionQuantityAbs)

		if isFlippingPosition {
			quantityDelta = newPositionQuantity.Abs().Sub(positionQuantityAbs)
		} else {
			quantityDelta = orderQuantity.Neg()
		}
	default:
		quantityDelta = orderQuantity
	}

	notionalDelta = quantityDelta.Mul(markPrice)
	return notionalDelta, quantityDelta, newPositionQuantity
}
