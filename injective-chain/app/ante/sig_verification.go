package ante

import (
	"errors"
	"fmt"

	cosmoserrors "cosmossdk.io/errors"
	txsigning "cosmossdk.io/x/tx/signing"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/crypto/types/multisig"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	gethsecp256k1 "github.com/ethereum/go-ethereum/crypto/secp256k1"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/InjectiveLabs/injective-core/injective-chain/app/ante/typeddata"
)

// SigVerificationDecorator verifies all signatures for a tx and return an error if any are invalid. Note,
// the SigVerificationDecorator will not check signatures on ReCheck. This decorator behaves the same as
// the original decorator in the SDK, except for the following:
//   - multisig signatures must be in EIP712 v2 format (used by clients, including Ledger CLI)
//   - nested multisig is not supported
//
// CONTRACT: Pubkeys are set in context for all signers before this decorator runs
// CONTRACT: Tx must implement SigVerifiableTx interface
type SigVerificationDecorator struct {
	ak              authante.AccountKeeper
	signModeHandler *txsigning.HandlerMap
}

func NewSigVerificationDecorator(
	ak authante.AccountKeeper,
	signModeHandler *txsigning.HandlerMap,
) SigVerificationDecorator {
	return SigVerificationDecorator{
		ak:              ak,
		signModeHandler: signModeHandler,
	}
}

