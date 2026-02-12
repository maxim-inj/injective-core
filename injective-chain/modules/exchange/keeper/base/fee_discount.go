package base

import (
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
	"github.com/InjectiveLabs/metrics"
)

// GetFeeDiscountAccountTierInfo fetches the account's fee discount Tier and TTL info
func (k *BaseKeeper) GetFeeDiscountAccountTierInfo(
	ctx sdk.Context,
	account sdk.AccAddress,
) *v2.FeeDiscountTierTTL {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetFeeDiscountAccountTierKey(account))
	if bz == nil {
		return nil
	}

	var accountTierTTL v2.FeeDiscountTierTTL
	k.cdc.MustUnmarshal(bz, &accountTierTTL)

	return &accountTierTTL
}

// DeleteFeeDiscountAccountTierInfo deletes the account's fee discount Tier and TTL info.
func (k *BaseKeeper) DeleteFeeDiscountAccountTierInfo(
	ctx sdk.Context,
	account sdk.AccAddress,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.GetFeeDiscountAccountTierKey(account))
}

// SetFeeDiscountAccountTierInfo sets the account's fee discount Tier and TTL info.
func (k *BaseKeeper) SetFeeDiscountAccountTierInfo(
	ctx sdk.Context,
	account sdk.AccAddress,
	tierTTL *v2.FeeDiscountTierTTL,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	key := types.GetFeeDiscountAccountTierKey(account)
	bz := k.cdc.MustMarshal(tierTTL)
	store.Set(key, bz)
}

// IterateFeeDiscountAccountTierInfo iterates over all accounts' fee discount Tier and TTL info
func (k *BaseKeeper) IterateFeeDiscountAccountTierInfo(
	ctx sdk.Context,
	process func(account sdk.AccAddress, tierInfo *v2.FeeDiscountTierTTL) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	accountTierStore := prefix.NewStore(store, types.FeeDiscountAccountTierPrefix)

	iterateSafe(accountTierStore.Iterator(nil, nil), func(key, value []byte) bool {
		addr := sdk.AccAddress(key)
		var accountTierTTL v2.FeeDiscountTierTTL
		k.cdc.MustUnmarshal(value, &accountTierTTL)
		return process(addr, &accountTierTTL)
	})
}

// GetFeeDiscountAccountVolumeInBucket fetches the volume for a given account for a given bucket
func (k *BaseKeeper) GetFeeDiscountAccountVolumeInBucket(
	ctx sdk.Context,
	bucketStartTimestamp int64,
	account sdk.AccAddress,
) math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetFeeDiscountAccountVolumeInBucketKey(bucketStartTimestamp, account))
	if bz == nil {
		return math.LegacyZeroDec()
	}
	return types.UnsignedDecBytesToDec(bz)
}

// DeleteFeeDiscountAccountVolumeInBucket deletes the volume for a given account for a given bucket.
func (k *BaseKeeper) DeleteFeeDiscountAccountVolumeInBucket(
	ctx sdk.Context,
	bucketStartTimestamp int64,
	account sdk.AccAddress,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.GetFeeDiscountAccountVolumeInBucketKey(bucketStartTimestamp, account))
}

// SetFeeDiscountAccountVolumeInBucket sets the trading reward points for a given account.
func (k *BaseKeeper) SetFeeDiscountAccountVolumeInBucket(
	ctx sdk.Context,
	bucketStartTimestamp int64,
	account sdk.AccAddress,
	points math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	key := types.GetFeeDiscountAccountVolumeInBucketKey(bucketStartTimestamp, account)
	bz := types.UnsignedDecToUnsignedDecBytes(points)
	store.Set(key, bz)
}

