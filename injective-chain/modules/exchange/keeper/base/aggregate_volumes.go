package base

import (
	"cosmossdk.io/store/prefix"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// GetSubaccountMarketAggregateVolume fetches the aggregate volume for a given subaccountID and marketID
func (k *BaseKeeper) GetSubaccountMarketAggregateVolume(
	ctx sdk.Context,
	subaccountID, marketID common.Hash,
) v2.VolumeRecord {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetSubaccountMarketVolumeKey(subaccountID, marketID))
	if bz == nil {
		return v2.NewZeroVolumeRecord()
	}

	var vc v2.VolumeRecord
	k.cdc.MustUnmarshal(bz, &vc)

	return vc
}

// SetSubaccountMarketAggregateVolume sets the trading volume for a given subaccountID and marketID
func (k *BaseKeeper) SetSubaccountMarketAggregateVolume(
	ctx sdk.Context,
	subaccountID, marketID common.Hash,
	volume v2.VolumeRecord,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	key := types.GetSubaccountMarketVolumeKey(subaccountID, marketID)
	bz := k.cdc.MustMarshal(&volume)
	store.Set(key, bz)
}

// IterateSubaccountMarketAggregateVolumes iterates over all of the aggregate subaccount market volumes
func (k *BaseKeeper) IterateSubaccountMarketAggregateVolumes(
	ctx sdk.Context,
	process func(subaccountID, marketID common.Hash, volume v2.VolumeRecord) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	volumeStore := prefix.NewStore(store, types.SubaccountMarketVolumePrefix)

	iterateSafe(volumeStore.Iterator(nil, nil), func(key, value []byte) bool {
		subaccountID := common.BytesToHash(key[:common.HashLength])
		marketID := common.BytesToHash(key[common.HashLength:])

		var volumes v2.VolumeRecord
		k.cdc.MustUnmarshal(value, &volumes)
		return process(subaccountID, marketID, volumes)
	})
}

// IterateSubaccountMarketAggregateVolumesBySubaccount iterates over all of the aggregate subaccount market volumes
// for the specified subaccount
func (k *BaseKeeper) IterateSubaccountMarketAggregateVolumesBySubaccount(
	ctx sdk.Context,
	subaccountID common.Hash,
	process func(marketID common.Hash, volume v2.VolumeRecord) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	volumeStore := prefix.NewStore(k.getStore(ctx), append(types.SubaccountMarketVolumePrefix, subaccountID.Bytes()...))

	iterateSafe(volumeStore.Iterator(nil, nil), func(key, value []byte) bool {
		marketID := common.BytesToHash(key)

		var volumes v2.VolumeRecord
		k.cdc.MustUnmarshal(value, &volumes)
		return process(marketID, volumes)
	})
}

// IterateSubaccountMarketAggregateVolumesByAccAddress iterates over all of the aggregate subaccount market volumes
// for the specified account address
func (k *BaseKeeper) IterateSubaccountMarketAggregateVolumesByAccAddress(
	ctx sdk.Context,
	accAddress sdk.AccAddress,
	process func(subaccountID, marketID common.Hash, volume v2.VolumeRecord) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	volumeStore := prefix.NewStore(store, append(types.SubaccountMarketVolumePrefix, accAddress.Bytes()...))

	iterateSafe(volumeStore.Iterator(nil, nil), func(key, value []byte) bool {
		subaccountID := common.BytesToHash(append(accAddress.Bytes(), key[:12]...))
		marketID := common.BytesToHash(key[12:])
		var volumes v2.VolumeRecord
		k.cdc.MustUnmarshal(value, &volumes)
		return process(subaccountID, marketID, volumes)
	})
}

// GetMarketAggregateVolume fetches the aggregate volume for a given marketID
func (k *BaseKeeper) GetMarketAggregateVolume(
	ctx sdk.Context,
	marketID common.Hash,
) v2.VolumeRecord {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetMarketVolumeKey(marketID))
	if bz == nil {
		return v2.NewZeroVolumeRecord()
	}

	var vc v2.VolumeRecord
	k.cdc.MustUnmarshal(bz, &vc)

	return vc
}

// SetMarketAggregateVolume sets the trading volume for a given subaccountID and marketID
func (k *BaseKeeper) SetMarketAggregateVolume(
	ctx sdk.Context,
	marketID common.Hash,
	volumes v2.VolumeRecord,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	key := types.GetMarketVolumeKey(marketID)

	bz := k.cdc.MustMarshal(&volumes)
	store.Set(key, bz)
}

// IterateMarketAggregateVolumes iterates over the aggregate volumes for all markets
func (k *BaseKeeper) IterateMarketAggregateVolumes(
	ctx sdk.Context,
	process func(marketID common.Hash, volume v2.VolumeRecord) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	volumeStore := prefix.NewStore(store, types.MarketVolumePrefix)

	iterateSafe(volumeStore.Iterator(nil, nil), func(key, value []byte) bool {
		marketID := common.BytesToHash(key)
		var volumes v2.VolumeRecord
		k.cdc.MustUnmarshal(value, &volumes)
		return process(marketID, volumes)
	})
}

func (k *BaseKeeper) GetAllMarketAggregateVolumes(ctx sdk.Context) []*v2.MarketVolume {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	volumes := make([]*v2.MarketVolume, 0)
	k.IterateMarketAggregateVolumes(ctx, func(marketID common.Hash, totalVolume v2.VolumeRecord) (stop bool) {
		volumes = append(volumes, &v2.MarketVolume{
			MarketId: marketID.Hex(),
			Volume:   totalVolume,
		})

		return false
	})

	return volumes
}
