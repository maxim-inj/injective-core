package rewards

import (
	"cosmossdk.io/errors"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// GetAllCampaignRewardPools gets all campaign reward pools
func (k TradingKeeper) GetAllCampaignRewardPools(ctx sdk.Context) []*v2.CampaignRewardPool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	rewardPools := make([]*v2.CampaignRewardPool, 0)
	k.IterateCampaignRewardPools(ctx, false, func(pool *v2.CampaignRewardPool) (stop bool) {
		rewardPools = append(rewardPools, pool)
		return false
	})

	return rewardPools
}

// GetFirstCampaignRewardPool gets the first campaign reward pool.
func (k TradingKeeper) GetFirstCampaignRewardPool(ctx sdk.Context) (rewardPool *v2.CampaignRewardPool) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	appendPool := func(pool *v2.CampaignRewardPool) (stop bool) {
		rewardPool = pool
		return true
	}

	k.IterateCampaignRewardPools(ctx, false, appendPool)
	return rewardPool
}

func (k TradingKeeper) AddRewardPools(
	ctx sdk.Context,
	poolsAdditions []*v2.CampaignRewardPool,
	campaignDurationSeconds int64,
	lastTradingRewardPoolStartTimestamp int64,
) error {
	for _, campaignRewardPool := range poolsAdditions {
		hasMatchingStartTimestamp := lastTradingRewardPoolStartTimestamp == 0 ||
			campaignRewardPool.StartTimestamp == lastTradingRewardPoolStartTimestamp+campaignDurationSeconds

		if !hasMatchingStartTimestamp {
			return errors.Wrap(types.ErrInvalidTradingRewardCampaign, "reward pool addition start timestamp not matching campaign duration")
		}

		k.SetCampaignRewardPool(ctx, campaignRewardPool)
		lastTradingRewardPoolStartTimestamp = campaignRewardPool.StartTimestamp
	}

	return nil
}
