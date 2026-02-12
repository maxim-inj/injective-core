package keeper

import (
	"context"

	"cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/erc20/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

// needsDenomCreationFee checks if we need to charge creation fee to mint denom
func (k Keeper) needsDenomCreationFee(ctx sdk.Context, denom string) bool {
	return !k.bankKeeper.HasSupply(ctx, denom)
}

// chargeDenomCreationFee sends denom creation fee to community pool
func (k Keeper) chargeDenomCreationFee(ctx sdk.Context, payerAddr sdk.AccAddress) error {
	// Send creation fee to community pool
	creationFee := k.GetParams(ctx).DenomCreationFee
	if creationFee.Amount.IsPositive() {
		if err := k.communityPoolKeeper.FundCommunityPool(ctx, sdk.NewCoins(creationFee), payerAddr); err != nil {
			return err
		}
	}
	return nil
}

// MintERC20 mints new erc20 denoms.
func (k Keeper) MintERC20(c context.Context, erc20Addr common.Address, minter sdk.AccAddress, amt sdkmath.Int) error {
	ctx := sdk.UnwrapSDKContext(c)

	denom := types.DenomPrefix + erc20Addr.Hex()

	if k.needsDenomCreationFee(ctx, denom) {
		// charge contract for denom creation
		if err := k.chargeDenomCreationFee(ctx, sdk.AccAddress(erc20Addr.Bytes())); err != nil {
			return errors.Wrap(types.ErrChargingDenomCreationFee, err.Error())
		}
	}

	coins := sdk.NewCoins(sdk.NewCoin(denom, amt))
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
		return errors.Wrap(err, "fail to mint coins in erc20 module")
	}
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, minter, coins); err != nil {
		return errors.Wrap(err, "fail to send minted coins to account")
	}

	return nil
}

// BurnERC20 burns the erc20 denom on burner address.
func (k Keeper) BurnERC20(c context.Context, erc20Addr common.Address, burner sdk.AccAddress, amt sdkmath.Int) error {
	ctx := sdk.UnwrapSDKContext(c)

	denom := types.DenomPrefix + erc20Addr.Hex()

	coins := sdk.NewCoins(sdk.NewCoin(denom, amt))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, burner, types.ModuleName, coins); err != nil {
		return errors.Wrap(err, "fail to send burn coins to module")
	}
	if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, coins); err != nil {
		return errors.Wrap(err, "fail to burn coins in erc20 module")
	}

	return nil
}
