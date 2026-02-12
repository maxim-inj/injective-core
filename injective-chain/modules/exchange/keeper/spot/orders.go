package spot

import (
	"bytes"
	"sort"

	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/events"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k SpotKeeper) CreateSpotLimitOrder(
	ctx sdk.Context,
	sender sdk.AccAddress,
	order *v2.SpotOrder,
	market *v2.SpotMarket,
) (hash common.Hash, err error) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := common.HexToHash(order.MarketId)

	// 0. Derive the subaccountID and populate the order with it
	subaccountID := types.MustGetSubaccountIDOrDeriveFromNonce(sender, order.OrderInfo.SubaccountId)

	// set the actual subaccountID value in the order, since it might be a nonce value
	order.OrderInfo.SubaccountId = subaccountID.Hex()

	// 1. Check and increment Subaccount Nonce, Compute Order Hash
	subaccountNonce := k.subaccount.IncrementSubaccountTradeNonce(ctx, subaccountID)
	orderHash, err := order.ComputeOrderHash(subaccountNonce.Nonce)
	if err != nil {
		metrics.ReportFuncError(k.svcTags)
		return orderHash, err
	}

	// Validate the order
	market, err = k.ValidateSpotOrder(ctx, order, market, marketID, subaccountID)
	if err != nil {
		return orderHash, err
	}

	// 6. Reject if the subaccount's available deposits does not have at least the required funds for the trade
	balanceHoldIncrement, marginDenom := order.GetBalanceHoldAndMarginDenom(market)
	var chainFormattedBalanceHoldIncrement math.LegacyDec
	if order.IsBuy() {
		chainFormattedBalanceHoldIncrement = market.NotionalToChainFormat(balanceHoldIncrement)
	} else {
		chainFormattedBalanceHoldIncrement = market.QuantityToChainFormat(balanceHoldIncrement)
	}

	// 8. Decrement the available balance or bank by the funds amount needed to fund the order
	if err := k.subaccount.ChargeAccount(ctx, subaccountID, marginDenom, chainFormattedBalanceHoldIncrement); err != nil {
		return orderHash, err
	}

	// 9. If Post Only, add the order to the resting orderbook
	//    Otherwise store the order in the transient limit order store and transient market indicator store
	spotLimitOrder := order.GetNewSpotLimitOrder(sender, orderHash)

	// 10b. store the order in the spot limit order store or transient spot limit order store
	if order.OrderType.IsPostOnly() {
		k.SaveNewSpotLimitOrder(ctx, spotLimitOrder, marketID, spotLimitOrder.IsBuy(), spotLimitOrder.Hash())

		var (
			buyOrders  = make([]*v2.SpotLimitOrder, 0)
			sellOrders = make([]*v2.SpotLimitOrder, 0)
		)
		if order.IsBuy() {
			buyOrders = append(buyOrders, spotLimitOrder)
		} else {
			sellOrders = append(sellOrders, spotLimitOrder)
		}

		events.Emit(ctx, k.BaseKeeper, &v2.EventNewSpotOrders{
			MarketId:   marketID.Hex(),
			BuyOrders:  buyOrders,
			SellOrders: sellOrders,
		})
	} else {
		k.SetTransientSpotLimitOrder(ctx, spotLimitOrder, marketID, order.IsBuy(), orderHash)
		k.CheckAndSetFeeDiscountAccountActivityIndicator(ctx, marketID, sender)
	}

	return orderHash, nil
}

