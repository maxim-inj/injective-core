package feediscounts

import (
	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/base"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/events"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

//nolint:revive // ok
type FeeDiscountsKeeper struct {
	*base.BaseKeeper

	staking types.StakingKeeper

	svcTags metrics.Tags
}

func New(
	b *base.BaseKeeper,
	s types.StakingKeeper,
) *FeeDiscountsKeeper {
	return &FeeDiscountsKeeper{
		BaseKeeper: b,
		staking:    s,
		svcTags:    metrics.Tags{"svc": "fee_discounts_k"},
	}
}

func (k FeeDiscountsKeeper) PersistFeeDiscountStakingInfoUpdates(ctx sdk.Context, stakingInfo *v2.FeeDiscountStakingInfo) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if stakingInfo == nil {
		return
	}

	accountTierTTLs := stakingInfo.GetSortedNewFeeDiscountAccountTiers()
	for _, accountTier := range accountTierTTLs {
		k.SetFeeDiscountAccountTierInfo(ctx, accountTier.Account, accountTier.TierTTL)
	}

	accountContributions := stakingInfo.GetSortedAccountVolumeContributions()
	bucketStartTimestamp := stakingInfo.CurrBucketStartTimestamp
	for _, accountContribution := range accountContributions {
		k.UpdateFeeDiscountAccountVolumeInBucket(ctx, accountContribution.Account, bucketStartTimestamp, accountContribution.Amount)
	}

	subaccountVolumeContributions, marketVolumeContributions := stakingInfo.GetSortedSubaccountAndMarketVolumes()

	for idx := range subaccountVolumeContributions {
		contribution := subaccountVolumeContributions[idx]
		k.IncrementSubaccountMarketAggregateVolume(ctx, contribution.SubaccountID, contribution.MarketID, contribution.Volume)
	}

	for idx := range marketVolumeContributions {
		contribution := marketVolumeContributions[idx]
		k.IncrementMarketAggregateVolume(ctx, contribution.MarketID, contribution.Volume)
	}

	granters, invalidGrants := stakingInfo.GetSortedGrantCheckpointGrantersAndInvalidGrants()
	currTime := ctx.BlockTime().Unix()

	for _, granter := range granters {
		k.SetLastValidGrantDelegationCheckTime(ctx, granter, currTime)
	}

	for _, invalidGrant := range invalidGrants {
		events.Emit(ctx, k.BaseKeeper, invalidGrant)
	}
}

// UpdateFeeDiscountAccountVolumeInBucket increments the existing volume.
func (k FeeDiscountsKeeper) UpdateFeeDiscountAccountVolumeInBucket(
	ctx sdk.Context,
	account sdk.AccAddress,
	bucketStartTimestamp int64,
	addedPoints math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if addedPoints.IsZero() {
		return
	}

	accountPoints := k.GetFeeDiscountAccountVolumeInBucket(ctx, bucketStartTimestamp, account)
	accountPoints = accountPoints.Add(addedPoints)
	k.SetFeeDiscountAccountVolumeInBucket(ctx, bucketStartTimestamp, account, accountPoints)
}

// IncrementSubaccountMarketAggregateVolume increments the aggregate volume.
func (k FeeDiscountsKeeper) IncrementSubaccountMarketAggregateVolume(
	ctx sdk.Context,
	subaccountID,
	marketID common.Hash,
	volume v2.VolumeRecord,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if volume.IsZero() {
		return
	}

	oldVolume := k.GetSubaccountMarketAggregateVolume(ctx, subaccountID, marketID)
	newVolume := oldVolume.Add(volume)
	k.SetSubaccountMarketAggregateVolume(ctx, subaccountID, marketID, newVolume)
}

// IncrementMarketAggregateVolume increments the aggregate volume.
func (k FeeDiscountsKeeper) IncrementMarketAggregateVolume(
	ctx sdk.Context,
	marketID common.Hash,
	volume v2.VolumeRecord,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if volume.IsZero() {
		return
	}

	oldVolume := k.GetMarketAggregateVolume(ctx, marketID)
	newVolume := oldVolume.Add(volume)
	k.SetMarketAggregateVolume(ctx, marketID, newVolume)
}

