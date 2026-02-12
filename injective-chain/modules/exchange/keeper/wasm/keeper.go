package wasm

import (
	"encoding/json"
	"sort"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/base"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/derivative"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/events"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/subaccount"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
	wasmxtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/wasmx/types"
)

type WasmKeeper struct { //nolint:revive // ok
	*base.BaseKeeper

	bank       bankkeeper.Keeper
	subaccount *subaccount.SubaccountKeeper
	derivative *derivative.DerivativeKeeper
	wasmv      types.WasmViewKeeper
	wasmx      types.WasmxExecutionKeeper
	svcTags    metrics.Tags
}

func New(
	b *base.BaseKeeper,
	bk bankkeeper.Keeper,
	sk *subaccount.SubaccountKeeper,
	d *derivative.DerivativeKeeper,
	wv types.WasmViewKeeper,
	wx types.WasmxExecutionKeeper,
) *WasmKeeper {
	return &WasmKeeper{
		BaseKeeper: b,
		bank:       bk,
		subaccount: sk,
		derivative: d,
		wasmv:      wv,
		wasmx:      wx,
		svcTags:    map[string]string{"svc": "wasm_k"},
	}
}

func (k WasmKeeper) PrivilegedExecuteContractWithVersion(
	ctx sdk.Context,
	msg *v2.MsgPrivilegedExecuteContract,
	exchangeTypeVersion types.ExchangeTypeVersion,
) (*v2.MsgPrivilegedExecuteContractResponse, error) {
	k.Logger(ctx).Debug("=============== ⭐️ [Start] PrivilegedExecuteContract ⭐️ ===============")

	sender, _ := sdk.AccAddressFromBech32(msg.Sender)
	contract, _ := sdk.AccAddressFromBech32(msg.ContractAddress)

	fundsBefore, totalFunds, err := k.handleFundsTransfer(ctx, msg, sender, contract)
	if err != nil {
		return nil, err
	}

	err = k.executeContractAndHandleAction(ctx, contract, sender, totalFunds, msg.Data, exchangeTypeVersion)
	if err != nil {
		return nil, err
	}

	filteredFundsDiff := k.calculateFundsDifference(ctx, sender, fundsBefore)

	k.Logger(ctx).Debug("=============== 🛏️ [End] Exec 🛏️ ===============")
	return &v2.MsgPrivilegedExecuteContractResponse{FundsDiff: filteredFundsDiff}, nil
}

func (k WasmKeeper) handleFundsTransfer(
	ctx sdk.Context,
	msg *v2.MsgPrivilegedExecuteContract,
	sender,
	contract sdk.AccAddress,
) (fundsBefore, totalFunds sdk.Coins, err error) {
	fundsBefore = sdk.Coins(make([]sdk.Coin, 0, len(msg.Funds)))
	totalFunds = sdk.Coins{}

	// Enforce sender has sufficient funds for execution
	if !msg.HasEmptyFunds() {
		coins, err := sdk.ParseCoinsNormalized(msg.Funds)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to parse coins %s", msg.Funds)
		}

		for _, coin := range coins {
			coinBefore := k.bank.GetBalance(ctx, sender, coin.Denom)
			fundsBefore = fundsBefore.Add(coinBefore)
		}

		// No need to check if receiver is a blocked address because it could never be a module account
		if err := k.bank.SendCoins(ctx, sender, contract, coins); err != nil {
			return nil, nil, errors.Wrap(err, "failed to send coins")
		}
		totalFunds = coins
	}

	return fundsBefore, totalFunds, nil
}

func (k WasmKeeper) executeContractAndHandleAction(
	ctx sdk.Context, contract, sender sdk.AccAddress, totalFunds sdk.Coins, data string, exchangeTypeVersion types.ExchangeTypeVersion,
) error {
	execMsg, err := wasmxtypes.NewInjectiveExecMsg(sender, data)
	if err != nil {
		return errors.Wrap(err, "failed to create exec msg")
	}

	res, err := k.wasmx.InjectiveExec(ctx, contract, totalFunds, execMsg)
	if err != nil {
		return errors.Wrap(err, "failed to execute msg")
	}

	action, err := types.ParseRequest(res)
	if err != nil {
		return errors.Wrap(err, "failed to execute msg")
	}

	if action != nil {
		err = k.HandlePrivilegedAction(ctx, contract, sender, action, exchangeTypeVersion)
		if err != nil {
			return errors.Wrap(err, "failed to execute msg")
		}
	}

	return nil
}

func (k WasmKeeper) HandlePrivilegedAction(
	ctx sdk.Context,
	contractAddress sdk.AccAddress,
	origin sdk.AccAddress,
	action types.InjectiveAction,
	exchangeTypeVersion types.ExchangeTypeVersion,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	switch t := action.(type) {
	case *types.SyntheticTradeAction:
		return k.handleSyntheticTradePrivilegedAction(ctx, contractAddress, origin, t, exchangeTypeVersion)
	case *types.PositionTransfer:
		return k.HandlePositionTransferAction(ctx, contractAddress, origin, t)
	default:
		return types.ErrUnsupportedAction
	}
}

