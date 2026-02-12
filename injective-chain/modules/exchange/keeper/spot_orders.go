package keeper

import (
	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k *Keeper) validateSpotMarketOrder(
	ctx sdk.Context,
	order *v2.SpotOrder,
	market *v2.SpotMarket,
	marketID common.Hash,
	subaccountID common.Hash,
) (*v2.SpotMarket, error) {
	if k.IsPostOnlyMode(ctx) {
		return nil, types.ErrPostOnlyMode.Wrapf(
			"cannot create market orders in post only mode until height %d",
			k.GetParams(ctx).PostOnlyModeHeightThreshold,
		)
	}

	if order.ExpirationBlock != 0 {
		metrics.ReportFuncError(k.svcTags)
		return nil, types.ErrInvalidExpirationBlock.Wrap("market orders cannot have expiration block")
	}

	return k.ValidateSpotOrder(ctx, order, market, marketID, subaccountID)
}

func (k *Keeper) createSpotMarketOrder(
	ctx sdk.Context,
	sender sdk.AccAddress,
	order *v2.SpotOrder,
	market *v2.SpotMarket,
) (hash common.Hash, err error) {
	_, possibleHash, err := k.createSpotMarketOrderWithResultsForAtomicExecution(ctx, sender, order, market)
	if possibleHash == nil {
		hash = common.Hash{}
	} else {
		hash = *possibleHash
	}

	return hash, err
}

func (k *Keeper) createSpotMarketOrderWithResultsForAtomicExecution(
	ctx sdk.Context,
	sender sdk.AccAddress,
	order *v2.SpotOrder,
	market *v2.SpotMarket,
) (marketOrderResults *v2.SpotMarketOrderResults, hash *common.Hash, err error) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	marketID := common.HexToHash(order.MarketId)
	subaccountID := types.MustGetSubaccountIDOrDeriveFromNonce(sender, order.OrderInfo.SubaccountId)

	// populate the order with the actual subaccountID value, since it might be a nonce value
	order.OrderInfo.SubaccountId = subaccountID.Hex()

	validatedMarket, err := k.validateSpotMarketOrder(ctx, order, market, marketID, subaccountID)
	if err != nil {
		return nil, nil, err
	}

	isAtomic := order.OrderType.IsAtomic()
	if isAtomic {
		// todo: ideally this should be in spot keeper but it depends wasm (much like derv)
		if err := k.EnsureValidAccessLevelForAtomicExecution(ctx, sender); err != nil {
			return nil, nil, err
		}
	}

	subaccountNonce := k.IncrementSubaccountTradeNonce(ctx, subaccountID)

	orderHash, err := order.ComputeOrderHash(subaccountNonce.Nonce)
	if err != nil {
		metrics.ReportFuncError(k.svcTags)
		return nil, nil, err
	}

	marginDenom := order.GetMarginDenom(validatedMarket)

	bestPrice := k.GetBestSpotLimitOrderPrice(ctx, marketID, !order.IsBuy())

	if err := k.validateMarketOrderBestPriceAgainstOrder(order, bestPrice); err != nil {
		metrics.ReportFuncError(k.svcTags)
		return nil, &orderHash, err
	}

	feeRate := validatedMarket.TakerFeeRate
	if order.OrderType.IsAtomic() {
		feeRate = feeRate.Mul(k.GetMarketAtomicExecutionFeeMultiplier(ctx, marketID, types.MarketType_Spot))
	}

	balanceHold, chainFormattedBalanceHold := k.computeMarketOrderBalanceHold(validatedMarket, order, feeRate, *bestPrice)

	if err := k.ChargeAccount(ctx, subaccountID, marginDenom, chainFormattedBalanceHold); err != nil {
		return nil, &orderHash, err
	}

	marketOrder := order.ToSpotMarketOrder(sender, balanceHold, orderHash)

	marketOrderResults = k.executeOrQueueMarketOrder(ctx, validatedMarket, marketOrder, feeRate, isAtomic, order, orderHash)

	k.CheckAndSetFeeDiscountAccountActivityIndicator(ctx, marketID, sender)

	return marketOrderResults, &orderHash, nil
}

// validateMarketOrderBestPriceAgainstOrder checks liquidity and worst-price slippage constraints
// for a spot market order relative to the current best opposing price.
func (*Keeper) validateMarketOrderBestPriceAgainstOrder(
	order *v2.SpotOrder,
	bestPrice *math.LegacyDec,
) error {
	if bestPrice == nil {
		return types.ErrNoLiquidity
	}
	if (order.IsBuy() && order.OrderInfo.Price.LT(*bestPrice)) ||
		(!order.IsBuy() && order.OrderInfo.Price.GT(*bestPrice)) {
		return types.ErrSlippageExceedsWorstPrice
	}
	return nil
}

// computeMarketOrderBalanceHold returns both logical and chain-formatted balance holds
// for a spot market order, accounting for buy/sell denomination differences.
func (*Keeper) computeMarketOrderBalanceHold(
	market *v2.SpotMarket,
	order *v2.SpotOrder,
	feeRate, bestPrice math.LegacyDec,
) (balanceHold, chainFormattedBalanceHold math.LegacyDec) {
	balanceHold = order.GetMarketOrderBalanceHold(feeRate, bestPrice)
	if order.IsBuy() {
		chainFormattedBalanceHold = market.NotionalToChainFormat(balanceHold)
	} else {
		chainFormattedBalanceHold = market.QuantityToChainFormat(balanceHold)
	}
	return balanceHold, chainFormattedBalanceHold
}

// executeOrQueueMarketOrder runs atomic execution immediately or stores the order transiently for batch execution.
func (k *Keeper) executeOrQueueMarketOrder(
	ctx sdk.Context,
	market *v2.SpotMarket,
	marketOrder *v2.SpotMarketOrder,
	feeRate math.LegacyDec,
	//revive:disable:flag-parameter // receiving isAtomic as a flag parameter instead of the DerivativeOrder
	isAtomic bool,
	originalOrder *v2.SpotOrder,
	orderHash common.Hash,
) (results *v2.SpotMarketOrderResults) {
	if isAtomic {
		return k.ExecuteAtomicSpotMarketOrder(ctx, market, marketOrder, feeRate)
	}
	k.SetNewTransientSpotMarketOrder(ctx, marketOrder, originalOrder, orderHash)
	return nil
}

func (k *Keeper) cancelSpotLimitOrderWithIdentifier(
	ctx sdk.Context,
	subaccountID common.Hash,
	identifier any, // either order hash or cid
	market *v2.SpotMarket,
	marketID common.Hash,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	orderHash, err := k.GetOrderHashFromIdentifier(ctx, subaccountID, identifier)
	if err != nil {
		return err
	}

	return k.CancelSpotLimitOrderByOrderHash(ctx, subaccountID, orderHash, market, marketID)
}
