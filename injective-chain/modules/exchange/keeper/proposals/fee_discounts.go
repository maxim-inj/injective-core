package proposals

import (
	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

const REQUIRED_FEE_DISCOUNT_QUOTE_DECIMALS = 6

//nolint:revive // ok
func (k *ProposalKeeper) HandleFeeDiscountProposal(ctx sdk.Context, p *v2.FeeDiscountProposal) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	prevSchedule := k.GetFeeDiscountSchedule(ctx)
	if prevSchedule != nil {
		k.DeleteAllFeeDiscountMarketQualifications(ctx)
		k.DeleteFeeDiscountSchedule(ctx)
	}

	for _, denom := range p.Schedule.QuoteDenoms {
		if !k.IsDenomValid(ctx, denom) {
			return errors.Wrapf(types.ErrInvalidQuoteDenom, "denom %s does not exist in supply", denom)
		}
		denomDecimals, _ := k.TokenDenomDecimals(ctx, denom)
		if denomDecimals != REQUIRED_FEE_DISCOUNT_QUOTE_DECIMALS {
			return errors.Wrapf(types.ErrInvalidQuoteDenom, "denom %s does not have 6 decimals", denom)
		}
	}

	maxTakerDiscount := p.Schedule.TierInfos[len(p.Schedule.TierInfos)-1].TakerDiscountRate

	spotMarkets := k.GetAllSpotMarkets(ctx)
	derivativeMarkets := k.GetAllDerivativeMarkets(ctx)
	binaryOptionsMarkets := k.GetAllBinaryOptionsMarkets(ctx)

	allMarkets := make([]v2.MarketI, 0, len(spotMarkets)+len(derivativeMarkets)+len(binaryOptionsMarkets))
	for _, market := range spotMarkets {
		allMarkets = append(allMarkets, market)
	}

	for _, market := range derivativeMarkets {
		allMarkets = append(allMarkets, market)
	}

	for _, market := range binaryOptionsMarkets {
		allMarkets = append(allMarkets, market)
	}

	disqualified := make(map[string]struct{})
	for _, marketID := range p.Schedule.DisqualifiedMarketIds {
		disqualified[marketID] = struct{}{}
	}

	filteredMarkets := make([]v2.MarketI, 0, len(allMarkets))
	for _, market := range allMarkets {
		if _, ok := disqualified[market.MarketID().String()]; ok {
			continue
		}

		filteredMarkets = append(filteredMarkets, market)
	}

	for _, market := range filteredMarkets {
		if !market.GetMakerFeeRate().IsNegative() {
			continue
		}

		minimalProtocolFeeRate := k.GetMinimalProtocolFeeRate(ctx, market)

		smallestTakerFeeRate := math.LegacyOneDec().Sub(maxTakerDiscount).Mul(market.GetTakerFeeRate())
		if err := types.ValidateMakerWithTakerFee(
			market.GetMakerFeeRate(),
			smallestTakerFeeRate,
			market.GetRelayerFeeShareRate(),
			minimalProtocolFeeRate,
		); err != nil {
			return err
		}
	}

	isBucketCountSame := k.GetFeeDiscountBucketCount(ctx) == p.Schedule.BucketCount
	isBucketDurationSame := k.GetFeeDiscountBucketDuration(ctx) == p.Schedule.BucketDuration

	var isQuoteDenomsSame bool
	if prevSchedule != nil {
		isQuoteDenomsSame = types.IsEqualDenoms(p.Schedule.QuoteDenoms, prevSchedule.QuoteDenoms)
	}

	hasBucketConfigChanged := !isBucketCountSame || !isBucketDurationSame || !isQuoteDenomsSame
	if hasBucketConfigChanged {
		k.DeleteAllAccountVolumeInAllBucketsWithMetadata(ctx)
		k.SetIsFirstFeeCycleFinished(ctx, false)

		startTimestamp := ctx.BlockTime().Unix()
		k.SetFeeDiscountCurrentBucketStartTimestamp(ctx, startTimestamp)
	} else if prevSchedule == nil {
		startTimestamp := ctx.BlockTime().Unix()
		k.SetFeeDiscountCurrentBucketStartTimestamp(ctx, startTimestamp)
	}

	k.SetFeeDiscountMarketQualificationForAllQualifyingMarkets(ctx, p.Schedule)
	k.SaveFeeDiscountSchedule(ctx, p.Schedule)

	return nil
}
