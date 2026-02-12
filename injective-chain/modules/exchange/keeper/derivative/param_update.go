package derivative

import (
	"fmt"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/events"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
	insurancetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/insurance/types"
)

func (k DerivativeKeeper) ExecuteDerivativeMarketParamUpdateProposal(ctx sdk.Context, p *v2.DerivativeMarketParamUpdateProposal) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := common.HexToHash(p.MarketId)
	prevMarket := k.GetDerivativeMarketByID(ctx, marketID)

	if prevMarket == nil {
		metrics.ReportFuncCall(k.svcTags)
		return fmt.Errorf("market is not available, market_id %s", p.MarketId)
	}

	// cancel resting orders in the market when it shuts down
	k.handleMarketStatusChange(ctx, p.Status, prevMarket)

	// handle fee rate changes
	k.handleMakerFeeRateChange(ctx, marketID, prevMarket.MakerFeeRate, p.MakerFeeRate, prevMarket)
	k.handleTakerFeeRateChange(ctx, marketID, prevMarket.TakerFeeRate, p.TakerFeeRate, prevMarket)

	if err := k.UpdateDerivativeMarketParam(
		ctx,
		common.HexToHash(p.MarketId),
		p.InitialMarginRatio,
		p.MaintenanceMarginRatio,
		p.ReduceMarginRatio,
		p.MakerFeeRate,
		p.TakerFeeRate,
		p.RelayerFeeShareRate,
		p.MinPriceTickSize,
		p.MinQuantityTickSize,
		p.MinNotional,
		p.HourlyInterestRate,
		p.HourlyFundingRateCap,
		p.OpenNotionalCap,
		p.HasDisabledMinimalProtocolFee,
		p.Status,
		p.OracleParams,
		p.Ticker,
		p.AdminInfo,
	); err != nil {
		return errors.Wrap(err, "UpdateDerivativeMarketParam failed during ExecuteDerivativeMarketParamUpdateProposal")
	}

	return nil
}

func (k DerivativeKeeper) handleMarketStatusChange(ctx sdk.Context, status v2.MarketStatus, market *v2.DerivativeMarket) {
	switch status {
	case v2.MarketStatus_Expired, v2.MarketStatus_Demolished:
		k.CancelAllRestingDerivativeLimitOrders(ctx, market)
		k.CancelAllConditionalDerivativeOrders(ctx, market)
	default:
	}
}

func (k DerivativeKeeper) handleMakerFeeRateChange(
	ctx sdk.Context, marketID common.Hash, prevRate math.LegacyDec, newRate *math.LegacyDec, market *v2.DerivativeMarket,
) {
	if newRate == nil {
		return
	}

	if newRate.LT(prevRate) {
		orders := k.GetAllDerivativeLimitOrdersByMarketID(ctx, marketID)
		k.HandleDerivativeFeeDecrease(ctx, orders, prevRate, *newRate, market)
	} else if newRate.GT(prevRate) {
		orders := k.GetAllDerivativeLimitOrdersByMarketID(ctx, marketID)
		k.HandleDerivativeFeeIncrease(ctx, orders, *newRate, market)
	}
}

func (k DerivativeKeeper) handleTakerFeeRateChange(
	ctx sdk.Context, marketID common.Hash, prevRate math.LegacyDec, newRate *math.LegacyDec, market *v2.DerivativeMarket,
) {
	if newRate == nil {
		return
	}

	if newRate.LT(prevRate) {
		orders := k.GetAllConditionalDerivativeOrdersUpToMarkPrice(ctx, marketID, nil)
		// NOTE: this won't work for conditional post only orders (currently not supported)
		k.handleDerivativeFeeDecreaseForConditionals(ctx, orders, prevRate, *newRate, market)
	} else if newRate.GT(prevRate) {
		orders := k.GetAllConditionalDerivativeOrdersUpToMarkPrice(ctx, marketID, nil)
		k.handleDerivativeFeeIncreaseForConditionals(ctx, orders, prevRate, *newRate, market)
	}
}

