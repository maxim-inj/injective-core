package derivative

import (
	"bytes"
	"fmt"
	"sort"
	"sync"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/events"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/utils"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// GetAllDerivativeLimitOrdersByMarketDirection returns all the Derivative Limit Orders for a given marketID and direction.
func (k DerivativeKeeper) GetAllDerivativeLimitOrdersByMarketDirection(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) []*v2.DerivativeLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.DerivativeLimitOrder, 0)
	k.IterateDerivativeLimitOrdersByMarketDirection(ctx, marketID, isBuy, func(order *v2.DerivativeLimitOrder) (stop bool) {
		orders = append(orders, order)
		return false
	})

	return orders
}

// InvalidateConditionalOrdersIfNoMarginLocked cancels all RO conditional orders if subaccount has no margin locked in a market
//
//nolint:revive // ok
func (k DerivativeKeeper) InvalidateConditionalOrdersIfNoMarginLocked(
	ctx sdk.Context,
	marketID, subaccountID common.Hash,
	hasPositionBeenJustDeleted bool,
	invalidMetadataIsBuy *bool,
	marketCache map[common.Hash]*v2.DerivativeMarket,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// early return if position exists (only need to check if we haven't already just deleted it)
	// we proceed if there is no position, since margin can still be locked in vanilla open orders
	if !hasPositionBeenJustDeleted && k.HasPosition(ctx, marketID, subaccountID) {
		return
	}

	if invalidMetadataIsBuy != nil {
		oppositeMetadata := k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, !*invalidMetadataIsBuy)

		if oppositeMetadata.VanillaLimitOrderCount+oppositeMetadata.VanillaConditionalOrderCount > 0 {
			return // we have margin locked on other side
		}
	} else {
		metadataBuy := k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, true)
		metadataSell := k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, false)

		hasNoReduceOnlyConditionals := (metadataBuy.ReduceOnlyConditionalOrderCount + metadataSell.ReduceOnlyConditionalOrderCount) == 0
		hasVanillaOrders := (metadataBuy.VanillaLimitOrderCount +
			metadataBuy.VanillaConditionalOrderCount +
			metadataSell.VanillaLimitOrderCount +
			metadataSell.VanillaConditionalOrderCount) > 0

		// skip invalidation if margin is locked OR no conditionals exist
		if hasNoReduceOnlyConditionals || hasVanillaOrders {
			return
		}
	}

	var market v2.DerivativeMarketI

	if marketCache != nil {
		m, ok := marketCache[marketID]
		if ok {
			market = m
		}
	}

	if market == nil {
		market = k.GetDerivativeOrBinaryOptionsMarket(ctx, marketID, nil)
	}

	// no position and no vanilla orders on both sides => cancel all conditional orders
	k.CancelAllConditionalDerivativeOrdersBySubaccountIDAndMarket(ctx, market, subaccountID)
}

// GetAllTriggeredConditionalOrders returns all conditional orders triggered in this block of each type for every market
//
//nolint:revive // ok
func (k DerivativeKeeper) GetAllTriggeredConditionalOrders(ctx sdk.Context) ([]*v2.TriggeredOrdersInMarket, map[common.Hash]*v2.DerivativeMarket) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	markets := k.GetAllActiveDerivativeMarkets(ctx)

	// TODO: refactor cache to separate type later for other markets
	derivativeMarketCache := make(map[common.Hash]*v2.DerivativeMarket, len(markets))
	marketTriggeredOrders := make([]*v2.TriggeredOrdersInMarket, len(markets))

	wg := new(sync.WaitGroup)
	mux := new(sync.Mutex)

	for idx, market := range markets {
		derivativeMarketCache[market.MarketID()] = market

		// don't trigger any conditional orders if in PO only mode
		if k.IsPostOnlyMode(ctx) {
			continue
		}

		wg.Add(1)

		go func(idx int, market *v2.DerivativeMarket) {
			defer wg.Done()
			triggeredOrders := k.processMarketForTriggeredOrders(ctx, market)
			if triggeredOrders == nil {
				return
			}

			mux.Lock()
			marketTriggeredOrders[idx] = triggeredOrders
			mux.Unlock()
		}(idx, market)
	}

	wg.Wait()

	validTriggers := make([]*v2.TriggeredOrdersInMarket, 0, len(marketTriggeredOrders))
	for _, order := range marketTriggeredOrders {
		if order != nil {
			validTriggers = append(validTriggers, order)
		}
	}

	return validTriggers, derivativeMarketCache
}

func (k DerivativeKeeper) GetAllSubaccountConditionalOrders(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
) []*v2.TrimmedDerivativeConditionalOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	trimmedMarketOrder := func(orderHash common.Hash, isTriggerPriceHigher bool) *v2.TrimmedDerivativeConditionalOrder {
		order, _ := k.GetConditionalDerivativeMarketOrderBySubaccountIDAndHash(
			ctx,
			marketID,
			&isTriggerPriceHigher,
			subaccountID,
			orderHash,
		)
		if order != nil && order.TriggerPrice != nil {
			return &v2.TrimmedDerivativeConditionalOrder{
				Price:        order.Price(),
				Quantity:     order.Quantity(),
				Margin:       order.GetMargin(),
				TriggerPrice: *order.TriggerPrice,
				IsBuy:        order.IsBuy(),
				IsLimit:      false,
				OrderHash:    common.BytesToHash(order.OrderHash).String(),
				Cid:          order.Cid(),
			}
		}

		return nil
	}

	trimmedLimitOrder := func(orderHash common.Hash, isTriggerPriceHigher bool) *v2.TrimmedDerivativeConditionalOrder {
		order, _ := k.GetConditionalDerivativeLimitOrderBySubaccountIDAndHash(ctx, marketID, &isTriggerPriceHigher, subaccountID, orderHash)
		if order != nil && order.TriggerPrice != nil {
			return &v2.TrimmedDerivativeConditionalOrder{
				Price:        order.Price(),
				Quantity:     order.GetQuantity(),
				Margin:       order.GetMargin(),
				TriggerPrice: *order.TriggerPrice,
				IsBuy:        order.IsBuy(),
				IsLimit:      true,
				OrderHash:    common.BytesToHash(order.OrderHash).String(),
				Cid:          order.Cid(),
			}
		}

		return nil
	}

	orders := make([]*v2.TrimmedDerivativeConditionalOrder, 0)
	for _, hash := range k.GetAllConditionalOrderHashesBySubaccountAndMarket(
		ctx, marketID, true, true, subaccountID,
	) {
		orders = append(orders, trimmedMarketOrder(hash, true))
	}

	for _, hash := range k.GetAllConditionalOrderHashesBySubaccountAndMarket(
		ctx, marketID, false, true, subaccountID,
	) {
		orders = append(orders, trimmedMarketOrder(hash, false))
	}

	for _, hash := range k.GetAllConditionalOrderHashesBySubaccountAndMarket(
		ctx, marketID, true, false, subaccountID,
	) {
		orders = append(orders, trimmedLimitOrder(hash, true))
	}

	for _, hash := range k.GetAllConditionalOrderHashesBySubaccountAndMarket(
		ctx, marketID, false, false, subaccountID,
	) {
		orders = append(orders, trimmedLimitOrder(hash, false))
	}

	return orders
}

func (k DerivativeKeeper) DerivativeOrderCrossesTopOfBook(ctx sdk.Context, order *v2.DerivativeOrder) bool {
	// get best price of TOB from opposite side
	bestPrice := k.GetBestDerivativeLimitOrderPrice(ctx, common.HexToHash(order.MarketId), !order.IsBuy())

	if bestPrice == nil {
		return false
	}

	if order.IsBuy() {
		return order.OrderInfo.Price.GTE(*bestPrice)
	}

	return order.OrderInfo.Price.LTE(*bestPrice)
}

// GetAllConditionalDerivativeOrderbooks returns all conditional orderbooks for all derivative markets.
func (k DerivativeKeeper) GetAllConditionalDerivativeOrderbooks(ctx sdk.Context) []*v2.ConditionalDerivativeOrderBook {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	markets := k.GetAllDerivativeMarkets(ctx)
	orderbooks := make([]*v2.ConditionalDerivativeOrderBook, 0, len(markets))

	for _, market := range markets {
		marketID := market.MarketID()

		orderbook := k.GetAllConditionalDerivativeOrdersUpToMarkPrice(ctx, marketID, nil)
		if orderbook.IsEmpty() {
			continue
		}

		orderbooks = append(orderbooks, orderbook)
	}
	return orderbooks
}

func (k DerivativeKeeper) CancelAllConditionalDerivativeOrdersBySubaccountIDAndMarket(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	subaccountID common.Hash,
) {
	marketID := market.MarketID()

	higherMarketOrders := k.GetAllConditionalOrderHashesBySubaccountAndMarket(ctx, marketID, true, true, subaccountID)
	lowerMarketOrders := k.GetAllConditionalOrderHashesBySubaccountAndMarket(ctx, marketID, false, true, subaccountID)
	higherLimitOrders := k.GetAllConditionalOrderHashesBySubaccountAndMarket(ctx, marketID, true, false, subaccountID)
	lowerLimitOrders := k.GetAllConditionalOrderHashesBySubaccountAndMarket(ctx, marketID, false, false, subaccountID)

	k.cancelConditionalDerivativeMarketOrders(ctx, market, subaccountID, higherMarketOrders, true)
	k.cancelConditionalDerivativeMarketOrders(ctx, market, subaccountID, lowerMarketOrders, false)
	k.cancelConditionalDerivativeLimitOrders(ctx, market, subaccountID, higherLimitOrders, true)
	k.cancelConditionalDerivativeLimitOrders(ctx, market, subaccountID, lowerLimitOrders, false)
}

// GetAllConditionalDerivativeOrdersUpToMarkPrice returns orderbook of conditional orders in current market up to
// triggerPrice (optional == return all orders)
func (k DerivativeKeeper) GetAllConditionalDerivativeOrdersUpToMarkPrice(
	ctx sdk.Context,
	marketID common.Hash,
	markPrice *math.LegacyDec,
) *v2.ConditionalDerivativeOrderBook {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketBuyOrders, marketSellOrders := k.GetAllConditionalDerivativeMarketOrdersInMarketUpToPrice(ctx, marketID, markPrice)
	limitBuyOrders, limitSellOrders := k.GetAllConditionalDerivativeLimitOrdersInMarketUpToPrice(ctx, marketID, markPrice)

	// filter further here if PO mode and order crosses TOB

	orderbook := &v2.ConditionalDerivativeOrderBook{
		MarketId:         marketID.String(),
		LimitBuyOrders:   limitBuyOrders,
		MarketBuyOrders:  marketBuyOrders,
		LimitSellOrders:  limitSellOrders,
		MarketSellOrders: marketSellOrders,
	}

	return orderbook
}

func (k DerivativeKeeper) CancelAllRestingDerivativeLimitOrdersForSubaccount(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	subaccountID common.Hash,
	shouldCancelReduceOnly,
	shouldCancelVanilla bool,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := market.MarketID()
	restingBuyOrderHashes := k.GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(ctx, marketID, true, subaccountID)
	restingSellOrderHashes := k.GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(ctx, marketID, false, subaccountID)

	for _, hash := range restingBuyOrderHashes {
		isBuy := true
		if err := k.CancelRestingDerivativeLimitOrder(
			ctx, market, subaccountID, &isBuy, hash, shouldCancelReduceOnly, shouldCancelVanilla,
		); err != nil {
			metrics.ReportFuncError(k.svcTags)
			events.Emit(ctx, k.BaseKeeper, v2.NewEventOrderCancelFail(marketID, subaccountID, hash.Hex(), "", err))
			continue
		}
	}

	for _, hash := range restingSellOrderHashes {
		isBuy := false
		if err := k.CancelRestingDerivativeLimitOrder(
			ctx, market, subaccountID, &isBuy, hash, shouldCancelReduceOnly, shouldCancelVanilla,
		); err != nil {
			metrics.ReportFuncError(k.svcTags)
			events.Emit(ctx, k.BaseKeeper, v2.NewEventOrderCancelFail(marketID, subaccountID, hash.Hex(), "", err))
			continue
		}
	}
}

