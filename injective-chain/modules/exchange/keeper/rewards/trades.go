package rewards

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
	"github.com/InjectiveLabs/metrics"
)

// GetAllOptedOutRewardAccounts gets all accounts that have opted out of rewards
func (k TradingKeeper) GetAllOptedOutRewardAccounts(ctx sdk.Context) []string {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	registeredDMMs := make([]string, 0)
	k.IterateOptedOutRewardAccounts(ctx, func(account sdk.AccAddress, isRegisteredDMM bool) (stop bool) {
		if isRegisteredDMM {
			registeredDMMs = append(registeredDMMs, account.String())
		}

		return false
	})

	return registeredDMMs
}

//nolint:revive // ok
func (k TradingKeeper) GetTradeDataAndIncrementVolumeContribution(
	ctx sdk.Context,
	subaccountID common.Hash,
	marketID common.Hash,
	fillQuantity, executionPrice math.LegacyDec,
	tradeFeeRate, relayerFeeShareRate math.LegacyDec,
	tradeRewardMultiplier math.LegacyDec,
	feeDiscountConfig *v2.FeeDiscountConfig,
	isMaker bool,
) *v2.TradeFeeData {
	discountedTradeFeeRate := k.feeDiscounts.FetchAndUpdateDiscountedTradingFeeRate(
		ctx,
		tradeFeeRate,
		isMaker,
		types.SubaccountIDToSdkAddress(subaccountID),
		feeDiscountConfig,
	)

	if fillQuantity.IsZero() {
		return v2.NewEmptyTradeFeeData(discountedTradeFeeRate)
	}

	orderFillNotional := fillQuantity.Mul(executionPrice)

	totalTradeFee, traderFee, feeRecipientReward, auctionFeeReward := GetOrderFillFeeInfo(
		orderFillNotional,
		discountedTradeFeeRate,
		relayerFeeShareRate,
	)

	feeDiscountConfig.IncrementAccountVolumeContribution(subaccountID, marketID, orderFillNotional, isMaker)

	tradingRewardPoints := orderFillNotional.Mul(tradeRewardMultiplier).Abs()

	return &v2.TradeFeeData{
		TotalTradeFee:          totalTradeFee,
		TraderFee:              traderFee,
		TradingRewardPoints:    tradingRewardPoints,
		FeeRecipientReward:     feeRecipientReward,
		AuctionFeeReward:       auctionFeeReward,
		DiscountedTradeFeeRate: discountedTradeFeeRate,
	}
}

//nolint:revive // ok
func GetOrderFillFeeInfo(
	orderFillNotional,
	tradeFeeRate,
	relayerFeeShareRate math.LegacyDec,
) (
	totalTradeFee,
	traderFee,
	feeRecipientReward,
	auctionFeeReward math.LegacyDec,
) {
	totalTradeFee = orderFillNotional.Mul(tradeFeeRate)
	feeRecipientReward = relayerFeeShareRate.Mul(totalTradeFee).Abs()

	if totalTradeFee.IsNegative() {
		// trader "pays" aka only receives the trading fee without the fee recipient reward component
		traderFee = totalTradeFee.Add(feeRecipientReward)
		auctionFeeReward = totalTradeFee // taker auction fees pay for maker
	} else {
		traderFee = totalTradeFee
		auctionFeeReward = totalTradeFee.Sub(feeRecipientReward)
	}

	return totalTradeFee, traderFee, feeRecipientReward, auctionFeeReward
}