func (k WasmKeeper) handleSyntheticTradePrivilegedAction(
	ctx sdk.Context,
	contractAddress sdk.AccAddress,
	origin sdk.AccAddress,
	action *types.SyntheticTradeAction,
	exchangeTypeVersion types.ExchangeTypeVersion,
) error {
	if exchangeTypeVersion == types.ExchangeTypeVersionV1 {
		newContractTrades, err := k.ConvertSyntheticTradesV1ToV2(ctx, action.ContractTrades)
		if err != nil {
			return err
		}

		newUserTrades, err := k.ConvertSyntheticTradesV1ToV2(ctx, action.UserTrades)
		if err != nil {
			return err
		}

		action.ContractTrades = newContractTrades
		action.UserTrades = newUserTrades
	}

	return k.HandleSyntheticTradeAction(ctx, contractAddress, origin, action)
}

func (k WasmKeeper) ConvertSyntheticTradesV1ToV2(
	ctx sdk.Context,
	trades []*types.SyntheticTrade,
) ([]*types.SyntheticTrade, error) {
	v2Trades := make([]*types.SyntheticTrade, 0, len(trades))
	for _, trade := range trades {
		derivativeMarket := k.derivative.GetDerivativeMarketByID(ctx, trade.MarketID)
		if derivativeMarket == nil {
			return nil, errors.Wrap(types.ErrDerivativeMarketNotFound, "failed to convert trade type to v2")
		}

		v2Trades = append(v2Trades, &types.SyntheticTrade{
			MarketID:     trade.MarketID,
			SubaccountID: trade.SubaccountID,
			IsBuy:        trade.IsBuy,
			Quantity:     trade.Quantity,
			Price:        derivativeMarket.PriceFromChainFormat(trade.Price),
			Margin:       derivativeMarket.NotionalFromChainFormat(trade.Margin),
		})
	}

	return v2Trades, nil
}

type capState struct {
	openNotionalCap   v2.OpenNotionalCap
	currOpenNotional  math.LegacyDec
	addedOpenNotional math.LegacyDec
	openInterestDelta math.LegacyDec
	posQty            map[common.Hash]math.LegacyDec
}

func (cs *capState) initSignedQty(subID common.Hash, pos *v2.Position) {
	if pos == nil || pos.Quantity.IsZero() {
		cs.posQty[subID] = math.LegacyZeroDec()
		return
	}

	if pos.IsLong {
		qty := pos.Quantity
		cs.posQty[subID] = qty
		return
	}

	neg := pos.Quantity.Neg()
	cs.posQty[subID] = neg
}

func (k WasmKeeper) HandlePositionTransferAction(
	ctx sdk.Context,
	contractAddress sdk.AccAddress,
	origin sdk.AccAddress,
	action *types.PositionTransfer,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	m := k.derivative.GetDerivativeMarketInfo(ctx, action.MarketID, true)

	var (
		market    = m.Market
		markPrice = m.MarkPrice
		funding   = m.Funding
	)

	if err := ensureActiveDerivativeMarket(market, markPrice, action.MarketID); err != nil {
		return err
	}

	sourcePosition := k.GetPosition(ctx, action.MarketID, action.SourceSubaccountID)
	destinationPosition := k.GetPosition(ctx, action.MarketID, action.DestinationSubaccountID)

	destinationPosition, err := preparePositionTransfer(
		contractAddress,
		origin,
		action,
		sourcePosition,
		destinationPosition,
		funding,
		market,
		markPrice,
	)
	if err != nil {
		return err
	}

	oiDelta := calcPositionTransferOpenInterestDelta(destinationPosition, sourcePosition, action.Quantity)

	executionPrice := sourcePosition.EntryPrice
	sourceMarginBefore := sourcePosition.Margin

	isSourceLongBefore := DirLong
	if !sourcePosition.IsLong {
		isSourceLongBefore = DirShort
	}
	isDestinationLongBefore := DirLong
	if !destinationPosition.IsLong {
		isDestinationLongBefore = DirShort
	}

	// Ignore payouts when applying position delta in source position, because margin + PNL is accounted for in destination position
	payout, closeExecutionMargin := applyPositionTransferDeltas(sourcePosition, destinationPosition, action.Quantity, executionPrice, sourceMarginBefore)

	receiverTradingFee := markPrice.Mul(action.Quantity).Mul(market.TakerFeeRate)

	if err := k.applyPositionTransferMarketBalanceDelta(ctx, action.MarketID, market, payout, closeExecutionMargin); err != nil {
		return err
	}

	k.derivative.SavePosition(ctx, action.MarketID, action.SourceSubaccountID, sourcePosition)
	k.derivative.SavePosition(ctx, action.MarketID, action.DestinationSubaccountID, destinationPosition)

	k.applyPositionTransferDeposits(ctx, action, market, payout, closeExecutionMargin, receiverTradingFee)

	k.applyOpenInterestDeltaIfNeeded(ctx, action.MarketID, oiDelta)

	k.checkAndResolveReduceOnlyConflicts(ctx, action.MarketID, action.SourceSubaccountID, sourcePosition, !sourcePosition.IsLong)

	k.resolvePositionTransferDestinationReduceOnlyEffects(
		ctx,
		action,
		destinationPosition,
		isSourceLongBefore,
		isDestinationLongBefore,
	)

	events.Emit(ctx, k.BaseKeeper, &v2.EventPositionTransfer{
		MarketId:                action.MarketID.Hex(),
		SourceSubaccountId:      action.SourceSubaccountID.Hex(),
		DestinationSubaccountId: action.DestinationSubaccountID.Hex(),
		Quantity:                action.Quantity,
	})

	return nil
}

type Direction uint8

const (
	DirLong Direction = iota
	DirShort
)

