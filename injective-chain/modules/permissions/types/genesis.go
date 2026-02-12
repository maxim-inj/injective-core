package types

import (
	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gethtypes "github.com/ethereum/go-ethereum/common"
)

// DefaultGenesis returns the default Permissions genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:     DefaultParams(),
		Namespaces: []Namespace{},
		Vouchers:   []*AddressVoucher{},
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	err := gs.Params.Validate()
	if err != nil {
		return err
	}

	seenDenoms := map[string]struct{}{}

	for i := range gs.GetNamespaces() {
		ns := gs.GetNamespaces()[i]

		// Validate denom is not empty
		if ns.GetDenom() == "" {
			return errors.Wrap(ErrUnknownDenom, "namespace denom cannot be empty")
		}

		if _, ok := seenDenoms[ns.GetDenom()]; ok {
			return errors.Wrapf(ErrInvalidGenesis, "duplicate denom: %s", ns.GetDenom())
		}
		seenDenoms[ns.GetDenom()] = struct{}{}

		// Validate WasmHook address if set
		if ns.WasmHook != "" {
			if _, err := sdk.AccAddressFromBech32(ns.WasmHook); err != nil {
				return errors.Wrapf(ErrInvalidWasmHook, "invalid WasmHook address for denom %s: %s", ns.GetDenom(), ns.WasmHook)
			}
		}

		// Validate EvmHook address if set
		if ns.EvmHook != "" {
			if ok := gethtypes.IsHexAddress(ns.EvmHook); !ok {
				return errors.Wrapf(ErrInvalidEVMHook, "invalid EvmHook address for denom %s: %s", ns.GetDenom(), ns.EvmHook)
			}
		}

		// Validate roles (including EVERYONE role, role uniqueness, valid permissions, actor addresses)
		if err := ns.ValidateRoles(false); err != nil {
			return errors.Wrapf(err, "invalid roles for denom %s", ns.GetDenom())
		}

		// Validate policies (policy statuses and policy manager capabilities)
		if err := ns.ValidatePolicies(); err != nil {
			return errors.Wrapf(err, "invalid policies for denom %s", ns.GetDenom())
		}
	}

	return nil
}