func (k DerivativeKeeper) HandleDerivativeFeeDecrease(
	ctx sdk.Context,
	orderbook []*v2.DerivativeLimitOrder,
	prevFeeRate,
	newFeeRate math.LegacyDec,
	market v2.DerivativeMarketI,
) {
	isFeeRefundRequired := prevFeeRate.IsPositive()
	if !isFeeRefundRequired {
		return
	}

	feeRefundRate := math.LegacyMinDec(prevFeeRate, prevFeeRate.Sub(newFeeRate)) // negative newFeeRate part is ignored

	for _, order := range orderbook {
		if order.IsReduceOnly() {
			continue
		}

		// nolint:all
		// FeeRefund = (PreviousMakerFeeRate - NewMakerFeeRate) * FillableQuantity * Price
		// AvailableBalance += FeeRefund
		feeRefund := feeRefundRate.Mul(order.GetFillable()).Mul(order.GetPrice())
		subaccountID := order.GetSubaccountID()
		chainFormatRefund := market.NotionalToChainFormat(feeRefund)
		k.subaccount.IncrementAvailableBalanceOrBank(ctx, subaccountID, market.GetQuoteDenom(), chainFormatRefund)
	}
}

func (k DerivativeKeeper) handleDerivativeFeeDecreaseForConditionals(
	ctx sdk.Context,
	orderbook *v2.ConditionalDerivativeOrderBook,
	prevFeeRate,
	newFeeRate math.LegacyDec,
	market v2.DerivativeMarketI,
) {
	isFeeRefundRequired := prevFeeRate.IsPositive()
	if !isFeeRefundRequired {
		return
	}

	feeRefundRate := math.LegacyMinDec(prevFeeRate, prevFeeRate.Sub(newFeeRate)) // negative newFeeRate part is ignored
	var decreaseRate = func(order types.IDerivativeOrder) {
		if order.IsReduceOnly() {
			return
		}

		// nolint:all
		// FeeRefund = (PreviousMakerFeeRate - NewMakerFeeRate) * FillableQuantity * Price
		// AvailableBalance += FeeRefund
		feeRefund := feeRefundRate.Mul(order.GetFillable()).Mul(order.GetPrice())
		chainFormatRefund := market.NotionalToChainFormat(feeRefund)
		k.subaccount.IncrementAvailableBalanceOrBank(ctx, order.GetSubaccountID(), market.GetQuoteDenom(), chainFormatRefund)
	}

	for _, order := range orderbook.GetMarketOrders() {
		decreaseRate(order)
	}

	for _, order := range orderbook.GetLimitOrders() {
		decreaseRate(order)
	}
}

//revive:disable:cognitive-complexity // The complexity is acceptable for now, to avoid creating more helper functions
func (k DerivativeKeeper) handleDerivativeFeeIncreaseForConditionals(
	ctx sdk.Context,
	orderbook *v2.ConditionalDerivativeOrderBook,
	prevFeeRate,
	newFeeRate math.LegacyDec,
	prevMarket v2.DerivativeMarketI,
) {
	isExtraFeeChargeRequired := newFeeRate.IsPositive()
	if !isExtraFeeChargeRequired {
		return
	}

	feeChargeRate := math.LegacyMinDec(newFeeRate, newFeeRate.Sub(prevFeeRate)) // negative prevFeeRate part is ignored
	denom := prevMarket.GetQuoteDenom()

	for _, order := range orderbook.GetMarketOrders() {
		if !k.tryChargeExtraFeeForDerivativeOrder(ctx, order, order.SubaccountID(), feeChargeRate, denom, prevMarket) {
			if err := k.CancelConditionalDerivativeMarketOrder(ctx, prevMarket, order.SubaccountID(), nil, order.Hash()); err != nil {
				k.Logger(ctx).Info(
					"CancelConditionalDerivativeMarketOrder failed during handleDerivativeFeeIncreaseForConditionals",
					"orderHash", common.BytesToHash(order.OrderHash).Hex(),
					"err", err,
				)
			}
		}
	}

	for _, order := range orderbook.GetLimitOrders() {
		if !k.tryChargeExtraFeeForDerivativeOrder(ctx, order, order.SubaccountID(), feeChargeRate, denom, prevMarket) {
			if err := k.CancelConditionalDerivativeLimitOrder(ctx, prevMarket, order.SubaccountID(), nil, order.Hash()); err != nil {
				k.Logger(ctx).Info(
					"CancelConditionalDerivativeLimitOrder failed during handleDerivativeFeeIncreaseForConditionals",
					"orderHash", common.BytesToHash(order.OrderHash).Hex(),
					"err", err,
				)
			}
		}
	}
}