func (k WasmKeeper) resolvePositionTransferDestinationReduceOnlyEffects(
	ctx sdk.Context,
	action *types.PositionTransfer,
	destinationPosition *v2.Position,
	sourceDirBefore Direction,
	destDirBefore Direction,
) {
	if sourceDirBefore == destDirBefore {
		return
	}

	destWasLong := destDirBefore == DirLong

	// if destination position flipped or is closed, cancel all RO orders
	if destWasLong != destinationPosition.IsLong || destinationPosition.Quantity.IsZero() {
		metadata := k.GetSubaccountOrderbookMetadata(ctx, action.MarketID, action.DestinationSubaccountID, !destWasLong)
		k.cancelAllReduceOnlyOrders(ctx, action.MarketID, action.DestinationSubaccountID, metadata, !destWasLong)
		return
	}

	// partial closing case
	k.checkAndResolveReduceOnlyConflicts(ctx, action.MarketID, action.DestinationSubaccountID, destinationPosition, !destinationPosition.IsLong)
}

func (k WasmKeeper) applyOpenInterestDeltaIfNeeded(ctx sdk.Context, marketID common.Hash, oiDelta math.LegacyDec) {
	if oiDelta.IsZero() {
		return
	}
	k.ApplyOpenInterestDeltaForMarket(ctx, marketID, oiDelta)
}

func (k WasmKeeper) checkAndResolveReduceOnlyConflicts(
	ctx sdk.Context,
	marketID common.Hash,
	subaccountID common.Hash,
	position *v2.Position,
	isReduceOnlyDirectionBuy bool,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	metadata := k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, isReduceOnlyDirectionBuy)

	if metadata.ReduceOnlyLimitOrderCount == 0 {
		return
	}

	if position.Quantity.IsZero() {
		k.cancelAllReduceOnlyOrders(ctx, marketID, subaccountID, metadata, isReduceOnlyDirectionBuy)
		return
	}

	cumulativeOrderSideQuantity := metadata.AggregateReduceOnlyQuantity.Add(metadata.AggregateVanillaQuantity)

	maxRoQuantityToCancel := cumulativeOrderSideQuantity.Sub(position.Quantity)
	if maxRoQuantityToCancel.IsNegative() || maxRoQuantityToCancel.IsZero() {
		return
	}

	subaccountEOBResults := v2.NewSubaccountOrderResults()
	k.derivative.CancelMinimumReduceOnlyOrders(
		ctx,
		marketID,
		subaccountID,
		metadata,
		isReduceOnlyDirectionBuy,
		position.Quantity,
		subaccountEOBResults,
		nil,
	)
}

func (k WasmKeeper) cancelAllReduceOnlyOrders(
	ctx sdk.Context,
	marketID,
	subaccountID common.Hash,
	metadata *v2.SubaccountOrderbookMetadata,
	isBuy bool,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	if metadata.ReduceOnlyLimitOrderCount == 0 {
		return
	}

	orders, totalQuantity := k.subaccount.GetWorstReduceOnlySubaccountOrdersUpToCount(
		ctx,
		marketID,
		subaccountID,
		isBuy,
		&metadata.ReduceOnlyLimitOrderCount,
	)

	k.derivative.CancelReduceOnlyOrders(ctx, marketID, subaccountID, metadata, isBuy, totalQuantity, orders)
}

func (k WasmKeeper) HandleSyntheticTradeAction(
	ctx sdk.Context,
	contractAddress sdk.AccAddress,
	origin sdk.AccAddress,
	action *types.SyntheticTradeAction,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	summary, err := action.Summarize()
	if err != nil {
		return err
	}

	// Enforce that subaccountIDs provided match either the contract address or the origin address
	if err := ensureSyntheticTradeParties(contractAddress, origin, summary.ContractAddress, summary.UserAddress); err != nil {
		return err
	}

	return k.processSyntheticTradeAction(ctx, contractAddress, summary.GetMarketIDs(), action)
}

type syntheticEventKey struct {
	marketID string
	isBuy    bool
}

type syntheticEventData struct {
	cumulativeFunding *math.LegacyDec
	trades            []*v2.DerivativeTradeLog
}