func (svd SigVerificationDecorator) AnteHandle( //nolint:revive // ok
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (newCtx sdk.Context, err error) {
	sigTx, ok := tx.(authsigning.Tx)
	if !ok {
		return ctx, cosmoserrors.Wrap(sdkerrors.ErrTxDecode, "invalid transaction type")
	}

	// stdSigs contains the sequence number, account number, and signatures.
	// When simulating, this would just be a 0-length slice.
	sigs, err := sigTx.GetSignaturesV2()
	if err != nil {
		return ctx, err
	}

	signers, err := sigTx.GetSigners()
	if err != nil {
		return ctx, err
	}

	// check that signer length and signature length are the same
	if len(sigs) != len(signers) {
		return ctx, cosmoserrors.Wrapf(sdkerrors.ErrUnauthorized,
			"invalid number of signer; expected: %d, got %d", len(signers), len(sigs),
		)
	}

	for i, sig := range sigs {
		acc := svd.ak.GetAccount(ctx, signers[i])
		if acc == nil {
			return ctx, cosmoserrors.Wrapf(sdkerrors.ErrUnknownAddress, "account %s does not exist", signers[i])
		}

		// retrieve pubkey
		pubKey := acc.GetPubKey()
		if !simulate && pubKey == nil {
			return ctx, cosmoserrors.Wrap(sdkerrors.ErrInvalidPubKey, "pubkey on account is not set")
		}

		// Check account sequence number.
		if sig.Sequence != acc.GetSequence() {
			return ctx, cosmoserrors.Wrapf(
				sdkerrors.ErrWrongSequence,
				"account sequence mismatch, expected %d, got %d", acc.GetSequence(), sig.Sequence,
			)
		}

		// retrieve signer data
		genesis := ctx.BlockHeight() == 0
		chainID := ctx.ChainID()
		var accNum uint64
		if !genesis {
			accNum = acc.GetAccountNumber()
		}

		// no need to verify signatures on recheck tx
		if !simulate && !ctx.IsReCheckTx() && ctx.IsSigverifyTx() {
			anyPk, err := codectypes.NewAnyWithValue(pubKey)
			if err != nil {
				return ctx, cosmoserrors.Wrap(err, "failed to marshal pubkey")
			}

			signerData := txsigning.SignerData{
				Address:       acc.GetAddress().String(),
				ChainID:       chainID,
				AccountNumber: accNum,
				Sequence:      acc.GetSequence(),
				PubKey: &anypb.Any{
					TypeUrl: anyPk.TypeUrl,
					Value:   anyPk.Value,
				},
			}
			adaptableTx, ok := tx.(authsigning.V2AdaptableTx)
			if !ok {
				return ctx, fmt.Errorf("expected tx to implement V2AdaptableTx, got %T", tx)
			}
			txData := adaptableTx.GetSigningTxData()

			if ms, ok := sig.Data.(*signing.MultiSignatureData); ok {
				err = VerifyMultiSigWithEIP712V2Signatures(pubKey, signerData, ms, sigTx)
			} else {
				err = authsigning.VerifySignature(ctx, pubKey, signerData, sig.Data, svd.signModeHandler, txData)
			}

			if err != nil {
				var errMsg string
				if OnlyLegacyAminoSigners(sig.Data) {
					// If all signers are using SIGN_MODE_LEGACY_AMINO, we rely on VerifySignature to check account sequence number,
					// and therefore communicate sequence number as a potential cause of error.
					errMsg = fmt.Sprintf("signature verification failed; please verify account number (%d), sequence (%d) and chain-id (%s)", accNum, acc.GetSequence(), chainID) //nolint:revive //ok
				} else {
					errMsg = fmt.Sprintf("signature verification failed; please verify account number (%d) and chain-id (%s): (%s)", accNum, chainID, err.Error()) //nolint:revive //ok
				}

				return ctx, cosmoserrors.Wrap(sdkerrors.ErrUnauthorized, errMsg)
			}
		}
	}

	return next(ctx, tx, simulate)
}

func VerifyMultiSigWithEIP712V2Signatures( //nolint:revive // ok
	pubKey cryptotypes.PubKey,
	signerData txsigning.SignerData,
	multiSigData *signing.MultiSignatureData,
	tx authsigning.Tx,
) error {
	multiPK, ok := pubKey.(multisig.PubKey)
	if !ok {
		return fmt.Errorf("expected %T, got %T", multisig.PubKey(nil), pubKey)
	}

	var (
		bitarray = multiSigData.BitArray
		sigs     = multiSigData.Signatures
		size     = bitarray.Count()
		pubKeys  = multiPK.GetPubKeys()
	)

	if bitarray == nil {
		return fmt.Errorf("bit array is nil")
	}

	// ensure bit array is the correct size
	if len(pubKeys) != size {
		return fmt.Errorf("bit array size is incorrect, expecting: %d", len(pubKeys))
	}

	// ensure size of signature list
	if len(sigs) < int(multiPK.GetThreshold()) || len(sigs) > size {
		return fmt.Errorf("signature size is incorrect %d", len(sigs))
	}

	trueCount := bitarray.NumTrueBitsBefore(size)
	// ensure at least k signatures are set
	if trueCount < int(multiPK.GetThreshold()) {
		return fmt.Errorf("not enough signatures set, have %d, expected %d", trueCount, int(multiPK.GetThreshold()))
	}

	if trueCount != len(sigs) {
		return fmt.Errorf("bit array and signature size mismatch, have %d, got %d", trueCount, len(sigs))
	}

	sd := authsigning.SignerData{
		Address:       signerData.Address,
		ChainID:       signerData.ChainID,
		AccountNumber: signerData.AccountNumber,
		Sequence:      signerData.Sequence,
	}

	var chainID int64
	if signerData.ChainID == "injective-1" {
		chainID = 1
	} else {
		chainID = 11155111
	}

	// WrapTxToEIP712V2 only cares about ChainID and FeePayer but
	// there is never a fee payer (or their sig) when using Ledger
	opts := Web3ExtensionOptions{
		ChainID: chainID,
	}

	typedData, err := WrapTxToEIP712V2(GlobalCdc, tx, &sd, opts)
	if err != nil {
		return cosmoserrors.Wrap(err, "failed to pack tx data in EIP712 object")
	}

	sigHash, _, err := typeddata.ComputeTypedDataAndHash(typedData)
	if err != nil {
		return err
	}

	getSignBytes := func(mode signing.SignMode) ([]byte, error) {
		if mode != signing.SignMode_SIGN_MODE_EIP712_V2 {
			return nil, fmt.Errorf("expected SIGN_MODE_EIP712_V2, got %v", mode.String())
		}

		return sigHash, nil
	}

	// index in the list of signatures which we are concerned with.
	sigIndex := 0
	for i := 0; i < size; i++ {
		if bitarray.GetIndex(i) {
			si := multiSigData.Signatures[sigIndex]
			switch si := si.(type) {
			case *signing.SingleSignatureData:
				digest, err := getSignBytes(si.SignMode)
				if err != nil {
					return err
				}

				if len(si.Signature) != 65 {
					return errors.New("signature length doesn't match typical [R||S||V] signature 65 bytes")
				}

				// VerifySignature of secp256k1 accepts 64 byte signature [R||S]
				// WARNING! Under NO CIRCUMSTANCES try to use pubKey.VerifySignature there
				if !gethsecp256k1.VerifySignature(pubKeys[i].Bytes(), digest, si.Signature[:64]) {
					return errors.New("unable to verify signer signature of EIP712 typed data")
				}
			case *signing.MultiSignatureData:
				return errors.New("nested multisig eip712 signatures are not supported")
			default:
				return fmt.Errorf("improper signature data type for index %d", sigIndex)
			}

			sigIndex++
		}
	}

	return nil
}

// OnlyLegacyAminoSigners checks SignatureData to see if all
// signers are using SIGN_MODE_LEGACY_AMINO_JSON. If this is the case
// then the corresponding SignatureV2 struct will not have account sequence
// explicitly set, and we should skip the explicit verification of sig.Sequence
// in the SigVerificationDecorator's AnteHandler function.
func OnlyLegacyAminoSigners(sigData signing.SignatureData) bool {
	switch v := sigData.(type) {
	case *signing.SingleSignatureData:
		return v.SignMode == signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON
	case *signing.MultiSignatureData:
		for _, s := range v.Signatures {
			if !OnlyLegacyAminoSigners(s) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