func (k SpotKeeper) ValidateSpotOrder(
	ctx sdk.Context,
	order *v2.SpotOrder,
	market *v2.SpotMarket,
	marketID common.Hash,
	subaccountID common.Hash,
) (*v2.SpotMarket, error) {
	if market == nil {
		market = k.GetSpotMarket(ctx, marketID, true)
		if market == nil {
			k.Logger(ctx).Error("active spot market doesn't exist", "marketId", order.MarketId)
			metrics.ReportFuncError(k.svcTags)
			return nil, types.ErrSpotMarketNotFound.Wrapf("active spot market doesn't exist %s", order.MarketId)
		}
	}

	if err := order.CheckTickSize(market.MinPriceTickSize, market.MinQuantityTickSize); err != nil {
		metrics.ReportFuncError(k.svcTags)
		return nil, err
	}

	if err := order.CheckNotional(market.MinNotional); err != nil {
		metrics.ReportFuncError(k.svcTags)
		return nil, err
	}

	if order.ExpirationBlock != 0 && order.ExpirationBlock <= ctx.BlockHeight() {
		metrics.ReportFuncError(k.svcTags)
		return nil, types.ErrInvalidExpirationBlock.Wrap("expiration block must be higher than current block")
	}

	isPostOnlyMode := k.IsPostOnlyMode(ctx)
	if (order.OrderType.IsPostOnly() || isPostOnlyMode) && k.SpotOrderCrossesTopOfBook(ctx, order) {
		metrics.ReportFuncError(k.svcTags)
		return nil, types.ErrExceedsTopOfBookPrice
	}

	if k.ExistsCid(ctx, subaccountID, order.OrderInfo.Cid) {
		metrics.ReportFuncError(k.svcTags)
		return nil, types.ErrClientOrderIdAlreadyExists
	}

	return market, nil
}

// CancelAllRestingLimitOrdersFromSpotMarket cancels all resting and transient spot limit orders for a marketID.
func (k SpotKeeper) CancelAllRestingLimitOrdersFromSpotMarket(
	ctx sdk.Context,
	market *v2.SpotMarket,
	marketID common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	cancelFunc := func(order *v2.SpotLimitOrder) bool {
		err := k.CancelSpotLimitOrderByOrderHash(ctx, order.SubaccountID(), order.Hash(), market, marketID)
		if err != nil {
			events.Emit(ctx, k.BaseKeeper, v2.NewEventOrderCancelFail(marketID, order.SubaccountID(), order.Hash().Hex(), order.Cid(), err))
		}
		return err != nil
	}

	_ = cancelFunc

	// todo: isn't this a bad practice with cosmos store
	k.IterateSpotLimitOrdersByMarketDirection(ctx, marketID, true, cancelFunc)
	k.IterateSpotLimitOrdersByMarketDirection(ctx, marketID, false, cancelFunc)
}

func (k SpotKeeper) CancelSpotLimitOrderByOrderHash(
	ctx sdk.Context,
	subaccountID common.Hash,
	orderHash common.Hash,
	market *v2.SpotMarket,
	marketID common.Hash,
) (err error) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if market == nil || !market.StatusSupportsOrderCancellations() {
		k.Logger(ctx).Error("active spot market doesn't exist")
		metrics.ReportFuncError(k.svcTags)
		return types.ErrSpotMarketNotFound.Wrapf("active spot market doesn't exist %s", marketID.Hex())
	}

	order := k.GetSpotLimitOrderBySubaccountID(ctx, marketID, nil, subaccountID, orderHash)
	var isTransient bool
	if order == nil {
		order = k.GetTransientSpotLimitOrderBySubaccountID(ctx, marketID, nil, subaccountID, orderHash)
		if order == nil {
			return types.ErrOrderDoesntExist.Wrap("Spot Limit Order is nil")
		}
		isTransient = true
	}

	if isTransient {
		k.CancelTransientSpotLimitOrder(ctx, market, marketID, subaccountID, order)
	} else {
		k.CancelSpotLimitOrder(ctx, market, marketID, subaccountID, order.IsBuy(), order)
	}
	return nil
}

