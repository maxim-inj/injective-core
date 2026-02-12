package rewards

import (
	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// MoveRewardPointsToPending moves the reward points to the pending pools
func (k TradingKeeper) MoveRewardPointsToPending(
	ctx sdk.Context, allAccountPoints []*types.TradingRewardAccountPoints, totalPoints math.LegacyDec, pendingPoolStartTimestamp int64,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	for _, accountPoint := range allAccountPoints {
		k.SetAccountCampaignTradingRewardPendingPoints(ctx, accountPoint.Account, pendingPoolStartTimestamp, accountPoint.Points)
		k.DeleteAccountCampaignTradingRewardPoints(ctx, accountPoint.Account)
	}

	k.SetTotalTradingRewardPendingPoints(ctx, totalPoints, pendingPoolStartTimestamp)
	k.DeleteTotalTradingRewardPoints(ctx)
}

// UpdateAccountCampaignTradingRewardPendingPoints applies a point delta to the existing points.
func (k TradingKeeper) UpdateAccountCampaignTradingRewardPendingPoints(
	ctx sdk.Context,
	account sdk.AccAddress,
	addedPoints math.LegacyDec,
	pendingPoolStartTimestamp int64,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if addedPoints.IsZero() {
		return
	}

	accountPoints := k.GetCampaignTradingRewardPendingPoints(ctx, account, pendingPoolStartTimestamp)
	accountPoints = accountPoints.Add(addedPoints)
	k.SetAccountCampaignTradingRewardPendingPoints(ctx, account, pendingPoolStartTimestamp, accountPoints)
}

// GetAllTradingRewardCampaignAccountPendingPoints gets the trading reward points for all accounts
func (k TradingKeeper) GetAllTradingRewardCampaignAccountPendingPoints(ctx sdk.Context) []*v2.TradingRewardCampaignAccountPendingPoints {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	accountPoints := make([]*v2.TradingRewardCampaignAccountPendingPoints, 0)
	appendPoints := func(
		pendingPoolStartTimestamp int64,
		account sdk.AccAddress,
		points math.LegacyDec,
	) (stop bool) {
		currentPoolCount := len(accountPoints)
		isNewPool := currentPoolCount == 0 || accountPoints[currentPoolCount-1].RewardPoolStartTimestamp != pendingPoolStartTimestamp

		if isNewPool {
			accountPoints = append(accountPoints, &v2.TradingRewardCampaignAccountPendingPoints{
				RewardPoolStartTimestamp: pendingPoolStartTimestamp,
				AccountPoints: []*v2.TradingRewardCampaignAccountPoints{{
					Account: account.String(),
					Points:  points,
				}},
			})

			return false
		}

		accountPoints[currentPoolCount-1].AccountPoints = append(
			accountPoints[currentPoolCount-1].AccountPoints,
			&v2.TradingRewardCampaignAccountPoints{
				Account: account.String(),
				Points:  points,
			},
		)

		return false
	}

	k.IterateAccountCampaignTradingRewardPendingPoints(ctx, appendPoints)
	return accountPoints
}

// GetAllAccountCampaignTradingRewardPendingPointsWithTotalPointsForPool gets the trading reward points for all accounts
func (k TradingKeeper) GetAllAccountCampaignTradingRewardPendingPointsWithTotalPointsForPool(
	ctx sdk.Context,
	pendingPoolStartTimestamp int64,
) (accountPoints []*types.TradingRewardAccountPoints, totalPoints math.LegacyDec) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	accountPoints = make([]*types.TradingRewardAccountPoints, 0)
	totalPoints = math.LegacyZeroDec()

	appendPoints := func(points *types.TradingRewardAccountPoints) (stop bool) {
		accountPoints = append(accountPoints, points)
		totalPoints = totalPoints.Add(points.Points)
		return false
	}

	k.IterateAccountTradingRewardPendingPointsForPool(ctx, pendingPoolStartTimestamp, appendPoints)
	return accountPoints, totalPoints
}

// IncrementTotalTradingRewardPendingPoints sets the total trading reward points
func (k TradingKeeper) IncrementTotalTradingRewardPendingPoints(
	ctx sdk.Context,
	points math.LegacyDec,
	pendingPoolStartTimestamp int64,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	currPoints := k.GetTotalTradingRewardPendingPoints(ctx, pendingPoolStartTimestamp)
	newPoints := currPoints.Add(points)
	k.SetTotalTradingRewardPendingPoints(ctx, newPoints, pendingPoolStartTimestamp)
}

// PersistTradingRewardPendingPoints persists the trading reward pending points
func (k TradingKeeper) PersistTradingRewardPendingPoints(
	ctx sdk.Context,
	tradingRewards types.TradingRewardPoints,
	pendingPoolStartTimestamp int64,
) {
	totalTradingRewardPoints := math.LegacyZeroDec()

	for _, account := range tradingRewards.GetSortedAccountKeys() {
		addr, _ := sdk.AccAddressFromBech32(account)
		accountTradingRewardPoints := tradingRewards[account]

		k.UpdateAccountCampaignTradingRewardPendingPoints(ctx, addr, accountTradingRewardPoints, pendingPoolStartTimestamp)
		totalTradingRewardPoints = totalTradingRewardPoints.Add(accountTradingRewardPoints)
	}

	k.IncrementTotalTradingRewardPendingPoints(ctx, totalTradingRewardPoints, pendingPoolStartTimestamp)
}