func (k DerivativeKeeper) tryChargeExtraFeeForDerivativeOrder(
	ctx sdk.Context,
	order types.IDerivativeOrder,
	subaccountID common.Hash,
	feeChargeRate math.LegacyDec,
	denom string,
	prevMarket v2.DerivativeMarketI,
) bool {
	if order.IsReduceOnly() {
		return true
	}

	// ExtraFee = (newFeeRate - prevFeeRate) * FillableQuantity * Price
	// AvailableBalance -= ExtraFee
	// If AvailableBalance < ExtraFee, cancel the order
	extraFee := feeChargeRate.Mul(order.GetFillable()).Mul(order.GetPrice())
	chainFormatExtraFee := prevMarket.NotionalToChainFormat(extraFee)

	hasSufficientFundsToPayExtraFee := k.subaccount.HasSufficientFunds(ctx, subaccountID, denom, chainFormatExtraFee)

	if hasSufficientFundsToPayExtraFee {
		// bank charge should fail if the account no longer has permissions to send the tokens
		chargeCtx := ctx.WithValue(baseapp.DoNotFailFastSendContextKey, nil)

		err := k.subaccount.ChargeAccount(chargeCtx, subaccountID, denom, chainFormatExtraFee)
		// defensive programming: continue to next order if charging the extra fee succeeds
		// otherwise cancel the order
		if err == nil {
			return true
		}

		k.Logger(ctx).Error("handleDerivativeFeeIncreaseForConditionals chargeAccount fail:", err)
	}

	return false
}

func (k DerivativeKeeper) HandleDerivativeFeeIncrease(
	ctx sdk.Context,
	orderbook []*v2.DerivativeLimitOrder,
	newMakerFeeRate math.LegacyDec,
	prevMarket v2.DerivativeMarketI,
) {
	isExtraFeeChargeRequired := newMakerFeeRate.IsPositive()
	if !isExtraFeeChargeRequired {
		return
	}

	feeChargeRate := math.LegacyMinDec(
		newMakerFeeRate, newMakerFeeRate.Sub(prevMarket.GetMakerFeeRate()),
	) // negative prevMarket.MakerFeeRate part is ignored
	denom := prevMarket.GetQuoteDenom()

	for _, order := range orderbook {
		k.processOrderForFeeIncrease(ctx, order, feeChargeRate, denom, prevMarket)
	}
}