// GetSpotOrdersToCancelUpToAmount returns the spot orders to cancel up to a given amount
//
//nolint:revive // ok
func (k SpotKeeper) GetSpotOrdersToCancelUpToAmount(
	_ sdk.Context,
	market *v2.SpotMarket,
	orders []*v2.TrimmedSpotLimitOrder,
	strategy v2.CancellationStrategy,
	referencePrice *math.LegacyDec,
	baseAmount, quoteAmount math.LegacyDec,
) ([]*v2.TrimmedSpotLimitOrder, bool) {
	switch strategy {
	case v2.CancellationStrategy_FromWorstToBest:
		sort.SliceStable(orders, func(i, j int) bool {
			return v2.GetIsOrderLess(*referencePrice, orders[i].Price, orders[j].Price, orders[i].IsBuy, orders[j].IsBuy, true)
		})
	case v2.CancellationStrategy_FromBestToWorst:
		sort.SliceStable(orders, func(i, j int) bool {
			return v2.GetIsOrderLess(*referencePrice, orders[i].Price, orders[j].Price, orders[i].IsBuy, orders[j].IsBuy, false)
		})
	case v2.CancellationStrategy_UnspecifiedOrder:
		// do nothing
	}

	positiveMakerFeePart := math.LegacyMaxDec(math.LegacyZeroDec(), market.MakerFeeRate)

	ordersToCancel := make([]*v2.TrimmedSpotLimitOrder, 0)
	cumulativeBaseAmount, cumulativeQuoteAmount := math.LegacyZeroDec(), math.LegacyZeroDec()

	for _, order := range orders {
		hasSufficientBase := cumulativeBaseAmount.GTE(baseAmount)
		hasSufficientQuote := cumulativeQuoteAmount.GTE(quoteAmount)

		if hasSufficientBase && hasSufficientQuote {
			break
		}

		doesOrderNotHaveRequiredFunds := (!order.IsBuy && hasSufficientBase) || (order.IsBuy && hasSufficientQuote)
		if doesOrderNotHaveRequiredFunds {
			continue
		}

		ordersToCancel = append(ordersToCancel, order)

		if !order.IsBuy {
			cumulativeBaseAmount = cumulativeBaseAmount.Add(order.Fillable)
			continue
		}

		notional := order.Fillable.Mul(order.Price)
		fee := notional.Mul(positiveMakerFeePart)
		cumulativeQuoteAmount = cumulativeQuoteAmount.Add(notional).Add(fee)
	}

	hasProcessedFullAmount := cumulativeBaseAmount.GTE(baseAmount) && cumulativeQuoteAmount.GTE(quoteAmount)
	return ordersToCancel, hasProcessedFullAmount
}

// UpdateSpotLimitOrder updates SpotLimitOrder, order index and cid in keeper.
func (k SpotKeeper) UpdateSpotLimitOrder(
	ctx sdk.Context,
	marketID common.Hash,
	orderDelta *v2.SpotLimitOrderDelta,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	isBuy := orderDelta.Order.IsBuy()
	k.DecrementOrderbookPriceLevelQuantity(ctx, marketID, isBuy, true, orderDelta.Order.GetPrice(), orderDelta.FillQuantity)
	if orderDelta.Order.Fillable.IsZero() {
		k.RemoveSpotLimitOrder(ctx, marketID, isBuy, orderDelta.Order)
		return
	}

	k.UpdateSpotLimitOrderWithDelta(ctx, marketID, orderDelta)
}

func (k SpotKeeper) SpotOrderCrossesTopOfBook(ctx sdk.Context, order *v2.SpotOrder) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()
	// get best price of TOB from opposite side
	bestPrice := k.GetBestSpotLimitOrderPrice(ctx, common.HexToHash(order.MarketId), !order.IsBuy())

	if bestPrice == nil {
		return false
	}

	if order.IsBuy() {
		return order.OrderInfo.Price.GTE(*bestPrice)
	}

	return order.OrderInfo.Price.LTE(*bestPrice)
}

