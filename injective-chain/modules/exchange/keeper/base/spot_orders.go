package base

import (
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k *BaseKeeper) GetTransientStoreKey() storetypes.StoreKey {
	return k.tStoreKey
}

func (k *BaseKeeper) UnmarshalSpotLimitOrder(bz []byte) v2.SpotLimitOrder {
	var order v2.SpotLimitOrder
	k.cdc.MustUnmarshal(bz, &order)
	return order
}

//nolint:revive // ok
func (k *BaseKeeper) SpotLimitOrderbookIterator(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) storetypes.Iterator {
	store := k.getStore(ctx)
	prefixKey := types.SpotLimitOrdersPrefix
	prefixKey = append(prefixKey, types.MarketDirectionPrefix(marketID, isBuy)...)
	ordersStore := prefix.NewStore(store, prefixKey)

	var iterator storetypes.Iterator
	if isBuy {
		iterator = ordersStore.ReverseIterator(nil, nil)
	} else {
		iterator = ordersStore.Iterator(nil, nil)
	}
	return iterator
}

func (k *BaseKeeper) SetSpotLimitOrder(
	ctx sdk.Context,
	order *v2.SpotLimitOrder,
	marketID common.Hash,
	isBuy bool,
	orderHash common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	ordersStore := prefix.NewStore(store, types.SpotLimitOrdersPrefix)
	ordersIndexStore := prefix.NewStore(store, types.SpotLimitOrdersIndexPrefix)

	priceKey := types.GetLimitOrderByPriceKeyPrefix(marketID, isBuy, order.OrderInfo.Price, orderHash)
	bz := k.cdc.MustMarshal(order)
	ordersStore.Set(priceKey, bz)

	// set subaccount index key store
	subaccountKey := types.GetLimitOrderIndexKey(marketID, isBuy, order.SubaccountID(), orderHash)
	ordersIndexStore.Set(subaccountKey, priceKey)
}

// IterateSpotLimitOrdersBySubaccount iterates over the spot limits order index for a given subaccountID and marketID and direction
//
//nolint:revive // ok
func (k *BaseKeeper) IterateSpotLimitOrdersBySubaccount(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	subaccountID common.Hash,
	process func(order v2.SpotLimitOrder) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	ordersStore := prefix.NewStore(store, types.SpotLimitOrdersPrefix)
	orderIndexStore := prefix.NewStore(store, types.GetSpotLimitOrderIndexPrefix(marketID, isBuy, subaccountID))
	var iterator storetypes.Iterator
	if isBuy {
		iterator = orderIndexStore.ReverseIterator(nil, nil)
	} else {
		iterator = orderIndexStore.Iterator(nil, nil)
	}

	iterateSafe(iterator, func(_, v []byte) bool {
		var order v2.SpotLimitOrder
		k.cdc.MustUnmarshal(ordersStore.Get(v), &order)
		return process(order)
	})
}

// IterateSpotLimitOrdersByAccountAddress iterates over the spot limits order index for a given account address and marketID and direction
//
//nolint:revive // ok
func (k *BaseKeeper) IterateSpotLimitOrdersByAccountAddress(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	accountAddress sdk.AccAddress,
	process func(order v2.SpotLimitOrder) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	orderStore := prefix.NewStore(store, types.SpotLimitOrdersPrefix)
	orderIndexStore := prefix.NewStore(store, types.GetSpotLimitOrderIndexByAccountAddressPrefix(marketID, isBuy, accountAddress))
	var iterator storetypes.Iterator
	if isBuy {
		iterator = orderIndexStore.ReverseIterator(nil, nil)
	} else {
		iterator = orderIndexStore.Iterator(nil, nil)
	}

	iterateSafe(iterator, func(_, v []byte) bool {
		var order v2.SpotLimitOrder
		k.cdc.MustUnmarshal(orderStore.Get(v), &order)
		return process(order)
	})
}

// GetSpotLimitOrderByPrice returns active spot limit Order from hash and price.
func (k *BaseKeeper) GetSpotLimitOrderByPrice(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	price math.LegacyDec,
	orderHash common.Hash,
) *v2.SpotLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	ordersStore := prefix.NewStore(store, types.SpotLimitOrdersPrefix)
	key := types.SpotMarketDirectionPriceHashPrefix(marketID, isBuy, price, orderHash)
	bz := ordersStore.Get(key)
	if bz == nil {
		return nil
	}

	var order v2.SpotLimitOrder
	k.cdc.MustUnmarshal(bz, &order)
	return &order
}

