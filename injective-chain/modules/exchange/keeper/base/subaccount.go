package base

import (
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k *BaseKeeper) IterateSubaccountOrderbookMetadataForMarket(
	ctx sdk.Context,
	marketID common.Hash,
	process func(subaccountID common.Hash, isBuy bool, metadata *v2.SubaccountOrderbookMetadata) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	prefixKey := types.SubaccountOrderbookMetadataPrefix
	prefixKey = append(prefixKey, marketID.Bytes()...)
	subaccountStore := prefix.NewStore(store, prefixKey)

	iterateSafe(subaccountStore.Iterator(nil, nil), func(key, value []byte) bool {
		subaccountID := common.BytesToHash(key[:common.HashLength])
		isBuy := key[common.HashLength] == types.TrueByte

		var metadata v2.SubaccountOrderbookMetadata
		k.cdc.MustUnmarshal(value, &metadata)
		return process(subaccountID, isBuy, &metadata)
	})
}

func (k *BaseKeeper) IterateSubaccountOrders(
	ctx sdk.Context,
	process func(marketID, subaccountID common.Hash, isBuy bool, order *v2.SubaccountOrder) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	prefixKey := types.SubaccountOrderPrefix
	ordersStore := prefix.NewStore(store, prefixKey)

	iterateSafe(ordersStore.Iterator(nil, nil), func(key, value []byte) bool {
		marketID := common.BytesToHash(key[:common.HashLength])
		subaccountID := common.BytesToHash(key[common.HashLength : +2*common.HashLength])
		isBuy := key[2*common.HashLength] == types.TrueByte

		var subaccountOrder v2.SubaccountOrder
		k.cdc.MustUnmarshal(value, &subaccountOrder)

		return process(marketID, subaccountID, isBuy, &subaccountOrder)
	})
}

// GetSubaccountTradeNonce gets the subaccount's trade nonce.
func (k *BaseKeeper) GetSubaccountTradeNonce(
	ctx sdk.Context,
	subaccountID common.Hash,
) *v2.SubaccountTradeNonce {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetSubaccountTradeNonceKey(subaccountID)
	bz := store.Get(key)
	if bz == nil {
		return &v2.SubaccountTradeNonce{Nonce: 0}
	}

	var nonce v2.SubaccountTradeNonce
	k.cdc.MustUnmarshal(bz, &nonce)

	return &nonce
}

// SetSubaccountTradeNonce sets the subaccount's trade nonce.
func (k *BaseKeeper) SetSubaccountTradeNonce(
	ctx sdk.Context,
	subaccountID common.Hash,
	subaccountTradeNonce *v2.SubaccountTradeNonce,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetSubaccountTradeNonceKey(subaccountID)
	bz := k.cdc.MustMarshal(subaccountTradeNonce)
	store.Set(key, bz)
}

// GetAllSubaccountTradeNonces gets all of trade nonces for all of the subaccounts.
func (k *BaseKeeper) GetAllSubaccountTradeNonces(
	ctx sdk.Context,
) []v2.SubaccountNonce {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	nonceStore := prefix.NewStore(store, types.SubaccountTradeNoncePrefix)

	subaccountNonces := make([]v2.SubaccountNonce, 0)

	iterateSafe(nonceStore.Iterator(nil, nil), func(key, value []byte) bool {
		subaccountID := common.BytesToHash(key[:common.HashLength])
		var subaccountTradeNonce v2.SubaccountTradeNonce
		k.cdc.MustUnmarshal(value, &subaccountTradeNonce)
		subaccountNonces = append(subaccountNonces, v2.SubaccountNonce{
			SubaccountId:         subaccountID.Hex(),
			SubaccountTradeNonce: subaccountTradeNonce,
		})
		return false
	})

	return subaccountNonces
}

func (k *BaseKeeper) GetSubaccountOrderbookMetadata(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
	isBuy bool,
) *v2.SubaccountOrderbookMetadata {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetSubaccountOrderbookMetadataKey(marketID, subaccountID, isBuy)

	bz := store.Get(key)
	if bz == nil {
		return v2.NewSubaccountOrderbookMetadata()
	}

	var metadata v2.SubaccountOrderbookMetadata
	k.cdc.MustUnmarshal(bz, &metadata)

	return &metadata
}

