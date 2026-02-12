package circle

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/crypto"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

	"github.com/InjectiveLabs/injective-core/interchaintest/helpers"
)

const WorkDir = "/apps/data"

// ConvertWalletToEthPrivateKeyHex converts a wallet mnemonic to an Ethereum
// private key hex string suitable for use with foundry tools like cast
func convertWalletToEthPrivateKeyHex(t *testing.T, wallet ibc.Wallet) string {
	// Use the helper function to generate Ethereum private key from mnemonic
	ethPrivKey, err := helpers.GenerateEthereumPrivateKey(wallet.Mnemonic())
	require.NoError(t, err)

	// Convert to hex string with 0x prefix for cast wallet import
	// Use go-ethereum's FromECDSA to ensure proper 32-byte encoding
	privKeyBytes := ethcrypto.FromECDSA(ethPrivKey)
	return "0x" + hex.EncodeToString(privKeyBytes)
}

// generateFunctionSelector generates a 4-byte function selector from a function signature
// This is done by taking the first 4 bytes of the keccak256 hash of the function signature
func generateFunctionSelector(functionSignature string) []byte {
	hash := crypto.Keccak256([]byte(functionSignature))
	return hash[:4]
}