//nolint:revive // ok
func (k DerivativeKeeper) CancelRestingDerivativeLimitOrder(
	ctx sdk.Context,
	market v2.MarketI,
	subaccountID common.Hash,
	isBuy *bool,
	orderHash common.Hash,
	shouldCancelReduceOnly,
	shouldCancelVanilla bool,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := market.MarketID()
	// 1. Add back the margin hold to available balance
	order := k.GetDerivativeLimitOrderBySubaccountIDAndHash(ctx, marketID, isBuy, subaccountID, orderHash)
	if order == nil {
		k.Logger(ctx).Debug(
			"Resting Derivative Limit Order doesn't exist to cancel",
			"marketId", marketID,
			"subaccountID", subaccountID,
			"orderHash", orderHash,
		)
		metrics.ReportFuncError(k.svcTags)
		return errors.Wrap(types.ErrOrderDoesntExist, "Derivative Limit Order doesn't exist")
	}

	// skip cancelling limit orders if their type shouldn't be cancelled
	if order.IsVanilla() && !shouldCancelVanilla || order.IsReduceOnly() && !shouldCancelReduceOnly {
		return nil
	}

	if order.IsVanilla() {
		refundAmount := order.GetCancelRefundAmount(market.GetMakerFeeRate())
		chainFormatRefund := market.NotionalToChainFormat(refundAmount)
		k.subaccount.IncrementAvailableBalanceOrBank(ctx, subaccountID, market.GetQuoteDenom(), chainFormatRefund)
	}

	// 2. Delete the order state from ordersStore, ordersIndexStore and subaccountOrderStore
	k.DeleteDerivativeLimitOrder(ctx, marketID, order)

	k.subaccount.UpdateSubaccountOrderbookMetadataFromOrderCancel(ctx, marketID, subaccountID, order)

	events.Emit(ctx, k.BaseKeeper, &v2.EventCancelDerivativeOrder{
		MarketId:      marketID.Hex(),
		IsLimitCancel: true,
		LimitOrder:    order,
	})

	return nil
}

func (k DerivativeKeeper) CancelAllRestingDerivativeLimitOrders(ctx sdk.Context, market v2.DerivativeMarketI) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := market.MarketID()

	buyOrders := k.GetAllDerivativeLimitOrdersByMarketDirection(ctx, marketID, true)
	sellOrders := k.GetAllDerivativeLimitOrdersByMarketDirection(ctx, marketID, false)

	for _, buyOrder := range buyOrders {
		isBuy := true
		if err := k.CancelRestingDerivativeLimitOrder(
			ctx,
			market,
			buyOrder.SubaccountID(),
			&isBuy,
			buyOrder.Hash(),
			true,
			true,
		); err != nil {
			k.Logger(ctx).Error("CancelRestingDerivativeLimitOrder (buy) failed during CancelAllRestingDerivativeLimitOrders:", err)

			events.Emit(
				ctx,
				k.BaseKeeper,
				v2.NewEventOrderCancelFail(
					marketID,
					buyOrder.SubaccountID(),
					buyOrder.Hash().Hex(),
					buyOrder.Cid(),
					err,
				),
			)
		}
	}

	for _, sellOrder := range sellOrders {
		isBuy := false
		if err := k.CancelRestingDerivativeLimitOrder(
			ctx,
			market,
			sellOrder.SubaccountID(),
			&isBuy,
			sellOrder.Hash(),
			true,
			true,
		); err != nil {
			//nolint:revive // ok
			k.Logger(ctx).Error("CancelRestingDerivativeLimitOrder (sell) failed during CancelAllRestingDerivativeLimitOrders:", err)
			events.Emit(
				ctx,
				k.BaseKeeper,
				v2.NewEventOrderCancelFail(
					marketID,
					sellOrder.SubaccountID(),
					sellOrder.Hash().Hex(),
					sellOrder.Cid(),
					err,
				),
			)
		}
	}
}

func (k DerivativeKeeper) SetNewDerivativeLimitOrderWithMetadata(
	ctx sdk.Context,
	order *v2.DerivativeLimitOrder,
	metadata *v2.SubaccountOrderbookMetadata,
	marketID common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var (
		subaccountID = order.SubaccountID()
		isBuy        = order.IsBuy()
		price        = order.Price()
		orderHash    = order.Hash()
	)

	k.SetNewDerivativeLimitOrder(ctx, order, marketID)

	// Set cid => orderHash
	k.SetCid(ctx, false, subaccountID, order.OrderInfo.Cid, marketID, order.IsBuy(), orderHash)

	if metadata == nil {
		metadata = k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, isBuy)
	}

	if order.IsReduceOnly() {
		metadata.ReduceOnlyLimitOrderCount++
		metadata.AggregateReduceOnlyQuantity = metadata.AggregateReduceOnlyQuantity.Add(order.Fillable)
	} else {
		metadata.VanillaLimitOrderCount++
		metadata.AggregateVanillaQuantity = metadata.AggregateVanillaQuantity.Add(order.Fillable)
	}

	k.SetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, isBuy, metadata)
	k.SetSubaccountOrder(ctx, marketID, subaccountID, isBuy, orderHash, v2.NewSubaccountOrder(order))

	// update the orderbook metadata
	k.IncrementOrderbookPriceLevelQuantity(ctx, marketID, isBuy, false, price, order.GetFillable())

	if order.ExpirationBlock > 0 {
		orderData := &v2.OrderData{
			MarketId:     marketID.Hex(),
			SubaccountId: order.SubaccountID().Hex(),
			OrderHash:    order.Hash().Hex(),
			Cid:          order.Cid(),
		}
		k.AppendOrderExpirations(ctx, marketID, order.ExpirationBlock, orderData)
	}
}

//nolint:revive // ok
func (k DerivativeKeeper) UpdateDerivativeLimitOrdersFromFilledDeltas(
	ctx sdk.Context,
	marketID common.Hash,
	isResting bool,
	filledDeltas []*v2.DerivativeLimitOrderDelta,
	partialCancelOrders map[common.Hash]struct{},
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if len(filledDeltas) == 0 {
		return
	}

	// subaccountID => metadataDelta
	metadataBuyDeltas := make(map[common.Hash]*v2.SubaccountOrderbookMetadata, len(filledDeltas))
	metadataSellDeltas := make(map[common.Hash]*v2.SubaccountOrderbookMetadata, len(filledDeltas))

	for _, filledDelta := range filledDeltas {
		var (
			subaccountID = filledDelta.SubaccountID()
			isBuy        = filledDelta.IsBuy()
			price        = filledDelta.Price()
			orderHash    = filledDelta.OrderHash()
			cid          = filledDelta.Cid()
		)

		var metadataDelta *v2.SubaccountOrderbookMetadata
		var found bool
		if isBuy {
			if metadataDelta, found = metadataBuyDeltas[subaccountID]; !found {
				metadataDelta = v2.NewSubaccountOrderbookMetadata()
				metadataBuyDeltas[subaccountID] = metadataDelta
			}
		} else {
			if metadataDelta, found = metadataSellDeltas[subaccountID]; !found {
				metadataDelta = v2.NewSubaccountOrderbookMetadata()
				metadataSellDeltas[subaccountID] = metadataDelta
			}
		}

		decrementQuantity := filledDelta.FillQuantity.Add(filledDelta.CancelQuantity)

		if filledDelta.Order.IsReduceOnly() {
			metadataDelta.AggregateReduceOnlyQuantity = metadataDelta.AggregateReduceOnlyQuantity.Sub(decrementQuantity)
		} else {
			metadataDelta.AggregateVanillaQuantity = metadataDelta.AggregateVanillaQuantity.Sub(decrementQuantity)
		}

		// Check if this is a partial cancel order (used for transient order optimizations)
		_, isPartialCancel := partialCancelOrders[orderHash]

		if filledDelta.FillableQuantity().IsZero() {
			// Delete primary order and CID only for resting orders.
			// Transient orders with FillableQuantity == 0 were never written to primary storage.
			if isResting {
				k.BasicDeleteDerivativeLimitOrder(ctx, marketID, filledDelta.Order)
				k.DeleteCid(ctx, false, subaccountID, filledDelta.Order.OrderInfo.Cid)
			}

			// SubaccountOrder is always deleted (written at order submission time)
			k.DeleteSubaccountOrder(ctx, marketID, filledDelta.Order)

			if filledDelta.Order.IsReduceOnly() {
				metadataDelta.ReduceOnlyLimitOrderCount--
			} else {
				metadataDelta.VanillaLimitOrderCount--
			}
		} else {
			// Handle storage writes for orders with remaining fillable quantity:
			// - Resting orders: update in permanent storage
			// - Transient non-partial-cancel: write to permanent storage (becomes resting)
			// - Transient partial cancel: skip all writes (will be cancelled immediately)
			if isResting {
				k.BasicSetNewDerivativeLimitOrder(ctx, filledDelta.Order, marketID)
			} else if !isPartialCancel {
				k.SetNewDerivativeLimitOrder(ctx, filledDelta.Order, marketID)
				k.SetCid(ctx, false, subaccountID, cid, marketID, isBuy, orderHash)

				if filledDelta.Order.ExpirationBlock > 0 {
					orderData := &v2.OrderData{
						MarketId:     marketID.Hex(),
						SubaccountId: filledDelta.Order.SubaccountID().Hex(),
						OrderHash:    filledDelta.Order.Hash().Hex(),
						Cid:          filledDelta.Order.Cid(),
					}
					k.AppendOrderExpirations(ctx, marketID, filledDelta.Order.ExpirationBlock, orderData)
				}
			}

			if isResting || !isPartialCancel {
				subaccountOrder := &v2.SubaccountOrder{
					Price:        price,
					Quantity:     filledDelta.Order.Fillable,
					IsReduceOnly: filledDelta.Order.IsReduceOnly(),
					Cid:          filledDelta.Order.Cid(),
				}

				k.SetSubaccountOrder(ctx, marketID, subaccountID, isBuy, orderHash, subaccountOrder)
			}
		}

		if isResting {
			// update orderbook metadata
			k.DecrementOrderbookPriceLevelQuantity(ctx, marketID, isBuy, false, price, decrementQuantity)
		} else if !isPartialCancel {
			// For transient orders, only increment price level if the order is NOT a partial cancel.
			// Partial cancel orders will be cancelled immediately, so they shouldn't be added to the orderbook.
			k.IncrementOrderbookPriceLevelQuantity(ctx, marketID, isBuy, false, price, filledDelta.FillableQuantity())
		}
	}

	k.applySubaccountOrderbookMetadataDeltas(ctx, marketID, true, metadataBuyDeltas)
	k.applySubaccountOrderbookMetadataDeltas(ctx, marketID, false, metadataSellDeltas)
}

func (k DerivativeKeeper) applySubaccountOrderbookMetadataDeltas(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	deltas map[common.Hash]*v2.SubaccountOrderbookMetadata,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if len(deltas) == 0 {
		return
	}

	subaccountIDs := make([]common.Hash, 0, len(deltas))
	for s := range deltas {
		subaccountIDs = append(subaccountIDs, s)
	}

	sort.SliceStable(subaccountIDs, func(i, j int) bool {
		return bytes.Compare(subaccountIDs[i].Bytes(), subaccountIDs[j].Bytes()) < 0
	})

	for _, subaccountID := range subaccountIDs {
		metadata := k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, isBuy)

		metadata.ApplyDelta(deltas[subaccountID])

		k.SetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, isBuy, metadata)
	}
}

func (k DerivativeKeeper) GetBestDerivativeLimitOrderPrice(ctx sdk.Context, marketID common.Hash, isBuy bool) *math.LegacyDec {
	var bestOrder *v2.DerivativeLimitOrder
	k.IterateDerivativeLimitOrdersByMarketDirection(ctx, marketID, isBuy, func(order *v2.DerivativeLimitOrder) (stop bool) {
		bestOrder = order
		return true
	})

	var bestPrice *math.LegacyDec
	if bestOrder != nil {
		bestPrice = &bestOrder.OrderInfo.Price
	}

	return bestPrice
}

func (k DerivativeKeeper) GetDerivativeMidPriceAndTOB(
	ctx sdk.Context,
	marketID common.Hash,
) (
	midPrice *math.LegacyDec,
	bestBuyPrice *math.LegacyDec,
	bestSellPrice *math.LegacyDec,
) {
	bestBuyPrice = k.GetBestDerivativeLimitOrderPrice(ctx, marketID, true)
	bestSellPrice = k.GetBestDerivativeLimitOrderPrice(ctx, marketID, false)

	if bestBuyPrice == nil || bestSellPrice == nil {
		return nil, bestBuyPrice, bestSellPrice
	}

	midPriceValue := bestBuyPrice.Add(*bestSellPrice).Quo(math.LegacyNewDec(2))
	return &midPriceValue, bestBuyPrice, bestSellPrice
}

func (k DerivativeKeeper) GetDerivativeMidPriceOrBestPrice(ctx sdk.Context, marketID common.Hash) *math.LegacyDec {
	bestBuyPrice := k.GetBestDerivativeLimitOrderPrice(ctx, marketID, true)
	bestSellPrice := k.GetBestDerivativeLimitOrderPrice(ctx, marketID, false)

	switch {
	case bestBuyPrice == nil && bestSellPrice == nil:
		return nil
	case bestBuyPrice == nil:
		return bestSellPrice
	case bestSellPrice == nil:
		return bestBuyPrice
	}

	midPrice := bestBuyPrice.Add(*bestSellPrice).Quo(math.LegacyNewDec(2))
	return &midPrice
}

