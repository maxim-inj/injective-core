package keeper

import (
	"context"

	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

type GeneralMsgServer struct {
	*Keeper
	svcTags metrics.Tags
}

func NewGeneralMsgServerImpl(keeper *Keeper) GeneralMsgServer {
	return GeneralMsgServer{
		Keeper: keeper,
		svcTags: metrics.Tags{
			"svc": "general_msg_h",
		},
	}
}

func (k GeneralMsgServer) UpdateParams(c context.Context, msg *v2.MsgUpdateParams) (*v2.MsgUpdateParamsResponse, error) {
	c, doneFn := metrics.ReportFuncCallAndTimingCtx(c, k.svcTags)
	defer doneFn()

	// Check if sender is governance authority
	if !k.IsGovernanceAuthorityAddress(msg.Authority) {
		return nil, govtypes.ErrInvalidSigner.Wrap("sender must be governance authority")
	}

	if err := msg.Params.Validate(); err != nil {
		return nil, err
	}

	k.SetParams(sdk.UnwrapSDKContext(c), msg.Params)

	return &v2.MsgUpdateParamsResponse{}, nil
}

func (k GeneralMsgServer) BatchUpdateOrders(
	goCtx context.Context,
	msg *v2.MsgBatchUpdateOrders,
) (*v2.MsgBatchUpdateOrdersResponse, error) {
	goCtx, doneFn := metrics.ReportFuncCallAndTimingCtx(goCtx, k.svcTags)
	defer doneFn()

	ctx := sdk.UnwrapSDKContext(goCtx)
	if k.IsFixedGasEnabled() {
		return k.FixedGasBatchUpdateOrders(ctx, msg)
	}

	sender, _ := sdk.AccAddressFromBech32(msg.Sender)

	return k.ExecuteBatchUpdateOrders(
		ctx,
		sender,
		msg.SubaccountId,
		msg.SpotMarketIdsToCancelAll,
		msg.DerivativeMarketIdsToCancelAll,
		msg.BinaryOptionsMarketIdsToCancelAll,
		msg.SpotOrdersToCancel,
		msg.DerivativeOrdersToCancel,
		msg.BinaryOptionsOrdersToCancel,
		msg.SpotOrdersToCreate,
		msg.DerivativeOrdersToCreate,
		msg.BinaryOptionsOrdersToCreate,
		msg.SpotMarketOrdersToCreate,
		msg.DerivativeMarketOrdersToCreate,
		msg.BinaryOptionsMarketOrdersToCreate,
	)
}

func (k GeneralMsgServer) BatchExchangeModification(
	goCtx context.Context,
	msg *v2.MsgBatchExchangeModification,
) (*v2.MsgBatchExchangeModificationResponse, error) {
	goCtx, doneFn := metrics.ReportFuncCallAndTimingCtx(goCtx, k.svcTags)
	defer doneFn()

	ctx := sdk.UnwrapSDKContext(goCtx)

	if !k.IsGovernanceAuthorityAddress(msg.Sender) {
		return nil, errortypes.ErrUnauthorized
	}

	for _, proposal := range msg.Proposal.SpotMarketParamUpdateProposals {
		if err := k.HandleSpotMarketParamUpdateProposal(ctx, proposal); err != nil {
			return nil, err
		}
	}

	for _, proposal := range msg.Proposal.DerivativeMarketParamUpdateProposals {
		if err := k.HandleDerivativeMarketParamUpdateProposal(ctx, proposal); err != nil {
			return nil, err
		}
	}

	for _, proposal := range msg.Proposal.SpotMarketLaunchProposals {
		if err := k.HandleSpotMarketLaunchProposal(ctx, proposal); err != nil {
			return nil, err
		}
	}

	for _, proposal := range msg.Proposal.PerpetualMarketLaunchProposals {
		if err := k.HandlePerpetualMarketLaunchProposal(ctx, proposal); err != nil {
			return nil, err
		}
	}

	for _, proposal := range msg.Proposal.ExpiryFuturesMarketLaunchProposals {
		if err := k.HandleExpiryFuturesMarketLaunchProposal(ctx, proposal); err != nil {
			return nil, err
		}
	}

	for _, proposal := range msg.Proposal.BinaryOptionsMarketLaunchProposals {
		if err := k.HandleBinaryOptionsMarketLaunchProposal(ctx, proposal); err != nil {
			return nil, err
		}
	}

	for _, proposal := range msg.Proposal.BinaryOptionsParamUpdateProposals {
		if err := k.HandleBinaryOptionsMarketParamUpdateProposal(ctx, proposal); err != nil {
			return nil, err
		}
	}

	if msg.Proposal.AuctionExchangeTransferDenomDecimalsUpdateProposal != nil {
		if err := k.HandleUpdateAuctionExchangeTransferDenomDecimalsProposal(
			ctx,
			msg.Proposal.AuctionExchangeTransferDenomDecimalsUpdateProposal,
		); err != nil {
			return nil, err
		}
	}

	if msg.Proposal.TradingRewardCampaignUpdateProposal != nil {
		if err := k.HandleTradingRewardCampaignUpdateProposal(
			ctx,
			msg.Proposal.TradingRewardCampaignUpdateProposal,
		); err != nil {
			return nil, err
		}
	}

	if msg.Proposal.FeeDiscountProposal != nil {
		if err := k.HandleFeeDiscountProposal(ctx, msg.Proposal.FeeDiscountProposal); err != nil {
			return nil, err
		}
	}

	for _, proposal := range msg.Proposal.MarketForcedSettlementProposals {
		if err := k.HandleMarketForcedSettlementProposal(ctx, proposal); err != nil {
			return nil, err
		}
	}

	if msg.Proposal.DenomMinNotionalProposal != nil {
		k.HandleDenomMinNotionalProposal(ctx, msg.Proposal.DenomMinNotionalProposal)
	}

	return &v2.MsgBatchExchangeModificationResponse{}, nil
}

func (k GeneralMsgServer) BatchSpendCommunityPool(
	goCtx context.Context, msg *v2.MsgBatchCommunityPoolSpend,
) (*v2.MsgBatchCommunityPoolSpendResponse, error) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	ctx := sdk.UnwrapSDKContext(goCtx)

	if !k.IsGovernanceAuthorityAddress(msg.Sender) {
		return nil, errortypes.ErrUnauthorized
	}

	if err := k.HandleBatchCommunityPoolSpendProposal(ctx, msg.Proposal); err != nil {
		return nil, err
	}

	return &v2.MsgBatchCommunityPoolSpendResponse{}, nil
}