//nolint:revive //ok
func (k FeeDiscountsKeeper) FetchAndUpdateDiscountedTradingFeeRate(
	ctx sdk.Context,
	tradingFeeRate math.LegacyDec,
	isMakerFee bool,
	account sdk.AccAddress,
	config *v2.FeeDiscountConfig,
) math.LegacyDec {
	// fee discounts not supported for negative fees
	if tradingFeeRate.IsNegative() {
		return tradingFeeRate
	}

	feeDiscountRate := config.GetFeeDiscountRate(account, isMakerFee)

	if feeDiscountRate == nil {
		if config.Schedule == nil {
			return tradingFeeRate
		}
		feeDiscountRates, tierLevel, isTTLExpired, effectiveGrant := k.GetAccountFeeDiscountRates(ctx, account, config)
		config.SetAccountTierInfo(account, feeDiscountRates)

		if isTTLExpired {
			config.SetNewAccountTierTTL(account, tierLevel)

			if effectiveGrant != nil {
				// only update the last valid grant delegation check time if the grant is valid
				if effectiveGrant.IsValid {
					config.AddCheckpoint(effectiveGrant.Granter)
				} else {
					config.AddInvalidGrant(account.String(), effectiveGrant.Granter)
				}
			}
		}

		if isMakerFee {
			feeDiscountRate = &feeDiscountRates.MakerDiscountRate
		} else {
			feeDiscountRate = &feeDiscountRates.TakerDiscountRate
		}
	}

	return math.LegacyOneDec().Sub(*feeDiscountRate).Mul(tradingFeeRate)
}

//nolint:revive // ok
func (k FeeDiscountsKeeper) GetAccountFeeDiscountRates(
	ctx sdk.Context,
	account sdk.AccAddress,
	config *v2.FeeDiscountConfig,
) (
	feeDiscountRates *types.FeeDiscountRates,
	tierLevel uint64,
	isTTLExpired bool,
	effectiveGrant *v2.EffectiveGrant,
) {
	tierTTL := k.GetFeeDiscountAccountTierInfo(ctx, account)
	isTTLExpired = tierTTL == nil || tierTTL.TtlTimestamp < config.MaxTTLTimestamp

	if !isTTLExpired {
		feeDiscountRates = config.FeeDiscountRatesCache[tierTTL.Tier]
		return feeDiscountRates, tierTTL.Tier, isTTLExpired, k.getEffectiveGrant(ctx, account)
	}

	_, tierOneVolume := config.Schedule.TierOneRequirements()

	highestTierVolumeAmount := config.Schedule.TierInfos[len(config.Schedule.TierInfos)-1].Volume
	tradingVolume := highestTierVolumeAmount

	// only check volume if one full cycle of volume tracking has passed
	isPastVolumeCheckRequired := config.GetIsPastTradingFeesCheckRequired()
	if isPastVolumeCheckRequired {
		tradingVolume = k.GetFeeDiscountTotalAccountVolume(ctx, account, config.CurrBucketStartTimestamp)
	}

	hasTierZeroTradingVolume := tradingVolume.LT(tierOneVolume)
	effectiveStakedAmount := math.ZeroInt()

	effectiveGrant = k.GetValidatedEffectiveGrant(ctx, account)

	// no need to calculate staked amount if volume is less than tier one volume
	if !hasTierZeroTradingVolume {
		personalStake := k.CalculateStakedAmountWithCache(ctx, account, config)
		effectiveStakedAmount = personalStake.Add(effectiveGrant.NetGrantedStake)
	}

	feeDiscountRates, tierLevel = config.Schedule.CalculateFeeDiscountTier(effectiveStakedAmount, tradingVolume)
	return feeDiscountRates, tierLevel, isTTLExpired, effectiveGrant
}

func (k FeeDiscountsKeeper) getEffectiveGrant(ctx sdk.Context, grantee sdk.AccAddress) *v2.EffectiveGrant {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()
	stakeGrantedToOthers := k.GetTotalGrantAmount(ctx, grantee)
	activeGrant := k.GetActiveGrant(ctx, grantee)

	if activeGrant == nil {
		return v2.NewEffectiveGrant("", stakeGrantedToOthers.Neg(), true)
	}

	netGrantedStake := activeGrant.Amount.Sub(stakeGrantedToOthers)
	return v2.NewEffectiveGrant(activeGrant.Granter, netGrantedStake, true)
}

// GetFeeDiscountTotalAccountVolume fetches the volume for a given account for all the buckets
func (k FeeDiscountsKeeper) GetFeeDiscountTotalAccountVolume(
	ctx sdk.Context,
	account sdk.AccAddress,
	currBucketStartTimestamp int64,
) math.LegacyDec {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	currBucketVolume := k.GetFeeDiscountAccountVolumeInBucket(ctx, currBucketStartTimestamp, account)
	pastBucketVolume := k.GetPastBucketTotalVolume(ctx, account)
	totalVolume := currBucketVolume.Add(pastBucketVolume)

	return totalVolume
}

