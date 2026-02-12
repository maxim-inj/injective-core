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

// GetIsOptedOutOfRewards returns if the account is opted out of rewards
func (k *BaseKeeper) GetIsOptedOutOfRewards(ctx sdk.Context, account sdk.AccAddress) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetIsOptedOutOfRewardsKey(account))
	if bz == nil {
		return false
	}

	return types.IsTrueByte(bz)
}

// SetIsOptedOutOfRewards sets if the account is opted out of rewards
//
//nolint:revive // ok
func (k *BaseKeeper) SetIsOptedOutOfRewards(ctx sdk.Context, account sdk.AccAddress, isOptedOut bool) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	key := types.GetIsOptedOutOfRewardsKey(account)

	isOptedOutUint := []byte{types.FalseByte}

	if isOptedOut {
		isOptedOutUint = []byte{types.TrueByte}
	}

	store.Set(key, isOptedOutUint)
}

// IterateOptedOutRewardAccounts iterates over registered DMMs
func (k *BaseKeeper) IterateOptedOutRewardAccounts(
	ctx sdk.Context,
	process func(account sdk.AccAddress, isOptedOut bool) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	rewardsOptOutStore := prefix.NewStore(store, types.IsOptedOutOfRewardsPrefix)

	iterateSafe(rewardsOptOutStore.Iterator(nil, nil), func(key, value []byte) bool {
		addr := sdk.AccAddress(key)
		isOptedOut := value != nil && types.IsTrueByte(value)
		return process(addr, isOptedOut)
	})
}

// GetCurrentCampaignEndTimestamp fetches the end timestamp of the current TradingRewardCampaign.
func (k *BaseKeeper) GetCurrentCampaignEndTimestamp(ctx sdk.Context) int64 {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.TradingRewardCurrentCampaignEndTimeKey)
	if bz == nil {
		return 0
	}

	timestamp := sdk.BigEndianToUint64(bz)
	return int64(timestamp)
}

// DeleteCurrentCampaignEndTimestamp deletes the end timestamp of the current TradingRewardCampaign.
func (k *BaseKeeper) DeleteCurrentCampaignEndTimestamp(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.TradingRewardCurrentCampaignEndTimeKey)
}

// SetCurrentCampaignEndTimestamp sets the end timestamp of the current TradingRewardCampaign.
func (k *BaseKeeper) SetCurrentCampaignEndTimestamp(ctx sdk.Context, endTimestamp int64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Set(types.TradingRewardCurrentCampaignEndTimeKey, sdk.Uint64ToBigEndian(uint64(endTimestamp)))
}

// GetCampaignInfo fetches the TradingRewardCampaignInfo.
func (k *BaseKeeper) GetCampaignInfo(ctx sdk.Context) *v2.TradingRewardCampaignInfo {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.TradingRewardCampaignInfoKey)
	if bz == nil {
		return nil
	}

	var campaignInfo v2.TradingRewardCampaignInfo
	k.cdc.MustUnmarshal(bz, &campaignInfo)

	return &campaignInfo
}

// DeleteCampaignInfo deletes the TradingRewardCampaignInfo.
func (k *BaseKeeper) DeleteCampaignInfo(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.TradingRewardCampaignInfoKey)
}

// SetCampaignInfo sets the TradingRewardCampaignInfo.
func (k *BaseKeeper) SetCampaignInfo(ctx sdk.Context, campaignInfo *v2.TradingRewardCampaignInfo) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := k.cdc.MustMarshal(campaignInfo)
	store.Set(types.TradingRewardCampaignInfoKey, bz)
}

// GetEffectiveTradingRewardsMarketPointsMultiplierConfig returns the market's points multiplier if the marketID is qualified
// and has a multiplier, and returns a multiplier of 0 otherwise
func (k *BaseKeeper) GetEffectiveTradingRewardsMarketPointsMultiplierConfig(ctx sdk.Context, marketID common.Hash) v2.PointsMultiplier {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetTradingRewardsMarketPointsMultiplierKey(marketID))
	isQualified := k.IsMarketQualifiedForTradingRewards(ctx, marketID)

	hasDefaultMultiplier := bz == nil && isQualified
	if hasDefaultMultiplier {
		return v2.PointsMultiplier{
			MakerPointsMultiplier: math.LegacyOneDec(),
			TakerPointsMultiplier: math.LegacyOneDec(),
		}
	}

	hasNoMultiplier := bz == nil && !isQualified
	if hasNoMultiplier {
		return v2.PointsMultiplier{
			MakerPointsMultiplier: math.LegacyZeroDec(),
			TakerPointsMultiplier: math.LegacyZeroDec(),
		}
	}

	var multiplier v2.PointsMultiplier
	k.cdc.MustUnmarshal(bz, &multiplier)

	return multiplier
}