// GetSpotLimitOrderBySubaccountID returns active spot limit Order from hash and subaccountID.
func (k *BaseKeeper) GetSpotLimitOrderBySubaccountID(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy *bool,
	subaccountID common.Hash,
	orderHash common.Hash,
) *v2.SpotLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	ordersStore := prefix.NewStore(store, types.SpotLimitOrdersPrefix)
	ordersIndexStore := prefix.NewStore(store, types.SpotLimitOrdersIndexPrefix)

	// Fetch price key from ordersIndexStore
	var priceKey []byte
	if isBuy == nil {
		subaccountKey := types.GetLimitOrderIndexKey(marketID, true, subaccountID, orderHash)
		priceKey = ordersIndexStore.Get(subaccountKey)
		if priceKey == nil {
			subaccountKey = types.GetLimitOrderIndexKey(marketID, false, subaccountID, orderHash)
			priceKey = ordersIndexStore.Get(subaccountKey)
		}
	} else {
		subaccountKey := types.GetLimitOrderIndexKey(marketID, *isBuy, subaccountID, orderHash)
		priceKey = ordersIndexStore.Get(subaccountKey)
	}

	if priceKey == nil {
		return nil
	}

	// Fetch LimitOrder from ordersStore
	bz := ordersStore.Get(priceKey)
	if bz == nil {
		return nil
	}

	var order v2.SpotLimitOrder
	k.cdc.MustUnmarshal(bz, &order)

	return &order
}

// DeleteSpotLimitOrder deletes the SpotLimitOrder.
func (k *BaseKeeper) DeleteSpotLimitOrder(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	order *v2.SpotLimitOrder,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	ordersIndexStore := prefix.NewStore(store, types.SpotLimitOrdersIndexPrefix)
	subaccountKey := types.GetLimitOrderIndexKey(marketID, isBuy, order.SubaccountID(), common.BytesToHash(order.OrderHash))

	priceKey := ordersIndexStore.Get(subaccountKey)

	// delete main spot order store
	ordersStore := prefix.NewStore(store, types.SpotLimitOrdersPrefix)
	ordersStore.Delete(priceKey)

	// delete from subaccount index key store
	ordersIndexStore.Delete(subaccountKey)
}

// IterateSpotLimitOrdersByMarketDirection iterates over spot limits for a given marketID and direction.
// For buy limit orders, starts iteration over the highest price spot limit orders.
// For sell limit orders, starts iteration over the lowest price spot limit orders.
//
//nolint:revive // ok
func (k *BaseKeeper) IterateSpotLimitOrdersByMarketDirection(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	process func(order *v2.SpotLimitOrder) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	prefixKey := types.SpotLimitOrdersPrefix
	prefixKey = append(prefixKey, types.MarketDirectionPrefix(marketID, isBuy)...)
	ordersStore := prefix.NewStore(store, prefixKey)

	var iter storetypes.Iterator
	if isBuy {
		iter = ordersStore.ReverseIterator(nil, nil)
	} else {
		iter = ordersStore.Iterator(nil, nil)
	}

	iterateSafe(iter, func(_, v []byte) bool {
		var order v2.SpotLimitOrder
		k.cdc.MustUnmarshal(v, &order)
		return process(&order)
	})
}

// SetTransientSpotLimitOrder stores SpotLimitOrder in the transient store.
func (k *BaseKeeper) SetTransientSpotLimitOrder(
	ctx sdk.Context,
	order *v2.SpotLimitOrder,
	marketID common.Hash,
	isBuy bool,
	orderHash common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// use transient store key
	store := k.getTransientStore(ctx)

	// set main spot order transient store
	ordersStore := prefix.NewStore(store, types.SpotLimitOrdersPrefix)
	key := types.GetLimitOrderByPriceKeyPrefix(marketID, isBuy, order.OrderInfo.Price, orderHash)
	bz := k.cdc.MustMarshal(order)
	ordersStore.Set(key, bz)

	// set subaccount index key store
	ordersIndexStore := prefix.NewStore(store, types.SpotLimitOrdersIndexPrefix)
	subaccountKey := types.GetLimitOrderIndexKey(marketID, isBuy, order.SubaccountID(), orderHash)
	bz = key
	ordersIndexStore.Set(subaccountKey, bz)

	// set spot order markets indicator store
	key = types.GetSpotMarketTransientMarketsKey(marketID, isBuy)
	if !store.Has(key) {
		store.Set(key, []byte{})
	}

	k.SetCid(ctx, true, order.SubaccountID(), order.Cid(), marketID, isBuy, orderHash)
}