func (k FeeDiscountsKeeper) GetValidatedEffectiveGrant(ctx sdk.Context, grantee sdk.AccAddress) *v2.EffectiveGrant {
	effectiveGrant := k.getEffectiveGrant(ctx, grantee)

	if effectiveGrant.Granter == "" {
		return effectiveGrant
	}

	granter := sdk.MustAccAddressFromBech32(effectiveGrant.Granter)

	lastDelegationsCheckTime := k.GetLastValidGrantDelegationCheckTime(ctx, granter)

	// use the fee discount bucket duration as our TTL for checking granter delegations
	isDelegationCheckExpired := ctx.BlockTime().Unix() > lastDelegationsCheckTime+k.GetFeeDiscountBucketDuration(ctx)

	if !isDelegationCheckExpired {
		return effectiveGrant
	}

	granterStake := k.CalculateStakedAmountWithoutCache(ctx, granter, types.MaxGranterDelegations)
	totalGrantAmount := k.GetTotalGrantAmount(ctx, granter)

	// invalidate the grant if the granter's real stake is less than the total grant amount
	if totalGrantAmount.GT(granterStake) {
		stakeGrantedToOthers := k.GetTotalGrantAmount(ctx, grantee)
		return v2.NewEffectiveGrant(effectiveGrant.Granter, stakeGrantedToOthers.Neg(), false)
	}

	return effectiveGrant
}

func (k FeeDiscountsKeeper) CalculateStakedAmountWithoutCache(
	ctx sdk.Context,
	staker sdk.AccAddress,
	maxDelegations uint16,
) math.Int {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	delegations, _ := k.staking.GetDelegatorDelegations(ctx, staker, maxDelegations)
	totalStaked := math.ZeroInt()

	for _, delegation := range delegations {
		validatorAddr, err := sdk.ValAddressFromBech32(delegation.GetValidatorAddr())
		if err != nil {
			continue
		}

		validator, err := k.staking.Validator(ctx, validatorAddr)
		if validator == nil || err != nil {
			// extra precaution, should never happen
			continue
		}

		stakedWithValidator := validator.TokensFromShares(delegation.Shares).TruncateInt()
		totalStaked = totalStaked.Add(stakedWithValidator)
	}

	return totalStaked
}

func (k FeeDiscountsKeeper) CalculateStakedAmountWithCache(
	ctx sdk.Context,
	trader sdk.AccAddress,
	feeDiscountConfig *v2.FeeDiscountConfig,
) math.Int {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	maxDelegations := uint16(10)
	delegations, _ := k.staking.GetDelegatorDelegations(ctx, trader, maxDelegations)

	totalStaked := math.ZeroInt()
	for _, delegation := range delegations {
		validatorAddr := delegation.GetValidatorAddr()

		feeDiscountConfig.ValidatorsMux.RLock()
		cachedValidator, ok := feeDiscountConfig.Validators[validatorAddr]
		feeDiscountConfig.ValidatorsMux.RUnlock()

		if !ok {
			cachedValidator = k.fetchValidatorAndUpdateCache(ctx, validatorAddr, feeDiscountConfig)
		}

		if cachedValidator == nil {
			// extra precaution, should never happen
			continue
		}

		stakedWithValidator := cachedValidator.TokensFromShares(delegation.Shares).TruncateInt()
		totalStaked = totalStaked.Add(stakedWithValidator)
	}

	return totalStaked
}

func (k FeeDiscountsKeeper) fetchValidatorAndUpdateCache(
	ctx sdk.Context,
	validatorAddr string,
	feeDiscountConfig *v2.FeeDiscountConfig,
) stakingtypes.ValidatorI {
	validatorAddress, _ := sdk.ValAddressFromBech32(validatorAddr)

	validator, err := k.staking.Validator(ctx, validatorAddress)
	if validator == nil || err != nil {
		return nil
	}

	feeDiscountConfig.ValidatorsMux.Lock()
	feeDiscountConfig.Validators[validatorAddr] = validator
	feeDiscountConfig.ValidatorsMux.Unlock()

	return validator
}

func (k FeeDiscountsKeeper) GetActiveGrantAmount(ctx sdk.Context, grantee sdk.AccAddress) math.Int {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()
	grant := k.GetActiveGrant(ctx, grantee)
	if grant == nil {
		return math.ZeroInt()
	}
	return grant.Amount
}