func (k DerivativeKeeper) CancelAllTransientDerivativeLimitOrders(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := market.MarketID()
	buyOrders := k.GetAllTransientDerivativeLimitOrdersByMarketDirection(ctx, marketID, true)
	sellOrders := k.GetAllTransientDerivativeLimitOrdersByMarketDirection(ctx, marketID, false)

	for _, buyOrder := range buyOrders {
		if err := k.CancelTransientDerivativeLimitOrder(ctx, market, buyOrder); err != nil {
			k.Logger(ctx).Error(
				"CancelTransientDerivativeLimitOrder for buyOrder failed during CancelAllTransientDerivativeLimitOrders",
				"orderHash", common.BytesToHash(buyOrder.OrderHash).Hex(),
				"err", err.Error(),
			)

			events.Emit(ctx, k.BaseKeeper, v2.NewEventOrderCancelFail(
				marketID,
				buyOrder.SubaccountID(),
				common.Bytes2Hex(buyOrder.GetOrderHash()),
				buyOrder.Cid(),
				err,
			))
		}
	}

	for _, sellOrder := range sellOrders {
		if err := k.CancelTransientDerivativeLimitOrder(ctx, market, sellOrder); err != nil {
			k.Logger(ctx).Error(
				"CancelTransientDerivativeLimitOrder for sellOrder failed during CancelAllTransientDerivativeLimitOrders",
				"orderHash", common.BytesToHash(sellOrder.OrderHash).Hex(),
				"err", err.Error(),
			)

			events.Emit(ctx, k.BaseKeeper, v2.NewEventOrderCancelFail(
				marketID,
				sellOrder.SubaccountID(),
				common.Bytes2Hex(sellOrder.GetOrderHash()),
				sellOrder.Cid(),
				err,
			))
		}
	}
}

// CancelTransientDerivativeLimitOrder cancels the transient derivative limit order
func (k DerivativeKeeper) CancelTransientDerivativeLimitOrder(
	ctx sdk.Context,
	market v2.MarketI,
	order *v2.DerivativeLimitOrder,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// 1. Add back the margin hold to available balance
	marketID := market.MarketID()
	subaccountID := order.SubaccountID()

	if order.IsVanilla() {
		refundAmount := order.GetCancelRefundAmount(market.GetTakerFeeRate())
		chainFormatRefund := market.NotionalToChainFormat(refundAmount)
		k.subaccount.IncrementAvailableBalanceOrBank(ctx, subaccountID, market.GetQuoteDenom(), chainFormatRefund)
	} else if order.IsReduceOnly() {
		position := k.GetPosition(ctx, marketID, subaccountID)
		if position == nil {
			k.Logger(ctx).Error("Derivative Position doesn't exist",
				"marketId", marketID,
				"subaccountID", subaccountID,
				"orderHash", order.Hash().Hex(),
			)
			metrics.ReportFuncError(k.svcTags)
			return errors.Wrapf(
				types.ErrPositionNotFound,
				"marketId %s subaccountID %s orderHash %s", marketID, subaccountID.Hex(), order.Hash().Hex(),
			)
		}
	}

	k.subaccount.UpdateSubaccountOrderbookMetadataFromOrderCancel(ctx, marketID, subaccountID, order)

	// 2. Delete the order state from ordersStore and ordersIndexStore
	k.DeleteTransientDerivativeLimitOrder(ctx, marketID, order)

	events.Emit(ctx, k.BaseKeeper, &v2.EventCancelDerivativeOrder{
		MarketId:      marketID.Hex(),
		IsLimitCancel: true,
		LimitOrder:    order,
	})

	return nil
}

// SetNewTransientDerivativeLimitOrderWithMetadata stores the DerivativeLimitOrder in the transient store.
func (k DerivativeKeeper) SetNewTransientDerivativeLimitOrderWithMetadata(
	ctx sdk.Context,
	order *v2.DerivativeLimitOrder,
	metadata *v2.SubaccountOrderbookMetadata,
	marketID common.Hash,
	isBuy bool,
	orderHash common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	k.SetNewTransientDerivativeLimitOrder(ctx, order, marketID, isBuy, orderHash)

	subaccountID := order.SubaccountID()

	// Set cid => orderHash
	k.SetCid(ctx, true, subaccountID, order.OrderInfo.GetCid(), marketID, order.IsBuy(), orderHash)

	if metadata == nil {
		metadata = k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, order.IsBuy())
	}

	if order.IsReduceOnly() {
		metadata.ReduceOnlyLimitOrderCount++
		metadata.AggregateReduceOnlyQuantity = metadata.AggregateReduceOnlyQuantity.Add(order.Fillable)
	} else {
		metadata.VanillaLimitOrderCount++
		metadata.AggregateVanillaQuantity = metadata.AggregateVanillaQuantity.Add(order.Fillable)
	}

	k.SetTransientDerivativeLimitOrderIndicator(ctx, marketID, isBuy)
	k.SetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, order.IsBuy(), metadata)
	k.SetSubaccountOrder(ctx, marketID, subaccountID, order.IsBuy(), order.Hash(), v2.NewSubaccountOrder(order))
}

func (k DerivativeKeeper) CancelAllTransientDerivativeLimitOrdersBySubaccountID(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	subaccountID common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := market.MarketID()
	buyOrders := k.GetAllTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID(ctx, marketID, &subaccountID, true)
	sellOrders := k.GetAllTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID(ctx, marketID, &subaccountID, false)

	for _, buyOrder := range buyOrders {
		if err := k.CancelTransientDerivativeLimitOrder(ctx, market, buyOrder); err != nil {
			orderHash := common.BytesToHash(buyOrder.OrderHash)
			k.Logger(ctx).Error(
				"CancelTransientDerivativeLimitOrder for buyOrder %s failed during CancelAllTransientDerivativeLimitOrdersBySubaccountID:",
				orderHash.Hex(),
				err,
			)

			events.Emit(ctx, k.BaseKeeper, v2.NewEventOrderCancelFail(marketID, subaccountID, orderHash.Hex(), buyOrder.Cid(), err))
		}
	}

	for _, sellOrder := range sellOrders {
		if err := k.CancelTransientDerivativeLimitOrder(ctx, market, sellOrder); err != nil {
			orderHash := common.BytesToHash(sellOrder.OrderHash)
			k.Logger(ctx).Error(
				"CancelTransientDerivativeLimitOrder for sellOrder %s failed during CancelAllTransientDerivativeLimitOrdersBySubaccountID:",
				orderHash.Hex(),
				err,
			)
			events.Emit(ctx, k.BaseKeeper, v2.NewEventOrderCancelFail(marketID, subaccountID, orderHash.Hex(), sellOrder.Cid(), err))
		}
	}
}

// CancelTransientDerivativeLimitOrdersForSubaccountUpToBalance cancels all of the derivative limit orders for a given
// subaccount and marketID until the given balance has been freed up, i.e., total balance becoming available balance.
func (k DerivativeKeeper) CancelTransientDerivativeLimitOrdersForSubaccountUpToBalance(
	ctx sdk.Context,
	market *v2.DerivativeMarket,
	subaccountID common.Hash,
	freeingUpBalance math.LegacyDec,
) (freedUpBalance math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	freedUpBalance = math.LegacyZeroDec()

	marketID := market.MarketID()
	transientBuyOrders := k.GetAllTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID(ctx, marketID, &subaccountID, true)

	for _, order := range transientBuyOrders {
		if freedUpBalance.GTE(freeingUpBalance) {
			return freedUpBalance
		}

		if err := k.CancelTransientDerivativeLimitOrder(ctx, market, order); err != nil {
			metrics.ReportFuncError(k.svcTags)

			events.Emit(
				ctx,
				k.BaseKeeper,
				v2.NewEventOrderCancelFail(
					marketID,
					subaccountID,
					common.Bytes2Hex(order.GetOrderHash()),
					order.Cid(),
					err,
				),
			)

			continue
		}

		notional := order.OrderInfo.Price.Mul(order.OrderInfo.Quantity)
		marginHoldRefund := order.Fillable.Mul(order.Margin.Add(notional.Mul(market.TakerFeeRate))).Quo(order.OrderInfo.Quantity)
		freedUpBalance = freedUpBalance.Add(marginHoldRefund)
	}

	transientSellOrders := k.GetAllTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID(ctx, marketID, &subaccountID, false)
	for _, order := range transientSellOrders {
		if freedUpBalance.GTE(freeingUpBalance) {
			return freedUpBalance
		}

		if err := k.CancelTransientDerivativeLimitOrder(ctx, market, order); err != nil {
			metrics.ReportFuncError(k.svcTags)

			events.Emit(
				ctx,
				k.BaseKeeper,
				v2.NewEventOrderCancelFail(
					marketID,
					subaccountID,
					common.Bytes2Hex(order.GetOrderHash()),
					order.Cid(),
					err,
				),
			)

			continue
		}

		notional := order.OrderInfo.Price.Mul(order.OrderInfo.Quantity)
		marginHoldRefund := order.Fillable.Mul(order.Margin.Add(notional.Mul(market.TakerFeeRate))).Quo(order.OrderInfo.Quantity)
		freedUpBalance = freedUpBalance.Add(marginHoldRefund)
	}

	return freedUpBalance
}

func (k DerivativeKeeper) SetPostOnlyDerivativeLimitOrderWithMetadata(
	ctx sdk.Context,
	order *v2.DerivativeLimitOrder,
	metadata *v2.SubaccountOrderbookMetadata,
	marketID common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	k.SetNewDerivativeLimitOrderWithMetadata(ctx, order, metadata, marketID)

	newOrdersEvent := &v2.EventNewDerivativeOrders{
		MarketId:   marketID.Hex(),
		BuyOrders:  make([]*v2.DerivativeLimitOrder, 0),
		SellOrders: make([]*v2.DerivativeLimitOrder, 0),
	}
	if order.IsBuy() {
		newOrdersEvent.BuyOrders = append(newOrdersEvent.BuyOrders, order)
	} else {
		newOrdersEvent.SellOrders = append(newOrdersEvent.SellOrders, order)
	}

	events.Emit(ctx, k.BaseKeeper, newOrdersEvent)
}

// CancelConditionalDerivativeLimitOrder cancels the conditional derivative limit order
func (k DerivativeKeeper) CancelConditionalDerivativeLimitOrder(
	ctx sdk.Context,
	market v2.MarketI,
	subaccountID common.Hash,
	isTriggerPriceHigher *bool,
	orderHash common.Hash,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := market.MarketID()

	order, direction := k.GetConditionalDerivativeLimitOrderBySubaccountIDAndHash(
		ctx,
		marketID,
		isTriggerPriceHigher,
		subaccountID,
		orderHash,
	)
	if order == nil {
		k.Logger(ctx).Debug(
			"Conditional Derivative Limit Order doesn't exist to cancel",
			"marketId", marketID,
			"subaccountID", subaccountID,
			"orderHash", orderHash.Hex(),
		)
		metrics.ReportFuncError(k.svcTags)
		return errors.Wrap(types.ErrOrderDoesntExist, "Conditional Derivative Limit Order doesn't exist")
	}

	refundAmount := order.GetCancelRefundAmount(market.GetTakerFeeRate())
	chainFormatRefundAmount := market.NotionalToChainFormat(refundAmount)
	k.subaccount.IncrementAvailableBalanceOrBank(ctx, subaccountID, market.GetQuoteDenom(), chainFormatRefundAmount)

	// 2. Delete the order state from ordersStore and ordersIndexStore
	k.DeleteConditionalDerivativeOrder(ctx, true, marketID, order.SubaccountID(), direction, *order.TriggerPrice, order.Hash(), order.Cid())

	// 3. update metadata
	metadata := k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, order.IsBuy())
	if order.IsVanilla() {
		metadata.VanillaConditionalOrderCount--
	} else {
		metadata.ReduceOnlyConditionalOrderCount--
	}
	k.SetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, order.IsBuy(), metadata)

	events.Emit(ctx, k.BaseKeeper, &v2.EventCancelConditionalDerivativeOrder{
		MarketId:      marketID.Hex(),
		IsLimitCancel: true,
		LimitOrder:    order,
	})

	return nil
}

func (k DerivativeKeeper) SetConditionalDerivativeLimitOrderWithMetadata(
	ctx sdk.Context,
	order *v2.DerivativeLimitOrder,
	metadata *v2.SubaccountOrderbookMetadata,
	marketID common.Hash,
	markPrice math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var (
		subaccountID         = order.SubaccountID()
		isTriggerPriceHigher = order.TriggerPrice.GT(markPrice)
	)

	k.SetConditionalDerivativeLimitOrder(ctx, order, marketID, markPrice)

	if metadata == nil {
		metadata = k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, isTriggerPriceHigher)
	}

	if order.IsVanilla() {
		metadata.VanillaConditionalOrderCount++
	} else {
		metadata.ReduceOnlyConditionalOrderCount++
	}
	k.SetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, order.IsBuy(), metadata)

	newOrderEvent := &v2.EventNewConditionalDerivativeOrder{
		MarketId: marketID.Hex(),
		Order:    order.ToDerivativeOrder(marketID.String()),
		Hash:     order.OrderHash,
		IsMarket: false,
	}

	events.Emit(ctx, k.BaseKeeper, newOrderEvent)
}