// DeleteTradingRewardsMarketPointsMultiplier deletes the market's points multiplier
func (k *BaseKeeper) DeleteTradingRewardsMarketPointsMultiplier(ctx sdk.Context, marketID common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.GetTradingRewardsMarketPointsMultiplierKey(marketID))
}

// SetTradingRewardsMarketPointsMultiplier sets the market's points multiplier
func (k *BaseKeeper) SetTradingRewardsMarketPointsMultiplier(ctx sdk.Context, marketID common.Hash, multiplier *v2.PointsMultiplier) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := k.cdc.MustMarshal(multiplier)
	store.Set(types.GetTradingRewardsMarketPointsMultiplierKey(marketID), bz)
}

// IterateTradingRewardsMarketPointsMultipliers iterates over the trading reward market point multipliers
func (k *BaseKeeper) IterateTradingRewardsMarketPointsMultipliers(
	ctx sdk.Context,
	process func(*v2.PointsMultiplier, common.Hash) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	multiplierStore := prefix.NewStore(store, types.TradingRewardMarketPointsMultiplierPrefix)

	iterateSafe(multiplierStore.Iterator(nil, nil), func(key, value []byte) bool {
		var multiplier v2.PointsMultiplier
		k.cdc.MustUnmarshal(value, &multiplier)
		marketID := common.BytesToHash(key)
		return process(&multiplier, marketID)
	})
}

// IsMarketQualifiedForTradingRewards returns true if the given marketID qualifies for trading rewards
func (k *BaseKeeper) IsMarketQualifiedForTradingRewards(ctx sdk.Context, marketID common.Hash) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetCampaignMarketQualificationKey(marketID))
	if bz == nil {
		return false
	}

	return types.IsTrueByte(bz)
}

// DeleteTradingRewardsMarketQualification deletes the market's trading reward qualification indicator
func (k *BaseKeeper) DeleteTradingRewardsMarketQualification(ctx sdk.Context, marketID common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.GetCampaignMarketQualificationKey(marketID))
}

// SetTradingRewardsMarketQualification sets the market's trading reward qualification indicator
//
//nolint:revive // ok
func (k *BaseKeeper) SetTradingRewardsMarketQualification(
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
	store.Set(types.GetCampaignMarketQualificationKey(marketID), qualificationBz)
}

// IterateTradingRewardsMarketQualifications iterates over the trading reward pools
func (k *BaseKeeper) IterateTradingRewardsMarketQualifications(
	ctx sdk.Context,
	process func(common.Hash, bool) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	marketQualificationStore := prefix.NewStore(store, types.TradingRewardMarketQualificationPrefix)

	iterateSafe(marketQualificationStore.Iterator(nil, nil), func(key, value []byte) bool {
		marketID := common.BytesToHash(key)
		return process(marketID, types.IsTrueByte(value))
	})
}

// GetCampaignTradingRewardPendingPoints fetches the trading reward points for a given account.
func (k *BaseKeeper) GetCampaignTradingRewardPendingPoints(
	ctx sdk.Context, account sdk.AccAddress, pendingPoolStartTimestamp int64,
) math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetTradingRewardAccountPendingPointsKey(account, pendingPoolStartTimestamp))
	if bz == nil {
		return math.LegacyZeroDec()
	}

	return types.UnsignedDecBytesToDec(bz)
}

// DeleteAccountCampaignTradingRewardPendingPoints deletes the trading reward points for a given account.
func (k *BaseKeeper) DeleteAccountCampaignTradingRewardPendingPoints(
	ctx sdk.Context,
	account sdk.AccAddress,
	pendingPoolStartTimestamp int64,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.GetTradingRewardAccountPendingPointsKey(account, pendingPoolStartTimestamp))
}

// SetAccountCampaignTradingRewardPendingPoints sets the trading reward points for a given account.
func (k *BaseKeeper) SetAccountCampaignTradingRewardPendingPoints(
	ctx sdk.Context, account sdk.AccAddress, pendingPoolStartTimestamp int64, points math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	key := types.GetTradingRewardAccountPendingPointsKey(account, pendingPoolStartTimestamp)
	bz := types.UnsignedDecToUnsignedDecBytes(points)
	store.Set(key, bz)
}