func (k FeeDiscountsKeeper) InitialFetchAndUpdateActiveAccountFeeDiscountStakingInfo(ctx sdk.Context) *v2.FeeDiscountStakingInfo {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	accounts := k.GetAllAccountsActivelyTradingQualifiedMarketsInBlockForFeeDiscounts(ctx)
	schedule := k.GetFeeDiscountSchedule(ctx)

	currBucketStartTimestamp := k.GetFeeDiscountCurrentBucketStartTimestamp(ctx)
	oldestBucketStartTimestamp := k.GetOldestBucketStartTimestamp(ctx)
	isFirstFeeCycleFinished := k.GetIsFirstFeeCycleFinished(ctx)
	maxTTLTimestamp := currBucketStartTimestamp
	nextTTLTimestamp := maxTTLTimestamp + k.GetFeeDiscountBucketDuration(ctx)

	stakingInfo := v2.NewFeeDiscountStakingInfo(
		schedule,
		currBucketStartTimestamp,
		oldestBucketStartTimestamp,
		maxTTLTimestamp,
		nextTTLTimestamp,
		isFirstFeeCycleFinished,
	)

	config := v2.NewFeeDiscountConfig(true, stakingInfo)

	for _, account := range accounts {
		k.setAccountFeeDiscountTier(
			ctx,
			account,
			config,
		)
	}

	return stakingInfo
}

// GetOldestBucketStartTimestamp gets the oldest bucket start timestamp.
func (k FeeDiscountsKeeper) GetOldestBucketStartTimestamp(ctx sdk.Context) (startTimestamp int64) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	appendVolumes := func(bucketStartTimestamp int64, _ sdk.AccAddress, _ math.LegacyDec) (stop bool) {
		startTimestamp = bucketStartTimestamp
		return true
	}

	k.IterateAccountVolume(ctx, appendVolumes)

	return startTimestamp
}

func (k FeeDiscountsKeeper) setAccountFeeDiscountTier(
	ctx sdk.Context,
	account sdk.AccAddress,
	config *v2.FeeDiscountConfig,
) {
	feeDiscountRates, tierLevel, isTTLExpired, effectiveGrant := k.GetAccountFeeDiscountRates(ctx, account, config)
	config.SetAccountTierInfo(account, feeDiscountRates)

	if isTTLExpired {
		k.SetFeeDiscountAccountTierInfo(ctx, account, v2.NewFeeDiscountTierTTL(tierLevel, config.NextTTLTimestamp))

		if effectiveGrant != nil {
			// only update the last valid grant delegation check time if the grant is valid
			if effectiveGrant.IsValid {
				k.SetLastValidGrantDelegationCheckTime(ctx, effectiveGrant.Granter, ctx.BlockTime().Unix())
			} else {
				events.Emit(ctx, k.BaseKeeper, &v2.EventInvalidGrant{
					Grantee: account.String(),
					Granter: effectiveGrant.Granter,
				})
			}
		}
	}
}

// IncrementPastBucketTotalVolume increments the total volume in past buckets for the given account
func (k FeeDiscountsKeeper) IncrementPastBucketTotalVolume(
	ctx sdk.Context,
	account sdk.AccAddress,
	addedBucketTotalFeesAmount math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	currVolume := k.GetPastBucketTotalVolume(ctx, account)
	newVolume := currVolume.Add(addedBucketTotalFeesAmount)

	k.SetPastBucketTotalVolume(ctx, account, newVolume)
}

// DecrementPastBucketTotalVolume decrements the total volume in past buckets for the given account
func (k FeeDiscountsKeeper) DecrementPastBucketTotalVolume(
	ctx sdk.Context,
	account sdk.AccAddress,
	removedBucketTotalFeesAmount math.LegacyDec,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	currVolume := k.GetPastBucketTotalVolume(ctx, account)
	newVolume := currVolume.Sub(removedBucketTotalFeesAmount)

	k.SetPastBucketTotalVolume(ctx, account, newVolume)
}

// DeleteAllPastBucketTotalVolume deletes the total volume in past buckets for all accounts
func (k FeeDiscountsKeeper) DeleteAllPastBucketTotalVolume(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	accountVolumes := k.GetAllPastBucketTotalVolume(ctx)
	for _, a := range accountVolumes {
		account, _ := sdk.AccAddressFromBech32(a.Account)
		k.DeletePastBucketTotalVolume(ctx, account)
	}
}

// GetAllPastBucketTotalVolume gets all total volume in past buckets for all accounts
func (k FeeDiscountsKeeper) GetAllPastBucketTotalVolume(ctx sdk.Context) []*v2.AccountVolume {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	accountVolumes := make([]*v2.AccountVolume, 0)

	appendFees := func(account sdk.AccAddress, volume math.LegacyDec) (stop bool) {
		accountVolumes = append(accountVolumes, &v2.AccountVolume{
			Account: account.String(),
			Volume:  volume,
		})
		return false
	}

	k.IteratePastBucketTotalVolume(ctx, appendFees)
	return accountVolumes
}

// DeleteAllFeeDiscountMarketQualifications deletes the fee discount qualifications for all markets
func (k FeeDiscountsKeeper) DeleteAllFeeDiscountMarketQualifications(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketIDs, _ := k.GetAllFeeDiscountMarketQualification(ctx)
	for _, marketID := range marketIDs {
		k.DeleteFeeDiscountMarketQualification(ctx, marketID)
	}
}