func (k SpotKeeper) GetAllSpotLimitOrderbook(ctx sdk.Context) []v2.SpotOrderBook {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	markets := k.GetAllSpotMarkets(ctx)
	orderbook := make([]v2.SpotOrderBook, 0, len(markets)*2)
	for _, market := range markets {
		orderbook = append(orderbook, v2.SpotOrderBook{
			MarketId:  market.MarketID().Hex(),
			IsBuySide: true,
			Orders:    k.GetAllSpotLimitOrdersByMarketDirection(ctx, market.MarketID(), true),
		},
			v2.SpotOrderBook{
				MarketId:  market.MarketID().Hex(),
				IsBuySide: false,
				Orders:    k.GetAllSpotLimitOrdersByMarketDirection(ctx, market.MarketID(), false),
			})
	}

	return orderbook
}

// GetSpotMidPriceAndTOB finds the spot mid price of the first Spot limit order on the orderbook between each side and returns TOB
func (k SpotKeeper) GetSpotMidPriceAndTOB(
	ctx sdk.Context,
	marketID common.Hash,
) (midPrice, bestBuyPrice, bestSellPrice *math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	bestBuyPrice = k.GetBestSpotLimitOrderPrice(ctx, marketID, true)
	bestSellPrice = k.GetBestSpotLimitOrderPrice(ctx, marketID, false)

	if bestBuyPrice == nil || bestSellPrice == nil {
		return nil, bestBuyPrice, bestSellPrice
	}

	midPriceValue := bestBuyPrice.Add(*bestSellPrice).Quo(math.LegacyNewDec(2))
	return &midPriceValue, bestBuyPrice, bestSellPrice
}

// GetSpotMidPriceOrBestPrice finds the mid price of the first spot limit order on the orderbook between each side
// or the best price if no orders are on the orderbook on one side
func (k SpotKeeper) GetSpotMidPriceOrBestPrice(
	ctx sdk.Context,
	marketID common.Hash,
) *math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	bestBuyPrice := k.GetBestSpotLimitOrderPrice(ctx, marketID, true)
	bestSellPrice := k.GetBestSpotLimitOrderPrice(ctx, marketID, false)

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

// CancelAllSpotLimitOrders cancels all resting and transient spot limit orders for a given subaccount and marketID.
func (k SpotKeeper) CancelAllSpotLimitOrders(
	ctx sdk.Context,
	market *v2.SpotMarket,
	subaccountID common.Hash,
	marketID common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	restingBuyOrders := k.GetAllSpotLimitOrdersBySubaccountAndMarket(ctx, marketID, true, subaccountID)
	restingSellOrders := k.GetAllSpotLimitOrdersBySubaccountAndMarket(ctx, marketID, false, subaccountID)
	transientBuyOrders := k.GetAllTransientSpotLimitOrdersBySubaccountAndMarket(ctx, marketID, true, subaccountID)
	transientSellOrders := k.GetAllTransientSpotLimitOrdersBySubaccountAndMarket(ctx, marketID, false, subaccountID)

	for idx := range restingBuyOrders {
		k.CancelSpotLimitOrder(ctx, market, marketID, subaccountID, true, restingBuyOrders[idx])
	}

	for idx := range restingSellOrders {
		k.CancelSpotLimitOrder(ctx, market, marketID, subaccountID, false, restingSellOrders[idx])
	}

	for idx := range transientBuyOrders {
		k.CancelTransientSpotLimitOrder(ctx, market, marketID, subaccountID, transientBuyOrders[idx])
	}

	for idx := range transientSellOrders {
		k.CancelTransientSpotLimitOrder(ctx, market, marketID, subaccountID, transientSellOrders[idx])
	}
}

// CancelSpotLimitOrder cancels the SpotLimitOrder
func (k SpotKeeper) CancelSpotLimitOrder(
	ctx sdk.Context,
	market *v2.SpotMarket,
	marketID common.Hash,
	subaccountID common.Hash,
	isBuy bool,
	order *v2.SpotLimitOrder,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marginHold, marginDenom := order.GetUnfilledMarginHoldAndMarginDenom(market, false)
	var chainFormattedMarginHold math.LegacyDec
	if order.IsBuy() {
		chainFormattedMarginHold = market.NotionalToChainFormat(marginHold)
	} else {
		chainFormattedMarginHold = market.QuantityToChainFormat(marginHold)
	}

	k.subaccount.IncrementAvailableBalanceOrBank(ctx, subaccountID, marginDenom, chainFormattedMarginHold)
	k.RemoveSpotLimitOrder(ctx, marketID, isBuy, order)

	events.Emit(ctx, k.BaseKeeper, &v2.EventCancelSpotOrder{
		MarketId: marketID.Hex(),
		Order:    *order,
	})
}