func (k WasmKeeper) processSyntheticTradeAction(
	ctx sdk.Context,
	contractAddress sdk.AccAddress,
	marketIDs []common.Hash,
	action *types.SyntheticTradeAction,
) error {
	totalMarginAndFees := make(map[string]math.LegacyDec)
	totalFees := make(map[string]math.LegacyDec)
	markets := make(map[common.Hash]*v2.DerivativeMarketInfo)
	caps := make(map[common.Hash]*capState)

	if err := k.initSyntheticTradeState(ctx, marketIDs, markets, totalMarginAndFees, totalFees, caps); err != nil {
		return err
	}

	initialPositions := v2.NewModifiedPositionCache()
	finalPositions := v2.NewModifiedPositionCache()

	trades := append(append([]*types.SyntheticTrade{}, action.UserTrades...), action.ContractTrades...)

	eventGroups := make(map[syntheticEventKey]*syntheticEventData)

	for _, trade := range trades {
		result, err := k.applySyntheticTrade(
			ctx,
			markets,
			caps,
			initialPositions,
			finalPositions,
			totalMarginAndFees,
			totalFees,
			trade,
		)
		if err != nil {
			return err
		}

		key := syntheticEventKey{marketID: result.marketID, isBuy: result.isBuy}
		if _, exists := eventGroups[key]; !exists {
			eventGroups[key] = &syntheticEventData{
				cumulativeFunding: result.cumulativeFunding,
				trades:            make([]*v2.DerivativeTradeLog, 0),
			}
		}
		eventGroups[key].trades = append(eventGroups[key].trades, result.tradeLog)
	}

	// Transfer funds from the contract to exchange module to pay for the synthetic trades
	coinsToTransfer := buildCoinsToTransfer(totalMarginAndFees)
	if err := k.transferSyntheticTradeFunds(ctx, contractAddress, coinsToTransfer, totalFees); err != nil {
		return err
	}

	for _, marketID := range marketIDs {
		k.resolveSyntheticTradeROConflictsForMarket(ctx, marketID, initialPositions, finalPositions)

		if cs := caps[marketID]; cs != nil && !cs.openInterestDelta.IsZero() {
			k.ApplyOpenInterestDeltaForMarket(ctx, marketID, cs.openInterestDelta)
		}
	}

	keys := make([]syntheticEventKey, 0, len(eventGroups))
	for key := range eventGroups {
		keys = append(keys, key)
	}

	sort.SliceStable(keys, func(i, j int) bool {
		if keys[i].marketID != keys[j].marketID {
			return keys[i].marketID < keys[j].marketID
		}
		return !keys[i].isBuy && keys[j].isBuy
	})

	for _, key := range keys {
		data := eventGroups[key]
		events.Emit(ctx, k.BaseKeeper, &v2.EventBatchDerivativeExecution{
			MarketId:          key.marketID,
			IsBuy:             key.isBuy,
			IsLiquidation:     false,
			ExecutionType:     v2.ExecutionType_Synthetic,
			Trades:            data.trades,
			CumulativeFunding: data.cumulativeFunding,
		})
	}

	return nil
}

func ensureSyntheticTradeParties(
	contractAddress sdk.AccAddress,
	origin sdk.AccAddress,
	contractSubaccountAddress sdk.Address,
	userSubaccountAddress sdk.Address,
) error {
	if !contractAddress.Equals(contractSubaccountAddress) || !origin.Equals(userSubaccountAddress) {
		return errors.Wrapf(
			types.ErrBadSubaccountID,
			"subaccountID address %s does not match either contract address %s or origin address %s",
			userSubaccountAddress.String(),
			contractAddress.String(), origin.String(),
		)
	}
	return nil
}

func ensureActiveDerivativeMarket(market *v2.DerivativeMarket, markPrice math.LegacyDec, marketID common.Hash) error {
	if market == nil || markPrice.IsNil() {
		return errors.Wrapf(types.ErrDerivativeMarketNotFound, "active derivative market for marketID %s not found", marketID.Hex())
	}
	return nil
}

func preparePositionTransfer(
	contractAddress sdk.AccAddress,
	origin sdk.AccAddress,
	action *types.PositionTransfer,
	sourcePosition *v2.Position,
	destinationPosition *v2.Position,
	funding *v2.PerpetualMarketFunding,
	market *v2.DerivativeMarket,
	markPrice math.LegacyDec,
) (*v2.Position, error) {
	if err := ensurePositionTransferParties(contractAddress, origin, action.SourceSubaccountID, action.DestinationSubaccountID); err != nil {
		return nil, err
	}

	// Enforce that source position has sufficient quantity for transfer
	if err := ensurePositionTransferSourceHasSufficientQty(sourcePosition, action.Quantity); err != nil {
		return nil, err
	}

	destinationPosition = initDestinationPositionForTransfer(destinationPosition, sourcePosition, funding)

	if market.IsPerpetual {
		destinationPosition.ApplyFunding(funding)
		sourcePosition.ApplyFunding(funding)
	}

	// Enforce each position's effectiveMargin / (markPrice * quantity) ≥ maintenanceMarginRatio
	if err := ensurePositionAboveMaintenanceMarginRatio(sourcePosition, market, markPrice); err != nil {
		return nil, err
	}
	if err := ensurePositionAboveMaintenanceMarginRatio(destinationPosition, market, markPrice); err != nil {
		return nil, err
	}

	return destinationPosition, nil
}

func ensurePositionTransferParties(
	contractAddress sdk.AccAddress,
	origin sdk.AccAddress,
	sourceSubaccountID common.Hash,
	destinationSubaccountID common.Hash,
) error {
	sourceAddress := types.SubaccountIDToSdkAddress(sourceSubaccountID)
	destinationAddress := types.SubaccountIDToSdkAddress(destinationSubaccountID)

	contractToUser := contractAddress.Equals(sourceAddress) && origin.Equals(destinationAddress)
	userToContract := origin.Equals(sourceAddress) && contractAddress.Equals(destinationAddress)

	if !contractToUser && !userToContract {
		return errors.Wrapf(
			types.ErrBadSubaccountID,
			"Invalid position transfer parties: source %s and destination %s must be a valid pair of contract address %s and origin address %s",
			sourceAddress.String(),
			destinationAddress.String(),
			contractAddress.String(),
			origin.String(),
		)
	}

	return nil
}

func ensurePositionTransferSourceHasSufficientQty(sourcePosition *v2.Position, quantity math.LegacyDec) error {
	if sourcePosition == nil || sourcePosition.Quantity.LT(quantity) {
		return errors.Wrapf(types.ErrInvalidQuantity, "Source subaccountID position quantity")
	}
	return nil
}