// GetAllFeeDiscountMarketQualification gets all market fee discount qualification statuses
func (k FeeDiscountsKeeper) GetAllFeeDiscountMarketQualification(ctx sdk.Context) ([]common.Hash, []bool) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketIDs := make([]common.Hash, 0)
	isQualified := make([]bool, 0)

	appendQualification := func(m common.Hash, q bool) (stop bool) {
		marketIDs = append(marketIDs, m)
		isQualified = append(isQualified, q)
		return false
	}

	k.IterateFeeDiscountMarketQualifications(ctx, appendQualification)
	return marketIDs, isQualified
}

func (k FeeDiscountsKeeper) CheckQuoteAndSetFeeDiscountQualification(
	ctx sdk.Context,
	marketID common.Hash,
	quoteDenom string,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if schedule := k.GetFeeDiscountSchedule(ctx); schedule != nil {
		disqualified := false
		for _, disqualifiedMarketID := range schedule.DisqualifiedMarketIds {
			if marketID == common.HexToHash(disqualifiedMarketID) {
				disqualified = true
			}
		}

		if disqualified {
			k.SetFeeDiscountMarketQualification(ctx, marketID, false)
			return
		}

		for _, q := range schedule.QuoteDenoms {
			if quoteDenom == q {
				k.SetFeeDiscountMarketQualification(ctx, marketID, true)
				break
			}
		}
	}
}

// AdvanceFeeDiscountCurrentBucketStartTimestamp increments the start timestamp for the fee discount bucket.
func (k FeeDiscountsKeeper) AdvanceFeeDiscountCurrentBucketStartTimestamp(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	currentStartTimestamp := k.GetFeeDiscountCurrentBucketStartTimestamp(ctx)
	bucketDuration := k.GetFeeDiscountBucketDuration(ctx)
	newStartTimestamp := currentStartTimestamp + bucketDuration
	k.SetFeeDiscountCurrentBucketStartTimestamp(ctx, newStartTimestamp)
}

// GetAllSubaccountMarketAggregateVolumes gets all of the aggregate subaccount market volumes
func (k FeeDiscountsKeeper) GetAllSubaccountMarketAggregateVolumes(ctx sdk.Context) []*v2.AggregateSubaccountVolumeRecord {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	volumes := make([]*v2.AggregateSubaccountVolumeRecord, 0)

	// subaccountID -> MarketVolume
	volumeTracker := make(map[common.Hash][]*v2.MarketVolume)
	subaccountIDs := make([]common.Hash, 0)

	appendVolumes := func(subaccountID, marketID common.Hash, totalVolume v2.VolumeRecord) (stop bool) {
		record := &v2.MarketVolume{
			MarketId: marketID.Hex(),
			Volume:   totalVolume,
		}

		records, ok := volumeTracker[subaccountID]
		if !ok {
			volumeTracker[subaccountID] = []*v2.MarketVolume{record}
			subaccountIDs = append(subaccountIDs, subaccountID)
		} else {
			volumeTracker[subaccountID] = append(records, record)
		}
		return false
	}

	k.IterateSubaccountMarketAggregateVolumes(ctx, appendVolumes)

	for _, subaccountID := range subaccountIDs {
		volumes = append(volumes, &v2.AggregateSubaccountVolumeRecord{
			SubaccountId:  subaccountID.Hex(),
			MarketVolumes: volumeTracker[subaccountID],
		})
	}

	return volumes
}

// GetAllComputedMarketAggregateVolumes gets all of the aggregate subaccount market volumes
func (k FeeDiscountsKeeper) GetAllComputedMarketAggregateVolumes(ctx sdk.Context) map[common.Hash]v2.VolumeRecord {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketVolumes := make(map[common.Hash]v2.VolumeRecord)

	addVolume := func(_, marketID common.Hash, volumeRecord v2.VolumeRecord) (stop bool) {
		if volumeRecord.IsZero() {
			return false
		}
		marketVolume, ok := marketVolumes[marketID]
		if !ok {
			marketVolumes[marketID] = volumeRecord
			return false
		}

		marketVolumes[marketID] = marketVolume.Add(volumeRecord)
		return false
	}

	k.IterateSubaccountMarketAggregateVolumes(ctx, addVolume)
	return marketVolumes
}