// IterateAccountVolume iterates over total volume in a given bucket for all accounts
func (k *BaseKeeper) IterateAccountVolume(
	ctx sdk.Context,
	process func(bucketStartTimestamp int64, account sdk.AccAddress, totalVolume math.LegacyDec) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	pastBucketVolumeStore := prefix.NewStore(store, types.FeeDiscountBucketAccountVolumePrefix)

	iterateSafe(pastBucketVolumeStore.Iterator(nil, nil), func(key, value []byte) bool {
		bucketStartTime, accountAddress := types.ParseFeeDiscountBucketAccountVolumeIteratorKey(key)
		return process(bucketStartTime, accountAddress, types.UnsignedDecBytesToDec(value))
	})
}

// IterateAccountVolumeInBucket iterates over total volume in a given bucket for all accounts
func (k *BaseKeeper) IterateAccountVolumeInBucket(
	ctx sdk.Context,
	bucketStartTimestamp int64,
	process func(account sdk.AccAddress, totalVolume math.LegacyDec) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	iteratorKey := types.FeeDiscountBucketAccountVolumePrefix
	iteratorKey = append(iteratorKey, sdk.Uint64ToBigEndian(uint64(bucketStartTimestamp))...)
	pastBucketVolumeStore := prefix.NewStore(store, iteratorKey)

	iterateSafe(pastBucketVolumeStore.Iterator(nil, nil), func(key, value []byte) bool {
		return process(sdk.AccAddress(key), types.UnsignedDecBytesToDec(value))
	})
}

//nolint:revive // ok
func (k *BaseKeeper) SetIsFirstFeeCycleFinished(ctx sdk.Context, isFirstFeeCycleFinished bool) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	isFirstFeeCycleFinishedUint := []byte{types.FalseByte}

	if isFirstFeeCycleFinished {
		isFirstFeeCycleFinishedUint = []byte{types.TrueByte}
	}

	store.Set(types.IsFirstFeeCycleFinishedKey, isFirstFeeCycleFinishedUint)
}

func (k *BaseKeeper) GetIsFirstFeeCycleFinished(ctx sdk.Context) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.IsFirstFeeCycleFinishedKey)
	if bz == nil {
		return false
	}

	return types.IsTrueByte(bz)
}

// GetFeeDiscountBucketDuration fetches the bucket duration of the fee discount buckets
func (k *BaseKeeper) GetFeeDiscountBucketDuration(ctx sdk.Context) int64 {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.FeeDiscountBucketDurationKey)
	if bz == nil {
		return 0
	}

	duration := sdk.BigEndianToUint64(bz)
	return int64(duration)
}

// DeleteFeeDiscountBucketDuration deletes the bucket duration of the fee discount buckets.
func (k *BaseKeeper) DeleteFeeDiscountBucketDuration(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.FeeDiscountBucketDurationKey)
}

// SetFeeDiscountBucketDuration sets the bucket duration of the fee discount buckets.
func (k *BaseKeeper) SetFeeDiscountBucketDuration(ctx sdk.Context, duration int64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Set(types.FeeDiscountBucketDurationKey, sdk.Uint64ToBigEndian(uint64(duration)))
}

// GetFeeDiscountCurrentBucketStartTimestamp fetches the start timestamp of the current fee discount bucket
func (k *BaseKeeper) GetFeeDiscountCurrentBucketStartTimestamp(ctx sdk.Context) int64 {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.FeeDiscountCurrentBucketStartTimeKey)
	if bz == nil {
		return 0
	}

	startTimestamp := sdk.BigEndianToUint64(bz)
	return int64(startTimestamp)
}

// DeleteFeeDiscountCurrentBucketStartTimestamp deletes the current bucket start timestamp
func (k *BaseKeeper) DeleteFeeDiscountCurrentBucketStartTimestamp(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.FeeDiscountCurrentBucketStartTimeKey)
}

// SetFeeDiscountCurrentBucketStartTimestamp sets the start timestamp of the current fee discount bucket.
func (k *BaseKeeper) SetFeeDiscountCurrentBucketStartTimestamp(ctx sdk.Context, timestamp int64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Set(types.FeeDiscountCurrentBucketStartTimeKey, sdk.Uint64ToBigEndian(uint64(timestamp)))
}