func initDestinationPositionForTransfer(
	destinationPosition *v2.Position,
	sourcePosition *v2.Position,
	funding *v2.PerpetualMarketFunding,
) *v2.Position {
	if destinationPosition != nil {
		return destinationPosition
	}

	var cumulativeFundingEntry math.LegacyDec
	if funding != nil {
		cumulativeFundingEntry = funding.CumulativeFunding
	}
	return v2.NewPosition(sourcePosition.IsLong, cumulativeFundingEntry)
}

func ensurePositionAboveMaintenanceMarginRatio(
	position *v2.Position,
	market *v2.DerivativeMarket,
	markPrice math.LegacyDec,
) error {
	if position.Quantity.IsPositive() {
		positionMarginRatio := position.GetEffectiveMarginRatio(markPrice, math.LegacyZeroDec())
		if positionMarginRatio.LT(market.MaintenanceMarginRatio) {
			return errors.Wrapf(
				types.ErrLowPositionMargin,
				"position margin ratio %s ≥ %s must hold", positionMarginRatio.String(), market.MaintenanceMarginRatio.String(),
			)
		}
	}
	return nil
}

func calcPositionTransferOpenInterestDelta(
	destinationPosition *v2.Position,
	sourcePosition *v2.Position,
	quantity math.LegacyDec,
) math.LegacyDec {
	oiDelta := math.LegacyZeroDec()
	if destinationPosition.Quantity.IsPositive() && (destinationPosition.IsLong != sourcePosition.IsLong) {
		minQuantity := math.LegacyMinDec(quantity, destinationPosition.Quantity)
		oiDeltaFromTrade := minQuantity.Mul(math.LegacyNewDec(2))
		oiDelta = oiDelta.Sub(oiDeltaFromTrade)
	}
	return oiDelta
}

func applyPositionTransferDeltas(
	sourcePosition *v2.Position,
	destinationPosition *v2.Position,
	quantity math.LegacyDec,
	executionPrice math.LegacyDec,
	sourceMarginBefore math.LegacyDec,
) (payout, closeExecutionMargin math.LegacyDec) {
	sourcePosition.ApplyPositionDelta(
		&v2.PositionDelta{
			IsLong:            !sourcePosition.IsLong,
			ExecutionQuantity: quantity,
			ExecutionMargin:   math.LegacyZeroDec(),
			ExecutionPrice:    executionPrice,
		},
		math.LegacyZeroDec(),
	)

	executionMargin := sourceMarginBefore.Sub(sourcePosition.Margin)
	payout, closeExecutionMargin, _, _ = destinationPosition.ApplyPositionDelta(
		&v2.PositionDelta{
			IsLong:            sourcePosition.IsLong,
			ExecutionQuantity: quantity,
			ExecutionMargin:   executionMargin,
			ExecutionPrice:    executionPrice,
		},
		math.LegacyZeroDec(),
	)

	return payout, closeExecutionMargin
}

// Special market balance handling for position transfers:
// - `collateralizationMargin` can be ignored because those funds came from the source position
// - `receiverTradingFee` can be ignored because its paid from user balances
// - `closeExecutionMargin` must be accounted for as those funds came from an existing position and are now leaving the market
func (k WasmKeeper) applyPositionTransferMarketBalanceDelta(
	ctx sdk.Context,
	marketID common.Hash,
	market *v2.DerivativeMarket,
	payout, closeExecutionMargin math.LegacyDec,
) error {
	marketBalanceDelta := payout.Add(closeExecutionMargin).Neg()
	chainFormattedMarketBalanceDelta := market.NotionalToChainFormat(marketBalanceDelta)

	availableMarketFunds := k.derivative.GetAvailableMarketFunds(ctx, marketID)
	isMarketSolvent := v2.IsMarketSolvent(availableMarketFunds, chainFormattedMarketBalanceDelta)
	if !isMarketSolvent {
		return types.ErrInsufficientMarketBalance
	}

	k.derivative.ApplyMarketBalanceDelta(ctx, marketID, chainFormattedMarketBalanceDelta)
	return nil
}

func (k WasmKeeper) applyPositionTransferDeposits(
	ctx sdk.Context,
	action *types.PositionTransfer,
	market *v2.DerivativeMarket,
	payout, closeExecutionMargin, receiverTradingFee math.LegacyDec,
) {
	chainFormattedDepositDeltaAmount := market.NotionalToChainFormat(payout.Add(closeExecutionMargin).Sub(receiverTradingFee))
	chainFormattedReceiverTradingFee := market.NotionalToChainFormat(receiverTradingFee)

	depositDelta := types.NewUniformDepositDelta(chainFormattedDepositDeltaAmount)
	k.subaccount.UpdateDepositWithDelta(ctx, action.DestinationSubaccountID, market.QuoteDenom, depositDelta)
	k.subaccount.UpdateDepositWithDelta(
		ctx,
		types.AuctionSubaccountID,
		market.QuoteDenom,
		types.NewUniformDepositDelta(chainFormattedReceiverTradingFee),
	)
}