// GetAllSubaccountMarketAggregateVolumesBySubaccount gets all the aggregate volumes for the subaccountID for all markets
func (k FeeDiscountsKeeper) GetAllSubaccountMarketAggregateVolumesBySubaccount(
	ctx sdk.Context,
	subaccountID common.Hash,
) []*v2.MarketVolume {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	volumes := make([]*v2.MarketVolume, 0)
	k.IterateSubaccountMarketAggregateVolumesBySubaccount(
		ctx,
		subaccountID,
		func(marketID common.Hash, totalVolume v2.VolumeRecord) (stop bool) {
			volumes = append(volumes, &v2.MarketVolume{
				MarketId: marketID.Hex(),
				Volume:   totalVolume,
			},
			)

			return false
		})

	return volumes
}

// GetAllSubaccountMarketAggregateVolumesByAccAddress gets all the aggregate volumes for all associated subaccounts for
// the accAddress in each market. The volume reported for a given marketID reflects the sum of all the volumes over all the
// subaccounts associated with the accAddress in the market.
func (k FeeDiscountsKeeper) GetAllSubaccountMarketAggregateVolumesByAccAddress(
	ctx sdk.Context,
	accAddress sdk.AccAddress,
) []*v2.MarketVolume {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// marketID => volume
	totalVolumes := make(map[common.Hash]v2.VolumeRecord)
	marketIDs := make([]common.Hash, 0)

	updateVolume := func(_, marketID common.Hash, volume v2.VolumeRecord) (stop bool) {
		if oldVolume, found := totalVolumes[marketID]; !found {
			totalVolumes[marketID] = volume
			marketIDs = append(marketIDs, marketID)
		} else {
			totalVolumes[marketID] = oldVolume.Add(volume)
		}
		return false
	}

	k.IterateSubaccountMarketAggregateVolumesByAccAddress(ctx, accAddress, updateVolume)

	volumes := make([]*v2.MarketVolume, 0, len(marketIDs))
	for _, marketID := range marketIDs {
		volumes = append(volumes, &v2.MarketVolume{
			MarketId: marketID.Hex(),
			Volume:   totalVolumes[marketID],
		})
	}

	return volumes
}

// DeleteAllAccountVolumeInAllBucketsWithMetadata deletes all total volume in all buckets for all accounts
func (k FeeDiscountsKeeper) DeleteAllAccountVolumeInAllBucketsWithMetadata(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	allVolumes := k.GetAllAccountVolumeInAllBuckets(ctx)

	accounts := make([]sdk.AccAddress, 0)
	accountsMap := make(map[string]struct{})

	for _, bucketVolumes := range allVolumes {
		bucketStartTimestamp := bucketVolumes.BucketStartTimestamp
		for _, accountVolumes := range bucketVolumes.AccountVolume {
			accountStr := accountVolumes.Account
			account, _ := sdk.AccAddressFromBech32(accountStr)
			k.DeleteFeeDiscountAccountVolumeInBucket(ctx, bucketStartTimestamp, account)

			if _, ok := accountsMap[accountStr]; !ok {
				accountsMap[accountStr] = struct{}{}
				accounts = append(accounts, account)
			}
		}
	}

	// Delete the other metadata/trackers for consistency as well
	k.DeleteFeeDiscountCurrentBucketStartTimestamp(ctx)
	for _, account := range accounts {
		k.DeletePastBucketTotalVolume(ctx, account)
	}
	k.DeleteAllFeeDiscountAccountTierInfo(ctx)
	k.DeleteAllPastBucketTotalVolume(ctx)
}

// GetAllAccountVolumeInAllBuckets gets all total volume in all buckets for all accounts
func (k FeeDiscountsKeeper) GetAllAccountVolumeInAllBuckets(ctx sdk.Context) []*v2.FeeDiscountBucketVolumeAccounts {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	accountVolumeInAllBuckets := make([]*v2.FeeDiscountBucketVolumeAccounts, 0)
	accountVolumeMap := make(map[int64][]*v2.AccountVolume)
	timestamps := make([]int64, 0)

	appendVolume := func(
		bucketStartTimestamp int64,
		account sdk.AccAddress,
		volume math.LegacyDec,
	) (stop bool) {
		accountVolume := &v2.AccountVolume{
			Account: account.String(),
			Volume:  volume,
		}

		if v, ok := accountVolumeMap[bucketStartTimestamp]; !ok {
			accountVolumeMap[bucketStartTimestamp] = make([]*v2.AccountVolume, 0)
			timestamps = append(timestamps, bucketStartTimestamp)
			accountVolumeMap[bucketStartTimestamp] = append(accountVolumeMap[bucketStartTimestamp], accountVolume)
		} else {
			accountVolumeMap[bucketStartTimestamp] = append(v, accountVolume)
		}

		return false
	}

	k.IterateAccountVolume(ctx, appendVolume)

	for _, timestamp := range timestamps {
		accountVolumeInAllBuckets = append(accountVolumeInAllBuckets, &v2.FeeDiscountBucketVolumeAccounts{
			BucketStartTimestamp: timestamp,
			AccountVolume:        accountVolumeMap[timestamp],
		})
	}

	return accountVolumeInAllBuckets
}