// IterateTransientSpotLimitOrdersBySubaccount iterates over the transient spot limits orders for a given subaccountID and marketID and direction
//
//nolint:revive // ok
func (k *BaseKeeper) IterateTransientSpotLimitOrdersBySubaccount(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	subaccountID common.Hash,
	process func(order *v2.SpotLimitOrder) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	ordersStore := prefix.NewStore(store, types.SpotLimitOrdersPrefix)
	orderIndexStore := prefix.NewStore(store, types.GetTransientLimitOrderIndexIteratorPrefix(marketID, isBuy, subaccountID))

	orders := []*v2.SpotLimitOrder{}

	var iterator storetypes.Iterator
	if isBuy {
		iterator = orderIndexStore.ReverseIterator(nil, nil)
	} else {
		iterator = orderIndexStore.Iterator(nil, nil)
	}

	iterateSafe(iterator, func(_, value []byte) bool {
		var order v2.SpotLimitOrder
		k.cdc.MustUnmarshal(ordersStore.Get(value), &order)
		orders = append(orders, &order)
		return false
	})

	// iterator is closed at this point
	for _, order := range orders {
		if process(order) {
			return
		}
	}
}

// GetTransientSpotLimitOrderBySubaccountID returns transient spot limit Order from hash and subaccountID.
func (k *BaseKeeper) GetTransientSpotLimitOrderBySubaccountID(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy *bool,
	subaccountID common.Hash,
	orderHash common.Hash,
) *v2.SpotLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()
	// use transient store key
	store := k.getTransientStore(ctx)

	ordersStore := prefix.NewStore(store, types.SpotLimitOrdersPrefix)
	ordersIndexStore := prefix.NewStore(store, types.SpotLimitOrdersIndexPrefix)

	// Fetch price key from ordersIndexStore
	var priceKey []byte
	if isBuy == nil {
		subaccountKey := types.GetLimitOrderIndexKey(marketID, true, subaccountID, orderHash)
		priceKey = ordersIndexStore.Get(subaccountKey)
		if priceKey == nil {
			subaccountKey = types.GetLimitOrderIndexKey(marketID, false, subaccountID, orderHash)
			priceKey = ordersIndexStore.Get(subaccountKey)
		}
	} else {
		subaccountKey := types.GetLimitOrderIndexKey(marketID, *isBuy, subaccountID, orderHash)
		priceKey = ordersIndexStore.Get(subaccountKey)
	}

	if priceKey == nil {
		return nil
	}

	// Fetch LimitOrder from ordersStore
	bz := ordersStore.Get(priceKey)
	if bz == nil {
		return nil
	}

	var order v2.SpotLimitOrder
	k.cdc.MustUnmarshal(bz, &order)

	return &order
}

// DeleteTransientSpotLimitOrder deletes the SpotLimitOrder from the transient store.
func (k *BaseKeeper) DeleteTransientSpotLimitOrder(
	ctx sdk.Context,
	marketID common.Hash,
	order *v2.SpotLimitOrder,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()
	store := k.getTransientStore(ctx)

	ordersStore := prefix.NewStore(store, types.SpotLimitOrdersPrefix)
	ordersIndexStore := prefix.NewStore(store, types.SpotLimitOrdersIndexPrefix)

	priceKey := types.GetLimitOrderByPriceKeyPrefix(marketID, order.IsBuy(), order.OrderInfo.Price, order.Hash())

	// delete from main spot order store
	ordersStore.Delete(priceKey)

	// delete from subaccount index key store
	subaccountKey := types.GetLimitOrderIndexKey(marketID, order.IsBuy(), order.SubaccountID(), order.Hash())
	ordersIndexStore.Delete(subaccountKey)

	k.DeleteCid(ctx, true, order.SubaccountID(), order.Cid())
}