func (k WasmKeeper) initSyntheticTradeState(
	ctx sdk.Context,
	marketIDs []common.Hash,
	markets map[common.Hash]*v2.DerivativeMarketInfo,
	totalMarginAndFees map[string]math.LegacyDec,
	totalFees map[string]math.LegacyDec,
	caps map[common.Hash]*capState,
) error {
	for _, marketID := range marketIDs {
		m := k.derivative.GetDerivativeMarketInfo(ctx, marketID, true)
		if m.Market == nil || m.MarkPrice.IsNil() {
			return errors.Wrapf(types.ErrDerivativeMarketNotFound, "active derivative market for marketID %s not found", marketID.Hex())
		}

		markets[marketID] = m
		totalMarginAndFees[m.Market.QuoteDenom] = math.LegacyZeroDec()
		totalFees[m.Market.QuoteDenom] = math.LegacyZeroDec()

		caps[marketID] = &capState{
			openNotionalCap:   m.Market.GetOpenNotionalCap(),
			currOpenNotional:  k.derivative.GetOpenNotionalForMarket(ctx, marketID, m.MarkPrice),
			addedOpenNotional: math.LegacyZeroDec(),
			openInterestDelta: math.LegacyZeroDec(),
			posQty:            make(map[common.Hash]math.LegacyDec),
		}
	}
	return nil
}

type syntheticTradeResult struct {
	marketID          string
	isBuy             bool
	cumulativeFunding *math.LegacyDec
	tradeLog          *v2.DerivativeTradeLog
}

func (k WasmKeeper) applySyntheticTrade(
	ctx sdk.Context,
	markets map[common.Hash]*v2.DerivativeMarketInfo,
	caps map[common.Hash]*capState,
	initialPositions v2.ModifiedPositionCache,
	finalPositions v2.ModifiedPositionCache,
	totalMarginAndFees map[string]math.LegacyDec,
	totalFees map[string]math.LegacyDec,
	trade *types.SyntheticTrade,
) (*syntheticTradeResult, error) {
	m := markets[trade.MarketID]
	market := m.Market
	markPrice := m.MarkPrice

	var fundingInfo *v2.PerpetualMarketFunding
	if market.IsPerpetual {
		fundingInfo = m.Funding
	}

	// Initialize position and apply funding
	position := k.GetPosition(ctx, trade.MarketID, trade.SubaccountID)
	position = initSyntheticTradePosition(position, trade.IsBuy, fundingInfo)

	cs := caps[trade.MarketID]
	cs.initSignedQty(trade.SubaccountID, position)

	recordInitialPositionIfNeeded(initialPositions, trade.MarketID, trade.SubaccountID, position)

	orderType := v2.OrderType_SELL
	if trade.IsBuy {
		orderType = v2.OrderType_BUY
	}

	if err := ensureNotionalCapNotBreached(orderType, trade, markPrice, cs); err != nil {
		return nil, err
	}

	tradingFee := trade.Quantity.Mul(markPrice).Mul(market.TakerFeeRate)

	isClosingPosition := trade.IsBuy != position.IsLong && !position.Quantity.IsZero()
	if isClosingPosition {
		closingPrice := trade.Price
		if err := k.ensurePositionAboveBankruptcyForClosing(position, market, closingPrice, tradingFee); err != nil {
			return nil, err
		}
	}

	isInvalidReduceOnly := trade.IsReduceOnly() && (!isClosingPosition || position.Quantity.LT(trade.Quantity))
	if isInvalidReduceOnly {
		return nil, errors.Wrapf(
			types.ErrInsufficientPositionQuantity,
			"invalid reduce-only synthetic trade (position quantity: %s, trade quantity: %s)",
			position.Quantity.String(), trade.Quantity.String(),
		)
	}

	positionDelta := &v2.PositionDelta{
		IsLong:            trade.IsBuy,
		ExecutionQuantity: trade.Quantity,
		ExecutionMargin:   trade.Margin,
		ExecutionPrice:    trade.Price,
	}
	payout, closeExecutionMargin, collateralizationMargin, pnl := position.ApplyPositionDelta(positionDelta, tradingFee)

	if err := ensureSyntheticTradePositionPostDelta(position, market, markPrice); err != nil {
		return nil, err
	}

	updateCapsAfterTrade(cs, orderType, trade.Quantity, markPrice, trade.SubaccountID)

	if err := k.ensureAndApplySyntheticTradeMarketBalanceDelta(ctx, trade, market, payout, collateralizationMargin, tradingFee); err != nil {
		return nil, err
	}

	finalPositions.SetPosition(trade.MarketID, trade.SubaccountID, position)
	k.derivative.SavePosition(ctx, trade.MarketID, trade.SubaccountID, position)

	chainFormattedDepositDeltaAmount := market.NotionalToChainFormat(payout.Add(closeExecutionMargin))
	depositDelta := types.NewUniformDepositDelta(chainFormattedDepositDeltaAmount)
	k.subaccount.UpdateDepositWithDelta(ctx, trade.SubaccountID, market.QuoteDenom, depositDelta)

	// defensive programming
	if k.GetDeposit(ctx, trade.SubaccountID, market.QuoteDenom).IsNegative() {
		return nil, errors.Wrapf(
			types.ErrInsufficientDeposit,
			"subaccountID %s has insufficient deposit for market quote denom %s",
			types.SubaccountIDToSdkAddress(trade.SubaccountID).String(),
			market.QuoteDenom,
		)
	}

	chainFormattedFee := market.NotionalToChainFormat(tradingFee)
	totalFees[market.QuoteDenom] = totalFees[market.QuoteDenom].Add(chainFormattedFee)

	totalTransferredFunds := trade.Margin

	// reduce-only trades already pay fees via margin, so we don't double count them here
	if !trade.IsReduceOnly() {
		totalTransferredFunds = totalTransferredFunds.Add(tradingFee)
	}

	chainFormattedMarginAndFee := market.NotionalToChainFormat(totalTransferredFunds)
	totalMarginAndFees[market.QuoteDenom] = totalMarginAndFees[market.QuoteDenom].Add(chainFormattedMarginAndFee)

	tradeLog := &v2.DerivativeTradeLog{
		SubaccountId:        trade.SubaccountID.Bytes(),
		PositionDelta:       positionDelta,
		Payout:              payout,
		Fee:                 tradingFee,
		Pnl:                 pnl,
		OrderHash:           common.Hash{}.Bytes(),
		FeeRecipientAddress: common.Address{}.Bytes(),
	}

	var cumulativeFunding *math.LegacyDec
	if fundingInfo != nil {
		cf := fundingInfo.CumulativeFunding
		cumulativeFunding = &cf
	}

	return &syntheticTradeResult{
		marketID:          market.MarketId,
		isBuy:             trade.IsBuy,
		cumulativeFunding: cumulativeFunding,
		tradeLog:          tradeLog,
	}, nil
}