//nolint:revive // ok
func (k DerivativeKeeper) TriggerConditionalDerivativeLimitOrder(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	markPrice math.LegacyDec,
	limitOrder *v2.DerivativeLimitOrder,
	skipCancel bool,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if !skipCancel {
		if err := k.CancelConditionalDerivativeLimitOrder(ctx, market, limitOrder.OrderInfo.SubaccountID(), nil, limitOrder.Hash()); err != nil {
			return err
		}
	}

	marketID := market.MarketID()

	senderAddr := types.SubaccountIDToSdkAddress(limitOrder.OrderInfo.SubaccountID())
	orderType := v2.OrderType_BUY
	if !limitOrder.IsBuy() {
		orderType = v2.OrderType_SELL
	}

	order := v2.DerivativeOrder{
		MarketId:     marketID.Hex(),
		OrderInfo:    limitOrder.OrderInfo,
		OrderType:    orderType,
		Margin:       limitOrder.Margin,
		TriggerPrice: nil,
	}

	orderMsg := v2.MsgCreateDerivativeLimitOrder{
		Sender: senderAddr.String(),
		Order:  order,
	}
	if err := orderMsg.ValidateBasic(); err != nil {
		return err
	}

	orderHash, err := k.CreateDerivativeLimitOrder(ctx, senderAddr, &order, market, markPrice)
	if err != nil {
		return err
	}

	events.Emit(ctx, k.BaseKeeper, &v2.EventConditionalDerivativeOrderTrigger{
		MarketId:           marketID.Bytes(),
		IsLimitTrigger:     true,
		TriggeredOrderHash: limitOrder.OrderHash,
		PlacedOrderHash:    orderHash.Bytes(),
		TriggeredOrderCid:  limitOrder.Cid(),
	})

	return nil
}

func (k DerivativeKeeper) CancelReduceOnlySubaccountOrders(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
	isBuy bool,
	orderData []*v2.SubaccountOrderData,
) (orders []*v2.DerivativeLimitOrder, cumulativeReduceOnlyQuantityToCancel math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders = make([]*v2.DerivativeLimitOrder, 0, len(orderData))
	cumulativeReduceOnlyQuantityToCancel = math.LegacyZeroDec()
	for _, o := range orderData {
		// 1. Add back the margin hold to available balance
		order := k.DeleteDerivativeLimitOrderByFields(ctx, marketID, o.Order.Price, isBuy, common.BytesToHash(o.OrderHash))
		if order == nil {
			message := fmt.Errorf(
				"DeleteDerivativeLimitOrderByFields returned nil order for order price: %v, hash: %v",
				o.Order.Price,
				common.BytesToHash(o.OrderHash).Hex(),
			)

			events.Emit(ctx, k.BaseKeeper, v2.NewEventOrderCancelFail(marketID, subaccountID, common.Bytes2Hex(o.OrderHash), "", message))
			panic(message)
		}

		cumulativeReduceOnlyQuantityToCancel = cumulativeReduceOnlyQuantityToCancel.Add(order.Fillable)
		orders = append(orders, order)

		events.Emit(ctx, k.BaseKeeper, &v2.EventCancelDerivativeOrder{
			MarketId:      marketID.Hex(),
			IsLimitCancel: true,
			LimitOrder:    order,
		})
	}

	return orders, cumulativeReduceOnlyQuantityToCancel
}

func (k DerivativeKeeper) cancelConditionalDerivativeLimitOrders(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	subaccountID common.Hash,
	orderHashes []common.Hash,
	isTriggerPriceHigher bool,
) {
	for _, hash := range orderHashes {
		triggerPriceHigher := isTriggerPriceHigher
		if err := k.CancelConditionalDerivativeLimitOrder(ctx, market, subaccountID, &triggerPriceHigher, hash); err != nil {
			metrics.ReportFuncError(k.svcTags)
			continue
		}
	}
}

func (k DerivativeKeeper) CreateDerivativeLimitOrder(
	ctx sdk.Context,
	sender sdk.AccAddress,
	order *v2.DerivativeOrder,
	market v2.DerivativeMarketI,
	markPrice math.LegacyDec,
) (hash common.Hash, err error) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	subaccountID := types.MustGetSubaccountIDOrDeriveFromNonce(sender, order.OrderInfo.SubaccountId)

	// set the actual subaccountID value in the order, since it might be a nonce value
	order.OrderInfo.SubaccountId = subaccountID.Hex()

	marketID := order.MarketID()

	metadata := k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, order.IsBuy())

	isMaker := order.OrderType.IsPostOnly()

	orderHash, err := k.EnsureValidDerivativeOrder(ctx, order, market, metadata, markPrice, false, nil, isMaker)
	if err != nil {
		metrics.ReportFuncError(k.svcTags)
		return orderHash, err
	}

	derivativeLimitOrder := v2.NewDerivativeLimitOrder(order, sender, orderHash)

	// Store the order in the conditionals store -or- transient limit order store and transient market indicator store
	if order.IsConditional() {
		// store the order in the conditional derivative market order store
		k.SetConditionalDerivativeLimitOrderWithMetadata(ctx, derivativeLimitOrder, metadata, marketID, markPrice)
		return orderHash, nil
	}

	if order.OrderType.IsPostOnly() {
		k.SetPostOnlyDerivativeLimitOrderWithMetadata(ctx, derivativeLimitOrder, metadata, marketID)
		return orderHash, nil
	}

	k.SetNewTransientDerivativeLimitOrderWithMetadata(
		ctx, derivativeLimitOrder, metadata, marketID, derivativeLimitOrder.IsBuy(), orderHash,
	)
	k.SetTransientSubaccountLimitOrderIndicator(ctx, marketID, subaccountID)
	k.feeDiscounts.CheckAndSetFeeDiscountAccountActivityIndicator(ctx, marketID, sender)

	return orderHash, nil
}

// CancelConditionalDerivativeMarketOrder cancels the conditional derivative market order
func (k DerivativeKeeper) CancelConditionalDerivativeMarketOrder(
	ctx sdk.Context,
	market v2.MarketI,
	subaccountID common.Hash,
	isTriggerPriceHigher *bool,
	orderHash common.Hash,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := market.MarketID()

	order, direction := k.GetConditionalDerivativeMarketOrderBySubaccountIDAndHash(
		ctx, marketID, isTriggerPriceHigher, subaccountID, orderHash)
	if order == nil {
		k.Logger(ctx).Debug(
			"Conditional Derivative Market Order doesn't exist to cancel",
			"marketId", marketID,
			"subaccountID", subaccountID,
			"orderHash", orderHash.Hex(),
		)
		metrics.ReportFuncError(k.svcTags)
		return errors.Wrap(types.ErrOrderDoesntExist, "Conditional Derivative Market Order doesn't exist")
	}

	if order.IsVanilla() {
		refundAmount := order.GetCancelRefundAmount()
		chainFormatRefundAmount := market.NotionalToChainFormat(refundAmount)
		k.subaccount.IncrementAvailableBalanceOrBank(ctx, order.SubaccountID(), market.GetQuoteDenom(), chainFormatRefundAmount)
	}

	// 2. Delete the order state from ordersStore and ordersIndexStore
	k.DeleteConditionalDerivativeOrder(
		ctx,
		false,
		marketID,
		order.SubaccountID(),
		direction,
		*order.TriggerPrice,
		order.Hash(),
		order.Cid(),
	)

	// 3. update metadata
	metadata := k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, order.IsBuy())
	if order.IsVanilla() {
		metadata.VanillaConditionalOrderCount--
	} else {
		metadata.ReduceOnlyConditionalOrderCount--
	}
	k.SetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, order.IsBuy(), metadata)

	events.Emit(ctx, k.BaseKeeper, &v2.EventCancelConditionalDerivativeOrder{
		MarketId:      marketID.Hex(),
		IsLimitCancel: false,
		MarketOrder:   order,
	})

	return nil
}

// CancelAllDerivativeMarketOrders cancels all of the derivative market orders for a given marketID.
func (k DerivativeKeeper) CancelAllDerivativeMarketOrders(ctx sdk.Context, market v2.DerivativeMarketI) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := market.MarketID()

	buyOrders := k.GetAllDerivativeMarketOrdersByMarketDirection(ctx, marketID, true)
	sellOrders := k.GetAllDerivativeMarketOrdersByMarketDirection(ctx, marketID, false)

	for _, order := range buyOrders {
		k.CancelDerivativeMarketOrder(ctx, market, order)
	}

	for _, order := range sellOrders {
		k.CancelDerivativeMarketOrder(ctx, market, order)
	}
}

func (k DerivativeKeeper) CancelDerivativeMarketOrder(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	order *v2.DerivativeMarketOrder,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := market.MarketID()
	subaccountID := order.SubaccountID()
	refundAmount := order.GetCancelRefundAmount()
	chainFormatRefund := market.NotionalToChainFormat(refundAmount)

	k.subaccount.IncrementAvailableBalanceOrBank(ctx, subaccountID, market.GetQuoteDenom(), chainFormatRefund)
	k.DeleteDerivativeMarketOrder(ctx, order, marketID)

	events.Emit(ctx, k.BaseKeeper, &v2.EventCancelDerivativeOrder{
		MarketId:      marketID.Hex(),
		IsLimitCancel: false,
		MarketOrderCancel: &v2.DerivativeMarketOrderCancel{
			MarketOrder:    order,
			CancelQuantity: order.OrderInfo.Quantity,
		},
	})
}

func (k DerivativeKeeper) TriggerConditionalDerivativeMarketOrder(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	markPrice math.LegacyDec,
	marketOrder *v2.DerivativeMarketOrder,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// skipCancel parameter was removed since the function was always called with skipCancel = true
	// if !skipCancel {
	// 	if err := k.CancelConditionalDerivativeMarketOrder(
	// 		ctx, market, marketOrder.OrderInfo.SubaccountID(), nil, marketOrder.Hash(),
	// 	); err != nil {
	// 		return err
	// 	}
	// }

	marketID := market.MarketID()

	senderAddr := types.SubaccountIDToSdkAddress(marketOrder.OrderInfo.SubaccountID())
	orderType := v2.OrderType_BUY
	if !marketOrder.IsBuy() {
		orderType = v2.OrderType_SELL
	}

	order := v2.DerivativeOrder{
		MarketId:     marketID.Hex(),
		OrderInfo:    marketOrder.OrderInfo,
		OrderType:    orderType,
		Margin:       marketOrder.Margin,
		TriggerPrice: nil,
	}

	orderMsg := v2.MsgCreateDerivativeMarketOrder{
		Sender: senderAddr.String(),
		Order:  order,
	}
	if err := orderMsg.ValidateBasic(); err != nil {
		return err
	}

	orderHash, _, err := k.CreateDerivativeMarketOrder(ctx, senderAddr, &order, market, markPrice)
	_ = orderHash
	if err != nil {
		return err
	}
	events.Emit(ctx, k.BaseKeeper, &v2.EventConditionalDerivativeOrderTrigger{
		MarketId:           marketID.Bytes(),
		IsLimitTrigger:     false,
		TriggeredOrderHash: marketOrder.OrderHash,
		PlacedOrderHash:    orderHash.Bytes(),
		TriggeredOrderCid:  marketOrder.Cid(),
	})

	return nil
}

// CancelMarketDerivativeOrdersForSubaccountUpToBalance cancels all of the derivative market orders for a given
// subaccount and marketID until the given balance has been freed up, i.e., total balance becoming available balance.
func (k DerivativeKeeper) CancelMarketDerivativeOrdersForSubaccountUpToBalance(
	ctx sdk.Context,
	market *v2.DerivativeMarket,
	subaccountID common.Hash,
	freeingUpBalance math.LegacyDec,
) (freedUpBalance math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	freedUpBalance = math.LegacyZeroDec()

	marketID := market.MarketID()
	marketBuyOrders := k.GetAllSubaccountDerivativeMarketOrdersByMarketDirection(ctx, marketID, subaccountID, true)

	for _, order := range marketBuyOrders {
		if freedUpBalance.GTE(freeingUpBalance) {
			return freedUpBalance
		}

		k.CancelDerivativeMarketOrder(ctx, market, order)
		freedUpBalance = freedUpBalance.Add(order.MarginHold)
	}

	marketSellOrders := k.GetAllSubaccountDerivativeMarketOrdersByMarketDirection(ctx, marketID, subaccountID, false)
	for _, order := range marketSellOrders {
		if freedUpBalance.GTE(freeingUpBalance) {
			return freedUpBalance
		}

		k.CancelDerivativeMarketOrder(ctx, market, order)
		freedUpBalance = freedUpBalance.Add(order.MarginHold)
	}

	return freedUpBalance
}

// CancelAllDerivativeMarketOrdersBySubaccountID cancels all of the derivative market orders for a given subaccount and marketID.
func (k DerivativeKeeper) CancelAllDerivativeMarketOrdersBySubaccountID(
	ctx sdk.Context,
	market *v2.DerivativeMarket,
	subaccountID common.Hash,
	marketID common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	buyOrders := k.GetAllSubaccountDerivativeMarketOrdersByMarketDirection(ctx, marketID, subaccountID, true)
	sellOrders := k.GetAllSubaccountDerivativeMarketOrdersByMarketDirection(ctx, marketID, subaccountID, false)

	for _, order := range buyOrders {
		k.CancelDerivativeMarketOrder(ctx, market, order)
	}

	for _, order := range sellOrders {
		k.CancelDerivativeMarketOrder(ctx, market, order)
	}
}