// GetFeeDiscountBucketCount fetches the bucket count of the fee discount buckets
func (k *BaseKeeper) GetFeeDiscountBucketCount(ctx sdk.Context) uint64 {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.FeeDiscountBucketCountKey)
	if bz == nil {
		return 0
	}

	count := sdk.BigEndianToUint64(bz)
	return count
}

// DeleteFeeDiscountBucketCount deletes the bucket count.
func (k *BaseKeeper) DeleteFeeDiscountBucketCount(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.FeeDiscountBucketCountKey)
}

// SetFeeDiscountBucketCount sets the bucket count of the fee discount buckets.
func (k *BaseKeeper) SetFeeDiscountBucketCount(ctx sdk.Context, count uint64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Set(types.FeeDiscountBucketCountKey, sdk.Uint64ToBigEndian(count))
}

// CheckAndSetFeeDiscountAccountActivityIndicator sets the transient active account indicator if applicable
// for fee discount for the given market
func (k *BaseKeeper) CheckAndSetFeeDiscountAccountActivityIndicator(
	ctx sdk.Context,
	marketID common.Hash,
	account sdk.AccAddress,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if k.HasFeeRewardTransientActiveAccountIndicator(ctx, account) {
		return
	}

	// check transient store first
	tStore := k.getTransientStore(ctx)
	key := types.GetFeeDiscountMarketQualificationKey(marketID)
	qualificationBz := tStore.Get(key)

	if qualificationBz == nil {
		store := k.getStore(ctx)
		qualificationBz = store.Get(key)

		if qualificationBz == nil {
			qualificationBz = []byte{types.FalseByte}
			tStore.Set(key, qualificationBz)
			return
		}

		tStore.Set(key, qualificationBz)
	}

	isQualified := types.IsTrueByte(qualificationBz)
	if isQualified {
		k.setFeeRewardTransientActiveAccountIndicator(ctx, account)
	}
}

// IsMarketQualifiedForFeeDiscount returns true if the given marketID qualifies for fee discount
func (k *BaseKeeper) IsMarketQualifiedForFeeDiscount(ctx sdk.Context, marketID common.Hash) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetFeeDiscountMarketQualificationKey(marketID))
	if bz == nil {
		return false
	}

	return types.IsTrueByte(bz)
}

// DeleteFeeDiscountMarketQualification deletes the market's fee discount qualification indicator
func (k *BaseKeeper) DeleteFeeDiscountMarketQualification(ctx sdk.Context, marketID common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.GetFeeDiscountMarketQualificationKey(marketID))
}

// SetFeeDiscountMarketQualification sets the market's fee discount qualification status in the KV Store
//
//nolint:revive // ok
func (k *BaseKeeper) SetFeeDiscountMarketQualification(
	ctx sdk.Context,
	marketID common.Hash,
	isQualified bool,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	qualificationBz := []byte{types.TrueByte}
	if !isQualified {
		qualificationBz = []byte{types.FalseByte}
	}
	store.Set(types.GetFeeDiscountMarketQualificationKey(marketID), qualificationBz)
}

// IterateFeeDiscountMarketQualifications iterates over the fee discount qualifications
func (k *BaseKeeper) IterateFeeDiscountMarketQualifications(
	ctx sdk.Context,
	process func(common.Hash, bool) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	marketQualificationStore := prefix.NewStore(store, types.FeeDiscountMarketQualificationPrefix)

	iterateSafe(marketQualificationStore.Iterator(nil, nil), func(key, value []byte) bool {
		marketID := common.BytesToHash(key)
		return process(marketID, types.IsTrueByte(value))
	})
}

// GetPastBucketTotalVolume gets the total volume in past buckets
func (k *BaseKeeper) GetPastBucketTotalVolume(
	ctx sdk.Context,
	account sdk.AccAddress,
) math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetFeeDiscountPastBucketAccountVolumeKey(account))
	if bz == nil {
		return math.LegacyZeroDec()
	}
	return types.UnsignedDecBytesToDec(bz)
}

