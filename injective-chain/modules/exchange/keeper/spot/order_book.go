package spot

import (
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

var _ SpotOrderbook = &SpotLimitOrderbook{}

// GetAllTransientSpotLimitOrderbook returns all transient orderbooks for all spot markets.
func (k SpotKeeper) GetAllTransientSpotLimitOrderbook(ctx sdk.Context) []v2.SpotOrderBook {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	markets := k.GetAllSpotMarkets(ctx)
	orderbook := make([]v2.SpotOrderBook, 0, len(markets)*2)
	for _, market := range markets {
		buyOrders := k.GetAllTransientSpotLimitOrdersByMarketDirection(ctx, market.MarketID(), true)
		orderbook = append(orderbook, v2.SpotOrderBook{
			MarketId:  market.MarketID().Hex(),
			IsBuySide: true,
			Orders:    buyOrders,
		})
		sellOrders := k.GetAllTransientSpotLimitOrdersByMarketDirection(ctx, market.MarketID(), false)
		orderbook = append(orderbook, v2.SpotOrderBook{
			MarketId:  market.MarketID().Hex(),
			IsBuySide: false,
			Orders:    sellOrders,
		})
	}

	return orderbook
}

func NewSpotOrderbookMatchingResults(transientBuyOrders, transientSellOrders []*v2.SpotLimitOrder) *v2.SpotOrderbookMatchingResults {
	orderbookResults := &v2.SpotOrderbookMatchingResults{
		TransientBuyOrderbookFills: &v2.OrderbookFills{
			Orders: transientBuyOrders,
		},
		TransientSellOrderbookFills: &v2.OrderbookFills{
			Orders: transientSellOrders,
		},
	}

	buyFillQuantities := make([]math.LegacyDec, len(transientBuyOrders))
	for idx := range transientBuyOrders {
		buyFillQuantities[idx] = math.LegacyZeroDec()
	}

	sellFillQuantities := make([]math.LegacyDec, len(transientSellOrders))
	for idx := range transientSellOrders {
		sellFillQuantities[idx] = math.LegacyZeroDec()
	}

	orderbookResults.TransientBuyOrderbookFills.FillQuantities = buyFillQuantities
	orderbookResults.TransientSellOrderbookFills.FillQuantities = sellFillQuantities

	return orderbookResults
}

//nolint:revive // ok
type SpotOrderbook interface {
	GetNotional() math.LegacyDec
	GetTotalQuantityFilled() math.LegacyDec
	GetTransientOrderbookFills() *v2.OrderbookFills
	GetRestingOrderbookFills() *v2.OrderbookFills
	Peek() *v2.PriceLevel
	Fill(math.LegacyDec) error
	Close() error
}

type OrderbookFills struct {
	Orders         []*v2.SpotLimitOrder
	FillQuantities []math.LegacyDec
}

//nolint:revive //ok
type SpotLimitOrderbook struct {
	isBuy         bool
	notional      math.LegacyDec
	totalQuantity math.LegacyDec

	transientOrderbookFills *v2.OrderbookFills
	transientOrderIdx       int

	restingOrderbookFills *v2.OrderbookFills
	restingOrderIterator  storetypes.Iterator

	// pointers to the current OrderbookFills
	currState *v2.OrderbookFills

	k SpotKeeper
}

func NewSpotLimitOrderbook(
	k SpotKeeper,
	iterator storetypes.Iterator,
	transientOrders []*v2.SpotLimitOrder,
	isBuy bool,
) *SpotLimitOrderbook {
	// return early if there are no limit orders in this direction
	if (len(transientOrders) == 0) && !iterator.Valid() {
		iterator.Close()
		return nil
	}

	var transientOrderbookState *v2.OrderbookFills
	if len(transientOrders) == 0 {
		transientOrderbookState = nil
	} else {
		newOrderFillQuantities := make([]math.LegacyDec, len(transientOrders))
		// pre-initialize to zero dec for convenience
		for idx := range newOrderFillQuantities {
			newOrderFillQuantities[idx] = math.LegacyZeroDec()
		}
		transientOrderbookState = &v2.OrderbookFills{
			Orders:         transientOrders,
			FillQuantities: newOrderFillQuantities,
		}
	}

	var restingOrderbookState *v2.OrderbookFills

	if iterator.Valid() {
		restingOrderbookState = &v2.OrderbookFills{
			Orders:         make([]*v2.SpotLimitOrder, 0),
			FillQuantities: make([]math.LegacyDec, 0),
		}
	}

	orderbook := SpotLimitOrderbook{
		isBuy:         isBuy,
		notional:      math.LegacyZeroDec(),
		totalQuantity: math.LegacyZeroDec(),

		transientOrderbookFills: transientOrderbookState,
		transientOrderIdx:       0,
		restingOrderbookFills:   restingOrderbookState,
		restingOrderIterator:    iterator,

		currState: nil,
		k:         k,
	}

	return &orderbook
}

func (b *SpotLimitOrderbook) GetNotional() math.LegacyDec            { return b.notional }
func (b *SpotLimitOrderbook) GetTotalQuantityFilled() math.LegacyDec { return b.totalQuantity }
func (b *SpotLimitOrderbook) GetTransientOrderbookFills() *v2.OrderbookFills {
	return b.transientOrderbookFills
}
func (b *SpotLimitOrderbook) GetRestingOrderbookFills() *v2.OrderbookFills {
	return b.restingOrderbookFills
}

//nolint:revive // ok
func (b *SpotLimitOrderbook) advanceNewOrder() {
	if b.currState != nil {
		return
	}

	restingOrder := b.getRestingOrder()
	transientOrder := b.getTransientOrder()

	switch {
	case restingOrder != nil && transientOrder != nil:
		// buy orders with higher prices or sell orders with lower prices are prioritized
		if (b.isBuy && restingOrder.OrderInfo.Price.LT(transientOrder.OrderInfo.Price)) ||
			(!b.isBuy && restingOrder.OrderInfo.Price.GT(transientOrder.OrderInfo.Price)) {
			b.currState = b.transientOrderbookFills
		} else {
			b.currState = b.restingOrderbookFills
		}
	case restingOrder != nil && transientOrder == nil:
		b.currState = b.restingOrderbookFills
	case restingOrder == nil && transientOrder != nil:
		b.currState = b.transientOrderbookFills
	default:
	}
}

func (b *SpotLimitOrderbook) Peek() *v2.PriceLevel {
	// Sets currState to the orderbook (transientOrderbook or restingOrderbook) with the next best priced order
	b.advanceNewOrder()

	if b.currState == nil {
		return nil
	}

	priceLevel := v2.PriceLevel{}

	idx := b.getCurrIndex()
	order := b.currState.Orders[idx]
	currMatchedQuantity := b.currState.FillQuantities[idx]

	priceLevel.Price = order.OrderInfo.Price
	priceLevel.Quantity = order.Fillable.Sub(currMatchedQuantity)
	return &priceLevel
}

// NOTE: b.currState must NOT be nil!
func (b *SpotLimitOrderbook) getCurrIndex() int {
	var idx int
	// obtain index according to the currState
	if b.currState == b.restingOrderbookFills {
		idx = len(b.restingOrderbookFills.Orders) - 1
	} else {
		idx = b.transientOrderIdx
	}
	return idx
}

func (b *SpotLimitOrderbook) Fill(fillQuantity math.LegacyDec) error {
	idx := b.getCurrIndex()

	orderCumulativeFillQuantity := b.currState.FillQuantities[idx].Add(fillQuantity)

	// Should never happen, might want to remove this once stable
	if orderCumulativeFillQuantity.GT(b.currState.Orders[idx].Fillable) {
		return types.ErrOrderbookFillInvalid
	}

	b.currState.FillQuantities[idx] = orderCumulativeFillQuantity

	order := b.currState.Orders[idx]
	fillNotional := fillQuantity.Mul(order.OrderInfo.Price)

	b.notional = b.notional.Add(fillNotional)
	b.totalQuantity = b.totalQuantity.Add(fillQuantity)

	// if currState is fully filled, set to nil
	if orderCumulativeFillQuantity.Equal(b.currState.Orders[idx].Fillable) {
		b.currState = nil
	}

	return nil
}

func (b *SpotLimitOrderbook) Close() error {
	return b.restingOrderIterator.Close()
}

func (b *SpotLimitOrderbook) getRestingFillableQuantity() math.LegacyDec {
	idx := len(b.restingOrderbookFills.Orders) - 1
	if idx == -1 {
		return math.LegacyZeroDec()
	}
	return b.restingOrderbookFills.Orders[idx].Fillable.Sub(b.restingOrderbookFills.FillQuantities[idx])
}

func (b *SpotLimitOrderbook) getTransientFillableQuantity() math.LegacyDec {
	idx := b.transientOrderIdx
	return b.transientOrderbookFills.Orders[idx].Fillable.Sub(b.transientOrderbookFills.FillQuantities[idx])
}

func (b *SpotLimitOrderbook) getRestingOrder() *v2.SpotLimitOrder {
	// if no more orders to iterate + fully filled, return nil
	if !b.restingOrderIterator.Valid() && (b.restingOrderbookFills == nil || b.getRestingFillableQuantity().IsZero()) {
		return nil
	}

	idx := len(b.restingOrderbookFills.Orders) - 1

	// if the current resting order state is fully filled, advance the iterator
	if b.getRestingFillableQuantity().IsZero() {
		order := b.k.UnmarshalSpotLimitOrder(b.restingOrderIterator.Value())

		b.restingOrderbookFills.Orders = append(b.restingOrderbookFills.Orders, &order)
		b.restingOrderbookFills.FillQuantities = append(b.restingOrderbookFills.FillQuantities, math.LegacyZeroDec())

		b.restingOrderIterator.Next()

		return &order
	}

	return b.restingOrderbookFills.Orders[idx]
}

func (b *SpotLimitOrderbook) getTransientOrder() *v2.SpotLimitOrder {
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

//nolint:revive // ok
type SpotMarketOrderbook struct {
	notional      math.LegacyDec
	totalQuantity math.LegacyDec

	orders         []*v2.SpotMarketOrder
	fillQuantities []math.LegacyDec
	orderIdx       int
}

func NewSpotMarketOrderbook(spotMarketOrders []*v2.SpotMarketOrder) *SpotMarketOrderbook {
	if len(spotMarketOrders) == 0 {
		return nil
	}

	fillQuantities := make([]math.LegacyDec, len(spotMarketOrders))
	for idx := range spotMarketOrders {
		fillQuantities[idx] = math.LegacyZeroDec()
	}

	orderGroup := SpotMarketOrderbook{
		notional:      math.LegacyZeroDec(),
		totalQuantity: math.LegacyZeroDec(),

		orders:         spotMarketOrders,
		fillQuantities: fillQuantities,
		orderIdx:       0,
	}

	return &orderGroup
}

func (b *SpotMarketOrderbook) GetNotional() math.LegacyDec                  { return b.notional }
func (b *SpotMarketOrderbook) GetTotalQuantityFilled() math.LegacyDec       { return b.totalQuantity }
func (b *SpotMarketOrderbook) GetOrderbookFillQuantities() []math.LegacyDec { return b.fillQuantities }
func (b *SpotMarketOrderbook) Done() bool                                   { return b.orderIdx == len(b.orders) }
func (b *SpotMarketOrderbook) Peek() *v2.PriceLevel {
	if b.Done() {
		return nil
	}

	if b.fillQuantities[b.orderIdx].Equal(b.orders[b.orderIdx].OrderInfo.Quantity) {
		b.orderIdx++
		return b.Peek()
	}

	return &v2.PriceLevel{
		Price:    b.orders[b.orderIdx].OrderInfo.Price,
		Quantity: b.orders[b.orderIdx].OrderInfo.Quantity.Sub(b.fillQuantities[b.orderIdx]),
	}
}

func (b *SpotMarketOrderbook) Fill(fillQuantity math.LegacyDec) error {
	newFillAmount := b.fillQuantities[b.orderIdx].Add(fillQuantity)

	if newFillAmount.GT(b.orders[b.orderIdx].OrderInfo.Quantity) {
		return types.ErrOrderbookFillInvalid
	}

	b.fillQuantities[b.orderIdx] = newFillAmount
	b.notional = b.notional.Add(fillQuantity.Mul(b.orders[b.orderIdx].OrderInfo.Price))
	b.totalQuantity = b.totalQuantity.Add(fillQuantity)

	return nil
}
