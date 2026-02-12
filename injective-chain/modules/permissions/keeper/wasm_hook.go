package keeper

import (
	"context"
	"encoding/json"
	"strings"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/types"
)

// validateWasmHook checks that contract exists and satisfies the expected interface
func (k Keeper) validateWasmHook(ctx context.Context, contract sdk.AccAddress) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if !k.wasmKeeper.HasContractInfo(ctx, contract) {
		return types.ErrUnknownWasmHook
	}

	userAddr := sdk.MustAccAddressFromBech32("inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r")
	wasmHookMsg := struct {
		SendRestriction types.WasmHookMsg `json:"send_restriction"`
	}{types.WasmHookMsg{
		From:    userAddr,
		To:      userAddr,
		Action:  types.Action_UNSPECIFIED.String(),
		Amounts: sdk.NewCoins(sdk.NewCoin("inj", math.NewInt(1))),
	}}
	bz, err := json.Marshal(wasmHookMsg)
	if err != nil {
		return err
	}

	sdkCtxMetered := sdkCtx.WithGasMeter(storetypes.NewGasMeter(k.GetParams(sdkCtx).ContractHookMaxGas))

	if _, err := k.wasmKeeper.QuerySmart(sdkCtxMetered, contract, bz); errors.IsOf(err, wasmtypes.ErrQueryFailed) && strings.HasPrefix(err.Error(), "Error parsing into type") {
		return types.ErrInvalidWasmHook
	}

	sdkCtx.GasMeter().ConsumeGas(sdkCtxMetered.GasMeter().GasConsumed(), "permissions wasm hook")

	return nil
}

func (k Keeper) executeWasmHook(sdkCtx sdk.Context, namespace *types.Namespace, fromAddr, toAddr sdk.AccAddress, action types.Action, amount sdk.Coin) error {
	if namespace.WasmHook == "" {
		return nil
	}

	contractAddr, err := sdk.AccAddressFromBech32(namespace.WasmHook)
	if err != nil { // defensive programming
		return types.ErrInvalidWasmHook.Wrapf("WasmHook address is incorrect: %s (%s)", namespace.WasmHook, err.Error())
	}

	bz, err := types.GetWasmHookMsgBytes(fromAddr, toAddr, action, amount)
	if err != nil {
		return types.ErrInvalidWasmHook.Wrap(err.Error())
	}

	// since transfer hook can be called in EndBlocker, which is not gas metered, we need to enforce MaxGas limits
	// during QuerySmart call to prevent DoS
	params := k.GetParams(sdkCtx)
	sdkCtxMetered := sdkCtx.WithGasMeter(storetypes.NewGasMeter(params.ContractHookMaxGas))

	// call wasm hook contract inside a closure to catch out of gas panics, if any
	func() {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				if _, ok := panicErr.(storetypes.ErrorOutOfGas); ok {
					err = errors.Wrapf(types.ErrContractHookError, "panic during wasm hook: out of gas, gas used = %d, gas limit = %d",
						sdkCtxMetered.GasMeter().GasConsumed(), params.ContractHookMaxGas)
				} else {
					err = errors.Wrapf(types.ErrContractHookError, "panic during wasm hook: %v", panicErr)
				}
			}
		}()

		// ignore query response for now, in future this could be used for more complex logic (e.g. rerouting)
		_, err = k.wasmKeeper.QuerySmart(sdkCtxMetered, contractAddr, bz)
	}()

	sdkCtx.GasMeter().ConsumeGas(sdkCtxMetered.GasMeter().GasConsumed(), "permissions wasm hook: "+amount.Denom)

	// if query returns error -> means permissions check failed
	// in any other case (query technical error like out-of-gas, stack-too-deep) we "pretend" that the query returned false for permissions check
	if err != nil {
		return errors.Wrap(types.ErrRestrictedAction, err.Error())
	}

	return nil
}