// DeleteSpotLimitOrder deletes the SpotLimitOrder.
func (k SpotKeeper) RemoveSpotLimitOrder(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	order *v2.SpotLimitOrder,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	k.DeleteSpotLimitOrder(ctx, marketID, isBuy, order)

	// delete cid
	k.DeleteCid(ctx, false, order.SubaccountID(), order.Cid())

	// update orderbook metadata
	k.DecrementOrderbookPriceLevelQuantity(ctx, marketID, isBuy, true, order.GetPrice(), order.GetFillable())
}

func (k SpotKeeper) CancelTransientSpotLimitOrder(
	ctx sdk.Context,
	market *v2.SpotMarket,
	marketID common.Hash,
	subaccountID common.Hash,
	order *v2.SpotLimitOrder,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marginHold, marginDenom := order.GetUnfilledMarginHoldAndMarginDenom(market, true)
	var chainFormattedMarginHold math.LegacyDec
	if order.IsBuy() {
		chainFormattedMarginHold = market.NotionalToChainFormat(marginHold)
	} else {
		chainFormattedMarginHold = market.QuantityToChainFormat(marginHold)
	}

	k.subaccount.IncrementAvailableBalanceOrBank(ctx, subaccountID, marginDenom, chainFormattedMarginHold)
	k.DeleteTransientSpotLimitOrder(ctx, marketID, order)

	events.Emit(ctx, k.BaseKeeper, &v2.EventCancelSpotOrder{
		MarketId: marketID.Hex(),
		Order:    *order,
	})
}

// SaveNewSpotLimitOrder stores SpotLimitOrder and order index in keeper.
func (k SpotKeeper) SaveNewSpotLimitOrder(
	ctx sdk.Context,
	order *v2.SpotLimitOrder,
	marketID common.Hash,
	isBuy bool,
	orderHash common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	k.SetSpotLimitOrder(ctx, order, marketID, order.IsBuy(), orderHash)

	// update the orderbook metadata
	k.IncrementOrderbookPriceLevelQuantity(ctx, marketID, isBuy, true, order.GetPrice(), order.GetFillable())

	if order.ExpirationBlock > 0 {
		orderData := &v2.OrderData{
			MarketId:     marketID.Hex(),
			SubaccountId: order.SubaccountID().Hex(),
			OrderHash:    order.Hash().Hex(),
			Cid:          order.Cid(),
		}
		k.AppendOrderExpirations(ctx, marketID, order.ExpirationBlock, orderData)
	}

	// set the cid
	k.SetCid(ctx, false, order.SubaccountID(), order.Cid(), marketID, isBuy, orderHash)
}

// GetBestSpotLimitOrderPrice returns the best price of the first limit order on the orderbook.
func (k SpotKeeper) GetBestSpotLimitOrderPrice(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) *math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var bestOrder *v2.SpotLimitOrder
	appendOrder := func(order *v2.SpotLimitOrder) (stop bool) {
		bestOrder = order
		return true
	}

	k.IterateSpotLimitOrdersByMarketDirection(ctx, marketID, isBuy, appendOrder)

	var bestPrice *math.LegacyDec
	if bestOrder != nil {
		bestPrice = &bestOrder.OrderInfo.Price
	}

	return bestPrice
}

// GetAllSpotLimitOrdersByMarketDirection returns an array of the updated SpotLimitOrders.
func (k SpotKeeper) GetAllSpotLimitOrdersByMarketDirection(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) []*v2.SpotLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.SpotLimitOrder, 0)
	appendOrder := func(order *v2.SpotLimitOrder) (stop bool) {
		orders = append(orders, order)
		return false
	}

	k.IterateSpotLimitOrdersByMarketDirection(ctx, marketID, isBuy, appendOrder)

	return orders
}