func initSyntheticTradePosition(
	position *v2.Position,
	isBuy bool,
	fundingInfo *v2.PerpetualMarketFunding,
) *v2.Position {
	if position == nil {
		var cumulativeFundingEntry math.LegacyDec
		if fundingInfo != nil {
			cumulativeFundingEntry = fundingInfo.CumulativeFunding
		}
		return v2.NewPosition(isBuy, cumulativeFundingEntry)
	}

	if fundingInfo != nil {
		position.ApplyFunding(fundingInfo)
	}
	return position
}

func recordInitialPositionIfNeeded(
	initialPositions v2.ModifiedPositionCache,
	marketID common.Hash,
	subaccountID common.Hash,
	position *v2.Position,
) {
	if initialPositions.HasPositionBeenModified(marketID, subaccountID) {
		return
	}
	initialPositions.SetPosition(marketID, subaccountID, &v2.Position{
		IsLong:                 position.IsLong,
		Quantity:               position.Quantity,
		EntryPrice:             position.EntryPrice,
		Margin:                 position.Margin,
		CumulativeFundingEntry: position.CumulativeFundingEntry,
	})
}

func ensureNotionalCapNotBreached(
	orderType v2.OrderType,
	trade *types.SyntheticTrade,
	markPrice math.LegacyDec,
	cs *capState,
) error {
	breach, _ := derivative.DoesBreachOpenNotionalCap(
		orderType,
		trade.Quantity,
		markPrice,
		cs.currOpenNotional.Add(cs.addedOpenNotional),
		cs.posQty[trade.SubaccountID],
		cs.openNotionalCap,
	)
	if breach {
		return errors.Wrapf(types.ErrOpenNotionalCapBreached, "market %s: cap breached", trade.MarketID.Hex())
	}
	return nil
}

func ensureSyntheticTradePositionPostDelta(
	position *v2.Position,
	market *v2.DerivativeMarket,
	markPrice math.LegacyDec,
) error {
	if position.Quantity.IsNegative() {
		return types.ErrNegativePositionQuantity
	}
	return ensurePositionAboveInitialMarginRatio(position, market, markPrice)
}

func updateCapsAfterTrade(
	cs *capState,
	orderType v2.OrderType,
	quantity math.LegacyDec,
	markPrice math.LegacyDec,
	subaccountID common.Hash,
) {
	notionalDelta, qtyDelta, newPositionQuantity := derivative.GetValuesForNotionalCapChecks(
		orderType,
		quantity,
		markPrice,
		cs.posQty[subaccountID],
	)
	cs.openInterestDelta = cs.openInterestDelta.Add(qtyDelta)
	cs.posQty[subaccountID] = newPositionQuantity
	cs.addedOpenNotional = cs.addedOpenNotional.Add(notionalDelta)
}

func (k WasmKeeper) ensureAndApplySyntheticTradeMarketBalanceDelta(
	ctx sdk.Context,
	trade *types.SyntheticTrade,
	market *v2.DerivativeMarket,
	payout math.LegacyDec,
	collateralizationMargin math.LegacyDec,
	tradingFee math.LegacyDec,
) error {
	marketBalanceDelta := v2.GetMarketBalanceDelta(payout, collateralizationMargin, tradingFee, trade.Margin.IsZero())
	chainFormattedMarketBalanceDelta := market.NotionalToChainFormat(marketBalanceDelta)
	availableMarketFunds := k.derivative.GetAvailableMarketFunds(ctx, trade.MarketID)

	isMarketSolvent := v2.IsMarketSolvent(availableMarketFunds, chainFormattedMarketBalanceDelta)
	if !isMarketSolvent {
		return types.ErrInsufficientMarketBalance
	}

	k.derivative.ApplyMarketBalanceDelta(ctx, trade.MarketID, chainFormattedMarketBalanceDelta)
	return nil
}

func buildCoinsToTransfer(totalMarginAndFees map[string]math.LegacyDec) sdk.Coins {
	coinsToTransfer := sdk.Coins{}
	for denom, fundsUsed := range totalMarginAndFees {
		fundsUsedCoin := sdk.NewCoin(denom, fundsUsed.Ceil().TruncateInt())
		if !fundsUsedCoin.IsPositive() {
			continue
		}
		coinsToTransfer = coinsToTransfer.Add(fundsUsedCoin)
	}
	return coinsToTransfer
}