func (k DerivativeKeeper) SetConditionalDerivativeMarketOrderWithMetadata(
	ctx sdk.Context,
	order *v2.DerivativeMarketOrder,
	metadata *v2.SubaccountOrderbookMetadata,
	marketID common.Hash,
	markPrice math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var (
		subaccountID         = order.SubaccountID()
		isTriggerPriceHigher = order.TriggerPrice.GT(markPrice)
	)

	k.SetConditionalDerivativeMarketOrder(ctx, order, marketID, markPrice)

	if metadata == nil {
		metadata = k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, isTriggerPriceHigher)
	}

	if order.IsVanilla() {
		metadata.VanillaConditionalOrderCount++
	} else {
		metadata.ReduceOnlyConditionalOrderCount++
	}
	k.SetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, order.IsBuy(), metadata)

	newOrderEvent := &v2.EventNewConditionalDerivativeOrder{
		MarketId: marketID.Hex(),
		Order:    order.ToDerivativeOrder(marketID.String()),
		Hash:     order.OrderHash,
		IsMarket: true,
	}

	events.Emit(ctx, k.BaseKeeper, newOrderEvent)
}

// CancelAllConditionalDerivativeOrders cancels all resting conditional derivative orders for a given market.
func (k DerivativeKeeper) CancelAllConditionalDerivativeOrders(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := market.MarketID()

	orderbook := k.GetAllConditionalDerivativeOrdersUpToMarkPrice(ctx, marketID, nil)

	for _, limitOrder := range orderbook.GetLimitOrders() {
		if err := k.CancelConditionalDerivativeLimitOrder(ctx, market, limitOrder.SubaccountID(), nil, limitOrder.Hash()); err != nil {
			k.Logger(ctx).Error("CancelConditionalDerivativeLimitOrder failed during CancelAllConditionalDerivativeOrders:", err)
		}
	}

	for _, marketOrder := range orderbook.GetMarketOrders() {
		if err := k.CancelConditionalDerivativeMarketOrder(ctx, market, marketOrder.SubaccountID(), nil, marketOrder.Hash()); err != nil {
			k.Logger(ctx).Error("CancelConditionalDerivativeMarketOrder failed during CancelAllConditionalDerivativeOrders:", err)
		}
	}
}

func (k DerivativeKeeper) CreateDerivativeMarketOrder(
	ctx sdk.Context,
	sender sdk.AccAddress,
	derivativeOrder *v2.DerivativeOrder,
	market v2.DerivativeMarketI,
	markPrice math.LegacyDec,
) (orderHash common.Hash, results *v2.DerivativeMarketOrderResults, err error) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()
	var (
		subaccountID = types.MustGetSubaccountIDOrDeriveFromNonce(sender, derivativeOrder.OrderInfo.SubaccountId)
		marketID     = derivativeOrder.MarketID()
	)

	// set the actual subaccountID value in the order, since it might be a nonce value
	derivativeOrder.OrderInfo.SubaccountId = subaccountID.Hex()

	metadata := k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, derivativeOrder.IsBuy())

	var orderMarginHold math.LegacyDec
	orderHash, err = k.EnsureValidDerivativeOrder(ctx, derivativeOrder, market, metadata, markPrice, true, &orderMarginHold, false)
	if err != nil {
		metrics.ReportFuncError(k.svcTags)
		return orderHash, nil, err
	}

	if derivativeOrder.OrderType.IsAtomic() {
		err = k.EnsureValidAccessLevelForAtomicExecution(ctx, sender)
		if err != nil {
			return orderHash, nil, err
		}
	}

	marketOrder := v2.NewDerivativeMarketOrder(derivativeOrder, sender, orderHash)

	// 4. Check Order/Position Margin amount
	if marketOrder.IsVanilla() {
		// Check available balance to fund the market order
		marketOrder.MarginHold = orderMarginHold
	}

	return k.processDerivativeMarketOrder(ctx, marketID, marketOrder, derivativeOrder, market, markPrice, metadata, sender, orderHash)
}

func (k DerivativeKeeper) EnsureValidAccessLevelForAtomicExecution(
	ctx sdk.Context,
	sender sdk.AccAddress,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	switch k.GetParams(ctx).AtomicMarketOrderAccessLevel {
	case v2.AtomicMarketOrderAccessLevel_Nobody:
		return types.ErrInvalidAccessLevel
	case v2.AtomicMarketOrderAccessLevel_SmartContractsOnly:
		if !k.wasm.HasContractInfo(ctx, sender) { // sender is not a smart-contract
			metrics.ReportFuncError(k.svcTags)
			return types.ErrInvalidAccessLevel
		}
	default:
	}
	// TODO: handle AtomicMarketOrderAccessLevel_BeginBlockerSmartContractsOnly level
	return nil
}

//nolint:revive // ok
func (k DerivativeKeeper) processDerivativeMarketOrder(
	ctx sdk.Context,
	marketID common.Hash,
	marketOrder *v2.DerivativeMarketOrder,
	derivativeOrder *v2.DerivativeOrder,
	market v2.DerivativeMarketI,
	markPrice math.LegacyDec,
	metadata *v2.SubaccountOrderbookMetadata,
	sender sdk.AccAddress,
	orderHash common.Hash,
) (common.Hash, *v2.DerivativeMarketOrderResults, error) {
	if derivativeOrder.IsConditional() {
		k.SetConditionalDerivativeMarketOrderWithMetadata(ctx, marketOrder, metadata, marketID, markPrice)
		return orderHash, nil, nil
	}

	if derivativeOrder.OrderType.IsAtomic() {
		return k.processAtomicDerivativeMarketOrder(ctx, market, markPrice, marketOrder, marketID, orderHash)
	}

	// 5. Store the order in the transient derivative market order store and transient market indicator store
	k.SetTransientDerivativeMarketOrder(ctx, marketOrder, derivativeOrder, orderHash)
	k.SetTransientSubaccountMarketOrderIndicator(ctx, marketID, marketOrder.SubaccountID())
	k.feeDiscounts.CheckAndSetFeeDiscountAccountActivityIndicator(ctx, marketID, sender)

	return orderHash, nil, nil
}

func (k DerivativeKeeper) processAtomicDerivativeMarketOrder(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	markPrice math.LegacyDec,
	marketOrder *v2.DerivativeMarketOrder,
	marketID common.Hash,
	orderHash common.Hash,
) (common.Hash, *v2.DerivativeMarketOrderResults, error) {
	var funding *v2.PerpetualMarketFunding
	if market.GetIsPerpetual() {
		funding = k.GetPerpetualMarketFunding(ctx, marketID)
	}
	positionStates := v2.NewPositionStates()
	positionCache := make(map[common.Hash]*v2.Position)

	results, isMarketSolvent, err := k.ExecuteDerivativeMarketOrderImmediately(
		ctx,
		market,
		markPrice,
		funding,
		marketOrder,
		positionStates,
		positionCache,
		false,
	)
	if err != nil {
		return orderHash, nil, err
	}

	if !isMarketSolvent {
		return orderHash, nil, types.ErrInsufficientMarketBalance
	}

	k.feeDiscounts.CheckAndSetFeeDiscountAccountActivityIndicator(ctx, marketID, marketOrder.SdkAccAddress())
	return orderHash, results, nil
}

func (k DerivativeKeeper) cancelConditionalDerivativeMarketOrders(
	ctx sdk.Context,
	market v2.DerivativeMarketI,
	subaccountID common.Hash,
	orderHashes []common.Hash,
	isTriggerPriceHigher bool,
) {
	for _, hash := range orderHashes {
		triggerPriceHigher := isTriggerPriceHigher
		if err := k.CancelConditionalDerivativeMarketOrder(ctx, market, subaccountID, &triggerPriceHigher, hash); err != nil {
			metrics.ReportFuncError(k.svcTags)
			continue
		}
	}
}

func (k DerivativeKeeper) processMarketForTriggeredOrders(ctx sdk.Context, market *v2.DerivativeMarket) *v2.TriggeredOrdersInMarket {
	marketID := market.MarketID()

	markPrice, _ := k.GetDerivativeMarketPrice(ctx, market.OracleBase, market.OracleQuote, market.OracleScaleFactor, market.OracleType)
	if markPrice == nil || markPrice.IsNil() {
		return nil
	}

	orderbook := k.GetAllConditionalDerivativeOrdersUpToMarkPrice(ctx, marketID, markPrice)

	triggeredOrders := &v2.TriggeredOrdersInMarket{
		Market:             market,
		MarkPrice:          *markPrice,
		MarketOrders:       orderbook.GetMarketOrders(),
		LimitOrders:        orderbook.GetLimitOrders(),
		HasLimitBuyOrders:  orderbook.HasLimitBuyOrders(),
		HasLimitSellOrders: orderbook.HasLimitSellOrders(),
	}

	if len(triggeredOrders.MarketOrders) == 0 && len(triggeredOrders.LimitOrders) == 0 {
		return nil
	}

	return triggeredOrders
}

