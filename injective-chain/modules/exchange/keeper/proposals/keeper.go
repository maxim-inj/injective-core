package proposals

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/base"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/binaryoptions"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/derivative"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/events"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/feediscounts"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/rewards"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/spot"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper/subaccount"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

//nolint:revive // ok
type ProposalKeeper struct {
	*base.BaseKeeper
	*binaryoptions.BinaryOptionsKeeper
	*derivative.DerivativeKeeper
	*spot.SpotKeeper
	*rewards.TradingKeeper
	*subaccount.SubaccountKeeper
	*feediscounts.FeeDiscountsKeeper

	account authkeeper.AccountKeeper
	oracle  types.OracleKeeper
	gov     govkeeper.Keeper
	distr   distrkeeper.Keeper
}

//nolint:revive // ok
func New(
	b *base.BaseKeeper,
	ak authkeeper.AccountKeeper,
	ok types.OracleKeeper,
	gk govkeeper.Keeper,
	distr distrkeeper.Keeper,
	bok *binaryoptions.BinaryOptionsKeeper,
	dk *derivative.DerivativeKeeper,
	sk *spot.SpotKeeper,
	tk *rewards.TradingKeeper,
	sak *subaccount.SubaccountKeeper,
	fk *feediscounts.FeeDiscountsKeeper,
) *ProposalKeeper {
	return &ProposalKeeper{
		BaseKeeper:          b,
		BinaryOptionsKeeper: bok,
		DerivativeKeeper:    dk,
		SpotKeeper:          sk,
		TradingKeeper:       tk,
		SubaccountKeeper:    sak,
		FeeDiscountsKeeper:  fk,
		account:             ak,
		oracle:              ok,
		gov:                 gk,
		distr:               distr,
	}
}

func (k *ProposalKeeper) HandleExchangeEnableProposal(
	ctx sdk.Context,
	p *v2.ExchangeEnableProposal,
) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	switch p.ExchangeType {
	case v2.ExchangeType_SPOT:
		k.SetSpotExchangeEnabled(ctx)
	case v2.ExchangeType_DERIVATIVES:
		k.SetDerivativesExchangeEnabled(ctx)
	default:
	}

	return nil
}

// Deprecated: HandleBatchExchangeModificationProposal is deprecated and will be removed in the future.
//
//nolint:revive // ok
func (k *ProposalKeeper) HandleBatchExchangeModificationProposal(
	ctx sdk.Context,
	p *v2.BatchExchangeModificationProposal,
) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	for _, proposal := range p.SpotMarketParamUpdateProposals {
		if err := k.HandleSpotMarketParamUpdateProposal(ctx, proposal); err != nil {
			return err
		}
	}

	for _, proposal := range p.DerivativeMarketParamUpdateProposals {
		if err := k.HandleDerivativeMarketParamUpdateProposal(ctx, proposal); err != nil {
			return err
		}
	}

	for _, proposal := range p.SpotMarketLaunchProposals {
		if err := k.HandleSpotMarketLaunchProposal(ctx, proposal); err != nil {
			return err
		}
	}

	for _, proposal := range p.PerpetualMarketLaunchProposals {
		if err := k.HandlePerpetualMarketLaunchProposal(ctx, proposal); err != nil {
			return err
		}
	}

	for _, proposal := range p.ExpiryFuturesMarketLaunchProposals {
		if err := k.HandleExpiryFuturesMarketLaunchProposal(ctx, proposal); err != nil {
			return err
		}
	}

	for _, proposal := range p.BinaryOptionsMarketLaunchProposals {
		if err := k.HandleBinaryOptionsMarketLaunchProposal(ctx, proposal); err != nil {
			return err
		}
	}

	for _, proposal := range p.BinaryOptionsParamUpdateProposals {
		if err := k.HandleBinaryOptionsMarketParamUpdateProposal(ctx, proposal); err != nil {
			return err
		}
	}

	if p.AuctionExchangeTransferDenomDecimalsUpdateProposal != nil {
		if err := k.HandleUpdateAuctionExchangeTransferDenomDecimalsProposal(
			ctx,
			p.AuctionExchangeTransferDenomDecimalsUpdateProposal,
		); err != nil {
			return err
		}
	}

	if p.TradingRewardCampaignUpdateProposal != nil {
		if err := k.HandleTradingRewardCampaignUpdateProposal(ctx, p.TradingRewardCampaignUpdateProposal); err != nil {
			return err
		}
	}

	if p.FeeDiscountProposal != nil {
		if err := k.HandleFeeDiscountProposal(ctx, p.FeeDiscountProposal); err != nil {
			return err
		}
	}

	for _, proposal := range p.MarketForcedSettlementProposals {
		if err := k.HandleMarketForcedSettlementProposal(ctx, proposal); err != nil {
			return err
		}
	}

	if p.DenomMinNotionalProposal != nil {
		k.HandleDenomMinNotionalProposal(ctx, p.DenomMinNotionalProposal)
	}

	return nil
}

func (k *ProposalKeeper) HandleUpdateAuctionExchangeTransferDenomDecimalsProposal(
	ctx sdk.Context,
	p *v2.UpdateAuctionExchangeTransferDenomDecimalsProposal,
) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	for _, denomDecimal := range p.DenomDecimals {
		k.SetAuctionExchangeTransferDenomDecimals(ctx, denomDecimal.Denom, denomDecimal.Decimals)
	}

	return nil
}

func (k *ProposalKeeper) HandleDenomMinNotionalProposal(
	ctx sdk.Context,
	p *v2.DenomMinNotionalProposal,
) {
	for _, denomMinNotional := range p.DenomMinNotionals {
		k.SetMinNotionalForDenom(ctx, denomMinNotional.Denom, denomMinNotional.MinNotional)
	}
}

func (k *ProposalKeeper) HandleBatchCommunityPoolSpendProposal(
	ctx sdk.Context,
	p *v2.BatchCommunityPoolSpendProposal,
) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	for _, proposal := range p.Proposals {
		recipient, addrErr := sdk.AccAddressFromBech32(proposal.Recipient)
		if addrErr != nil {
			return addrErr
		}

		err := k.distr.DistributeFromFeePool(ctx, proposal.Amount, recipient)
		if err != nil {
			return err
		}

		ctx.Logger().Info(
			"transferred from the community pool to recipient",
			"amount", proposal.Amount.String(),
			"recipient", proposal.Recipient,
		)
	}

	return nil
}

func (k *ProposalKeeper) HandleAtomicMarketOrderFeeMultiplierScheduleProposal(
	ctx sdk.Context,
	p *v2.AtomicMarketOrderFeeMultiplierScheduleProposal,
) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	k.SetAtomicMarketOrderFeeMultipliers(ctx, p.MarketFeeMultipliers)

	events.Emit(ctx, k.BaseKeeper, &v2.EventAtomicMarketOrderFeeMultipliersUpdated{
		MarketFeeMultipliers: p.MarketFeeMultipliers,
	})

	return nil
}