func (k GeneralMsgServer) ForceSettleMarket(
	goCtx context.Context, msg *v2.MsgMarketForcedSettlement,
) (*v2.MsgMarketForcedSettlementResponse, error) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	ctx := sdk.UnwrapSDKContext(goCtx)

	if k.IsAdmin(ctx, msg.Sender) {
		if err := msg.Proposal.ValidateBasic(); err != nil {
			return &v2.MsgMarketForcedSettlementResponse{}, err
		}

		marketID := common.HexToHash(msg.Proposal.MarketId)
		if err := k.HandleForceSettleMarketByAdmin(ctx, marketID, msg.Proposal.SettlementPrice); err != nil {
			return nil, err
		}

		return &v2.MsgMarketForcedSettlementResponse{}, nil
	}

	if !k.IsGovernanceAuthorityAddress(msg.Sender) {
		return nil, errortypes.ErrUnauthorized
	}

	if err := k.HandleMarketForcedSettlementProposal(ctx, msg.Proposal); err != nil {
		return nil, err
	}

	return &v2.MsgMarketForcedSettlementResponse{}, nil
}

func (k GeneralMsgServer) LaunchTradingRewardCampaign(
	goCtx context.Context, msg *v2.MsgTradingRewardCampaignLaunch,
) (*v2.MsgTradingRewardCampaignLaunchResponse, error) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	ctx := sdk.UnwrapSDKContext(goCtx)

	if !k.IsGovernanceAuthorityAddress(msg.Sender) {
		return nil, errortypes.ErrUnauthorized
	}

	if err := k.HandleTradingRewardCampaignLaunchProposal(ctx, msg.Proposal); err != nil {
		return nil, err
	}

	return &v2.MsgTradingRewardCampaignLaunchResponse{}, nil
}

func (k GeneralMsgServer) UpdateTradingRewardCampaign(
	goCtx context.Context, msg *v2.MsgTradingRewardCampaignUpdate,
) (*v2.MsgTradingRewardCampaignUpdateResponse, error) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	ctx := sdk.UnwrapSDKContext(goCtx)

	if !k.IsGovernanceAuthorityAddress(msg.Sender) {
		return nil, errortypes.ErrUnauthorized
	}

	if err := k.HandleTradingRewardCampaignUpdateProposal(ctx, msg.Proposal); err != nil {
		return nil, err
	}

	return &v2.MsgTradingRewardCampaignUpdateResponse{}, nil
}

func (k GeneralMsgServer) EnableExchange(goCtx context.Context, msg *v2.MsgExchangeEnable) (*v2.MsgExchangeEnableResponse, error) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	ctx := sdk.UnwrapSDKContext(goCtx)

	if !k.IsGovernanceAuthorityAddress(msg.Sender) {
		return nil, errortypes.ErrUnauthorized
	}

	if err := k.HandleExchangeEnableProposal(ctx, msg.Proposal); err != nil {
		return nil, err
	}

	return &v2.MsgExchangeEnableResponse{}, nil
}