func (k *BaseKeeper) SetSubaccountOrderbookMetadata(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
	isBuy bool,
	metadata *v2.SubaccountOrderbookMetadata,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// no more margin locked while having placed RO conditionals => raise the flag for later invalidation of RO conditional orders
	if (metadata.VanillaLimitOrderCount+metadata.VanillaConditionalOrderCount) == 0 && metadata.ReduceOnlyConditionalOrderCount > 0 {
		k.markForConditionalOrderInvalidation(ctx, marketID, subaccountID, isBuy)
	} else {
		k.removeConditionalOrderInvalidationFlag(ctx, marketID, subaccountID, isBuy)
	}

	store := k.getStore(ctx)
	key := types.GetSubaccountOrderbookMetadataKey(marketID, subaccountID, isBuy)
	bz := k.cdc.MustMarshal(metadata)
	store.Set(key, bz)
}

func (k *BaseKeeper) SetSubaccountOrder(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
	isBuy bool,
	orderHash common.Hash,
	subaccountOrder *v2.SubaccountOrder,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetSubaccountOrderKey(marketID, subaccountID, isBuy, subaccountOrder.Price, orderHash)
	bz := k.cdc.MustMarshal(subaccountOrder)
	store.Set(key, bz)
}

// IterateSubaccountOrdersStartingFromOrder iterates over a subaccount's limit orders for a given marketID and direction
// For buy limit orders, starts iteration over the highest price orders if isStartingIterationFromBestPrice is true
// For sell limit orders, starts iteration over the lowest price orders if isStartingIterationFromBestPrice is true
// Will start iteration from specified order (or default order if nil)
//
//nolint:revive // ok
func (k *BaseKeeper) IterateSubaccountOrdersStartingFromOrder(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
	isBuy bool,
	isStartingIterationFromBestPrice bool,
	startFromInfix []byte, // if set will start iteration from this element, else from the first
	process func(order *v2.SubaccountOrder, orderHash common.Hash) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	prefixKey := types.GetSubaccountOrderPrefixByMarketSubaccountDirection(marketID, subaccountID, isBuy)
	ordersStore := prefix.NewStore(store, prefixKey)

	var iter storetypes.Iterator

	if isBuy && isStartingIterationFromBestPrice || !isBuy && !isStartingIterationFromBestPrice {
		var endInfix []byte
		if startFromInfix != nil {
			endInfix = SubtractBitFromPrefix(startFromInfix) // startFrom is infix of the last found order, so we need to move before it
		}
		iter = ordersStore.ReverseIterator(nil, endInfix)
	} else if !isBuy && isStartingIterationFromBestPrice || isBuy && !isStartingIterationFromBestPrice {
		var startInfix []byte
		if startFromInfix != nil {
			startInfix = AddBitToPrefix(startFromInfix) // startFrom is infix of the last found order, so we need to move beyond it
		}
		iter = ordersStore.Iterator(startInfix, nil)
	}

	iterateSafe(iter, func(key, value []byte) bool {
		var order v2.SubaccountOrder
		k.cdc.MustUnmarshal(value, &order)
		orderHash := common.BytesToHash(key[len(key)-common.HashLength:])
		return process(&order, orderHash)
	})
}

func (k *BaseKeeper) HasSubaccountAlreadyPlacedMarketOrder(ctx sdk.Context, marketID, subaccountID common.Hash) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// use transient store key
	store := k.getTransientStore(ctx)

	key := types.GetSubaccountMarketOrderIndicatorKey(marketID, subaccountID)

	return store.Has(key)
}

func (k *BaseKeeper) HasSubaccountAlreadyPlacedLimitOrder(ctx sdk.Context, marketID, subaccountID common.Hash) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// use transient store key
	store := k.getTransientStore(ctx)

	key := types.GetSubaccountLimitOrderIndicatorKey(marketID, subaccountID)

	return store.Has(key)
}

func (k *BaseKeeper) SetTransientSubaccountMarketOrderIndicator(ctx sdk.Context, marketID, subaccountID common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// use transient store key
	store := k.getTransientStore(ctx)

	key := types.GetSubaccountMarketOrderIndicatorKey(marketID, subaccountID)
	store.Set(key, []byte{})
}

func (k *BaseKeeper) SetTransientSubaccountLimitOrderIndicator(ctx sdk.Context, marketID, subaccountID common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// use transient store key
	store := k.getTransientStore(ctx)

	key := types.GetSubaccountLimitOrderIndicatorKey(marketID, subaccountID)
	store.Set(key, []byte{})
}
