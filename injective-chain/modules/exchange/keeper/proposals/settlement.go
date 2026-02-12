package proposals

import (
	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

func (k *ProposalKeeper) HandleMarketForcedSettlementProposal(ctx sdk.Context, p *v2.MarketForcedSettlementProposal) error {
	if err := p.ValidateBasic(); err != nil {
		return err
	}

	marketID := common.HexToHash(p.MarketId)
	derivativeMarket := k.GetDerivativeMarketByID(ctx, marketID)

	if derivativeMarket == nil {
		spotMarket := k.GetSpotMarketByID(ctx, marketID)
		if spotMarket == nil {
			return types.ErrGenericMarketNotFound
		}

		if p.SettlementPrice != nil {
			return errors.Wrap(types.ErrInvalidSettlement, "settlement price must be nil for spot markets")
		}

		return scheduleSpotMarketForceClosure(ctx, k, spotMarket)
	}

	return scheduleDerivativeMarketSettlement(ctx, k, derivativeMarket, p.SettlementPrice)
}

func scheduleSpotMarketForceClosure(
	ctx sdk.Context,
	k *ProposalKeeper,
	spotMarket *v2.SpotMarket,
) error {
	settlementInfo := k.GetSpotMarketForceCloseInfo(ctx, common.HexToHash(spotMarket.MarketId))
	if settlementInfo != nil {
		return types.ErrMarketAlreadyScheduledToSettle
	}

	k.SetSpotMarketForceCloseInfo(ctx, common.HexToHash(spotMarket.MarketId))

	return nil
}

func scheduleDerivativeMarketSettlement(
	ctx sdk.Context,
	k *ProposalKeeper,
	derivativeMarket *v2.DerivativeMarket,
	settlementPrice *math.LegacyDec,
) error {
	if settlementPrice == nil {
		// zero is a reserved value for fetching the latest price from oracle
		zeroDec := math.LegacyZeroDec()
		settlementPrice = &zeroDec
	} else if !types.SafeIsPositiveDec(*settlementPrice) {
		return errors.Wrap(types.ErrInvalidSettlement, "settlement price must be positive for derivative markets")
	}

	settlementInfo := k.GetDerivativesMarketScheduledSettlementInfo(ctx, common.HexToHash(derivativeMarket.MarketId))
	if settlementInfo != nil {
		return types.ErrMarketAlreadyScheduledToSettle
	}

	k.SetDerivativesMarketScheduledSettlementInfo(ctx, &v2.DerivativeMarketSettlementInfo{
		MarketId:        derivativeMarket.MarketId,
		SettlementPrice: *settlementPrice,
	})

	return nil
}