func (k GeneralMsgServer) UpdateTradingRewardPendingPoints(
	goCtx context.Context, msg *v2.MsgTradingRewardPendingPointsUpdate,
) (*v2.MsgTradingRewardPendingPointsUpdateResponse, error) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	ctx := sdk.UnwrapSDKContext(goCtx)

	if !k.IsGovernanceAuthorityAddress(msg.Sender) {
		return nil, errortypes.ErrUnauthorized
	}

	if err := k.HandleTradingRewardPendingPointsUpdateProposal(ctx, msg.Proposal); err != nil {
		return nil, err
	}

	return &v2.MsgTradingRewardPendingPointsUpdateResponse{}, nil
}

func (k GeneralMsgServer) UpdateFeeDiscount(goCtx context.Context, msg *v2.MsgFeeDiscount) (*v2.MsgFeeDiscountResponse, error) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	ctx := sdk.UnwrapSDKContext(goCtx)

	if !k.IsGovernanceAuthorityAddress(msg.Sender) {
		return nil, errortypes.ErrUnauthorized
	}

	if err := k.HandleFeeDiscountProposal(ctx, msg.Proposal); err != nil {
		return nil, err
	}

	return &v2.MsgFeeDiscountResponse{}, nil
}

func (k GeneralMsgServer) UpdateAtomicMarketOrderFeeMultiplierSchedule(
	goCtx context.Context, msg *v2.MsgAtomicMarketOrderFeeMultiplierSchedule,
) (*v2.MsgAtomicMarketOrderFeeMultiplierScheduleResponse, error) {
	defer metrics.ReportFuncCallAndTiming(k.svcTags)()

	ctx := sdk.UnwrapSDKContext(goCtx)

	if !k.IsGovernanceAuthorityAddress(msg.Sender) {
		return nil, errortypes.ErrUnauthorized
	}

	if err := k.HandleAtomicMarketOrderFeeMultiplierScheduleProposal(ctx, msg.Proposal); err != nil {
		return nil, err
	}

	return &v2.MsgAtomicMarketOrderFeeMultiplierScheduleResponse{}, nil
}

// CancelPostOnlyMode sets a flag to cancel post-only mode in the next BeginBlock
// This method can only be called by governance authority or exchange admin.
func (k GeneralMsgServer) CancelPostOnlyMode(
	goCtx context.Context,
	msg *v2.MsgCancelPostOnlyMode,
) (*v2.MsgCancelPostOnlyModeResponse, error) {
	doneFn := metrics.ReportFuncCallAndTiming(k.svcTags)
	defer doneFn()

	ctx := sdk.UnwrapSDKContext(goCtx)

	// Check if sender is governance authority or exchange admin
	if !k.IsGovernanceAuthorityAddress(msg.Sender) && !k.IsAdmin(ctx, msg.Sender) {
		return nil, govtypes.ErrInvalidSigner.Wrap("sender must be governance authority or exchange admin")
	}

	// Set the flag to cancel post-only mode in the next BeginBlock
	k.SetPostOnlyModeCancellationFlag(ctx)

	return &v2.MsgCancelPostOnlyModeResponse{}, nil
}

// ActivatePostOnlyMode activates post-only mode for a specified number of blocks.
// Can only be called by governance authority or exchange admin.
func (k GeneralMsgServer) ActivatePostOnlyMode(
	goCtx context.Context,
	msg *v2.MsgActivatePostOnlyMode,
) (*v2.MsgActivatePostOnlyModeResponse, error) {
	doneFn := metrics.ReportFuncCallAndTiming(k.svcTags)
	defer doneFn()

	ctx := sdk.UnwrapSDKContext(goCtx)

	if !k.IsGovernanceAuthorityAddress(msg.Sender) && !k.IsAdmin(ctx, msg.Sender) {
		return nil, govtypes.ErrInvalidSigner.Wrap("sender must be governance authority or exchange admin")
	}

	// Clear any pending cancellation flag
	if k.HasPostOnlyModeCancellationFlag(ctx) {
		k.DeletePostOnlyModeCancellationFlag(ctx)
	}

	newThreshold := ctx.BlockHeight() + int64(msg.BlocksAmount)

	// Only extend, never shorten existing post-only mode
	params := k.GetParams(ctx)
	if newThreshold > params.PostOnlyModeHeightThreshold {
		params.PostOnlyModeHeightThreshold = newThreshold
		k.SetParams(ctx, params)
	}

	return &v2.MsgActivatePostOnlyModeResponse{}, nil
}