//nolint:revive // ok
func (k DerivativeKeeper) EnsureValidDerivativeOrder(
	ctx sdk.Context,
	derivativeOrder *v2.DerivativeOrder,
	market v2.DerivativeMarketI,
	metadata *v2.SubaccountOrderbookMetadata,
	markPrice math.LegacyDec,
	isMarketOrder bool,
	orderMarginHold *math.LegacyDec,
	isMaker bool,
) (orderHash common.Hash, err error) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var (
		subaccountID = derivativeOrder.SubaccountID()
		marketID     = derivativeOrder.MarketID()
		marketType   = market.GetMarketType()
	)

	// always increase nonce first
	subaccountNonce := k.subaccount.IncrementSubaccountTradeNonce(ctx, subaccountID)

	orderHash, err = derivativeOrder.ComputeOrderHash(subaccountNonce.Nonce)
	if err != nil {
		metrics.ReportFuncError(k.svcTags)
		return orderHash, err
	}

	// reject if client order id is already used
	if k.ExistsCid(ctx, subaccountID, derivativeOrder.OrderInfo.Cid) {
		metrics.ReportFuncError(k.svcTags)
		return orderHash, types.ErrClientOrderIdAlreadyExists
	}

	if derivativeOrder.ExpirationBlock != 0 {
		if isMarketOrder {
			metrics.ReportFuncError(k.svcTags)
			return orderHash, types.ErrInvalidExpirationBlock.Wrap("market orders cannot have expiration block")
		}

		if derivativeOrder.ExpirationBlock <= ctx.BlockHeight() {
			metrics.ReportFuncError(k.svcTags)
			return orderHash, types.ErrInvalidExpirationBlock.Wrap("expiration block must be higher than current block")
		}
	}

	doesOrderCrossTopOfBook := k.DerivativeOrderCrossesTopOfBook(ctx, derivativeOrder)

	isPostOnlyMode := k.IsPostOnlyMode(ctx)

	if isMarketOrder && isPostOnlyMode {
		metrics.ReportFuncError(k.svcTags)
		return orderHash, errors.Wrapf(types.ErrPostOnlyMode, "cannot create market orders in post only mode until height %d", k.GetParams(ctx).PostOnlyModeHeightThreshold)
	}

	// enforce that post only limit orders don't cross the top of the book
	if (derivativeOrder.OrderType.IsPostOnly() || isPostOnlyMode) && doesOrderCrossTopOfBook {
		metrics.ReportFuncError(k.svcTags)
		return orderHash, types.ErrExceedsTopOfBookPrice
	}

	// enforce that market orders cross TOB
	if !derivativeOrder.IsConditional() && isMarketOrder && !doesOrderCrossTopOfBook {
		metrics.ReportFuncError(k.svcTags)
		return orderHash, types.ErrSlippageExceedsWorstPrice
	}

	// allow single vanilla market order in each block in order to prevent inconsistencies in metadata (since market orders don't update metadata upon placement for simplicity purposes)
	if !derivativeOrder.IsConditional() && isMarketOrder && k.HasSubaccountAlreadyPlacedMarketOrder(ctx, marketID, subaccountID) {
		return orderHash, types.ErrMarketOrderAlreadyExists
	}

	// check that market exists and has mark price (except for non-conditional binary options)
	isMissingRequiredMarkPrice := (!marketType.IsBinaryOptions() || derivativeOrder.IsConditional()) && markPrice.IsNil()
	if market == nil || isMissingRequiredMarkPrice {
		metrics.ReportFuncError(k.svcTags)
		k.Logger(ctx).Debug("active market with valid mark price doesn't exist", "marketId", derivativeOrder.MarketId, "mark price", markPrice)
		return orderHash, errors.Wrapf(types.ErrDerivativeMarketNotFound, "active derivative market for marketID %s not found", derivativeOrder.MarketId)
	}

	if err := derivativeOrder.CheckValidConditionalPrice(markPrice); err != nil {
		metrics.ReportFuncError(k.svcTags)
		return orderHash, err
	}

	if err := derivativeOrder.CheckTickSize(market.GetMinPriceTickSize(), market.GetMinQuantityTickSize()); err != nil {
		metrics.ReportFuncError(k.svcTags)
		return orderHash, err
	}

	if err := derivativeOrder.CheckNotional(market.GetMinNotional()); err != nil {
		metrics.ReportFuncError(k.svcTags)
		return orderHash, err
	}

	// check binary options max order prices
	if marketType.IsBinaryOptions() {
		if err := derivativeOrder.CheckBinaryOptionsPricesWithinBounds(market.GetOracleScaleFactor()); err != nil {
			metrics.ReportFuncError(k.svcTags)
			return orderHash, err
		}
	}

	// only limit number of conditional (both market & limit) & regular limit orders
	shouldRestrictOrderSideCount := derivativeOrder.IsConditional() || !isMarketOrder
	if shouldRestrictOrderSideCount && metadata.GetOrderSideCount() >= k.GetParams(ctx).MaxDerivativeOrderSideCount {
		metrics.ReportFuncError(k.svcTags)
		return orderHash, types.ErrExceedsOrderSideCount
	}

	// also limit conditional market orders: 1 per subaccount per market per side
	if derivativeOrder.IsConditional() && isMarketOrder {
		isHigher := derivativeOrder.TriggerPrice.GT(markPrice)
		if k.HasSubaccountAlreadyPlacedConditionalMarketOrderInDirection(ctx, marketID, subaccountID, isHigher, marketType) {
			metrics.ReportFuncError(k.svcTags)
			return orderHash, types.ErrConditionalMarketOrderAlreadyExists
		}
	}

	position := k.GetPosition(ctx, marketID, subaccountID)

	var tradeFeeRate math.LegacyDec
	if isMaker {
		tradeFeeRate = market.GetMakerFeeRate()
	} else {
		tradeFeeRate = market.GetTakerFeeRate()
		if derivativeOrder.OrderType.IsAtomic() {
			tradeFeeRate = tradeFeeRate.Mul(k.GetMarketAtomicExecutionFeeMultiplier(ctx, marketID, market.GetMarketType()))
		}
	}

	if derivativeOrder.IsConditional() {
		// for conditional orders we skip position validation, it will be checked after conversion
		// what we should enforce here is basic requirements of at least some margin is locked
		// (on any side, spam protection) before posting conditional orders

		// outer IF only checks that margin is locked on the same side (isBuy() side) of the order
		if derivativeOrder.IsReduceOnly() &&
			position == nil &&
			metadata.VanillaLimitOrderCount == 0 &&
			metadata.VanillaConditionalOrderCount == 0 {
			// inner IF is checking that we have some margin locked on the opposite side
			oppositeMetadata := k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, !derivativeOrder.IsBuy())
			if oppositeMetadata.VanillaLimitOrderCount == 0 && oppositeMetadata.VanillaConditionalOrderCount == 0 {
				metrics.ReportFuncError(k.svcTags)
				return orderHash, errors.Wrapf(types.ErrNoMarginLocked, "Should have a position or open vanilla orders before posting conditional reduce-only orders")
			}
		}
	} else {
		// check that the position can actually be closed (position is not beyond bankruptcy)
		isClosingPosition := position != nil && derivativeOrder.IsBuy() != position.IsLong && position.Quantity.IsPositive()
		if isClosingPosition {
			var funding *v2.PerpetualMarketFunding
			if marketType.IsPerpetual() {
				funding = k.GetPerpetualMarketFunding(ctx, marketID)
			}
			// Check that the order can close the position
			if err := position.CheckValidPositionToReduce(
				marketType,
				derivativeOrder.Price(),
				derivativeOrder.IsBuy(),
				tradeFeeRate,
				funding,
				derivativeOrder.Margin,
			); err != nil {
				metrics.ReportFuncError(k.svcTags)
				return orderHash, err
			}
		}

		if derivativeOrder.IsReduceOnly() {
			if position == nil {
				metrics.ReportFuncError(k.svcTags)
				return orderHash, errors.Wrapf(
					types.ErrPositionNotFound,
					"Position for marketID %s subaccountID %s not found",
					marketID,
					subaccountID,
				)
			}

			if derivativeOrder.IsBuy() == position.IsLong {
				metrics.ReportFuncError(k.svcTags)
				return orderHash, types.ErrInvalidReduceOnlyPositionDirection
			}
		}
	}

	// Check Order/Position Margin amount
	if derivativeOrder.IsVanilla() {
		// Reject if the subaccount's available deposits does not have at least the required funds for the trade
		var markPriceToCheck = markPrice
		if derivativeOrder.IsConditional() {
			markPriceToCheck = *derivativeOrder.TriggerPrice // for conditionals trigger price == mark price at the point in the future when the order will materialize
		}
		marginHold, err := derivativeOrder.CheckMarginAndGetMarginHold(
			market.GetInitialMarginRatio(),
			markPriceToCheck,
			tradeFeeRate,
			marketType,
			market.GetOracleScaleFactor(),
		)
		if err != nil {
			metrics.ReportFuncError(k.svcTags)
			return orderHash, err
		}

		// Decrement the available balance by the funds amount needed to fund the order
		chainFormattedMarginHold := market.NotionalToChainFormat(marginHold)
		if err := k.subaccount.ChargeAccount(ctx, subaccountID, market.GetQuoteDenom(), chainFormattedMarginHold); err != nil {
			metrics.ReportFuncError(k.svcTags)
			return orderHash, err
		}

		// set back order margin hold
		if orderMarginHold != nil {
			*orderMarginHold = marginHold
		}
	}

	if !derivativeOrder.IsConditional() {
		if err := k.resolveReduceOnlyConflicts(ctx, derivativeOrder, subaccountID, marketID, metadata, position); err != nil {
			metrics.ReportFuncError(k.svcTags)
			return orderHash, err
		}
	}
	return orderHash, nil
}

func (k DerivativeKeeper) resolveReduceOnlyConflicts(
	ctx sdk.Context,
	order types.IMutableDerivativeOrder,
	subaccountID common.Hash,
	marketID common.Hash,
	metadata *v2.SubaccountOrderbookMetadata,
	position *v2.Position,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if position == nil || position.IsLong == order.IsBuy() {
		return nil
	}

	// For an opposing position, if position.quantity < order.FillableQuantity + AggregateReduceOnlyQuantity + AggregateVanillaQuantity
	// the new order might invalidate some existing reduce-only orders or itself be invalid (if it's reduce-only).
	cumulativeOrderSideQuantity := order.GetQuantity().Add(metadata.AggregateReduceOnlyQuantity).Add(metadata.AggregateVanillaQuantity)

	hasPotentialOrdersConflict := position.Quantity.LT(cumulativeOrderSideQuantity)
	if !hasPotentialOrdersConflict {
		return nil
	}

	subaccountEOBOrderResults := k.GetEqualOrBetterPricedSubaccountOrderResults(ctx, marketID, subaccountID, order)

	if order.IsReduceOnly() {
		if err := k.resizeNewReduceOnlyIfRequired(ctx, metadata, order, position, subaccountEOBOrderResults); err != nil {
			return err
		}
	}

	k.cancelWorseOrdersToCancelIfRequired(ctx, marketID, subaccountID, metadata, order, position, subaccountEOBOrderResults)
	return nil
}

func (k DerivativeKeeper) cancelWorseOrdersToCancelIfRequired(
	ctx sdk.Context,
	marketID, subaccountID common.Hash,
	metadata *v2.SubaccountOrderbookMetadata,
	newOrder types.IDerivativeOrder,
	position *v2.Position,
	eobResults *v2.SubaccountOrderResults,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	maxRoQuantityToCancel := metadata.AggregateReduceOnlyQuantity.Sub(eobResults.GetCumulativeBetterReduceOnlyQuantity())
	if maxRoQuantityToCancel.IsNegative() || maxRoQuantityToCancel.IsZero() {
		return
	}

	k.CancelMinimumReduceOnlyOrders(ctx, marketID, subaccountID, metadata, newOrder.IsBuy(), position.Quantity, eobResults, newOrder)
}

//nolint:revive //ok
func (k DerivativeKeeper) CancelMinimumReduceOnlyOrders(
	ctx sdk.Context,
	marketID, subaccountID common.Hash,
	metadata *v2.SubaccountOrderbookMetadata,
	isReduceOnlyDirectionBuy bool,
	positionQuantity math.LegacyDec,
	eobResults *v2.SubaccountOrderResults,
	newOrder types.IDerivativeOrder,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()
	worstROandBetterOrders, totalQuantityFromWorstRO := k.GetWorstROAndAllBetterPricedSubaccountOrders(
		ctx,
		marketID,
		subaccountID,
		metadata.AggregateReduceOnlyQuantity,
		isReduceOnlyDirectionBuy,
		eobResults,
	)

	// positionFlippingQuantity - quantity by which orders from worst RO order surpass position size
	// and would cause flipping via RO orders which must be prevented
	positionFlippingQuantity := totalQuantityFromWorstRO.Sub(positionQuantity)

	isAddingNewOrder := newOrder != nil

	if isAddingNewOrder {
		positionFlippingQuantity = positionFlippingQuantity.Add(newOrder.GetQuantity())
	}

	if !positionFlippingQuantity.IsPositive() {
		return
	}

	checkedFlippingQuantity, totalReduceOnlyCancelQuantity := math.LegacyZeroDec(), math.LegacyZeroDec()
	ordersToCancel := make([]*v2.SubaccountOrderData, 0)

	for _, order := range worstROandBetterOrders {
		if isAddingNewOrder &&
			((newOrder.IsBuy() && order.Order.Price.GT(newOrder.GetPrice())) ||
				(!newOrder.IsBuy() && order.Order.Price.LT(newOrder.GetPrice()))) {
			break
		}

		if order.Order.IsReduceOnly {
			ordersToCancel = append(ordersToCancel, order)
			totalReduceOnlyCancelQuantity = totalReduceOnlyCancelQuantity.Add(order.Order.Quantity)
		}

		checkedFlippingQuantity = checkedFlippingQuantity.Add(order.Order.Quantity)
		if checkedFlippingQuantity.GTE(positionFlippingQuantity) {
			break
		}
	}

	k.CancelReduceOnlyOrders(ctx, marketID, subaccountID, metadata, isReduceOnlyDirectionBuy, totalReduceOnlyCancelQuantity, ordersToCancel)
}

func (DerivativeKeeper) resizeNewReduceOnlyIfRequired(
	_ sdk.Context,
	metadata *v2.SubaccountOrderbookMetadata,
	order types.IMutableDerivativeOrder,
	position *v2.Position,
	betterOrEqualOrders *v2.SubaccountOrderResults,
) error {
	existingClosingQuantity := betterOrEqualOrders.GetCumulativeEOBReduceOnlyQuantity().
		Add(betterOrEqualOrders.GetCumulativeEOBVanillaQuantity())
	reducibleQuantity := position.Quantity.Sub(existingClosingQuantity)

	hasReducibleQuantity := reducibleQuantity.IsPositive()
	if !hasReducibleQuantity {
		//nolint:revive //ok
		return errors.Wrapf(types.ErrInsufficientPositionQuantity, "position quantity %s > AggregateReduceOnlyQuantity %s + CumulativeEOBVanillaQuantity %s must hold", utils.GetReadableDec(position.Quantity), utils.GetReadableDec(metadata.AggregateReduceOnlyQuantity), utils.GetReadableDec(betterOrEqualOrders.GetCumulativeEOBVanillaQuantity()))
	}

	// min() is a defensive programming check, should always be reducibleQuantity, otherwise we wouldn't reach this point
	newResizedOrderQuantity := math.LegacyMinDec(order.GetQuantity(), reducibleQuantity)
	if newResizedOrderQuantity.GTE(order.GetQuantity()) {
		return nil
	}

	return types.ResizeReduceOnlyOrder(order, newResizedOrderQuantity)
}

func (k DerivativeKeeper) CancelReduceOnlyOrders(
	ctx sdk.Context,
	marketID, subaccountID common.Hash,
	metadata *v2.SubaccountOrderbookMetadata,
	isBuy bool,
	totalReduceOnlyCancelQuantity math.LegacyDec,
	ordersToCancel []*v2.SubaccountOrderData,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if len(ordersToCancel) == 0 {
		return
	}

	k.CancelReduceOnlySubaccountOrders(ctx, marketID, subaccountID, isBuy, ordersToCancel)
	metadata.ReduceOnlyLimitOrderCount -= uint32(len(ordersToCancel))
	metadata.AggregateReduceOnlyQuantity = metadata.AggregateReduceOnlyQuantity.Sub(totalReduceOnlyCancelQuantity)
	k.SetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, isBuy, metadata)
}

