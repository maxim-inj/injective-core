package proposals

import (
	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/events"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k *ProposalKeeper) HandleTradingRewardPendingPointsUpdateProposal(
	ctx sdk.Context,
	p *v2.TradingRewardPendingPointsUpdateProposal,
) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	pendingPool := k.GetCampaignRewardPendingPool(ctx, p.PendingPoolTimestamp)

	if pendingPool == nil {
		return errors.Wrap(types.ErrInvalidTradingRewardsPendingPointsUpdate, "no pending reward pool with timestamp found") //nolint:revive // ok
	}

	currentTotalTradingRewardPoints := k.GetTotalTradingRewardPendingPoints(ctx, pendingPool.StartTimestamp)
	newTotalPoints := currentTotalTradingRewardPoints

	for _, rewardPointUpdates := range p.RewardPointUpdates {
		account, _ := sdk.AccAddressFromBech32(rewardPointUpdates.AccountAddress)
		currentPoints := k.GetCampaignTradingRewardPendingPoints(ctx, account, pendingPool.StartTimestamp)

		newPoints := rewardPointUpdates.NewPoints
		// prevent points from being increased, only decreased
		if newPoints.GTE(currentPoints) {
			continue
		}

		pointsDecrease := currentPoints.Sub(newPoints)
		newTotalPoints = newTotalPoints.Sub(pointsDecrease)
		k.SetAccountCampaignTradingRewardPendingPoints(ctx, account, pendingPool.StartTimestamp, newPoints)
	}

	k.SetTotalTradingRewardPendingPoints(ctx, newTotalPoints, pendingPool.StartTimestamp)

	return nil
}

func (k *ProposalKeeper) HandleTradingRewardCampaignLaunchProposal(ctx sdk.Context, p *v2.TradingRewardCampaignLaunchProposal) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	tradingRewardPoolCampaignSchedule := k.GetAllCampaignRewardPools(ctx)
	doesCampaignAlreadyExist := len(tradingRewardPoolCampaignSchedule) > 0
	if doesCampaignAlreadyExist {
		return errors.Wrap(types.ErrInvalidTradingRewardCampaign, "already existing trading reward campaign")
	}

	if p.CampaignRewardPools[0].StartTimestamp <= ctx.BlockTime().Unix() {
		return errors.Wrap(types.ErrInvalidTradingRewardCampaign, "campaign start timestamp has already passed")
	}

	for _, denom := range p.CampaignInfo.QuoteDenoms {
		if !k.IsDenomValid(ctx, denom) {
			return errors.Wrapf(types.ErrInvalidBaseDenom, "denom %s does not exist in supply", denom)
		}
	}

	if err := k.AddRewardPools(ctx, p.CampaignRewardPools, p.CampaignInfo.CampaignDurationSeconds, 0); err != nil {
		return err
	}

	k.SetCampaignInfo(ctx, p.CampaignInfo)
	k.SetTradingRewardsMarketQualificationForAllQualifyingMarkets(ctx, p.CampaignInfo)
	k.SetTradingRewardsMarketPointsMultipliersFromCampaign(ctx, p.CampaignInfo)

	events.Emit(ctx, k.BaseKeeper, &v2.EventTradingRewardCampaignUpdate{
		CampaignInfo:        p.CampaignInfo,
		CampaignRewardPools: k.GetAllCampaignRewardPools(ctx),
	})

	return nil
}

func (k *ProposalKeeper) HandleTradingRewardCampaignUpdateProposal(ctx sdk.Context, p *v2.TradingRewardCampaignUpdateProposal) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	tradingRewardPoolCampaignSchedule := k.GetAllCampaignRewardPools(ctx)
	doesCampaignAlreadyExist := len(tradingRewardPoolCampaignSchedule) > 0
	if !doesCampaignAlreadyExist {
		return errors.Wrap(types.ErrInvalidTradingRewardCampaign, "no existing trading reward campaign")
	}

	campaignInfo := k.GetCampaignInfo(ctx)
	if campaignInfo.CampaignDurationSeconds != p.CampaignInfo.CampaignDurationSeconds {
		return errors.Wrap(types.ErrInvalidTradingRewardCampaign, "campaign duration does not match existing campaign")
	}

	for _, denom := range p.CampaignInfo.QuoteDenoms {
		if !k.IsDenomValid(ctx, denom) {
			return errors.Wrapf(types.ErrInvalidBaseDenom, "denom %s does not exist in supply", denom)
		}
	}

	k.DeleteAllTradingRewardsMarketQualifications(ctx)
	k.DeleteAllTradingRewardsMarketPointsMultipliers(ctx)

	firstTradingRewardPoolStartTimestamp := tradingRewardPoolCampaignSchedule[0].StartTimestamp
	lastTradingRewardPoolStartTimestamp := tradingRewardPoolCampaignSchedule[len(tradingRewardPoolCampaignSchedule)-1].StartTimestamp

	if err := k.updateRewardPool(ctx, p.CampaignRewardPoolsUpdates, firstTradingRewardPoolStartTimestamp); err != nil {
		return err
	}
	if err := k.AddRewardPools(
		ctx, p.CampaignRewardPoolsAdditions, campaignInfo.CampaignDurationSeconds, lastTradingRewardPoolStartTimestamp,
	); err != nil {
		return err
	}

	k.SetCampaignInfo(ctx, p.CampaignInfo)
	k.SetTradingRewardsMarketQualificationForAllQualifyingMarkets(ctx, p.CampaignInfo)
	k.SetTradingRewardsMarketPointsMultipliersFromCampaign(ctx, p.CampaignInfo)

	events.Emit(ctx, k.BaseKeeper, &v2.EventTradingRewardCampaignUpdate{
		CampaignInfo:        p.CampaignInfo,
		CampaignRewardPools: k.GetAllCampaignRewardPools(ctx),
	})

	return nil
}

func (k *ProposalKeeper) updateRewardPool(
	ctx sdk.Context,
	poolsUpdates []*v2.CampaignRewardPool,
	firstTradingRewardPoolStartTimestamp int64,
) error {
	if len(poolsUpdates) == 0 {
		return nil
	}

	isUpdatingCurrentRewardPool := poolsUpdates[0].StartTimestamp == firstTradingRewardPoolStartTimestamp
	if isUpdatingCurrentRewardPool {
		return errors.Wrap(types.ErrInvalidTradingRewardCampaign, "cannot update reward pools for running campaign")
	}

	for _, campaignRewardPool := range poolsUpdates {
		existingCampaignRewardPool := k.GetCampaignRewardPool(ctx, campaignRewardPool.StartTimestamp)

		if existingCampaignRewardPool == nil {
			return errors.Wrap(types.ErrInvalidTradingRewardCampaign, "reward pool update not matching existing reward pool")
		}

		if campaignRewardPool.MaxCampaignRewards == nil {
			k.DeleteCampaignRewardPool(ctx, campaignRewardPool.StartTimestamp)
			return nil
		}

		k.SetCampaignRewardPool(ctx, campaignRewardPool)
	}

	return nil
}
