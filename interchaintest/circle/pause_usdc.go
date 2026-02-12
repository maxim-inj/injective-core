package circle

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/InjectiveLabs/injective-core/interchaintest/helpers"
)

// Function selectors for FiatToken pause/unpause functions
var (
	// FiatToken pause/unpause functions
	PauseSelector   = generateFunctionSelector("pause()")   // 0x8456cb59
	UnpauseSelector = generateFunctionSelector("unpause()") // 0x3f4ba83a
)

// PauseUSDCToken pauses the USDC token contract using direct EVM transaction
func PauseUSDCToken(
	t *testing.T,
	ctx context.Context,
	chainNode *cosmos.ChainNode,
	pauserWallet ibc.Wallet,
	tokenAddress common.Address,
	sequence uint64,
) {
	t.Logf("Pausing USDC token at address: %s", tokenAddress.Hex())

	// Get the pauser private key
	pauserPrivKey, err := helpers.NewPrivKeyFromMnemonic(pauserWallet.Mnemonic())
	require.NoError(t, err)

	ethChainID := big.NewInt(1776)

	// Generate pause() function selector
	// Function signature: "pause()" -> keccak256 -> first 4 bytes = 0x8456cb59
	methodID := PauseSelector
	t.Logf("Using pause() function selector: 0x%x", methodID)

	legacyTx := &ethtypes.LegacyTx{
		Nonce:    sequence,
		To:       &tokenAddress,
		Value:    big.NewInt(0),
		Gas:      1000000,
		GasPrice: big.NewInt(10),
		Data:     methodID,
	}

	cosmosTxHash, _, err := helpers.SignAndBroadcastEthTxs(
		ctx, chainNode, ethChainID,
		pauserWallet.KeyName(),
		pauserPrivKey,
		true,
		legacyTx,
	)
	require.NoError(t, err)

	t.Logf("Successfully paused USDC token. Cosmos Tx: %s", cosmosTxHash)
}

// UnpauseUSDCToken unpauses the USDC token contract using direct EVM transaction
func UnpauseUSDCToken(
	t *testing.T,
	ctx context.Context,
	chainNode *cosmos.ChainNode,
	pauserWallet ibc.Wallet,
	tokenAddress common.Address,
	sequence uint64,
) {
	t.Logf("Unpausing USDC token at address: %s", tokenAddress.Hex())

	// Get the pauser private key
	pauserPrivKey, err := helpers.NewPrivKeyFromMnemonic(pauserWallet.Mnemonic())
	require.NoError(t, err)

	// Get the Ethereum chain ID from the cosmos chain ID
	ethChainID, err := helpers.ParseEthChainID(chainNode.Chain.Config().ChainID)
	require.NoError(t, err)

	// Generate unpause() function selector
	// Function signature: "unpause()" -> keccak256 -> first 4 bytes = 0x3f4ba83a
	methodID := UnpauseSelector
	t.Logf("Using unpause() function selector: 0x%x", methodID)

	legacyTx := &ethtypes.LegacyTx{
		Nonce:    sequence,
		To:       &tokenAddress,
		Value:    big.NewInt(0),
		Gas:      1000000,
		GasPrice: big.NewInt(10),
		Data:     methodID,
	}

	cosmosTxHash, _, err := helpers.SignAndBroadcastEthTxs(
		ctx, chainNode, ethChainID,
		pauserWallet.KeyName(),
		pauserPrivKey,
		true,
		legacyTx,
	)
	require.NoError(t, err)

	t.Logf("Successfully unpaused USDC token. Cosmos Tx: %s", cosmosTxHash)
}
