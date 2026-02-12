package keeper

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/modules/peggy/types"
	"github.com/InjectiveLabs/metrics"
)

func (k *Keeper) CheckBadSignatureEvidence(
	ctx sdk.Context,
	msg *types.MsgSubmitBadSignatureEvidence,
) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	var subject types.EthereumSigned

	err := k.cdc.UnpackAny(msg.Subject, &subject)
	if err != nil {
		return err
	}

	switch subject := subject.(type) {
	case *types.OutgoingTxBatch:
		return k.checkBadSignatureEvidenceInternal(ctx, subject, msg.Signature)
	case *types.Valset:
		return k.checkBadSignatureEvidenceInternal(ctx, subject, msg.Signature)

	default:
		metrics.ReportFuncError(k.svcTags)
		return errors.Wrap(types.ErrInvalid, "Bad signature must be over a batch, valset, or logic call")
	}
}

func (k *Keeper) checkBadSignatureEvidenceInternal(ctx sdk.Context, subject types.EthereumSigned, signature string) error {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	// Get checkpoint of the supposed bad signature (fake valset, batch, or logic call submitted to eth)
	peggyID := k.GetPeggyID(ctx)
	checkpoint := subject.GetCheckpoint(peggyID)

	// Try to find the checkpoint in the archives. If it exists, we don't slash because
	// this is not a bad signature
	if k.GetPastEthSignatureCheckpoint(ctx, checkpoint) {
		metrics.ReportFuncError(k.svcTags)
		return errors.Wrap(types.ErrInvalid, "Checkpoint exists, cannot slash")
	}

	// Decode Eth signature to bytes
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		metrics.ReportFuncError(k.svcTags)
		return errors.Wrap(types.ErrInvalid, "signature decoding")
	}

	ethAddress, err := types.EthAddressFromSignature(checkpoint, sigBytes)
	if err != nil {
		metrics.ReportFuncError(k.svcTags)
		return errors.Wrap(types.ErrInvalid, fmt.Sprintf("signature to eth address failed with checkpoint %s and signature %s", checkpoint.Hex(), signature))
	}

	// Find the offending validator by eth address
	val, found := k.GetValidatorByEthAddress(ctx, ethAddress)
	if !found {
		metrics.ReportFuncError(k.svcTags)
		return errors.Wrap(types.ErrInvalid, fmt.Sprintf("Did not find validator for eth address %s", ethAddress))
	}

	cons, err := val.GetConsAddr()
	if err != nil {
		metrics.ReportFuncError(k.svcTags)
		return errors.Wrap(err, "Could not get consensus key address for validator")
	}

	// return if the offending validator was already slashed for this evidence
	if k.IsValidatorSlashedForFakeCheckpoint(ctx, checkpoint[:], cons) {
		metrics.ReportFuncError(k.svcTags)
		return errors.Wrap(types.ErrInvalid, fmt.Sprintf("validator already slashed for fake checkpoint %s and signature %s", checkpoint.Hex(), signature))
	}

	if !val.IsJailed() {
		err = k.StakingKeeper.Jail(ctx, cons)
		if err != nil {
			metrics.ReportFuncError(k.svcTags)
			return errors.Wrap(err, "Could not jail validator")
		}
	}

	// set the fake checkpoint under provided signature so validator cannot be slashed again for the same evidence
	k.SetValidatorSlashedForFakeCheckpoint(ctx, checkpoint[:], cons)

	return nil
}

// SetPastEthSignatureCheckpoint puts the checkpoint of a valset, batch, or logic call into a set
// in order to prove later that it existed at one point.
func (k *Keeper) SetPastEthSignatureCheckpoint(ctx sdk.Context, checkpoint common.Hash) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetPastEthSignatureCheckpointKey(checkpoint), []byte{0x1})
}

// GetPastEthSignatureCheckpoint tells you whether a given checkpoint has ever existed
func (k *Keeper) GetPastEthSignatureCheckpoint(ctx sdk.Context, checkpoint common.Hash) (found bool) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := ctx.KVStore(k.storeKey)
	if bytes.Equal(store.Get(types.GetPastEthSignatureCheckpointKey(checkpoint)), []byte{0x1}) {
		return true
	} else {
		return false
	}
}

func (k *Keeper) IsValidatorSlashedForFakeCheckpoint(ctx sdk.Context, checkpoint, addr []byte) bool {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.GetFakeCheckpointKey(checkpoint, addr))
	if !bytes.Equal(bz, []byte{1}) {
		return false
	}

	return true
}

func (k *Keeper) SetValidatorSlashedForFakeCheckpoint(ctx sdk.Context, checkpoint, addr []byte) {
	ctx, doneFn := metrics.ReportFuncCallAndTimingSdkCtx(ctx, k.svcTags)
	defer doneFn()

	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetFakeCheckpointKey(checkpoint, addr), []byte{1})
}
