package types

import (
	"fmt"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
)

// ParamTable
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

func NewParams(contractHookQueryMaxGas uint64) Params {
	return Params{
		ContractHookMaxGas: contractHookQueryMaxGas,
	}
}

// default module parameters.
func DefaultParams() Params {
	return Params{
		ContractHookMaxGas: 200_000,
	}
}

// validate params.
func (p Params) Validate() error {
	if err := ValidateEVMAddresses(p.EnforcedRestrictionsContracts); err != nil {
		return fmt.Errorf("invalid contracts with enforced restrictions: %w", err)
	}
	return nil
}

func ValidateEVMAddresses(addresses []string) error {
	for _, addr := range addresses {
		if !ethcommon.IsHexAddress(addr) {
			return fmt.Errorf("is not valid EVM address: %s", addr)
		}
	}

	return nil
}

// Implements params.ParamSet.
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{}
}