// DeleteAllFeeDiscountAccountTierInfo deletes all accounts' fee discount Tier and TTL info.
func (k FeeDiscountsKeeper) DeleteAllFeeDiscountAccountTierInfo(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	allAccountTiers := k.GetAllFeeDiscountAccountTierInfo(ctx)
	for _, accountTier := range allAccountTiers {
		account, _ := sdk.AccAddressFromBech32(accountTier.Account)
		k.DeleteFeeDiscountAccountTierInfo(ctx, account)
	}
}

// GetAllFeeDiscountAccountTierInfo gets all accounts' fee discount Tier and TTL info
func (k FeeDiscountsKeeper) GetAllFeeDiscountAccountTierInfo(ctx sdk.Context) []*v2.FeeDiscountAccountTierTTL {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	accountTierTTL := make([]*v2.FeeDiscountAccountTierTTL, 0)
	k.IterateFeeDiscountAccountTierInfo(ctx, func(account sdk.AccAddress, tierInfo *v2.FeeDiscountTierTTL) (stop bool) {
		accountTierTTL = append(accountTierTTL, &v2.FeeDiscountAccountTierTTL{
			Account: account.String(),
			TierTtl: tierInfo,
		})
		return false
	})

	return accountTierTTL
}

// GetAllAccountVolumeInBucket gets all total volume in a given bucket for all accounts
func (k FeeDiscountsKeeper) GetAllAccountVolumeInBucket(ctx sdk.Context, bucketStartTimestamp int64) []*v2.AccountVolume {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	accountVolumes := make([]*v2.AccountVolume, 0)

	appendFees := func(account sdk.AccAddress, totalVolume math.LegacyDec) (stop bool) {
		accountVolumes = append(accountVolumes, &v2.AccountVolume{
			Account: account.String(),
			Volume:  totalVolume,
		})
		return false
	}

	k.IterateAccountVolumeInBucket(ctx, bucketStartTimestamp, appendFees)
	return accountVolumes
}

func (k FeeDiscountsKeeper) ProcessFeeDiscountBuckets(ctx sdk.Context) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	currBucketStartTimestamp := k.GetFeeDiscountCurrentBucketStartTimestamp(ctx)
	if currBucketStartTimestamp == 0 {
		return
	}

	blockTime := ctx.BlockTime().Unix()
	bucketDuration := k.GetFeeDiscountBucketDuration(ctx)

	nextBucketStartTime := currBucketStartTimestamp + bucketDuration

	hasReachedNextBucket := blockTime >= nextBucketStartTime
	if !hasReachedNextBucket {
		return
	}

	k.AdvanceFeeDiscountCurrentBucketStartTimestamp(ctx)

	oldestBucketStartTimestamp := k.GetOldestBucketStartTimestamp(ctx)
	bucketCount := k.GetFeeDiscountBucketCount(ctx)
	shouldPruneLastBucket := oldestBucketStartTimestamp != 0 && oldestBucketStartTimestamp < blockTime-int64(bucketCount)*bucketDuration

	allAccountVolumeInCurrentBucket := k.GetAllAccountVolumeInBucket(ctx, currBucketStartTimestamp)
	for i := range allAccountVolumeInCurrentBucket {
		account, _ := sdk.AccAddressFromBech32(allAccountVolumeInCurrentBucket[i].Account)
		amountFromCurrentBucket := allAccountVolumeInCurrentBucket[i].Volume
		k.IncrementPastBucketTotalVolume(ctx, account, amountFromCurrentBucket)
	}

	if !shouldPruneLastBucket {
		return
	}

	isFirstFeeCycleFinishedAlreadySet := k.GetIsFirstFeeCycleFinished(ctx)
	if !isFirstFeeCycleFinishedAlreadySet {
		k.SetIsFirstFeeCycleFinished(ctx, true)
	}

	allAccountVolumeInOldestBucket := k.GetAllAccountVolumeInBucket(ctx, oldestBucketStartTimestamp)
	for i := range allAccountVolumeInOldestBucket {
		account, _ := sdk.AccAddressFromBech32(allAccountVolumeInOldestBucket[i].Account)
		k.DeleteFeeDiscountAccountTierInfo(ctx, account)
		k.DeleteFeeDiscountAccountVolumeInBucket(ctx, oldestBucketStartTimestamp, account)

		removedBucketTotalFeesAmount := allAccountVolumeInOldestBucket[i].Volume
		k.DecrementPastBucketTotalVolume(ctx, account, removedBucketTotalFeesAmount)
	}
}

