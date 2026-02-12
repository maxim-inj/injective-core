package keeper

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	mempool1559 "github.com/InjectiveLabs/injective-core/injective-chain/modules/txfees/keeper/mempool-1559"
)

func (k *Keeper) RefreshMempool1559Parameters(ctx sdk.Context) {
	params := k.GetParams(ctx)

	if k.CurFeeState == nil {
		k.Logger(ctx).Warn("RefreshMempool1559Parameters: CurFeeState is nil, setting to default")
		k.CurFeeState = mempool1559.DefaultFeeState()
	}

	k.CurFeeState.MinBaseFee = params.MinGasPrice
	k.CurFeeState.DefaultBaseFee = params.MinGasPrice.Mul(params.DefaultBaseFeeMultiplier)
	k.CurFeeState.MaxBaseFee = params.MinGasPrice.Mul(params.MaxBaseFeeMultiplier)
	k.CurFeeState.ResetInterval = params.ResetInterval
	k.CurFeeState.MaxBlockChangeRate = params.MaxBlockChangeRate
	k.CurFeeState.TargetBlockSpacePercent = params.TargetBlockSpacePercentRate
	k.CurFeeState.RecheckFeeLowBaseFee = params.RecheckFeeLowBaseFee
	k.CurFeeState.RecheckFeeHighBaseFee = params.RecheckFeeHighBaseFee
	k.CurFeeState.RecheckFeeBaseFeeThreshold = params.MinGasPrice.Mul(params.RecheckFeeBaseFeeThresholdMultiplier)
}

// On start, we unmarshal the consensus params once and cache them.
// Then, on every block, we check if the current consensus param bytes have changed in comparison to the cached value.
// If they have, we unmarshal the current consensus params, update the target gas, and cache the value.
// This is done to improve performance by not having to fetch and unmarshal the consensus params on every block.
func (k *Keeper) CheckAndSetTargetGas(ctx sdk.Context) error {
	// Check if the block gas limit has changed.
	// If it has, update the target gas for eip1559.
	consParams, err := k.GetConsParams(ctx)
	if err != nil {
		return fmt.Errorf("txfees: failed to get consensus parameters: %w", err)
	}

	// Check if Block params are nil or MaxGas is invalid/unlimited
	if consParams.Params.Block == nil || consParams.Params.Block.MaxGas <= 0 {
		if consParams.Params.Block != nil && consParams.Params.Block.MaxGas == 0 {
			// This should never happen in practice as the chain wouldn't process any txs
			k.Logger(ctx).Error("CheckAndSetTargetGas: MaxGas is 0 - chain cannot process transactions")
		}
		// Use default target gas to avoid division by zero in UpdateBaseFee
		// For MaxGas == -1 (unlimited), this maintains normal EIP-1559 operation
		k.CurFeeState.TargetGas = mempool1559.DefaultFeeState().TargetGas
		return nil
	}

	// Always update the target gas as TargetBlockSpacePercent might have changed even if MaxGas hasn't.
	k.CurFeeState.TargetGas = k.calculateTargetGas(consParams.Params.Block.MaxGas)

	return nil
}

func (k *Keeper) calculateTargetGas(maxGas int64) int64 {
	targetGas := k.CurFeeState.TargetBlockSpacePercent.Mul(
		math.LegacyNewDec(maxGas),
	).TruncateInt().Int64()

	// Prevent division by zero in UpdateBaseFee by ensuring TargetGas is never less than 1
	return max(targetGas, 1)
}
