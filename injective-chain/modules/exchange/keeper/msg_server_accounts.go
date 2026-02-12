package keeper

import (
	"context"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/InjectiveLabs/metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

type AccountsMsgServer struct {
	*Keeper
	svcTags metrics.Tags
}

// AccountsMsgServerImpl returns an implementation of the bank MsgServer interface for the provided Keeper for account functions.
func AccountsMsgServerImpl(keeper *Keeper) AccountsMsgServer {
	return AccountsMsgServer{
		Keeper: keeper,
		svcTags: metrics.Tags{
			"svc": "acc_msg_h",
		},
	}
}

func (k AccountsMsgServer) Deposit(
	c context.Context,
	msg *v2.MsgDeposit,
) (*v2.MsgDepositResponse, error) {
	c, doneFn := metrics.ReportFuncCallAndTimingCtx(c, k.svcTags)
	defer doneFn()

	ctx := sdk.UnwrapSDKContext(c)
	if k.IsFixedGasEnabled() {
		ctx.GasMeter().ConsumeGas(DetermineGas(msg), "MsgDeposit")
		ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	}

	if err := k.ExecuteDeposit(ctx, msg); err != nil {
		return nil, err
	}

	return &v2.MsgDepositResponse{}, nil
}

func (k AccountsMsgServer) Withdraw(
	c context.Context,
	msg *v2.MsgWithdraw,
) (*v2.MsgWithdrawResponse, error) {
	c, doneFn := metrics.ReportFuncCallAndTimingCtx(c, k.svcTags)
	defer doneFn()

	ctx := sdk.UnwrapSDKContext(c)
	if k.IsFixedGasEnabled() {
		ctx.GasMeter().ConsumeGas(DetermineGas(msg), "MsgWithdraw")
		ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	}

	if err := k.ExecuteWithdraw(ctx, msg); err != nil {
		return nil, err
	}

	return &v2.MsgWithdrawResponse{}, nil
}

func (k AccountsMsgServer) SubaccountTransfer(
	goCtx context.Context,
	msg *v2.MsgSubaccountTransfer,
) (*v2.MsgSubaccountTransferResponse, error) {
	goCtx, doneFn := metrics.ReportFuncCallAndTimingCtx(goCtx, k.svcTags)
	defer doneFn()

	var (
		denom           = msg.Amount.Denom
		amount          = msg.Amount.Amount.ToLegacyDec()
		sender          = sdk.MustAccAddressFromBech32(msg.Sender)
		srcSubaccountID = types.MustGetSubaccountIDOrDeriveFromNonce(sender, msg.SourceSubaccountId)
		dstSubaccountID = types.MustGetSubaccountIDOrDeriveFromNonce(sender, msg.DestinationSubaccountId)
	)

	ctx := sdk.UnwrapSDKContext(goCtx)
	if k.IsFixedGasEnabled() {
		ctx.GasMeter().ConsumeGas(DetermineGas(msg), "MsgSubaccountTransfer")
		ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	}

	if err := k.Keeper.DecrementDeposit(ctx, srcSubaccountID, denom, amount); err != nil {
		metrics.ReportFuncError(k.svcTags)
		return nil, err
	}

	if err := k.Keeper.IncrementDepositForNonDefaultSubaccount(ctx, dstSubaccountID, denom, amount); err != nil {
		return nil, err
	}

	k.EmitEvent(ctx, &v2.EventSubaccountBalanceTransfer{
		SrcSubaccountId: srcSubaccountID.Hex(),
		DstSubaccountId: dstSubaccountID.Hex(),
		Amount:          msg.Amount,
	})

	return &v2.MsgSubaccountTransferResponse{}, nil
}

func (k AccountsMsgServer) ExternalTransfer(
	goCtx context.Context,
	msg *v2.MsgExternalTransfer,
) (*v2.MsgExternalTransferResponse, error) {
	goCtx, doneFn := metrics.ReportFuncCallAndTimingCtx(goCtx, k.svcTags)
	defer doneFn()

	var (
		denom           = msg.Amount.Denom
		amount          = msg.Amount.Amount.ToLegacyDec()
		sender          = sdk.MustAccAddressFromBech32(msg.Sender)
		srcSubaccountID = types.MustGetSubaccountIDOrDeriveFromNonce(sender, msg.SourceSubaccountId)
		dstSubaccountID = common.HexToHash(msg.DestinationSubaccountId)
		recipientAddr   = types.SubaccountIDToSdkAddress(dstSubaccountID)
	)

	ctx := sdk.UnwrapSDKContext(goCtx)
	if k.IsFixedGasEnabled() {
		ctx.GasMeter().ConsumeGas(DetermineGas(msg), "MsgExternalTransfer")
		ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	}

	// disable subaccount transfers from and to permissioned addresses
	if k.permissionsKeeper.IsEnforcedRestrictionsDenom(ctx, denom) {
		if _, err := k.permissionsKeeper.SendRestrictionFn(ctx, sender, recipientAddr, msg.Amount); err != nil {
			return nil, errors.Wrapf(err, "can't transfer deposit %s from %s to %s", msg.Amount, sender, recipientAddr)
		}
	}

	if err := k.Keeper.DecrementDeposit(ctx, srcSubaccountID, denom, amount); err != nil {
		metrics.ReportFuncError(k.svcTags)
		return nil, err
	}

	// create new account for recipient if it doesn't exist already
	if !k.AccountKeeper.HasAccount(ctx, recipientAddr) {
		defer telemetry.IncrCounter(1, "new", "account")
		k.AccountKeeper.SetAccount(ctx, k.AccountKeeper.NewAccountWithAddress(ctx, recipientAddr))
	}

	if types.IsDefaultSubaccountID(dstSubaccountID) {
		k.IncrementDepositOrSendToBank(ctx, dstSubaccountID, denom, amount)
	} else {
		if err := k.IncrementDepositForNonDefaultSubaccount(ctx, dstSubaccountID, denom, amount); err != nil {
			return nil, err
		}
	}

	k.EmitEvent(ctx, &v2.EventSubaccountBalanceTransfer{
		SrcSubaccountId: srcSubaccountID.Hex(),
		DstSubaccountId: dstSubaccountID.Hex(),
		Amount:          msg.Amount,
	})

	return &v2.MsgExternalTransferResponse{}, nil
}

func (k AccountsMsgServer) RewardsOptOut(
	goCtx context.Context,
	msg *v2.MsgRewardsOptOut,
) (*v2.MsgRewardsOptOutResponse, error) {
	goCtx, doneFn := metrics.ReportFuncCallAndTimingCtx(goCtx, k.svcTags)
	defer doneFn()

	ctx := sdk.UnwrapSDKContext(goCtx)
	account, _ := sdk.AccAddressFromBech32(msg.Sender)
	if isAlreadyOptedOut := k.GetIsOptedOutOfRewards(ctx, account); isAlreadyOptedOut {
		return nil, types.ErrAlreadyOptedOutOfRewards
	}

	k.SetIsOptedOutOfRewards(ctx, account, true)

	return &v2.MsgRewardsOptOutResponse{}, nil
}

func (k AccountsMsgServer) AuthorizeStakeGrants(
	goCtx context.Context,
	msg *v2.MsgAuthorizeStakeGrants,
) (*v2.MsgAuthorizeStakeGrantsResponse, error) {
	goCtx, doneFn := metrics.ReportFuncCallAndTimingCtx(goCtx, k.svcTags)
	defer doneFn()
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	granter := sdk.MustAccAddressFromBech32(msg.Sender)
	granterStake := k.CalculateStakedAmountWithoutCache(ctx, granter, types.MaxGranterDelegations)

	// ensure that the granter has enough stake to cover the grants
	grantAmountDelta := math.ZeroInt()

	// calculate the net change in grant amounts
	for _, grant := range msg.Grants {
		grantee := sdk.MustAccAddressFromBech32(grant.Grantee)
		newAmount := grant.Amount
		oldAmount := k.GetGrantAuthorization(ctx, granter, grantee)
		grantAmountDelta = grantAmountDelta.Add(newAmount).Sub(oldAmount)
	}

	existingTotalGrantAmount := k.GetTotalGrantAmount(ctx, granter)
	newTotalGrantAmount := existingTotalGrantAmount.Add(grantAmountDelta)

	if newTotalGrantAmount.GT(granterStake) {
		return nil, errors.Wrapf(types.ErrInsufficientStake,
			"new total grant amount %s exceeds total stake %s",
			newTotalGrantAmount.String(),
			granterStake.String(),
		)
	}

	// update the last delegation check time
	k.SetLastValidGrantDelegationCheckTime(ctx, msg.Sender, ctx.BlockTime().Unix())

	// process the grants
	for _, grant := range msg.Grants {
		grantee := sdk.MustAccAddressFromBech32(grant.Grantee)
		k.AuthorizeStakeGrant(ctx, granter, grantee, grant.Amount)
	}

	k.EmitEvent(ctx, &v2.EventGrantAuthorizations{
		Granter: granter.String(),
		Grants:  msg.Grants,
	})

	return &v2.MsgAuthorizeStakeGrantsResponse{}, nil
}

func (k AccountsMsgServer) ActivateStakeGrant(
	goCtx context.Context,
	msg *v2.MsgActivateStakeGrant,
) (*v2.MsgActivateStakeGrantResponse, error) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	grantee := sdk.MustAccAddressFromBech32(msg.Sender)
	granter := sdk.MustAccAddressFromBech32(msg.Granter)

	if !k.ExistsGrantAuthorization(ctx, granter, grantee) {
		return nil, errors.Wrapf(types.ErrInvalidStakeGrant, "grant from %s for %s does not exist", granter.String(), grantee.String())
	}

	granterStake := k.CalculateStakedAmountWithoutCache(ctx, granter, types.MaxGranterDelegations)
	totalGrantAmount := k.GetTotalGrantAmount(ctx, granter)

	if totalGrantAmount.GT(granterStake) {
		return nil, errors.Wrapf(
			types.ErrInvalidStakeGrant,
			"grant from %s to %s is invalid since granter staked amount %v is smaller than granter total stake delegated amount %v",
			granter.String(),
			grantee.String(),
			granterStake,
			totalGrantAmount,
		)
	}

	grantAuthorizationAmount := k.GetGrantAuthorization(ctx, granter, grantee)
	k.SetActiveGrant(ctx, grantee, v2.NewActiveGrant(granter, grantAuthorizationAmount))

	return &v2.MsgActivateStakeGrantResponse{}, nil
}
