package circle

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"

	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

	"github.com/InjectiveLabs/injective-core/interchaintest/foundry"
)

// DeployPermissionsHook deploys the PermissionsHook_Inj contract using a deployer container
// Note: Call setupStablecoinRepo before calling this function
func DeployPermissionsHook(
	t *testing.T,
	ctx context.Context,
	chainNode *cosmos.ChainNode,
	deployerWallet ibc.Wallet,
	fiatTokenAddress common.Address,
	deployer *foundry.Container,
) common.Address {

	// Get deployer private key
	privKeyHex := convertWalletToEthPrivateKeyHex(t, deployerWallet)

	// Get chain node hostname in docker network
	chainNodeHostname := chainNode.HostName()

	// Create .env file content with network-accessible endpoints
	envContent := fmt.Sprintf(`# Deployment configuration for PermissionsHook
# Contract configuration
DEMO_TOKEN_ADDRESS=%s

# Deployment configuration
DEPLOYER_PRIVATE_KEY=%s
ETH_RPC_URL=http://%s:8545
TESTNET_RPC_URL=http://%s:8545
INJ_URL=http://%s:26657
GAS_PRICE=10
`,
		fiatTokenAddress.Hex(),
		privKeyHex,
		chainNodeHostname,
		chainNodeHostname,
		chainNodeHostname,
	)

	t.Logf("Generated permissions hook .env file content:\n%s", envContent)

	// Write .env file inside container
	t.Log("Writing permissions hook .env file inside container...")
	err := deployer.WriteFile(
		ctx,
		fmt.Sprintf("%s/stablecoin-evm/.env", WorkDir),
		envContent,
	)
	require.NoError(t, err, "Failed to write .env file")

	// Run deployment script
	t.Log("Running permissions hook deployment script inside container...")
	stdout, stderr, err := deployer.Exec(
		ctx,
		[]string{
			"bash", "-c",
			fmt.Sprintf("cd %s/stablecoin-evm && source .env && bash ./scripts/demo/deploy-permissions-hook.sh", WorkDir),
		},
	)

	t.Logf("Permissions hook script stdout:\n%s", stdout)
	if stderr != "" {
		t.Logf("Permissions hook script stderr:\n%s", stderr)
	}
	require.NoError(t, err, "Permissions hook deployment script failed")

	// Parse output
	hookAddress := parsePermissionsHookOutput(t, stdout)

	t.Logf("Permissions hook deployment completed successfully!")
	t.Logf("Hook address: %s", hookAddress)

	return hookAddress
}

func parsePermissionsHookOutput(t *testing.T, output string) common.Address {
	// Parse the output for the permissions hook address
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for the deployed contract line
		if strings.Contains(line, "PermissionsHook_Inj deployed at:") {
			addr := extractAddress(line)
			if addr != (common.Address{}) {
				return addr
			}
		}

		// Alternative: look for "PermissionsHook:" in the summary
		if strings.Contains(line, "PermissionsHook:") {
			addr := extractAddress(line)
			if addr != (common.Address{}) {
				return addr
			}
		}
	}

	require.Fail(t, "Failed to parse permissions hook address from script output", "Output: %s", output)
	return common.Address{}
}