func (k DerivativeKeeper) processOrderForFeeIncrease(
	ctx sdk.Context,
	order *v2.DerivativeLimitOrder,
	feeChargeRate math.LegacyDec,
	denom string,
	prevMarket v2.DerivativeMarketI,
) {
	if order.IsReduceOnly() {
		return
	}

	// ExtraFee = (NewMakerFeeRate - PreviousMakerFeeRate) * FillableQuantity * Price
	// AvailableBalance -= ExtraFee
	// If AvailableBalance < ExtraFee, Cancel the order
	extraFee := feeChargeRate.Mul(order.Fillable).Mul(order.OrderInfo.Price)
	chainFormatExtraFee := prevMarket.NotionalToChainFormat(extraFee)

	subaccountID := order.SubaccountID()

	hasSufficientFundsToPayExtraFee := k.subaccount.HasSufficientFunds(ctx, subaccountID, denom, chainFormatExtraFee)

	if hasSufficientFundsToPayExtraFee {
		// bank charge should fail if the account no longer has permissions to send the tokens
		chargeCtx := ctx.WithValue(baseapp.DoNotFailFastSendContextKey, nil)

		err := k.subaccount.ChargeAccount(chargeCtx, subaccountID, denom, chainFormatExtraFee)

		// defensive programming: continue to next order if charging the extra fee succeeds
		// otherwise cancel the order
		if err == nil {
			return
		}
	}

	k.cancelDerivativeOrderDuringFeeIncrease(ctx, prevMarket, order)
}

func (k DerivativeKeeper) cancelDerivativeOrderDuringFeeIncrease(
	ctx sdk.Context,
	prevMarket v2.DerivativeMarketI,
	order *v2.DerivativeLimitOrder,
) {
	subaccountID := order.SubaccountID()
	isBuy := order.IsBuy()
	if err := k.CancelRestingDerivativeLimitOrder(
		ctx,
		prevMarket,
		subaccountID,
		&isBuy,
		common.BytesToHash(order.OrderHash),
		true,
		true,
	); err != nil {
		k.Logger(ctx).Error(
			"CancelRestingDerivativeLimitOrder failed during handleDerivativeFeeIncrease",
			"orderHash", common.BytesToHash(order.OrderHash).Hex(),
			"err", err.Error(),
		)

		events.Emit(
			ctx,
			k.BaseKeeper,
			v2.NewEventOrderCancelFail(
				prevMarket.MarketID(),
				subaccountID,
				order.Hash().Hex(),
				order.Cid(),
				err,
			),
		)
	}
}

