package keeper

import (
	"encoding/json"

	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	evmtypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/evm/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
)

var (
	bytesType    abi.Type
	verifyMethod abi.Method
)

func init() {
	var err error
	// ABI types for verify function: verify(bytes payload, bytes parameterPayload) returns (bytes)
	bytesType, err = abi.NewType("bytes", "bytes", nil)
	if err != nil {
		panic("failed to create bytes ABI type: " + err.Error())
	}

	verifyMethod = abi.NewMethod("verify", "verify", abi.Function, "", false, true,
		abi.Arguments{
			{Name: "payload", Type: bytesType},
			{Name: "parameterPayload", Type: bytesType},
		},
		abi.Arguments{{Type: bytesType}},
	)
}

// verifyChainlinkReport verifies a Chainlink report via the configured verifier proxy
// contract and returns the verified report bytes.
func (k *Keeper) verifyChainlinkReport(ctx sdk.Context, fullReport []byte) ([]byte, error) {
	params := k.GetParams(ctx)

	// Verification required - check if verifier is configured
	if params.ChainlinkVerifierProxyContract == "" {
		return nil, errors.Wrap(types.ErrChainlinkVerificationFailed, "verifier not configured")
	}

	if k.evmKeeper == nil {
		return nil, errors.Wrap(types.ErrChainlinkVerificationFailed, "EVM keeper not available")
	}

	verifierAddress := params.ChainlinkVerifierProxyContract
	verifierAddr := common.HexToAddress(verifierAddress)

	// Encode: verify(bytes payload, bytes parameterPayload)
	// payload = fullReport, parameterPayload = empty bytes (no fee payment)
	callData, err := verifyMethod.Inputs.Pack(fullReport, []byte{})
	if err != nil {
		return nil, errors.Wrap(types.ErrChainlinkVerificationFailed, "failed to encode call data")
	}

	callDataWithSelector := hexutil.Bytes(append(verifyMethod.ID, callData...))

	args, err := json.Marshal(evmtypes.TransactionArgs{
		To:    &verifierAddr,
		Input: &callDataWithSelector,
	})
	if err != nil {
		return nil, errors.Wrap(types.ErrChainlinkVerificationFailed, "failed to marshal transaction args")
	}

	resp, err := k.evmKeeper.EthCall(ctx, &evmtypes.EthCallRequest{
		Args:   args,
		GasCap: params.ChainlinkDataStreamsVerificationGasLimit,
	})
	if err != nil {
		return nil, errors.Wrapf(types.ErrChainlinkVerificationFailed, "EVM call failed: %s", err.Error())
	}

	if resp.VmError != "" {
		return nil, errors.Wrapf(types.ErrChainlinkVerificationFailed, "verification failed: %s", resp.VmError)
	}

	if len(resp.Ret) == 0 {
		return nil, errors.Wrap(types.ErrChainlinkVerificationFailed, "empty verifier response")
	}

	decoded, err := verifyMethod.Outputs.Unpack(resp.Ret)
	if err != nil {
		return nil, errors.Wrap(types.ErrChainlinkVerificationFailed, "failed to decode verifier response")
	}
	if len(decoded) != 1 {
		return nil, errors.Wrap(types.ErrChainlinkVerificationFailed, "unexpected verifier response size")
	}
	verified, ok := decoded[0].([]byte)
	if !ok {
		return nil, errors.Wrap(types.ErrChainlinkVerificationFailed, "unexpected verifier response type")
	}

	if len(verified) == 0 {
		return nil, errors.Wrap(types.ErrChainlinkVerificationFailed, "empty verified report")
	}

	return verified, nil
}