func (k FeeDiscountsKeeper) SetFeeDiscountMarketQualificationForAllQualifyingMarkets(ctx sdk.Context, schedule *v2.FeeDiscountSchedule) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketIDQuoteDenoms := k.GetAllMarketIDsWithQuoteDenoms(ctx)

	quoteDenomMap := make(map[string]struct{})
	for _, quoteDenom := range schedule.QuoteDenoms {
		quoteDenomMap[quoteDenom] = struct{}{}
	}

	for _, m := range marketIDQuoteDenoms {
		if _, ok := quoteDenomMap[m.QuoteDenom]; ok {
			k.SetFeeDiscountMarketQualification(ctx, m.MarketID, true)
		}
	}

	for _, marketID := range schedule.DisqualifiedMarketIds {
		k.SetFeeDiscountMarketQualification(ctx, common.HexToHash(marketID), false)
	}
}

func (k FeeDiscountsKeeper) GetFeeDiscountConfigForMarket(
	ctx sdk.Context,
	marketID common.Hash,
	stakingInfo *v2.FeeDiscountStakingInfo,
) *v2.FeeDiscountConfig {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	isQualifiedForFeeDiscounts := k.IsMarketQualifiedForFeeDiscount(ctx, marketID)
	return v2.NewFeeDiscountConfig(isQualifiedForFeeDiscounts, stakingInfo)
}

func (k FeeDiscountsKeeper) GetFeeDiscountConfigAndStakingInfoForMarket(
	ctx sdk.Context,
	marketID common.Hash,
) (*v2.FeeDiscountStakingInfo, *v2.FeeDiscountConfig) {
	var stakingInfo *v2.FeeDiscountStakingInfo

	schedule := k.GetFeeDiscountSchedule(ctx)
	currBucketStartTimestamp := k.GetFeeDiscountCurrentBucketStartTimestamp(ctx)
	oldestBucketStartTimestamp := k.GetOldestBucketStartTimestamp(ctx)
	isFirstFeeCycleFinished := k.GetIsFirstFeeCycleFinished(ctx)
	maxTTLTimestamp := currBucketStartTimestamp
	nextTTLTimestamp := maxTTLTimestamp + k.GetFeeDiscountBucketDuration(ctx)

	stakingInfo = v2.NewFeeDiscountStakingInfo(
		schedule,
		currBucketStartTimestamp,
		oldestBucketStartTimestamp,
		maxTTLTimestamp,
		nextTTLTimestamp,
		isFirstFeeCycleFinished,
	)

	feeDiscountConfig := k.GetFeeDiscountConfigForMarket(ctx, marketID, stakingInfo)
	return stakingInfo, feeDiscountConfig
}

func (k FeeDiscountsKeeper) SaveFeeDiscountSchedule(ctx sdk.Context, schedule *v2.FeeDiscountSchedule) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	k.SetFeeDiscountSchedule(ctx, schedule)
	k.SetFeeDiscountBucketCount(ctx, schedule.BucketCount)
	k.SetFeeDiscountBucketDuration(ctx, schedule.BucketDuration)

	events.Emit(ctx, k.BaseKeeper, &v2.EventFeeDiscountSchedule{Schedule: schedule})
}

func (k FeeDiscountsKeeper) AuthorizeStakeGrant(
	ctx sdk.Context,
	granter sdk.AccAddress,
	grantee sdk.AccAddress,
	amount math.Int,
) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	existingGrantAmount := k.GetGrantAuthorization(ctx, granter, grantee)
	existingTotalGrantAmount := k.GetTotalGrantAmount(ctx, granter)

	// update total grant amount accordingly
	totalGrantAmount := existingTotalGrantAmount.Sub(existingGrantAmount).Add(amount)

	k.SetTotalGrantAmount(ctx, granter, totalGrantAmount)
	k.SetGrantAuthorization(ctx, granter, grantee, amount)

	activeGrant := k.GetActiveGrant(ctx, grantee)

	// TODO: consider not activating the grant authorization if no active grant for the grantee exists, as the grantee
	// may not necessarily desire a grant
	hasActiveGrant := activeGrant != nil

	// update the grantee's active stake grant if the granter matches
	hasActiveGrantFromGranter := hasActiveGrant && activeGrant.Granter == granter.String()

	if hasActiveGrantFromGranter || !hasActiveGrant {
		grant := v2.NewActiveGrant(granter, amount)
		k.SetActiveGrant(ctx, grantee, grant)
		events.Emit(ctx, k.BaseKeeper, &v2.EventGrantActivation{
			Grantee: grantee.String(),
			Granter: grant.Granter,
			Amount:  grant.Amount,
		})
	}
}
