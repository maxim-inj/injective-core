package peggy

import (
	"context"
	"math/big"

	"github.com/InjectiveLabs/coretracer"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
)

// Gets the latest transaction batch nonce
func (s *peggyContract) GetTxBatchNonce(
	ctx context.Context,
	erc20ContractAddress common.Address,
	callerAddress common.Address,
) (*big.Int, error) {
	defer coretracer.Trace(&ctx, s.svcTags)()

	nonce, err := s.ethPeggy.LastBatchNonce(&bind.CallOpts{
		From:    callerAddress,
		Context: ctx,
	}, erc20ContractAddress)

	if err != nil {
		err = errors.Wrap(err, "LastBatchNonce call failed")
		return nil, err
	}

	return nonce, nil
}

// Gets the latest validator set nonce
func (s *peggyContract) GetValsetNonce(
	ctx context.Context,
	callerAddress common.Address,
) (*big.Int, error) {
	defer coretracer.Trace(&ctx, s.svcTags)()

	nonce, err := s.ethPeggy.StateLastValsetNonce(&bind.CallOpts{
		From:    callerAddress,
		Context: ctx,
	})

	if err != nil {
		coretracer.TraceError(ctx, err)
		return nil, errors.Wrap(err, "StateLastValsetNonce call failed")
	}

	return nonce, nil
}

// Gets the peggyID
func (s *peggyContract) GetPeggyID(
	ctx context.Context,
	callerAddress common.Address,
) (common.Hash, error) {
	defer coretracer.Trace(&ctx, s.svcTags)()

	peggyID, err := s.ethPeggy.StatePeggyId(&bind.CallOpts{
		From:    callerAddress,
		Context: ctx,
	})

	if err != nil {
		coretracer.TraceError(ctx, err)
		return common.Hash{}, errors.Wrap(err, "StatePeggyId call failed")
	}

	return peggyID, nil
}

func (s *peggyContract) GetERC20Symbol(
	ctx context.Context,
	erc20ContractAddress common.Address,
	callerAddress common.Address,
) (symbol string, err error) {
	defer coretracer.Trace(&ctx, s.svcTags)()

	erc20Wrapper := bind.NewBoundContract(erc20ContractAddress, erc20ABI, s.ethProvider, nil, nil)

	callOpts := &bind.CallOpts{
		From:    callerAddress,
		Context: ctx,
	}
	var out []interface{}

	if err = erc20Wrapper.Call(callOpts, &out, "symbol"); err != nil {
		coretracer.TraceError(ctx, err)
		return "", errors.Wrap(err, "ERC20 [symbol] call failed")
	}

	symbol = *abi.ConvertType(out[0], new(string)).(*string)

	return symbol, nil
}