// GetWorstROAndAllBetterPricedSubaccountOrders returns the subaccount orders starting with the worst priced
// reduce-only order for a given direction. Shouldn't be used if betterResults contains already all RO
//
//nolint:revive // ok
func (k DerivativeKeeper) GetWorstROAndAllBetterPricedSubaccountOrders(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
	totalROQuantity math.LegacyDec,
	isBuy bool,
	eobResults *v2.SubaccountOrderResults,
) (worstROandBetterOrders []*v2.SubaccountOrderData, totalQuantityFromWorstRO math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	foundROQuantity := eobResults.GetCumulativeEOBReduceOnlyQuantity()
	totalQuantityFromWorstRO = eobResults.GetCumulativeEOBReduceOnlyQuantity().Add(eobResults.GetCumulativeEOBVanillaQuantity())

	worstROandBetterOrders = make([]*v2.SubaccountOrderData, 0, len(eobResults.VanillaOrders)+len(eobResults.ReduceOnlyOrders))
	worstROandBetterOrders = append(worstROandBetterOrders, eobResults.VanillaOrders...)
	worstROandBetterOrders = append(worstROandBetterOrders, eobResults.ReduceOnlyOrders...)

	worstROPrice := math.LegacyZeroDec()

	processOrder := func(order *v2.SubaccountOrder, orderHash common.Hash) (stop bool) {
		if foundROQuantity.GTE(totalROQuantity) {
			doesVanillaWithSameWorstROPriceExist := order.Price.Equal(worstROPrice)

			// no guarantee which one would be matched first, need to include same priced vanillas too
			if !doesVanillaWithSameWorstROPriceExist {
				return true
			}
		}

		totalQuantityFromWorstRO = totalQuantityFromWorstRO.Add(order.Quantity)
		worstROandBetterOrders = append(worstROandBetterOrders, &v2.SubaccountOrderData{
			Order:     order,
			OrderHash: orderHash.Bytes(),
		})

		if order.IsReduceOnly {
			foundROQuantity = foundROQuantity.Add(order.Quantity)
			worstROPrice = order.Price
		}

		return false
	}

	var startOrderKey []byte
	if eobResults.LastFoundOrderHash != nil {
		startOrderKey = types.GetSubaccountOrderIterationKey(*eobResults.LastFoundOrderPrice, *eobResults.LastFoundOrderHash)
	}

	k.IterateSubaccountOrdersStartingFromOrder(ctx, marketID, subaccountID, isBuy, true, startOrderKey, processOrder)

	sort.SliceStable(worstROandBetterOrders, func(i, j int) bool {
		if worstROandBetterOrders[i].Order.Price.Equal(worstROandBetterOrders[j].Order.Price) {
			return worstROandBetterOrders[i].Order.IsReduceOnly
		}

		if isBuy {
			return worstROandBetterOrders[i].Order.Price.LT(worstROandBetterOrders[j].Order.Price)
		}

		return worstROandBetterOrders[i].Order.Price.GT(worstROandBetterOrders[j].Order.Price)
	})

	return worstROandBetterOrders, totalQuantityFromWorstRO
}

// GetEqualOrBetterPricedSubaccountOrderResults does.
func (k DerivativeKeeper) GetEqualOrBetterPricedSubaccountOrderResults(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
	order types.IDerivativeOrder,
) *v2.SubaccountOrderResults {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	isBuy := order.IsBuy()
	price := order.GetPrice()
	results := v2.NewSubaccountOrderResults()

	processOrder := func(order *v2.SubaccountOrder, orderHash common.Hash) (stop bool) {
		if isBuy && order.Price.LT(price) || !isBuy && order.Price.GT(price) {
			return true
		}
		results.LastFoundOrderHash = &orderHash
		results.LastFoundOrderPrice = &order.Price

		results.AddSubaccountOrder(&v2.SubaccountOrderData{
			Order:     order,
			OrderHash: orderHash.Bytes(),
		})

		if !price.Equal(order.Price) && order.IsReduceOnly {
			results.IncrementCumulativeBetterReduceOnlyQuantity(order.Quantity)
		}
		return false
	}

	k.IterateSubaccountOrdersStartingFromOrder(ctx, marketID, subaccountID, isBuy, true, nil, processOrder)
	return results
}

func (k DerivativeKeeper) HasSubaccountAlreadyPlacedConditionalMarketOrderInDirection(
	ctx sdk.Context,
	marketID,
	subaccountID common.Hash,
	isTriggerPriceHigher bool,
	_ types.MarketType,
) bool {
	// TODO: extract into HasConditionalMarketOrder
	var existingOrderHash *common.Hash
	k.IterateConditionalOrdersBySubaccount(
		ctx,
		marketID,
		subaccountID,
		isTriggerPriceHigher,
		true,
		func(orderHash common.Hash) (stop bool) {
			existingOrderHash = &orderHash
			return true
		},
	)

	return existingOrderHash != nil
}

// GetAllConditionalOrderHashesBySubaccountAndMarket gets all the conditional derivative orders for a given subaccountID and marketID
func (k DerivativeKeeper) GetAllConditionalOrderHashesBySubaccountAndMarket(
	ctx sdk.Context,
	marketID common.Hash,
	isTriggerPriceHigher bool,
	isMarketOrders bool,
	subaccountID common.Hash,
) (orderHashes []common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orderHashes = make([]common.Hash, 0)
	k.IterateConditionalOrdersBySubaccount(
		ctx,
		marketID,
		subaccountID,
		isTriggerPriceHigher,
		isMarketOrders,
		func(orderHash common.Hash) (stop bool) {
			orderHashes = append(orderHashes, orderHash)
			return false
		},
	)

	return orderHashes
}

func (k DerivativeKeeper) GetAllTransientDerivativeLimitOrdersWithPotentiallyConflictingReduceOnlyOrders(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	modifiedPositionCache v2.ModifiedPositionCache,
) ([]*v2.DerivativeLimitOrder, v2.ReduceOnlyOrdersTracker) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.DerivativeLimitOrder, 0)

	roTracker := v2.NewReduceOnlyOrdersTracker()
	hasAnyModifiedPositionsInMarket := modifiedPositionCache.HasAnyModifiedPositionsInMarket(marketID)

	appendOrder := func(o *v2.DerivativeLimitOrder) (stop bool) {
		orders = append(orders, o)

		if !hasAnyModifiedPositionsInMarket {
			return false
		}

		if o.IsReduceOnly() && modifiedPositionCache.HasPositionBeenModified(marketID, o.SubaccountID()) {
			roTracker.AppendOrder(o.SubaccountID(), o)
		}

		return false
	}

	k.IterateTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID(
		ctx,
		marketID,
		isBuy,
		appendOrder,
	)

	return orders, roTracker
}

// GetAllDerivativeLimitOrdersByMarketID returns all of the Derivative Limit Orders for a given marketID.
func (k DerivativeKeeper) GetAllDerivativeLimitOrdersByMarketID(ctx sdk.Context, marketID common.Hash) (orders []*v2.DerivativeLimitOrder) {
	buyOrderbook := k.GetAllDerivativeLimitOrdersByMarketDirection(ctx, marketID, true)
	sellOrderbook := k.GetAllDerivativeLimitOrdersByMarketDirection(ctx, marketID, false)

	return append(buyOrderbook, sellOrderbook...)
}

// GetAllPositionsByMarket returns all positions in a given derivative market
func (k DerivativeKeeper) GetAllPositionsByMarket(ctx sdk.Context, marketID common.Hash) []*v2.DerivativePosition {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	positions := make([]*v2.DerivativePosition, 0)
	appendPosition := func(p *v2.Position, key []byte) (stop bool) {
		subaccountID := types.GetSubaccountIDFromPositionKey(key)

		derivativePosition := &v2.DerivativePosition{
			SubaccountId: subaccountID.Hex(),
			MarketId:     marketID.Hex(),
			Position:     p,
		}
		positions = append(positions, derivativePosition)
		return false
	}

	k.IteratePositionsByMarket(ctx, marketID, appendPosition)

	return positions
}

// GetAllSubaccountDerivativeMarketOrdersByMarketDirection retrieves all of a subaccount's
// DerivativeMarketOrders for a given market and direction.
func (k DerivativeKeeper) GetAllSubaccountDerivativeMarketOrdersByMarketDirection(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
	isBuy bool,
) []*v2.DerivativeMarketOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.DerivativeMarketOrder, 0)
	appendOrder := func(order *v2.DerivativeMarketOrder) (stop bool) {
		// only append orders with the same subaccountID
		if bytes.Equal(order.OrderInfo.SubaccountID().Bytes(), subaccountID.Bytes()) {
			orders = append(orders, order)
		}
		return false
	}

	k.IterateDerivativeMarketOrders(ctx, marketID, isBuy, appendOrder)

	return orders
}

// GetAllTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID retrieves all transient DerivativeLimitOrders
// for a given market, subaccountID and direction.
//
//nolint:revive // ok
func (k DerivativeKeeper) GetAllTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID *common.Hash,
	isBuy bool,
) []*v2.DerivativeLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.DerivativeLimitOrder, 0)
	appendOrder := func(o *v2.DerivativeLimitOrder) (stop bool) {
		// only append orders with the same subaccountID
		if subaccountID == nil || bytes.Equal(o.OrderInfo.SubaccountID().Bytes(), subaccountID.Bytes()) {
			orders = append(orders, o)
		}
		return false
	}

	k.IterateTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID(
		ctx,
		marketID,
		isBuy,
		appendOrder,
	)

	return orders
}

// GetAllDerivativeMarketOrdersByMarketDirection retrieves all of DerivativeMarketOrders for a given market and direction.
func (k DerivativeKeeper) GetAllDerivativeMarketOrdersByMarketDirection(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) []*v2.DerivativeMarketOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.DerivativeMarketOrder, 0)
	appendOrder := func(order *v2.DerivativeMarketOrder) (stop bool) {
		orders = append(orders, order)
		return false
	}

	k.IterateDerivativeMarketOrders(ctx, marketID, isBuy, appendOrder)
	return orders
}

// GetAllTransientDerivativeLimitOrdersByMarketDirection retrieves all transient DerivativeLimitOrders for a given market and direction.
func (k DerivativeKeeper) GetAllTransientDerivativeLimitOrdersByMarketDirection(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) []*v2.DerivativeLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	return k.GetAllTransientDerivativeLimitOrdersByMarketDirectionBySubaccountID(ctx, marketID, nil, isBuy)
}

// GetAllTransientDerivativeMarketOrdersByMarketDirection retrieves all transient DerivativeMarketOrders for a given market and direction.
func (k DerivativeKeeper) GetAllTransientDerivativeMarketOrdersByMarketDirection(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) []*v2.DerivativeMarketOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.DerivativeMarketOrder, 0)
	appendOrder := func(order *v2.DerivativeMarketOrder) (stop bool) {
		orders = append(orders, order)
		return false
	}

	k.IterateDerivativeMarketOrders(ctx, marketID, isBuy, appendOrder)
	return orders
}

// GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket gets all the derivative limit orders for
// a given direction for a given subaccountID and marketID
func (k DerivativeKeeper) GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	subaccountID common.Hash,
) (orderHashes []common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orderHashes = make([]common.Hash, 0)
	appendOrderHash := func(orderHash common.Hash) (stop bool) {
		orderHashes = append(orderHashes, orderHash)
		return false
	}

	k.IterateRestingDerivativeLimitOrderHashesBySubaccount(ctx, marketID, isBuy, subaccountID, appendOrderHash)
	return orderHashes
}

// GetAllTransientTraderDerivativeLimitOrders gets all the transient derivative limit orders for a given subaccountID and marketID
func (k DerivativeKeeper) GetAllTransientTraderDerivativeLimitOrders(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
) []*v2.TrimmedDerivativeLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.TrimmedDerivativeLimitOrder, 0)
	appendOrder := func(order *v2.DerivativeLimitOrder) (stop bool) {
		orders = append(orders, order.ToTrimmed())
		return false
	}

	k.IterateTransientDerivativeLimitOrdersBySubaccount(ctx, marketID, true, subaccountID, appendOrder)
	k.IterateTransientDerivativeLimitOrdersBySubaccount(ctx, marketID, false, subaccountID, appendOrder)

	return orders
}

// GetAllTransientDerivativeLimitOrderbook returns all transient orderbooks for all derivative markets.
func (k DerivativeKeeper) GetAllTransientDerivativeLimitOrderbook(ctx sdk.Context) []v2.DerivativeOrderBook {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	markets := k.GetAllDerivativeMarkets(ctx)
	orderbook := make([]v2.DerivativeOrderBook, 0, len(markets)*2)
	for _, market := range markets {
		marketID := market.MarketID()
		buyOrders := k.GetAllDerivativeLimitOrdersByMarketDirection(ctx, marketID, true)
		orderbook = append(orderbook, v2.DerivativeOrderBook{
			MarketId:  marketID.Hex(),
			IsBuySide: true,
			Orders:    buyOrders,
		})
		sellOrders := k.GetAllDerivativeLimitOrdersByMarketDirection(ctx, marketID, false)
		orderbook = append(orderbook, v2.DerivativeOrderBook{
			MarketId:  marketID.Hex(),
			IsBuySide: false,
			Orders:    sellOrders,
		})
	}

	return orderbook
}