// IterateAccountCampaignTradingRewardPendingPoints iterates over the trading reward account points
func (k *BaseKeeper) IterateAccountCampaignTradingRewardPendingPoints(
	ctx sdk.Context,
	process func(pendingPoolStartTimestamp int64, account sdk.AccAddress, points math.LegacyDec) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	pointsStore := prefix.NewStore(store, types.TradingRewardAccountPendingPointsPrefix)

	iterateSafe(pointsStore.Iterator(nil, nil), func(key, value []byte) bool {
		pendingPoolStartTimestamp, account := types.ParseTradingRewardAccountPendingPointsKey(key)
		return process(pendingPoolStartTimestamp, account, types.UnsignedDecBytesToDec(value))
	})
}

// IterateAccountTradingRewardPendingPointsForPool iterates over the trading reward account points
func (k *BaseKeeper) IterateAccountTradingRewardPendingPointsForPool(
	ctx sdk.Context,
	pendingPoolStartTimestamp int64,
	process func(*types.TradingRewardAccountPoints) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	pointsStore := prefix.NewStore(store, types.GetTradingRewardAccountPendingPointsPrefix(pendingPoolStartTimestamp))

	iterateSafe(pointsStore.Iterator(nil, nil), func(key, value []byte) bool {
		points := types.UnsignedDecBytesToDec(value)
		account := sdk.AccAddress(key)
		return process(&types.TradingRewardAccountPoints{
			Account: account,
			Points:  points,
		})
	})
}

// GetTotalTradingRewardPendingPoints gets the total trading reward points
func (k *BaseKeeper) GetTotalTradingRewardPendingPoints(
	ctx sdk.Context,
	pendingPoolStartTimestamp int64,
) math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetTradingRewardTotalPendingPointsKey(pendingPoolStartTimestamp))
	if bz == nil {
		return math.LegacyZeroDec()
	}
	return types.UnsignedDecBytesToDec(bz)
}

// SetTotalTradingRewardPendingPoints sets the total trading reward points
func (k *BaseKeeper) SetTotalTradingRewardPendingPoints(
	ctx sdk.Context,
	points math.LegacyDec,
	pendingPoolStartTimestamp int64,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := types.UnsignedDecToUnsignedDecBytes(points)
	store.Set(types.GetTradingRewardTotalPendingPointsKey(pendingPoolStartTimestamp), bz)
}

// DeleteTotalTradingRewardPendingPoints deletes the total trading reward points
func (k *BaseKeeper) DeleteTotalTradingRewardPendingPoints(
	ctx sdk.Context,
	pendingPoolStartTimestamp int64,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.GetTradingRewardTotalPendingPointsKey(pendingPoolStartTimestamp))
}

// GetCampaignRewardPendingPool fetches the trading reward pool corresponding to a given start timestamp.
func (k *BaseKeeper) GetCampaignRewardPendingPool(ctx sdk.Context, startTimestamp int64) *v2.CampaignRewardPool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetCampaignRewardPendingPoolKey(startTimestamp))
	if bz == nil {
		return nil
	}

	var rewardPool v2.CampaignRewardPool
	k.cdc.MustUnmarshal(bz, &rewardPool)
	return &rewardPool
}

// DeleteCampaignRewardPendingPool deletes the trading reward pool corresponding to a given start timestamp.
func (k *BaseKeeper) DeleteCampaignRewardPendingPool(ctx sdk.Context, startTimestamp int64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.GetCampaignRewardPendingPoolKey(startTimestamp))
}

// SetCampaignRewardPendingPool sets the trading reward pool corresponding to a given start timestamp.
func (k *BaseKeeper) SetCampaignRewardPendingPool(ctx sdk.Context, rewardPool *v2.CampaignRewardPool) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := k.cdc.MustMarshal(rewardPool)
	store.Set(types.GetCampaignRewardPendingPoolKey(rewardPool.StartTimestamp), bz)
}

// IterateCampaignRewardPendingPools iterates over the trading reward pools
//
//nolint:revive // ok
func (k *BaseKeeper) IterateCampaignRewardPendingPools(
	ctx sdk.Context,
	shouldReverseIterate bool,
	process func(*v2.CampaignRewardPool) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	rewardPoolStore := prefix.NewStore(store, types.TradingRewardCampaignRewardPendingPoolPrefix)

	var iter storetypes.Iterator
	if shouldReverseIterate {
		iter = rewardPoolStore.ReverseIterator(nil, nil)
	} else {
		iter = rewardPoolStore.Iterator(nil, nil)
	}

	iterateSafe(iter, func(key, value []byte) bool {
		var pool v2.CampaignRewardPool
		k.cdc.MustUnmarshal(value, &pool)
		return process(&pool)
	})
}

