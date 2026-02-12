package base

import (
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k *BaseKeeper) GetMarketAtomicExecutionFeeMultiplier(
	ctx sdk.Context,
	marketId common.Hash,
	marketType types.MarketType,
) math.LegacyDec {
	metrics.ReportFuncCall(k.svcTags)
	defer metrics.ReportFuncTiming(k.svcTags)()

	store := k.getStore(ctx)
	takerFeeStore := prefix.NewStore(store, types.AtomicMarketOrderTakerFeeMultiplierKey)

	bz := takerFeeStore.Get(marketId.Bytes())

	if bz == nil {
		return k.GetDefaultAtomicMarketOrderFeeMultiplier(ctx, marketType)
	}

	var multiplier v2.MarketFeeMultiplier
	k.cdc.MustUnmarshal(bz, &multiplier)

	return multiplier.FeeMultiplier
}

// GetDefaultAtomicMarketOrderFeeMultiplier returns the default atomic orders taker fee multiplier for a given market type
func (k *BaseKeeper) GetDefaultAtomicMarketOrderFeeMultiplier(ctx sdk.Context, marketType types.MarketType) math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	params := k.GetParams(ctx)

	switch marketType {
	case types.MarketType_Spot:
		return params.SpotAtomicMarketOrderFeeMultiplier
	case types.MarketType_Expiry, types.MarketType_Perpetual:
		return params.DerivativeAtomicMarketOrderFeeMultiplier
	case types.MarketType_BinaryOption:
		return params.BinaryOptionsAtomicMarketOrderFeeMultiplier
	default:
		return math.LegacyDec{}
	}
}

func (k *BaseKeeper) GetAllMarketAtomicExecutionFeeMultipliers(ctx sdk.Context) []*v2.MarketFeeMultiplier {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	takerFeeStore := prefix.NewStore(store, types.AtomicMarketOrderTakerFeeMultiplierKey)

	multipliers := make([]*v2.MarketFeeMultiplier, 0)

	iterateSafe(takerFeeStore.Iterator(nil, nil), func(_, value []byte) bool {
		var multiplier v2.MarketFeeMultiplier
		k.cdc.MustUnmarshal(value, &multiplier)
		multipliers = append(multipliers, &multiplier)
		return false
	})

	return multipliers
}

func (k *BaseKeeper) SetAtomicMarketOrderFeeMultipliers(ctx sdk.Context, marketFeeMultipliers []*v2.MarketFeeMultiplier) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	takerFeeStore := prefix.NewStore(store, types.AtomicMarketOrderTakerFeeMultiplierKey)

	for _, multiplier := range marketFeeMultipliers {
		marketID := common.HexToHash(multiplier.MarketId)
		bz := k.cdc.MustMarshal(multiplier)
		takerFeeStore.Set(marketID.Bytes(), bz)
	}
}

func (k *BaseKeeper) AppendOrderExpirations(
	ctx sdk.Context,
	marketID common.Hash,
	expirationBlock int64,
	order *v2.OrderData,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	expirationStore := prefix.NewStore(store, types.GetOrderExpirationPrefix(expirationBlock, marketID))

	bz := k.cdc.MustMarshal(order)
	expirationStore.Set(common.HexToHash(order.OrderHash).Bytes(), bz)

	expirationMarketsStore := prefix.NewStore(store, types.GetOrderExpirationMarketPrefix(expirationBlock))
	expirationMarketsStore.Set(marketID.Bytes(), []byte{types.TrueByte})
}

func (k *BaseKeeper) DeleteMarketWithOrderExpirations(
	ctx sdk.Context,
	marketID common.Hash,
	expirationBlock int64,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	expirationMarketsStore := prefix.NewStore(store, types.GetOrderExpirationMarketPrefix(expirationBlock))
	expirationMarketsStore.Delete(marketID.Bytes())
}

func (k *BaseKeeper) DeleteOrderExpiration(
	ctx sdk.Context,
	marketID common.Hash,
	expirationBlock int64,
	orderHash common.Hash,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	expirationStore := prefix.NewStore(store, types.GetOrderExpirationPrefix(expirationBlock, marketID))
	expirationStore.Delete(orderHash.Bytes())
}

// GetMarketsWithOrderExpirations retrieves all markets with orders expiring at a given block
func (k *BaseKeeper) GetMarketsWithOrderExpirations(
	ctx sdk.Context,
	expirationBlock int64,
) []common.Hash {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	expirationMarketsStore := prefix.NewStore(store, types.GetOrderExpirationMarketPrefix(expirationBlock))

	markets := make([]common.Hash, 0)

	iterateSafe(expirationMarketsStore.Iterator(nil, nil), func(key, _ []byte) bool {
		marketID := common.BytesToHash(key)
		markets = append(markets, marketID)
		return false
	})

	return markets
}

// GetOrdersByExpiration retrieves all derivative limit orders expiring at a specific block for a market
func (k *BaseKeeper) GetOrdersByExpiration(
	ctx sdk.Context,
	marketID common.Hash,
	expirationBlock int64,
) ([]*v2.OrderData, error) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orders := make([]*v2.OrderData, 0)
	store := k.getStore(ctx)

	expirationStore := prefix.NewStore(store, types.GetOrderExpirationPrefix(expirationBlock, marketID))

	var err error

	iterateSafe(expirationStore.Iterator(nil, nil), func(_, value []byte) bool {
		var order v2.OrderData
		if err = k.cdc.Unmarshal(value, &order); err != nil {
			return true
		}
		orders = append(orders, &order)
		return false
	})

	if err != nil {
		return nil, err
	}

	return orders, nil
}

func (k *BaseKeeper) GetAllMarketIDsWithQuoteDenoms(ctx sdk.Context) []*v2.MarketIDQuoteDenomMakerFee {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	derivativeMarkets := k.GetAllDerivativeMarkets(ctx)
	spotMarkets := k.GetAllSpotMarkets(ctx)
	binaryOptionsMarkets := k.GetAllBinaryOptionsMarkets(ctx)

	marketIDQuoteDenoms := make([]*v2.MarketIDQuoteDenomMakerFee, 0, len(derivativeMarkets)+len(spotMarkets)+len(binaryOptionsMarkets))

	for _, m := range derivativeMarkets {
		marketIDQuoteDenoms = append(marketIDQuoteDenoms, &v2.MarketIDQuoteDenomMakerFee{
			MarketID:   common.HexToHash(m.MarketId),
			QuoteDenom: m.QuoteDenom,
			MakerFee:   m.MakerFeeRate,
		})
	}

	for _, m := range spotMarkets {
		marketIDQuoteDenoms = append(marketIDQuoteDenoms, &v2.MarketIDQuoteDenomMakerFee{
			MarketID:   m.MarketID(),
			QuoteDenom: m.QuoteDenom,
			MakerFee:   m.MakerFeeRate,
		})
	}

	for _, m := range binaryOptionsMarkets {
		marketIDQuoteDenoms = append(marketIDQuoteDenoms, &v2.MarketIDQuoteDenomMakerFee{
			MarketID:   m.MarketID(),
			QuoteDenom: m.QuoteDenom,
			MakerFee:   m.MakerFeeRate,
		})
	}

	return marketIDQuoteDenoms
}
