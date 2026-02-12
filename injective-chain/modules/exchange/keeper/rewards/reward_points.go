package rewards

import (
	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// UpdateAccountCampaignTradingRewardPoints applies a point delta to the existing points.
func (k TradingKeeper) UpdateAccountCampaignTradingRewardPoints(
	ctx sdk.Context,
	account sdk.AccAddress,
	addedPoints math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if addedPoints.IsZero() {
		return
	}

	accountPoints := k.GetCampaignTradingRewardPoints(ctx, account)
	accountPoints = accountPoints.Add(addedPoints)
	k.SetAccountCampaignTradingRewardPoints(ctx, account, accountPoints)
}

// GetAllTradingRewardCampaignAccountPoints gets the trading reward points for all accounts
func (k TradingKeeper) GetAllTradingRewardCampaignAccountPoints(ctx sdk.Context) []*v2.TradingRewardCampaignAccountPoints {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	accountPoints := make([]*v2.TradingRewardCampaignAccountPoints, 0)
	k.IterateAccountCampaignTradingRewardPoints(ctx, func(points *types.TradingRewardAccountPoints) (stop bool) {
		accountPoints = append(accountPoints, &v2.TradingRewardCampaignAccountPoints{
			Account: points.Account.String(),
			Points:  points.Points,
		})

		return false
	})

	return accountPoints
}

// GetAllAccountCampaignTradingRewardPointsWithTotalPoints gets the trading reward points for all accounts
func (k TradingKeeper) GetAllAccountCampaignTradingRewardPointsWithTotalPoints(ctx sdk.Context) (accountPoints []*types.TradingRewardAccountPoints, totalPoints math.LegacyDec) { //nolint:revive // ok
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	accountPoints = make([]*types.TradingRewardAccountPoints, 0)
	totalPoints = math.LegacyZeroDec()

	appendPoints := func(points *types.TradingRewardAccountPoints) (stop bool) {
		accountPoints = append(accountPoints, points)
		totalPoints = totalPoints.Add(points.Points)
		return false
	}

	k.IterateAccountCampaignTradingRewardPoints(ctx, appendPoints)
	return accountPoints, totalPoints
}

// IncrementTotalTradingRewardPoints sets the total trading reward points
func (k TradingKeeper) IncrementTotalTradingRewardPoints(
	ctx sdk.Context,
	points math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	currPoints := k.GetTotalTradingRewardPoints(ctx)
	newPoints := currPoints.Add(points)
	k.SetTotalTradingRewardPoints(ctx, newPoints)
}

// PersistTradingRewardPoints persists the trading reward points
func (k TradingKeeper) PersistTradingRewardPoints(ctx sdk.Context, tradingRewards types.TradingRewardPoints) {
	totalTradingRewardPoints := math.LegacyZeroDec()

	for _, account := range tradingRewards.GetSortedAccountKeys() {
		addr, _ := sdk.AccAddressFromBech32(account)
		accountTradingRewardPoints := tradingRewards[account]

		isRegisteredDMM := k.GetIsOptedOutOfRewards(ctx, addr)
		if isRegisteredDMM {
			continue
		}

		k.UpdateAccountCampaignTradingRewardPoints(ctx, addr, accountTradingRewardPoints)
		totalTradingRewardPoints = totalTradingRewardPoints.Add(accountTradingRewardPoints)
	}
	k.IncrementTotalTradingRewardPoints(ctx, totalTradingRewardPoints)
}