// GetAllTransientSpotLimitOrdersBySubaccountAndMarket gets all the transient spot limit orders for a given direction
// for a given subaccountID and marketID
func (k SpotKeeper) GetAllTransientSpotLimitOrdersBySubaccountAndMarket(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	subaccountID common.Hash,
) []*v2.SpotLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.SpotLimitOrder, 0)
	appendOrder := func(order *v2.SpotLimitOrder) (stop bool) {
		orders = append(orders, order)
		return false
	}

	k.IterateTransientSpotLimitOrdersBySubaccount(ctx, marketID, isBuy, subaccountID, appendOrder)

	return orders
}

// GetAllSpotLimitOrdersBySubaccountAndMarket gets all the spot limit orders for a given direction for a given subaccountID and marketID
func (k SpotKeeper) GetAllSpotLimitOrdersBySubaccountAndMarket(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	subaccountID common.Hash,
) []*v2.SpotLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.SpotLimitOrder, 0)
	appendOrder := func(order v2.SpotLimitOrder) (stop bool) {
		orders = append(orders, &order)
		return false
	}

	k.IterateSpotLimitOrdersBySubaccount(ctx, marketID, isBuy, subaccountID, appendOrder)

	return orders
}

// todo: this is purely used in tests
// GetFillableSpotLimitOrdersByMarketDirection returns an array of the updated SpotLimitOrders.
func (k SpotKeeper) GetFillableSpotLimitOrdersByMarketDirection(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	maxQuantity math.LegacyDec,
) (limitOrders []*v2.SpotLimitOrder, clearingPrice, clearingQuantity math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	limitOrders = make([]*v2.SpotLimitOrder, 0)
	clearingQuantity = math.LegacyZeroDec()
	notional := math.LegacyZeroDec()

	appendOrder := func(order *v2.SpotLimitOrder) (stop bool) {
		// stop iterating if the quantity needed will be exhausted
		if (clearingQuantity.Add(order.Fillable)).GTE(maxQuantity) {
			neededQuantity := maxQuantity.Sub(clearingQuantity)
			clearingQuantity = clearingQuantity.Add(neededQuantity)
			notional = notional.Add(neededQuantity.Mul(order.OrderInfo.Price))

			limitOrders = append(limitOrders, order)
			return true
		}
		limitOrders = append(limitOrders, order)
		clearingQuantity = clearingQuantity.Add(order.Fillable)
		notional = notional.Add(order.Fillable.Mul(order.OrderInfo.Price))
		return false
	}

	k.IterateSpotLimitOrdersByMarketDirection(ctx, marketID, isBuy, appendOrder)
	if clearingQuantity.IsPositive() {
		clearingPrice = notional.Quo(clearingQuantity)
	}

	return limitOrders, clearingPrice, clearingQuantity
}

func (k SpotKeeper) SetNewTransientSpotMarketOrder(
	ctx sdk.Context,
	marketOrder *v2.SpotMarketOrder,
	order *v2.SpotOrder,
	orderHash common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	k.SetTransientSpotMarketOrder(ctx, marketOrder, order, orderHash)

	marketId := common.HexToHash(order.MarketId)
	k.SetCid(ctx, true, order.SubaccountID(), order.Cid(), marketId, order.IsBuy(), orderHash)

	// increment spot order markets total quantity indicator transient store
	k.SetTransientMarketOrderIndicator(ctx, marketId, order.IsBuy())
}