// GetAllTransientMatchedSpotLimitOrderMarkets retrieves all markets referenced by this block's transient SpotLimitOrders.
func (k *BaseKeeper) GetAllTransientMatchedSpotLimitOrderMarkets(
	ctx sdk.Context,
) []*types.MatchedMarketDirection {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// use transient store key
	store := k.getTransientStore(ctx)

	// set spot order markets indicator store
	marketIndicatorStore := prefix.NewStore(store, types.SpotMarketsPrefix)

	matchedMarketDirections := make([]*types.MatchedMarketDirection, 0)
	marketIDs := make([]common.Hash, 0)
	marketDirectionMap := make(map[common.Hash]*types.MatchedMarketDirection)

	iterateSafe(marketIndicatorStore.Iterator(nil, nil), func(key, _ []byte) bool {
		marketId, isBuy := types.GetMarketIdDirectionFromTransientKey(key)
		if marketDirectionMap[marketId] == nil {
			marketIDs = append(marketIDs, marketId)
			matchedMarketDirection := types.MatchedMarketDirection{
				MarketId: marketId,
			}
			marketDirectionMap[marketId] = &matchedMarketDirection
		}
		marketDirectionMap[marketId].BuysExists = isBuy
		marketDirectionMap[marketId].SellsExists = !isBuy

		return false
	})

	for _, marketId := range marketIDs {
		matchedMarketDirections = append(matchedMarketDirections, marketDirectionMap[marketId])
	}

	return matchedMarketDirections
}

// IterateSpotMarketOrders iterates over the spot market orders calling process on each one.
//
//nolint:revive // ok
func (k *BaseKeeper) IterateSpotMarketOrders(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
	process func(order *v2.SpotMarketOrder) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// use transient store key
	store := k.getTransientStore(ctx)

	prefixKey := types.SpotMarketOrdersPrefix
	prefixKey = append(prefixKey, types.MarketDirectionPrefix(marketID, isBuy)...)
	ordersStore := prefix.NewStore(store, prefixKey)

	orders := []*v2.SpotMarketOrder{}

	var iterator storetypes.Iterator
	if isBuy {
		// iterate over market buy orders from highest to lowest price
		iterator = ordersStore.ReverseIterator(nil, nil)
	} else {
		iterator = ordersStore.Iterator(nil, nil)
	}

	iterateSafe(iterator, func(_, v []byte) bool {
		var order v2.SpotMarketOrder
		k.cdc.MustUnmarshal(v, &order)
		orders = append(orders, &order)
		return false
	})

	for _, order := range orders {
		if process(order) {
			return
		}
	}
}

// GetAllTransientSpotLimitOrdersByMarketDirection retrieves all transient SpotLimitOrders for
// a given market and direction.
//
//nolint:revive // ok
func (k *BaseKeeper) GetAllTransientSpotLimitOrdersByMarketDirection(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) []*v2.SpotLimitOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// use transient store key
	store := k.getTransientStore(ctx)

	prefixKey := types.SpotLimitOrdersPrefix
	prefixKey = append(prefixKey, types.MarketDirectionPrefix(marketID, isBuy)...)
	ordersStore := prefix.NewStore(store, prefixKey)
	var iterator storetypes.Iterator

	if isBuy {
		// iterate over market buy orders from highest to lowest price
		iterator = ordersStore.ReverseIterator(nil, nil)
	} else {
		iterator = ordersStore.Iterator(nil, nil)
	}

	orders := make([]*v2.SpotLimitOrder, 0)

	iterateSafe(iterator, func(_, v []byte) bool {
		var order v2.SpotLimitOrder
		k.cdc.MustUnmarshal(v, &order)
		orders = append(orders, &order)
		return false
	})

	return orders
}

// SetTransientSpotMarketOrder stores SpotMarketOrder in the transient store.
func (k *BaseKeeper) SetTransientSpotMarketOrder(
	ctx sdk.Context,
	marketOrder *v2.SpotMarketOrder,
	order *v2.SpotOrder,
	orderHash common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// use transient store key
	store := k.getTransientStore(ctx)

	marketId := common.HexToHash(order.MarketId)

	// set main spot market order state transient store
	ordersStore := prefix.NewStore(store, types.SpotMarketOrdersPrefix)
	key := types.GetOrderByPriceKeyPrefix(marketId, order.IsBuy(), marketOrder.OrderInfo.Price, orderHash)
	bz := k.cdc.MustMarshal(marketOrder)
	ordersStore.Set(key, bz)
}