// GetCampaignTradingRewardPoints fetches the trading reward points for a given account.
func (k *BaseKeeper) GetCampaignTradingRewardPoints(ctx sdk.Context, account sdk.AccAddress) math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetTradingRewardAccountPointsKey(account))
	if bz == nil {
		return math.LegacyZeroDec()
	}
	return types.UnsignedDecBytesToDec(bz)
}

// DeleteAccountCampaignTradingRewardPoints deletes the trading reward points for a given account.
func (k *BaseKeeper) DeleteAccountCampaignTradingRewardPoints(ctx sdk.Context, account sdk.AccAddress) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.GetTradingRewardAccountPointsKey(account))
}

// SetAccountCampaignTradingRewardPoints sets the trading reward points for a given account.
func (k *BaseKeeper) SetAccountCampaignTradingRewardPoints(ctx sdk.Context, account sdk.AccAddress, points math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)

	key := types.GetTradingRewardAccountPointsKey(account)
	bz := types.UnsignedDecToUnsignedDecBytes(points)
	store.Set(key, bz)
}

// IterateAccountCampaignTradingRewardPoints iterates over the trading reward account points
func (k *BaseKeeper) IterateAccountCampaignTradingRewardPoints(
	ctx sdk.Context,
	process func(*types.TradingRewardAccountPoints) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	pointsStore := prefix.NewStore(store, types.TradingRewardAccountPointsPrefix)

	iterateSafe(pointsStore.Iterator(nil, nil), func(key, value []byte) bool {
		accountPoints := &types.TradingRewardAccountPoints{
			Account: sdk.AccAddress(key),
			Points:  types.UnsignedDecBytesToDec(value),
		}
		return process(accountPoints)
	})
}

// GetTotalTradingRewardPoints gets the total trading reward points
func (k *BaseKeeper) GetTotalTradingRewardPoints(
	ctx sdk.Context,
) math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.TradingRewardCampaignTotalPointsKey)
	if bz == nil {
		return math.LegacyZeroDec()
	}
	return types.UnsignedDecBytesToDec(bz)
}

// SetTotalTradingRewardPoints sets the total trading reward points
func (k *BaseKeeper) SetTotalTradingRewardPoints(
	ctx sdk.Context,
	points math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := types.UnsignedDecToUnsignedDecBytes(points)
	store.Set(types.TradingRewardCampaignTotalPointsKey, bz)
}

// DeleteTotalTradingRewardPoints deletes the total trading reward points
func (k *BaseKeeper) DeleteTotalTradingRewardPoints(
	ctx sdk.Context,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.TradingRewardCampaignTotalPointsKey)
}

// GetCampaignRewardPool fetches the trading reward pool corresponding to a given start timestamp.
func (k *BaseKeeper) GetCampaignRewardPool(ctx sdk.Context, startTimestamp int64) *v2.CampaignRewardPool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := store.Get(types.GetCampaignRewardPoolKey(startTimestamp))
	if bz == nil {
		return nil
	}

	var rewardPool v2.CampaignRewardPool
	k.cdc.MustUnmarshal(bz, &rewardPool)
	return &rewardPool
}

// DeleteCampaignRewardPool deletes the trading reward pool corresponding to a given start timestamp.
func (k *BaseKeeper) DeleteCampaignRewardPool(ctx sdk.Context, startTimestamp int64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	store.Delete(types.GetCampaignRewardPoolKey(startTimestamp))
}

// SetCampaignRewardPool sets the trading reward pool corresponding to a given start timestamp.
func (k *BaseKeeper) SetCampaignRewardPool(ctx sdk.Context, rewardPool *v2.CampaignRewardPool) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	bz := k.cdc.MustMarshal(rewardPool)
	store.Set(types.GetCampaignRewardPoolKey(rewardPool.StartTimestamp), bz)
}

// IterateCampaignRewardPools iterates over the trading reward pools
//
//nolint:revive // ok
func (k *BaseKeeper) IterateCampaignRewardPools(
	ctx sdk.Context,
	shouldReverseIterate bool,
	process func(*v2.CampaignRewardPool) (stop bool),
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := k.getStore(ctx)
	rewardPoolStore := prefix.NewStore(store, types.TradingRewardCampaignRewardPoolPrefix)

	var iter storetypes.Iterator
	if shouldReverseIterate {
		iter = rewardPoolStore.ReverseIterator(nil, nil)
	} else {
		iter = rewardPoolStore.Iterator(nil, nil)
	}

	iterateSafe(iter, func(key, value []byte) bool {
		var pool v2.CampaignRewardPool
		k.cdc.MustUnmarshal(value, &pool)
		return process(&pool)
	})
}