func (k DerivativeKeeper) GetAllStandardizedDerivativeLimitOrdersByMarketDirection(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) (orders []*v2.TrimmedLimitOrder) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders = make([]*v2.TrimmedLimitOrder, 0)
	appendOrder := func(order *v2.DerivativeLimitOrder) (stop bool) {
		orders = append(orders, order.ToStandardized())
		return false
	}

	k.IterateDerivativeLimitOrdersByMarketDirection(ctx, marketID, isBuy, appendOrder)
	return orders
}

// CancelRestingDerivativeLimitOrdersForSubaccountUpToBalance cancels all of the derivative limit orders for a
// given subaccount and marketID until the given balance has been freed up, i.e., total balance becoming
// available balance.
func (k DerivativeKeeper) CancelRestingDerivativeLimitOrdersForSubaccountUpToBalance(
	ctx sdk.Context,
	market *v2.DerivativeMarket,
	subaccountID common.Hash,
	freeingUpBalance math.LegacyDec,
) (freedUpBalance math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	freedUpBalance = math.LegacyZeroDec()

	marketID := market.MarketID()
	positiveFeePart := math.LegacyMaxDec(math.LegacyZeroDec(), market.MakerFeeRate)

	restingBuyOrderHashes := k.GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(ctx, marketID, true, subaccountID)

	for _, hash := range restingBuyOrderHashes {
		if freedUpBalance.GTE(freeingUpBalance) {
			return freedUpBalance
		}

		isBuy := true
		order := k.GetDerivativeLimitOrderBySubaccountIDAndHash(ctx, marketID, &isBuy, subaccountID, hash)
		if err := k.CancelRestingDerivativeLimitOrder(ctx, market, subaccountID, &isBuy, hash, false, true); err != nil {
			metrics.ReportFuncError(k.svcTags)
			events.Emit(ctx, k.BaseKeeper, v2.NewEventOrderCancelFail(marketID, subaccountID, hash.Hex(), order.Cid(), err))
			continue
		}

		notional := order.OrderInfo.Price.Mul(order.OrderInfo.Quantity)
		marginHoldRefund := order.Fillable.Mul(order.Margin.Add(notional.Mul(positiveFeePart))).Quo(order.OrderInfo.Quantity)
		freedUpBalance = freedUpBalance.Add(marginHoldRefund)
	}

	restingSellOrderHashes := k.GetAllRestingDerivativeLimitOrderHashesBySubaccountAndMarket(ctx, marketID, false, subaccountID)
	for _, hash := range restingSellOrderHashes {
		if freedUpBalance.GTE(freeingUpBalance) {
			return freedUpBalance
		}

		isBuy := false
		order := k.GetDerivativeLimitOrderBySubaccountIDAndHash(ctx, marketID, &isBuy, subaccountID, hash)
		if err := k.CancelRestingDerivativeLimitOrder(ctx, market, subaccountID, &isBuy, hash, false, true); err != nil {
			metrics.ReportFuncError(k.svcTags)
			events.Emit(ctx, k.BaseKeeper, v2.NewEventOrderCancelFail(marketID, subaccountID, hash.Hex(), order.Cid(), err))
			continue
		}

		notional := order.OrderInfo.Price.Mul(order.OrderInfo.Quantity)
		marginHoldRefund := order.Fillable.Mul(order.Margin.Add(notional.Mul(positiveFeePart))).Quo(order.OrderInfo.Quantity)
		freedUpBalance = freedUpBalance.Add(marginHoldRefund)
	}

	return freedUpBalance
}

// GetComputedDerivativeLimitOrderbook returns the orderbook of a given market.
func (k DerivativeKeeper) GetComputedDerivativeLimitOrderbook(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	limit uint64,
) (priceLevel []*v2.Level) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	priceLevel = make([]*v2.Level, 0, limit)

	appendPriceLevel := func(order *v2.DerivativeLimitOrder) (stop bool) {
		lastIdx := len(priceLevel) - 1
		if lastIdx+1 == int(limit) {
			return true
		}

		if lastIdx == -1 || !priceLevel[lastIdx].P.Equal(order.OrderInfo.Price) {
			if order.Fillable.IsPositive() {
				priceLevel = append(priceLevel, &v2.Level{
					P: order.OrderInfo.Price,
					Q: order.Fillable,
				})
			}
		} else {
			priceLevel[lastIdx].Q = priceLevel[lastIdx].Q.Add(order.Fillable)
		}
		return false
	}

	k.IterateDerivativeLimitOrdersByMarketDirection(ctx, marketID, isBuy, appendPriceLevel)

	return priceLevel
}

// GetAllTraderDerivativeLimitOrders gets all the derivative limit orders for a given subaccountID and marketID
func (k DerivativeKeeper) GetAllTraderDerivativeLimitOrders(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
) []*v2.TrimmedDerivativeLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.TrimmedDerivativeLimitOrder, 0)
	appendOrder := func(order v2.DerivativeLimitOrder) (stop bool) {
		orders = append(orders, order.ToTrimmed())
		return false
	}

	k.IterateDerivativeLimitOrdersBySubaccount(ctx, marketID, true, subaccountID, appendOrder)
	k.IterateDerivativeLimitOrdersBySubaccount(ctx, marketID, false, subaccountID, appendOrder)

	return orders
}

func (k DerivativeKeeper) GetDerivativeLimitOrdersByAddress(
	ctx sdk.Context,
	marketID common.Hash,
	accountAddress sdk.AccAddress,
) []*v2.TrimmedDerivativeLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.TrimmedDerivativeLimitOrder, 0)
	appendOrder := func(order v2.DerivativeLimitOrder) (stop bool) {
		orders = append(orders, order.ToTrimmed())
		return false
	}

	k.IterateDerivativeLimitOrdersByAddress(ctx, marketID, true, accountAddress, appendOrder)
	k.IterateDerivativeLimitOrdersByAddress(ctx, marketID, false, accountAddress, appendOrder)

	return orders
}

// Note: this does NOT cancel the trader's resting reduce-only orders
func (k DerivativeKeeper) CancelAllOrdersFromTraderInCurrentMarket(
	ctx sdk.Context,
	market *v2.DerivativeMarket,
	subaccountID common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	k.CancelAllRestingDerivativeLimitOrdersForSubaccount(ctx, market, subaccountID, false, true)
	k.CancelAllTransientDerivativeLimitOrdersBySubaccountID(ctx, market, subaccountID)
}

func (k DerivativeKeeper) CancelDerivativeOrder(
	ctx sdk.Context,
	subaccountID common.Hash,
	identifier any,
	market v2.DerivativeMarketI,
	marketID common.Hash,
	orderMask int32,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orderHash, err := k.GetOrderHashFromIdentifier(ctx, subaccountID, identifier)
	if err != nil {
		return err
	}

	return k.cancelDerivativeOrderByOrderHash(ctx, subaccountID, orderHash, market, marketID, orderMask)
}

func (k DerivativeKeeper) cancelDerivativeOrderByOrderHash(
	ctx sdk.Context,
	subaccountID common.Hash,
	orderHash common.Hash,
	market v2.DerivativeMarketI,
	marketID common.Hash,
	orderMask int32,
) (err error) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()
	// Reject if derivative market id does not reference an active derivative market
	if market == nil || !market.StatusSupportsOrderCancellations() {
		k.Logger(ctx).Debug("active derivative market doesn't exist", "marketID", marketID)
		metrics.ReportFuncError(k.svcTags)
		return types.ErrDerivativeMarketNotFound.Wrapf("active derivative market doesn't exist %s", marketID.Hex())
	}

	isBuy, shouldCheckIsRegular, shouldCheckIsConditional, shouldCheckIsMarketOrder, shouldCheckIsLimitOrder := processOrderMaskFlags(
		orderMask,
	)

	if shouldCheckIsRegular {
		orderFound, err := k.checkAndCancelRegularDerivativeOrder(
			ctx, marketID, subaccountID, orderHash, isBuy, market, shouldCheckIsConditional,
		)
		if err != nil || orderFound {
			return err
		}
	}

	if shouldCheckIsConditional {
		return k.checkAndCancelConditionalDerivativeOrder(
			ctx, marketID, subaccountID, orderHash, isBuy, market, shouldCheckIsMarketOrder, shouldCheckIsLimitOrder,
		)
	}

	return nil
}

func (k DerivativeKeeper) checkAndCancelRegularDerivativeOrder(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
	orderHash common.Hash,
	//revive:disable:flag-parameter // the isBuy flag parameter should be removed when after fixing the same error in the called functions
	isBuy *bool,
	market v2.DerivativeMarketI,
	shouldCheckConditional bool,
) (bool, error) {
	var isTransient = false

	order := k.GetDerivativeLimitOrderBySubaccountIDAndHash(ctx, marketID, isBuy, subaccountID, orderHash)

	if order == nil {
		order = k.GetTransientDerivativeLimitOrderBySubaccountIDAndHash(ctx, marketID, isBuy, subaccountID, orderHash)
		if order == nil && !shouldCheckConditional {
			return false, types.ErrOrderDoesntExist.Wrap("Derivative Limit Order doesn't exist")
		}
		isTransient = true
	}

	if order != nil {
		var err error
		if isTransient {
			err = k.CancelTransientDerivativeLimitOrder(ctx, market, order)
		} else {
			direction := order.OrderType.IsBuy()
			err = k.CancelRestingDerivativeLimitOrder(ctx, market, subaccountID, &direction, orderHash, true, true)
		}
		return true, err
	}

	return false, nil
}

func (k DerivativeKeeper) checkAndCancelConditionalDerivativeOrder(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
	orderHash common.Hash,
	//revive:disable:flag-parameter // to be removed in the future
	isBuy *bool,
	market v2.DerivativeMarketI,
	//revive:disable:flag-parameter // to be removed in the future
	shouldCheckMarketOrder bool,
	//revive:disable:flag-parameter // to be removed in the future
	shouldCheckLimitOrder bool,
) error {
	if shouldCheckMarketOrder {
		order, direction := k.GetConditionalDerivativeMarketOrderBySubaccountIDAndHash(ctx, marketID, isBuy, subaccountID, orderHash)
		if order != nil {
			return k.CancelConditionalDerivativeMarketOrder(ctx, market, subaccountID, &direction, orderHash)
		}

		if !shouldCheckLimitOrder {
			return types.ErrOrderDoesntExist.Wrap("Derivative Market Order doesn't exist")
		}
	}

	if !shouldCheckLimitOrder {
		return nil
	}

	order, direction := k.GetConditionalDerivativeLimitOrderBySubaccountIDAndHash(ctx, marketID, isBuy, subaccountID, orderHash)
	if order == nil {
		return types.ErrOrderDoesntExist.Wrap("Derivative Limit Order doesn't exist")
	}

	return k.CancelConditionalDerivativeLimitOrder(ctx, market, subaccountID, &direction, orderHash)
}

//revive:disable:cognitive-complexity // this function has slightly higher complexity but is still readable
//revive:disable:function-result-limit // we need all the results
func processOrderMaskFlags(orderMask int32) (
	isBuy *bool,
	shouldCheckIsRegular,
	shouldCheckIsConditional,
	shouldCheckIsMarketOrder,
	shouldCheckIsLimitOrder bool,
) {
	shouldCheckIsBuy := orderMask&int32(types.OrderMask_BUY_OR_HIGHER) > 0
	shouldCheckIsSell := orderMask&int32(types.OrderMask_SELL_OR_LOWER) > 0
	shouldCheckIsRegular = orderMask&int32(types.OrderMask_REGULAR) > 0
	shouldCheckIsConditional = orderMask&int32(types.OrderMask_CONDITIONAL) > 0
	shouldCheckIsMarketOrder = orderMask&int32(types.OrderMask_MARKET) > 0
	shouldCheckIsLimitOrder = orderMask&int32(types.OrderMask_LIMIT) > 0

	areRegularAndConditionalFlagsBothUnspecified := !shouldCheckIsRegular && !shouldCheckIsConditional
	areBuyAndSellFlagsBothUnspecified := !shouldCheckIsBuy && !shouldCheckIsSell
	areMarketAndLimitFlagsBothUnspecified := !shouldCheckIsMarketOrder && !shouldCheckIsLimitOrder

	// if both conditional flags are unspecified, check both
	if areRegularAndConditionalFlagsBothUnspecified {
		shouldCheckIsRegular, shouldCheckIsConditional = true, true
	}

	// if both market and limit flags are unspecified, check both
	if areMarketAndLimitFlagsBothUnspecified {
		shouldCheckIsMarketOrder, shouldCheckIsLimitOrder = true, true
	}

	// if both buy/sell flags are unspecified, check both
	if areBuyAndSellFlagsBothUnspecified {
		shouldCheckIsBuy, shouldCheckIsSell = true, true
	}

	isBuyOrSellFlagExplicitlySet := !shouldCheckIsBuy || !shouldCheckIsSell

	// if the buy flag is explicitly set, check it
	if isBuyOrSellFlagExplicitlySet {
		isBuy = &shouldCheckIsBuy
	}

	return isBuy, shouldCheckIsRegular, shouldCheckIsConditional, shouldCheckIsMarketOrder, shouldCheckIsLimitOrder
}
