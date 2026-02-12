package keeper

import (
	"context"
	"slices"

	"cosmossdk.io/math"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"cosmossdk.io/errors"
	"github.com/InjectiveLabs/metrics"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/auction/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"
)

var _ types.MsgServer = msgServer{}

type msgServer struct {
	Keeper
	svcTags metrics.Tags
}

// NewMsgServerImpl returns an implementation of the bank MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{
		Keeper: keeper,
		svcTags: metrics.Tags{
			"svc": "auction_h",
		},
	}
}

func (k msgServer) UpdateParams(c context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	c, doneFn := metrics.ReportFuncCallAndTimingCtx(c, k.svcTags)
	defer doneFn()

	if msg.Authority != k.authority {
		return nil, errors.Wrapf(govtypes.ErrInvalidSigner, "invalid authority: expected %s, got %s", k.authority, msg.Authority)
	}

	if err := msg.Params.Validate(); err != nil {
		return nil, err
	}

	k.SetParams(sdk.UnwrapSDKContext(c), msg.Params)

	return &types.MsgUpdateParamsResponse{}, nil
}

func (k msgServer) Bid(goCtx context.Context, msg *types.MsgBid) (*types.MsgBidResponse, error) {
	goCtx, doneFn := metrics.ReportFuncCallAndTimingCtx(goCtx, k.svcTags)
	defer doneFn()

	ctx := sdk.UnwrapSDKContext(goCtx)

	// We are sure the Sender is a valid address because it is validated in the ValidateBasic method
	senderAddr := sdk.MustAccAddressFromBech32(msg.Sender)

	params := k.GetParams(ctx)

	if len(params.BiddersWhitelist) > 0 {
		isWhitelisted := slices.Contains(params.BiddersWhitelist, msg.Sender)
		if !isWhitelisted {
			metrics.ReportFuncError(k.svcTags)
			return nil, errors.Wrapf(sdkerrors.ErrUnauthorized, "sender %s is not in bidders whitelist", msg.Sender)
		}
	}

	round := k.GetAuctionRound(ctx)

	if round == 0 {
		return nil, errors.Wrap(types.ErrBidRound, "auction has not been initialized")
	}

	if msg.Round != round {
		metrics.ReportFuncError(k.svcTags)
		return nil, errors.Wrapf(types.ErrBidRound, "current round is %d but got bid for %d", round, msg.Round)
	}

	// can only happen in chain halts
	endingTimeStamp := k.GetEndingTimeStamp(ctx)
	if ctx.BlockTime().Unix() >= endingTimeStamp {
		metrics.ReportFuncError(k.svcTags)
		return nil, errors.Wrap(types.ErrBidRound, "Bid round end timestamp is already reached")
	}

	lastBid := k.GetHighestBid(ctx)

	isFirstBidder := !lastBid.Amount.Amount.IsPositive()
	isCurrentBidder := lastBid.Bidder == msg.Sender

	var amountToDeposit sdk.Coin
	if isCurrentBidder {
		if msg.BidAmount.IsLT(lastBid.Amount) {
			metrics.ReportFuncError(k.svcTags)
			return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "new bid must be >= previous bid")
		}
		amountToDeposit = msg.BidAmount.Sub(lastBid.Amount)
	} else {
		amountToDeposit = msg.BidAmount
	}

	// ensure last_bid * (1+min_next_increment_rate) <= msg.BidAmount
	if lastBid.Amount.Amount.ToLegacyDec().Mul(math.LegacyOneDec().Add(params.MinNextBidIncrementRate)).GT(msg.BidAmount.Amount.ToLegacyDec()) {
		metrics.ReportFuncError(k.svcTags)
		return nil, errors.Wrapf(sdkerrors.ErrInvalidRequest, "new bid should be bigger than last bid + min increment percentage")
	}

	depositAmount := sdk.NewCoins(amountToDeposit)
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, senderAddr, types.ModuleName, depositAmount); err != nil {
		metrics.ReportFuncError(k.svcTags)
		k.Logger(ctx).Error("Bidder deposit failed", "senderAddr", senderAddr.String(), "coin", amountToDeposit.String())
		return nil, errors.Wrap(err, "deposit failed")
	}

	// Only refund if there was a previous bidder AND it's not the current bidder increasing
	if !isFirstBidder && !isCurrentBidder {
		err := k.refundLastBidder(ctx)
		if err != nil {
			metrics.ReportFuncError(k.svcTags)
			return nil, err
		}
	}

	k.SetBid(ctx, msg.Sender, msg.BidAmount)

	_ = ctx.EventManager().EmitTypedEvent(&types.EventBid{
		Bidder: msg.Sender,
		Amount: msg.BidAmount,
		Round:  round,
	})
	return &types.MsgBidResponse{}, nil
}

func (k msgServer) refundLastBidder(ctx sdk.Context) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	lastBid := k.GetHighestBid(ctx)
	lastBidAmount := lastBid.Amount.Amount
	lastBidder, err := sdk.AccAddressFromBech32(lastBid.Bidder)
	if err != nil {
		metrics.ReportFuncError(k.svcTags)
		k.Logger(ctx).Error(err.Error())
		return err
	}

	bidAmount := sdk.NewCoins(sdk.NewCoin(chaintypes.InjectiveCoin, lastBidAmount))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, lastBidder, bidAmount); err != nil {
		metrics.ReportFuncError(k.svcTags)
		k.Logger(ctx).Error("Bidder refund failed", "lastBidderAddr", lastBidder.String(), "coin", bidAmount.String())
		return errors.Wrap(err, "deposit failed")
	}

	return nil
}