func (k WasmKeeper) transferSyntheticTradeFunds(
	ctx sdk.Context,
	contractAddress sdk.AccAddress,
	coinsToTransfer sdk.Coins,
	totalFees map[string]math.LegacyDec,
) error {
	if coinsToTransfer.IsZero() {
		return nil
	}

	if err := k.bank.SendCoinsFromAccountToModule(ctx, contractAddress, types.ModuleName, coinsToTransfer); err != nil {
		return errors.Wrap(err, "failed SyntheticTradeAction")
	}

	sortedDenomKeys := GetSortedFeesKeys(totalFees)
	for _, denom := range sortedDenomKeys {
		k.subaccount.UpdateDepositWithDelta(ctx, types.AuctionSubaccountID, denom, types.NewUniformDepositDelta(totalFees[denom]))
	}

	return nil
}

func (WasmKeeper) ensurePositionAboveBankruptcyForClosing(
	position *v2.Position,
	market *v2.DerivativeMarket,
	closingPrice, closingFee math.LegacyDec,
) error {
	if !position.Quantity.IsPositive() {
		return nil
	}

	positionMarginRatio := position.GetEffectiveMarginRatio(closingPrice, closingFee)
	bankruptcyMarginRatio := math.LegacyZeroDec()

	if positionMarginRatio.LT(bankruptcyMarginRatio) {
		return errors.Wrapf(
			types.ErrLowPositionMargin,
			"position margin ratio %s ≥ %s must hold", positionMarginRatio.String(), market.InitialMarginRatio.String(),
		)
	}

	return nil
}

func ensurePositionAboveInitialMarginRatio(
	position *v2.Position,
	market *v2.DerivativeMarket,
	markPrice math.LegacyDec,
) error {
	if !position.Quantity.IsPositive() {
		return nil
	}

	positionMarginRatio := position.GetEffectiveMarginRatio(markPrice, math.LegacyZeroDec())

	if positionMarginRatio.LT(market.InitialMarginRatio) {
		return errors.Wrapf(
			types.ErrLowPositionMargin,
			"position margin ratio %s ≥ %s must hold", positionMarginRatio.String(), market.InitialMarginRatio.String(),
		)
	}

	return nil
}

func GetSortedFeesKeys(p map[string]math.LegacyDec) []string {
	denoms := make([]string, 0)
	for k := range p {
		denoms = append(denoms, k)
	}
	sort.SliceStable(denoms, func(i, j int) bool {
		return denoms[i] < denoms[j]
	})
	return denoms
}

func (k WasmKeeper) resolveSyntheticTradeROConflictsForMarket(
	ctx sdk.Context,
	marketID common.Hash,
	initialPositions,
	finalPositions v2.ModifiedPositionCache,
) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	subaccountIDs := initialPositions.GetSortedSubaccountIDsByMarket(marketID)

	for _, subaccountID := range subaccountIDs {
		initialPosition := initialPositions.GetPosition(marketID, subaccountID)
		finalPosition := finalPositions.GetPosition(marketID, subaccountID)

		hasNoPossibleContentions := initialPosition.IsLong == finalPosition.IsLong && finalPosition.Quantity.GTE(initialPosition.Quantity)
		if hasNoPossibleContentions {
			continue
		}

		metadata := k.GetSubaccountOrderbookMetadata(ctx, marketID, subaccountID, !initialPosition.IsLong)
		if initialPosition.IsLong != finalPosition.IsLong || finalPosition.Quantity.IsZero() {
			k.cancelAllReduceOnlyOrders(ctx, marketID, subaccountID, metadata, !initialPosition.IsLong)
			continue
		}

		// partial closing case
		k.checkAndResolveReduceOnlyConflicts(ctx, marketID, subaccountID, finalPosition, !finalPosition.IsLong)
	}
}

func (k WasmKeeper) calculateFundsDifference(ctx sdk.Context, sender sdk.AccAddress, fundsBefore sdk.Coins) sdk.Coins {
	fundsAfter := sdk.Coins(make([]sdk.Coin, 0, len(fundsBefore)))

	for _, coin := range fundsBefore {
		coinAfter := k.bank.GetBalance(ctx, sender, coin.Denom)
		fundsAfter = fundsAfter.Add(coinAfter)
	}

	fundsDiff, _ := fundsAfter.SafeSub(fundsBefore...)

	return filterNonPositiveCoins(fundsDiff)
}

func filterNonPositiveCoins(coins sdk.Coins) sdk.Coins {
	var filteredCoins sdk.Coins
	for _, coin := range coins {
		if coin.IsPositive() {
			filteredCoins = append(filteredCoins, coin)
		}
	}
	return filteredCoins
}

func (k WasmKeeper) QueryMarketID(ctx sdk.Context, contractAddress string) (common.Hash, error) {
	type getMarketIDQuery struct {
	}

	type queryDataStruct struct {
		Data getMarketIDQuery `json:"get_market_id"`
	}

	type baseMsgWrapper struct {
		Base queryDataStruct `json:"base"`
	}

	queryData := baseMsgWrapper{
		queryDataStruct{
			Data: getMarketIDQuery{},
		},
	}

	queryDataBz, err := json.Marshal(queryData)
	if err != nil {
		return common.Hash{}, err
	}

	contractAddressAcc := sdk.MustAccAddressFromBech32(contractAddress)
	bz, err := k.wasmv.QuerySmart(ctx, contractAddressAcc, queryDataBz)
	if err != nil {
		return common.Hash{}, err
	}

	type Data struct {
		MarketId string `json:"market_id"`
	}

	var result Data
	if err := json.Unmarshal(bz, &result); err != nil {
		return common.Hash{}, err
	}

	return common.HexToHash(result.MarketId), nil
}