// SetPastBucketTotalVolume sets the total volume in past buckets for the given account
func (k *BaseKeeper) SetPastBucketTotalVolume(
	ctx sdk.Context,
	account sdk.AccAddress,
	volume math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := types.UnsignedDecToUnsignedDecBytes(volume)
	store.Set(types.GetFeeDiscountPastBucketAccountVolumeKey(account), bz)
}

// DeletePastBucketTotalVolume deletes the total volume in past buckets for the given account
func (k *BaseKeeper) DeletePastBucketTotalVolume(
	ctx sdk.Context,
	account sdk.AccAddress,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.GetFeeDiscountPastBucketAccountVolumeKey(account))
}

// IteratePastBucketTotalVolume iterates over total volume in past buckets for all accounts
func (k *BaseKeeper) IteratePastBucketTotalVolume(
	ctx sdk.Context,
	process func(account sdk.AccAddress, totalVolume math.LegacyDec) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	pastBucketVolumeStore := prefix.NewStore(store, types.FeeDiscountAccountPastBucketTotalVolumePrefix)

	iterateSafe(pastBucketVolumeStore.Iterator(nil, nil), func(key, value []byte) bool {
		return process(sdk.AccAddress(key), types.UnsignedDecBytesToDec(value))
	})
}

// GetFeeDiscountSchedule fetches the FeeDiscountSchedule.
func (k *BaseKeeper) GetFeeDiscountSchedule(ctx sdk.Context) *v2.FeeDiscountSchedule {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.FeeDiscountScheduleKey)
	if bz == nil {
		return nil
	}

	var campaignInfo v2.FeeDiscountSchedule
	k.cdc.MustUnmarshal(bz, &campaignInfo)

	return &campaignInfo
}

// DeleteFeeDiscountSchedule deletes the FeeDiscountSchedule.
func (k *BaseKeeper) DeleteFeeDiscountSchedule(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.FeeDiscountScheduleKey)
}

// SetFeeDiscountSchedule sets the FeeDiscountSchedule.
func (k *BaseKeeper) SetFeeDiscountSchedule(ctx sdk.Context, schedule *v2.FeeDiscountSchedule) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := k.cdc.MustMarshal(schedule)
	store.Set(types.FeeDiscountScheduleKey, bz)
}

func (k *BaseKeeper) HasFeeRewardTransientActiveAccountIndicator(ctx sdk.Context, account sdk.AccAddress) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// use transient store key
	tStore := k.getTransientStore(ctx)

	key := types.GetFeeDiscountAccountOrderIndicatorKey(account)
	return tStore.Has(key)
}

// GetAllAccountsActivelyTradingQualifiedMarketsInBlockForFeeDiscounts gets all the accounts that have placed an order
// in qualified markets in this block, not including post-only orders.
func (k *BaseKeeper) GetAllAccountsActivelyTradingQualifiedMarketsInBlockForFeeDiscounts(
	ctx sdk.Context,
) []sdk.AccAddress {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	tStore := k.getTransientStore(ctx)
	accountStore := prefix.NewStore(tStore, types.FeeDiscountAccountOrderIndicatorPrefix)

	iterator := accountStore.Iterator(nil, nil)
	defer iterator.Close()

	accounts := make([]sdk.AccAddress, 0)

	for ; iterator.Valid(); iterator.Next() {
		bz := iterator.Key()
		if len(bz) == 0 {
			continue
		}
		accounts = append(accounts, sdk.AccAddress(bz))
	}

	return accounts
}

func (k *BaseKeeper) setFeeRewardTransientActiveAccountIndicator(ctx sdk.Context, account sdk.AccAddress) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// use transient store key
	tStore := k.getTransientStore(ctx)

	key := types.GetFeeDiscountAccountOrderIndicatorKey(account)
	tStore.Set(key, []byte{})
}
