package keeper

import (
	"context"
	"encoding/json"
	"strings"

	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ethereum/go-ethereum/accounts/abi"
	gethtypes "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	erc20types "github.com/InjectiveLabs/injective-core/injective-chain/modules/erc20/types"
	bankbinding "github.com/InjectiveLabs/injective-core/injective-chain/modules/evm/precompiles/bindings/cosmos/precompile/bank"
	evmtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/evm/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/types"
)

var (
	// ownableABI is initialized once for reuse across all calls
	ownableABI *abi.ABI
	// ownerCallData is the pre-computed call data for owner() function
	ownerCallData []byte
)

func init() {
	var err error
	ownableABI, err = bankbinding.MintBurnBankERC20MetaData.GetAbi()
	if err != nil {
		panic("failed to initialize ownable ABI: " + err.Error())
	}
	ownerCallData, err = ownableABI.Pack("owner")
	if err != nil {
		panic("failed to encode owner() call data: " + err.Error())
	}
}

// getEVMContractOwner retrieves the owner address of an ERC20 contract deployed on the EVM.
// This method calls the standard Ownable interface's owner() function on the contract to determine
// the current owner address.
//
// Parameters:
//   - c: The context for the operation
//   - denom: The token denomination in the format "erc20:{contract_address}"
//
// Returns:
//   - sdk.AccAddress: The Cosmos SDK address of the contract owner
//   - error: Any error that occurred during the process
//
// The method performs the following steps:
// 1. Extracts the contract address from the denomination string
// 2. Constructs an EthCall request to invoke the owner() function
// 3. Executes the call using the EVM keeper with a 300,000 gas limit
// 4. Unpacks the returned address using the pre-compiled Ownable ABI
// 5. Converts the Ethereum address to a Cosmos SDK address format
func (k msgServer) getEVMContractOwner(c context.Context, denom string) (sdk.AccAddress, error) {
	address, ok := strings.CutPrefix(denom, erc20types.DenomPrefix)
	if !ok {
		return sdk.AccAddress{}, types.ErrInvalidERC20Denom
	}

	to := gethtypes.HexToAddress(address)
	input := hexutil.Bytes(ownerCallData)
	args, _ := json.Marshal(evmtypes.TransactionArgs{
		To:    &to,
		Input: &input,
	})
	req := evmtypes.EthCallRequest{
		Args:   args,
		GasCap: uint64(300_000),
	}

	resp, err := k.evmKeeper.EthCall(c, &req)
	if err != nil || resp.VmError != "" {
		var errText string
		if err != nil {
			errText = err.Error()
		} else {
			errText = resp.VmError
		}
		return sdk.AccAddress{}, errors.Wrapf(types.ErrInvalidERC20Denom, "could not retrieve erc20 contract owner: %s", errText)
	}

	unpacked, err := ownableABI.Unpack("owner", resp.Ret)
	if err != nil {
		return sdk.AccAddress{}, errors.Wrapf(types.ErrInvalidERC20Denom, "failed to unpack owner response: %s", err.Error())
	}

	if len(unpacked) == 0 {
		return sdk.AccAddress{}, errors.Wrapf(types.ErrInvalidERC20Denom, "empty unpacked result")
	}

	ethAddr, ok := unpacked[0].(gethtypes.Address)
	if !ok {
		return sdk.AccAddress{}, errors.Wrapf(types.ErrInvalidERC20Denom, "could not cast input to address")
	}

	return sdk.AccAddress(ethAddr.Bytes()), nil
}
