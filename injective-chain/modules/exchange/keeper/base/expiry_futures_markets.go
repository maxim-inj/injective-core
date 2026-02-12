package base

import (
	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/metrics"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// GetExpiryFuturesMarketInfo gets the expiry futures market's market info from the keeper.
func (k *BaseKeeper) GetExpiryFuturesMarketInfo(ctx sdk.Context, marketID common.Hash) *v2.ExpiryFuturesMarketInfo {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	expiryFuturesMarketInfoStore := prefix.NewStore(store, types.ExpiryFuturesMarketInfoPrefix)

	bz := expiryFuturesMarketInfoStore.Get(marketID.Bytes())
	if bz == nil {
		return nil
	}

	var marketInfo v2.ExpiryFuturesMarketInfo
	k.cdc.MustUnmarshal(bz, &marketInfo)

	return &marketInfo
}

// SetExpiryFuturesMarketInfo saves the expiry futures market's market info to the keeper.
func (k *BaseKeeper) SetExpiryFuturesMarketInfo(ctx sdk.Context, marketID common.Hash, marketInfo *v2.ExpiryFuturesMarketInfo) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	expiryFuturesMarketInfoStore := prefix.NewStore(store, types.ExpiryFuturesMarketInfoPrefix)
	key := marketID.Bytes()
	bz := k.cdc.MustMarshal(marketInfo)
	expiryFuturesMarketInfoStore.Set(key, bz)

	if marketInfo.ExpirationTwapStartBaseCumulativePrice.IsNil() || marketInfo.ExpirationTwapStartBaseCumulativePrice.IsZero() {
		k.SetExpiryFuturesMarketInfoByTimestamp(ctx, marketID, marketInfo.TwapStartTimestamp)
	} else {
		k.SetExpiryFuturesMarketInfoByTimestamp(ctx, marketID, marketInfo.ExpirationTimestamp)
	}
}

// DeleteExpiryFuturesMarketInfo deletes the expiry futures market's market info from the keeper.
func (k *BaseKeeper) DeleteExpiryFuturesMarketInfo(ctx sdk.Context, marketID common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	expiryFuturesMarketInfoStore := prefix.NewStore(store, types.ExpiryFuturesMarketInfoPrefix)
	expiryFuturesMarketInfoStore.Delete(marketID.Bytes())
}

// SetExpiryFuturesMarketInfoByTimestamp saves the expiry futures market's market info index to the keeper.
func (k *BaseKeeper) SetExpiryFuturesMarketInfoByTimestamp(ctx sdk.Context, marketID common.Hash, timestamp int64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetExpiryFuturesMarketInfoByTimestampKey(timestamp, marketID)
	store.Set(key, marketID.Bytes())
}

// DeleteExpiryFuturesMarketInfoByTimestamp deletes the expiry futures market's market info index from the keeper.
func (k *BaseKeeper) DeleteExpiryFuturesMarketInfoByTimestamp(ctx sdk.Context, marketID common.Hash, timestamp int64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetExpiryFuturesMarketInfoByTimestampKey(timestamp, marketID)
	store.Delete(key)
}

// IterateExpiryFuturesMarketInfos iterates over expiry futures market's market info calling process on each market info.
func (k *BaseKeeper) IterateExpiryFuturesMarketInfos(ctx sdk.Context, process func(*v2.ExpiryFuturesMarketInfo, common.Hash) (stop bool)) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	expiryFuturesMarketInfoStore := prefix.NewStore(store, types.ExpiryFuturesMarketInfoPrefix)

	iterateSafe(expiryFuturesMarketInfoStore.Iterator(nil, nil), func(key, value []byte) bool {
		var marketInfo v2.ExpiryFuturesMarketInfo
		k.cdc.MustUnmarshal(value, &marketInfo)
		return process(&marketInfo, common.BytesToHash(key))
	})
}

func (k *BaseKeeper) IterateExpiryFuturesMarketInfoByTimestamp(ctx sdk.Context, process func(common.Hash) (stop bool)) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	expiryFuturesMarketInfoStore := prefix.NewStore(store, types.ExpiryFuturesMarketInfoByTimestampPrefix)

	iterateSafe(expiryFuturesMarketInfoStore.Iterator(nil, nil), func(_, value []byte) bool {
		marketID := common.BytesToHash(value)
		return process(marketID)
	})
}