func (k SpotKeeper) GetAllTransientTraderSpotLimitOrders(ctx sdk.Context, marketID, subaccountID common.Hash) []*v2.TrimmedSpotLimitOrder {
	buyOrders := k.GetAllTransientSpotLimitOrdersBySubaccountAndMarket(ctx, marketID, true, subaccountID)
	sellOrders := k.GetAllTransientSpotLimitOrdersBySubaccountAndMarket(ctx, marketID, false, subaccountID)

	orders := make([]*v2.TrimmedSpotLimitOrder, 0, len(buyOrders)+len(sellOrders))
	for _, order := range buyOrders {
		orders = append(orders, order.ToTrimmed())
	}

	for _, order := range sellOrders {
		orders = append(orders, order.ToTrimmed())
	}

	return orders
}

// GetAllSubaccountSpotMarketOrdersByMarketDirection retrieves all of a subaccount's SpotMarketOrders for a given market and direction.
func (k SpotKeeper) GetAllSubaccountSpotMarketOrdersByMarketDirection(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
	isBuy bool,
) []*v2.SpotMarketOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.SpotMarketOrder, 0)
	appendOrder := func(order *v2.SpotMarketOrder) (stop bool) {
		// only append orders with the same subaccountID
		if bytes.Equal(order.OrderInfo.SubaccountID().Bytes(), subaccountID.Bytes()) {
			orders = append(orders, order)
		}
		return false
	}

	k.IterateSpotMarketOrders(ctx, marketID, isBuy, appendOrder)
	return orders
}

func (k SpotKeeper) GetAllStandardizedSpotLimitOrdersByMarketDirection(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) (orders []*v2.TrimmedLimitOrder) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders = make([]*v2.TrimmedLimitOrder, 0)
	appendOrder := func(order *v2.SpotLimitOrder) (stop bool) {
		orders = append(orders, order.ToStandardized())
		return false
	}

	k.IterateSpotLimitOrdersByMarketDirection(ctx, marketID, isBuy, appendOrder)
	return orders
}

// GetAllTraderSpotLimitOrders gets all the spot limit orders for a given subaccountID and marketID
func (k SpotKeeper) GetAllTraderSpotLimitOrders(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
) []*v2.TrimmedSpotLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.TrimmedSpotLimitOrder, 0)
	appendOrder := func(order v2.SpotLimitOrder) (stop bool) {
		orders = append(orders, order.ToTrimmed())
		return false
	}

	k.IterateSpotLimitOrdersBySubaccount(ctx, marketID, true, subaccountID, appendOrder)
	k.IterateSpotLimitOrdersBySubaccount(ctx, marketID, false, subaccountID, appendOrder)

	return orders
}

func (k SpotKeeper) GetAccountAddressSpotLimitOrders(
	ctx sdk.Context,
	marketID common.Hash,
	accountAddress sdk.AccAddress,
) []*v2.TrimmedSpotLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.TrimmedSpotLimitOrder, 0)
	appendOrder := func(order v2.SpotLimitOrder) (stop bool) {
		orders = append(orders, order.ToTrimmed())
		return false
	}

	k.IterateSpotLimitOrdersByAccountAddress(ctx, marketID, true, accountAddress, appendOrder)
	k.IterateSpotLimitOrdersByAccountAddress(ctx, marketID, false, accountAddress, appendOrder)

	return orders
}

// GetComputedSpotLimitOrderbook returns the orderbook of a given market.
func (k SpotKeeper) GetComputedSpotLimitOrderbook(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	limit uint64,
) []*v2.Level {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	priceLevels := make([]*v2.Level, 0, limit)
	k.IterateSpotLimitOrdersByMarketDirection(ctx, marketID, isBuy, func(order *v2.SpotLimitOrder) (stop bool) {
		lastIdx := len(priceLevels) - 1
		if lastIdx+1 == int(limit) {
			return true
		}

		if lastIdx == -1 || !priceLevels[lastIdx].GetPrice().Equal(order.OrderInfo.Price) {
			priceLevels = append(priceLevels, &v2.Level{
				P: order.OrderInfo.Price,
				Q: order.Fillable,
			})
		} else {
			priceLevels[lastIdx].Q = priceLevels[lastIdx].Q.Add(order.Fillable)
		}
		return false
	})

	return priceLevels
}