// GetAllTransientSpotMarketOrders iterates over spot market exchange over a given direction.
//
//nolint:revive // ok
func (k *BaseKeeper) GetAllTransientSpotMarketOrders(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) []*v2.SpotMarketOrder {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)

	// set main spot market order state transient store
	prefixKey := types.SpotMarketOrdersPrefix
	prefixKey = append(prefixKey, types.MarketDirectionPrefix(marketID, isBuy)...)
	ordersStore := prefix.NewStore(store, prefixKey)
	var iterator storetypes.Iterator
	if isBuy {
		// iterate over market buy orders from highest to lowest price
		iterator = ordersStore.ReverseIterator(nil, nil)
	} else {
		iterator = ordersStore.Iterator(nil, nil)
	}

	spotMarketOrders := make([]*v2.SpotMarketOrder, 0)

	iterateSafe(iterator, func(_, v []byte) bool {
		var order v2.SpotMarketOrder
		k.cdc.MustUnmarshal(v, &order)
		spotMarketOrders = append(spotMarketOrders, &order)
		return false
	})

	return spotMarketOrders
}

// GetTransientMarketOrderIndicator gets the transient market order indicator in the transient store.
func (k *BaseKeeper) GetTransientMarketOrderIndicator(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) *v2.MarketOrderIndicator {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	marketQuantityStore := prefix.NewStore(store, types.SpotMarketOrderIndicatorPrefix)
	quantityKey := types.MarketDirectionPrefix(marketID, isBuy)
	bz := marketQuantityStore.Get(quantityKey)
	if bz == nil {
		return &v2.MarketOrderIndicator{
			MarketId: marketID.Hex(),
			IsBuy:    isBuy,
		}
	}
	var marketQuantity v2.MarketOrderIndicator
	k.cdc.MustUnmarshal(bz, &marketQuantity)

	return &marketQuantity
}

// GetTransientMarketOrderIndicator sets the transient market order indicator in the transient store.
func (k *BaseKeeper) SetTransientMarketOrderIndicator(
	ctx sdk.Context,
	marketID common.Hash,
	isBuy bool,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	marketIndicatorStore := prefix.NewStore(store, types.SpotMarketOrderIndicatorPrefix)
	quantityKey := types.MarketDirectionPrefix(marketID, isBuy)
	marketOrderIndicator := &v2.MarketOrderIndicator{
		MarketId: marketID.Hex(),
		IsBuy:    isBuy,
	}
	bz := k.cdc.MustMarshal(marketOrderIndicator)
	marketIndicatorStore.Set(quantityKey, bz)
}

// GetAllTransientSpotMarketOrderIndicators iterates over all of a spot market's marketID directions for this block.
func (k *BaseKeeper) GetAllTransientSpotMarketOrderIndicators(
	ctx sdk.Context,
) []*v2.MarketOrderIndicator {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getTransientStore(ctx)
	marketQuantityStore := prefix.NewStore(store, types.SpotMarketOrderIndicatorPrefix)

	marketQuantities := make([]*v2.MarketOrderIndicator, 0)

	iterateSafe(marketQuantityStore.Iterator(nil, nil), func(_, v []byte) bool {
		var marketQuantity v2.MarketOrderIndicator
		k.cdc.MustUnmarshal(v, &marketQuantity)
		marketQuantities = append(marketQuantities, &marketQuantity)
		return false
	})

	return marketQuantities
}

func (k *BaseKeeper) UpdateSpotLimitOrderWithDelta(
	ctx sdk.Context,
	marketID common.Hash,
	orderDelta *v2.SpotLimitOrderDelta,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	ordersStore := prefix.NewStore(store, types.SpotLimitOrdersPrefix)
	priceKey := types.GetLimitOrderByPriceKeyPrefix(
		marketID,
		orderDelta.Order.IsBuy(),
		orderDelta.Order.GetPrice(),
		orderDelta.Order.Hash(),
	)

	orderBz := k.cdc.MustMarshal(orderDelta.Order)
	ordersStore.Set(priceKey, orderBz)
}
