package circle

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

	"cosmossdk.io/math"

	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/interchaintest/foundry"
)

// MintUSDCTokens mints USDC tokens to a recipient address using a deployer container
// Note: Call setupStablecoinRepo before calling this function
func MintUSDCTokens(
	t *testing.T,
	ctx context.Context,
	chainNode *cosmos.ChainNode,
	masterMinterOwnerWallet ibc.Wallet,
	suite StablecoinContractSuite,
	recipient ibc.Wallet,
	amount math.Int,
	deployer *foundry.Container,
) {
	t.Logf("Minting %s USDC tokens to %s...", amount.String(), recipient.FormattedAddress())

	// Get master minter owner private key as hex string for Ethereum format
	privKeyHex := convertWalletToEthPrivateKeyHex(t, masterMinterOwnerWallet)

	// Get recipient address in Ethereum format
	recipientAddress := common.Address(recipient.Address()).Hex()

	// Get chain node hostname in docker network
	chainNodeHostname := chainNode.HostName()

	// Create .env file content with network-accessible endpoints
	envContent := fmt.Sprintf(`# Minting configuration for USDC
# Token configuration
DEMO_TOKEN_ADDRESS=%s
DEMO_RECIPIENT_ADDRESS=%s
DEMO_MINT_AMOUNT=%s
DEMO_MASTER_MINTER_OWNER_PRIVATE_KEY=%s

# Network configuration
TESTNET_RPC_URL=http://%s:8545
ETH_RPC_URL=http://%s:8545
INJ_URL=http://%s:26657
GAS_PRICE=10
`,
		suite.Proxy.Hex(),
		recipientAddress,
		amount.String(),
		privKeyHex,
		chainNodeHostname,
		chainNodeHostname,
		chainNodeHostname,
	)

	t.Logf("Generated minting .env file content:\n%s", envContent)

	// Write .env file inside container
	t.Log("Writing minting .env file inside container...")
	err := deployer.WriteFile(
		ctx,
		fmt.Sprintf("%s/stablecoin-evm/.env", WorkDir),
		envContent,
	)
	require.NoError(t, err, "Failed to write .env file")

	// Run minting script
	t.Log("Running minting script inside container...")
	stdout, stderr, err := deployer.Exec(
		ctx,
		[]string{
			"bash", "-c",
			fmt.Sprintf("cd %s/stablecoin-evm && source .env && bash ./scripts/demo/mint-some-usdc.sh", WorkDir),
		},
	)

	t.Logf("Minting script stdout:\n%s", stdout)
	if stderr != "" {
		t.Logf("Minting script stderr:\n%s", stderr)
	}
	require.NoError(t, err, "Minting script failed")

	// Check if minting was successful by looking for success message in output
	if !strings.Contains(stdout, "Minting Complete!") {
		t.Fatalf("Minting did not complete successfully. Output: %s", stdout)
	}

	t.Logf("Successfully minted %s USDC tokens to %s", amount.String(), recipientAddress)
}
