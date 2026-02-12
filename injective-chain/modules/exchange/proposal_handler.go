package exchange

import (
	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/proposals"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// NewExchangeProposalHandler creates a governance handler to manage new exchange proposal types.
//
//revive:disable:cyclomatic // Any refactoring to the function would make it less readable
func NewExchangeProposalHandler(k *keeper.Keeper) govtypes.Handler {
	return func(ctx sdk.Context, content govtypes.Content) error {
		content, err := proposals.ConvertExchangeProposal(ctx, k.ProposalKeeper, content)
		if err != nil {
			return err
		}

		switch c := content.(type) {
		case *v2.ExchangeEnableProposal:
			return k.HandleExchangeEnableProposal(ctx, c)
		case *v2.BatchExchangeModificationProposal:
			return k.HandleBatchExchangeModificationProposal(ctx, c) //nolint:staticcheck // ok
		case *v2.SpotMarketParamUpdateProposal:
			return k.HandleSpotMarketParamUpdateProposal(ctx, c)
		case *v2.SpotMarketLaunchProposal:
			return k.HandleSpotMarketLaunchProposal(ctx, c)
		case *v2.PerpetualMarketLaunchProposal:
			return k.HandlePerpetualMarketLaunchProposal(ctx, c)
		case *v2.BinaryOptionsMarketLaunchProposal:
			return k.HandleBinaryOptionsMarketLaunchProposal(ctx, c)
		case *v2.BinaryOptionsMarketParamUpdateProposal:
			return k.HandleBinaryOptionsMarketParamUpdateProposal(ctx, c)
		case *v2.ExpiryFuturesMarketLaunchProposal:
			return k.HandleExpiryFuturesMarketLaunchProposal(ctx, c)
		case *v2.DerivativeMarketParamUpdateProposal:
			return k.HandleDerivativeMarketParamUpdateProposal(ctx, c)
		case *v2.MarketForcedSettlementProposal:
			return k.HandleMarketForcedSettlementProposal(ctx, c)
		case *v2.UpdateAuctionExchangeTransferDenomDecimalsProposal:
			return k.HandleUpdateAuctionExchangeTransferDenomDecimalsProposal(ctx, c)
		case *v2.TradingRewardCampaignLaunchProposal:
			return k.HandleTradingRewardCampaignLaunchProposal(ctx, c)
		case *v2.TradingRewardCampaignUpdateProposal:
			return k.HandleTradingRewardCampaignUpdateProposal(ctx, c)
		case *v2.TradingRewardPendingPointsUpdateProposal:
			return k.HandleTradingRewardPendingPointsUpdateProposal(ctx, c)
		case *v2.FeeDiscountProposal:
			return k.HandleFeeDiscountProposal(ctx, c)
		case *v2.BatchCommunityPoolSpendProposal:
			return k.HandleBatchCommunityPoolSpendProposal(ctx, c)
		case *v2.AtomicMarketOrderFeeMultiplierScheduleProposal:
			return k.HandleAtomicMarketOrderFeeMultiplierScheduleProposal(ctx, c)
		default:
			return errors.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized exchange proposal content type: %T", c)
		}
	}
}
