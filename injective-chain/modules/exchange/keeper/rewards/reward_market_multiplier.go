package rewards

import (
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// SetTradingRewardsMarketPointsMultipliersFromCampaign sets the market's points multiplier for the specified spot and derivative markets
func (k TradingKeeper) SetTradingRewardsMarketPointsMultipliersFromCampaign(ctx sdk.Context, campaignInfo *v2.TradingRewardCampaignInfo) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if campaignInfo.TradingRewardBoostInfo == nil {
		return
	}

	for idx, marketID := range campaignInfo.TradingRewardBoostInfo.BoostedSpotMarketIds {
		multiplier := campaignInfo.TradingRewardBoostInfo.SpotMarketMultipliers[idx]
		k.SetTradingRewardsMarketPointsMultiplier(ctx, common.HexToHash(marketID), &multiplier)
	}

	for idx, marketID := range campaignInfo.TradingRewardBoostInfo.BoostedDerivativeMarketIds {
		multiplier := campaignInfo.TradingRewardBoostInfo.DerivativeMarketMultipliers[idx]
		k.SetTradingRewardsMarketPointsMultiplier(ctx, common.HexToHash(marketID), &multiplier)
	}
}

// DeleteAllTradingRewardsMarketPointsMultipliers deletes the points multipliers for all markets
func (k TradingKeeper) DeleteAllTradingRewardsMarketPointsMultipliers(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	_, marketIDs := k.GetAllTradingRewardsMarketPointsMultiplier(ctx)
	for _, marketID := range marketIDs {
		k.DeleteTradingRewardsMarketPointsMultiplier(ctx, marketID)
	}
}

// GetAllTradingRewardsMarketPointsMultiplier gets all points multipliers for all markets
func (k TradingKeeper) GetAllTradingRewardsMarketPointsMultiplier(ctx sdk.Context) ([]*v2.PointsMultiplier, []common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	multipliers := make([]*v2.PointsMultiplier, 0)
	marketIDs := make([]common.Hash, 0)
	appendMultiplier := func(multiplier *v2.PointsMultiplier, marketID common.Hash) (stop bool) {
		marketIDs = append(marketIDs, marketID)
		multipliers = append(multipliers, multiplier)
		return false
	}

	k.IterateTradingRewardsMarketPointsMultipliers(ctx, appendMultiplier)

	return multipliers, marketIDs
}
