package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	erc20types "github.com/InjectiveLabs/injective-core/injective-chain/modules/erc20/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types/v2"
)

// OnEnforcedRestrictionsEVMContractPause pauses all derivative and binary options markets
// denominated in the token corresponding to the given EVM contract.
func (k *Keeper) OnEnforcedRestrictionsEVMContractPause(ctx sdk.Context, contract common.Address) error {
	tokenBankDenom := erc20types.DenomPrefix + contract.Hex()
	derivativeMarkets := k.GetAllActiveDerivativeAndBinaryOptionsMarkets(ctx)

	for _, market := range derivativeMarkets {
		if market.GetQuoteDenom() != tokenBankDenom {
			continue
		}

		markPriceAtPausing, err := k.GetDerivativeOrBinaryOptionsMarkPrice(ctx, market)
		if err != nil {
			k.Logger(ctx).Error("failed to get market price when pausing market due to quote denom being paused", "market_id", market.MarketID(), "error", err)
		}

		err = k.ForcePauseGenericMarket(ctx, market, markPriceAtPausing)
		if err != nil {
			k.Logger(ctx).Error("failed to pause market", "market_id", market.MarketID(), "error", err)
		}

		k.Logger(ctx).Warn("Paused market due to quote denom being paused", "market_id", market.MarketID())
	}

	return nil
}

// OnEnforcedRestrictionsEVMContractBlacklist cancels all spot and derivative orders for the blacklisted address
// across all markets denominated in the token corresponding to the given EVM contract.
func (k *Keeper) OnEnforcedRestrictionsEVMContractBlacklist(ctx sdk.Context, contract, user common.Address) error {
	tokenBankDenom := erc20types.DenomPrefix + contract.Hex()
	accountAddress := sdk.AccAddress(user.Bytes())

	k.CancelAllDerivativeOrdersForAddress(ctx, tokenBankDenom, user)

	k.IterateSpotMarkets(ctx, nil, func(market *v2.SpotMarket) (stop bool) {
		if market.QuoteDenom != tokenBankDenom && market.BaseDenom != tokenBankDenom {
			return false
		}

		marketID := market.MarketID()
		k.CancelAllSpotLimitOrdersForAddress(ctx, market, marketID, accountAddress)

		k.Logger(ctx).Warn("Cancelled all spot orders for blacklisted user",
			"market_id", marketID,
			"user", user.Hex(),
		)

		return false
	})

	return nil
}