//nolint:revive // ok
func (k DerivativeKeeper) UpdateDerivativeMarketParam(
	ctx sdk.Context,
	marketID common.Hash,
	initialMarginRatio, maintenanceMarginRatio, reduceMarginRatio *math.LegacyDec,
	makerFeeRate, takerFeeRate, relayerFeeShareRate, minPriceTickSize *math.LegacyDec,
	minQuantityTickSize, minNotional, hourlyInterestRate, hourlyFundingRateCap *math.LegacyDec,
	openNotionalCap *v2.OpenNotionalCap,
	hasDisabledMinimalProtocolFeeUpdate v2.DisableMinimalProtocolFeeUpdate,

	status v2.MarketStatus,
	oracleParams *v2.OracleParams,
	ticker string,
	adminInfo *v2.AdminInfo,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	market := k.GetDerivativeMarketByID(ctx, marketID)
	originalMarketStatus := market.Status

	isActiveStatusChange := market.IsActive() && status != v2.MarketStatus_Active || (market.IsInactive() && status == v2.MarketStatus_Active)

	shouldUpdateNextFundingTimestamp := false

	if isActiveStatusChange {
		isEnabled := true
		if market.Status != v2.MarketStatus_Active {
			isEnabled = false

			if market.IsPerpetual {
				// the next funding timestamp should be updated if the market status changes to active
				shouldUpdateNextFundingTimestamp = true
			}
		}
		k.DeleteDerivativeMarket(ctx, marketID, isEnabled)
	}

	if initialMarginRatio == nil {
		return errors.Wrap(types.ErrInvalidMarginRatio, "initial_margin_ratio is nil")
	}
	if maintenanceMarginRatio == nil {
		return errors.Wrap(types.ErrInvalidMarginRatio, "maintenance_margin_ratio is nil")
	}
	if reduceMarginRatio == nil {
		return errors.Wrap(types.ErrInvalidMarginRatio, "reduce_margin_ratio is nil")
	}
	if openNotionalCap == nil {
		return errors.Wrap(types.ErrInvalidOpenNotionalCap, "open_notional_cap is nil")
	}

	market.InitialMarginRatio = *initialMarginRatio
	market.MaintenanceMarginRatio = *maintenanceMarginRatio
	market.ReduceMarginRatio = *reduceMarginRatio
	market.MakerFeeRate = *makerFeeRate
	market.TakerFeeRate = *takerFeeRate
	market.RelayerFeeShareRate = *relayerFeeShareRate
	market.MinPriceTickSize = *minPriceTickSize
	market.MinQuantityTickSize = *minQuantityTickSize
	market.MinNotional = *minNotional
	market.OpenNotionalCap = *openNotionalCap
	market.Status = status
	market.Ticker = ticker

	if hasDisabledMinimalProtocolFeeUpdate != v2.DisableMinimalProtocolFeeUpdate_NoUpdate {
		market.HasDisabledMinimalProtocolFee = hasDisabledMinimalProtocolFeeUpdate == v2.DisableMinimalProtocolFeeUpdate_True
	}

	if adminInfo != nil {
		market.Admin = adminInfo.Admin
		market.AdminPermissions = adminInfo.AdminPermissions
	} else {
		market.Admin = ""
		market.AdminPermissions = 0
	}

	if oracleParams != nil {
		market.OracleBase = oracleParams.OracleBase
		market.OracleQuote = oracleParams.OracleQuote
		market.OracleType = oracleParams.OracleType
		market.OracleScaleFactor = oracleParams.OracleScaleFactor
	}

	var perpetualMarketInfo *v2.PerpetualMarketInfo
	isUpdatingFundingRate := shouldUpdateNextFundingTimestamp || hourlyInterestRate != nil || hourlyFundingRateCap != nil

	if isUpdatingFundingRate {
		perpetualMarketInfo = k.GetPerpetualMarketInfo(ctx, marketID)

		if shouldUpdateNextFundingTimestamp {
			perpetualMarketInfo.NextFundingTimestamp = getNextIntervalTimestamp(ctx.BlockTime().Unix(), perpetualMarketInfo.FundingInterval)
		}

		if hourlyFundingRateCap != nil {
			perpetualMarketInfo.HourlyFundingRateCap = *hourlyFundingRateCap
		}

		if hourlyInterestRate != nil {
			perpetualMarketInfo.HourlyInterestRate = *hourlyInterestRate
		}
	}

	insuranceFund := k.insurance.GetInsuranceFund(ctx, marketID)
	if insuranceFund == nil {
		return errors.Wrapf(insurancetypes.ErrInsuranceFundNotFound, "ticker %s marketID %s", market.Ticker, marketID.Hex())
	}

	shouldUpdateInsuranceFundOracleParams := insuranceFund.OracleBase != market.OracleBase ||
		insuranceFund.OracleQuote != market.OracleQuote ||
		insuranceFund.OracleType != market.OracleType
	if shouldUpdateInsuranceFundOracleParams {
		oracleParamsV1 := types.NewOracleParams(market.OracleBase, market.OracleQuote, market.OracleScaleFactor, market.OracleType)
		if err := k.insurance.UpdateInsuranceFundOracleParams(ctx, marketID, oracleParamsV1); err != nil {
			return errors.Wrap(err, "UpdateInsuranceFundOracleParams failed during UpdateDerivativeMarketParam")
		}
	}

	// reactivation of a market should only reset the market balance to zero if there are no positions
	if originalMarketStatus != v2.MarketStatus_Active && status == v2.MarketStatus_Active {
		if !k.HasPositionsInMarket(ctx, marketID) {
			k.SetMarketBalance(ctx, marketID, math.LegacyZeroDec())
		}
	}
	k.SetDerivativeMarketWithInfo(ctx, market, nil, perpetualMarketInfo, nil)
	return nil
}

// equivalent to floor(currTime / interval) * interval + interval
func getNextIntervalTimestamp(currTime, interval int64) int64 {
	return (currTime/interval)*interval + interval
}
