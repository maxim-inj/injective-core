// Add to the existing circle-stablecoins test file
package circle

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"

	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

	"github.com/InjectiveLabs/injective-core/interchaintest/foundry"
)

type StablecoinContractSuite struct {
	SignatureChecker common.Address
	Implementation   common.Address
	Proxy            common.Address
	MasterMinter     common.Address
}

type StablecoinConfig struct {
	TokenName     string
	TokenSymbol   string
	TokenCurrency string
	TokenDecimals uint8

	ProxyAdmin        common.Address
	MasterMinterOwner common.Address
	Owner             common.Address
	Pauser            common.Address
	Blacklister       common.Address
}

// SetupStablecoinRepo clones and builds the stablecoin-evm repository in the deployer container
func SetupStablecoinRepo(
	t *testing.T,
	ctx context.Context,
	deployerContainer *foundry.Container,
) {
	t.Log("Setting up stablecoin-evm repository in deployer container...")

	// Clone stablecoin-evm repository
	t.Log("Cloning stablecoin-evm repository...")
	_, _, err := deployerContainer.Exec(
		ctx,
		[]string{
			"bash", "-c",
			fmt.Sprintf("cd %s && git clone --single-branch https://github.com/InjectiveLabs/stablecoin-evm.git", WorkDir),
		},
	)
	require.NoError(t, err, "Failed to clone stablecoin-evm repository")

	// Install dependencies
	t.Log("Installing Node.js dependencies...")
	stdout, stderr, err := deployerContainer.Exec(
		ctx,
		[]string{
			"bash", "-c",
			fmt.Sprintf("cd %s/stablecoin-evm && yarn install", WorkDir),
		},
	)
	if err != nil {
		t.Logf("yarn install stdout: %s", stdout)
		t.Logf("yarn install stderr: %s", stderr)
		require.NoError(t, err, "Failed to install dependencies")
	}

	t.Log("Stablecoin repo setup complete!")
}

// DeployStablecoinContractSuite deploys contracts using a separate deployer container
// Note: Call SetupStablecoinRepo before calling this function
func DeployStablecoinContractSuite(
	t *testing.T,
	ctx context.Context,
	chainNode *cosmos.ChainNode,
	deployerWallet ibc.Wallet,
	config StablecoinConfig,
	deployer *foundry.Container,
) StablecoinContractSuite {

	// Get deployer private key
	privKeyHex := convertWalletToEthPrivateKeyHex(t, deployerWallet)

	// Get chain node hostname in docker network
	chainNodeHostname := chainNode.HostName()

	// Create .env file content with network-accessible endpoints
	envContent := fmt.Sprintf(`# Deployment configuration for FiatToken
# Token configuration
TOKEN_NAME="%s"
TOKEN_SYMBOL="%s"
TOKEN_CURRENCY="%s"
TOKEN_DECIMALS=%d

# Addresses
PROXY_ADMIN_ADDRESS=%s
MASTER_MINTER_OWNER_ADDRESS=%s
OWNER_ADDRESS=%s
PAUSER_ADDRESS=%s
BLACKLISTER_ADDRESS=%s

# Deployment configuration
DEPLOYER_PRIVATE_KEY=%s
ETH_RPC_URL=http://%s:8545
TESTNET_RPC_URL=http://%s:8545
INJ_URL=http://%s:26657
GAS_PRICE=10

# Optional - leave empty to deploy new implementation
FIAT_TOKEN_IMPLEMENTATION_ADDRESS=
`,
		config.TokenName,
		config.TokenSymbol,
		config.TokenCurrency,
		config.TokenDecimals,
		config.ProxyAdmin,
		config.MasterMinterOwner,
		config.Owner,
		config.Pauser,
		config.Blacklister,
		privKeyHex,
		chainNodeHostname,
		chainNodeHostname,
		chainNodeHostname,
	)

	t.Logf("Generated .env file content:\n%s", envContent)

	// Write .env file
	t.Log("Writing .env file...")
	err := deployer.WriteFile(
		ctx,
		fmt.Sprintf("%s/stablecoin-evm/.env", WorkDir),
		envContent,
	)
	require.NoError(t, err, "Failed to write .env file")

	// Run deployment script
	t.Log("Running deployment script...")
	stdout, stderr, err := deployer.Exec(
		ctx,
		[]string{
			"bash", "-c",
			fmt.Sprintf("cd %s/stablecoin-evm && source .env && bash ./scripts/demo/deploy-fiat-token.sh", WorkDir),
		},
	)

	t.Logf("Script stdout:\n%s", stdout)
	if stderr != "" {
		t.Logf("Script stderr:\n%s", stderr)
	}
	require.NoError(t, err, "Deployment script failed")

	// Parse output
	suite := parseDeploymentOutput(t, stdout)

	t.Logf("Deployment completed successfully!")
	t.Logf("Implementation address: %s", suite.Implementation)
	t.Logf("Proxy address: %s", suite.Proxy)
	t.Logf("MasterMinter address: %s", suite.MasterMinter)

	return suite
}

func parseDeploymentOutput(t *testing.T, output string) StablecoinContractSuite {
	var suite StablecoinContractSuite

	// Parse the final summary section for contract addresses
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Look for the deployed contracts section
		if strings.Contains(line, "Deployed Contracts:") {
			// Parse the next few lines for addresses
			for j := i + 1; j < len(lines) && j < i+10; j++ {
				addrLine := strings.TrimSpace(lines[j])

				if strings.Contains(addrLine, "SignatureChecker:") {
					addr := extractAddress(addrLine)
					if addr != (common.Address{}) {
						suite.SignatureChecker = addr
					}
				} else if strings.Contains(addrLine, "Implementation:") {
					addr := extractAddress(addrLine)
					if addr != (common.Address{}) {
						suite.Implementation = addr
					}
				} else if strings.Contains(addrLine, "Proxy:") {
					addr := extractAddress(addrLine)
					if addr != (common.Address{}) {
						suite.Proxy = addr
					}
				} else if strings.Contains(addrLine, "MasterMinter:") {
					addr := extractAddress(addrLine)
					if addr != (common.Address{}) {
						suite.MasterMinter = addr
					}
				}
			}
			break
		}
	}

	require.NotEqual(t, common.Address{}, suite.Implementation, "Failed to parse implementation address from script output")
	require.NotEqual(t, common.Address{}, suite.Proxy, "Failed to parse proxy address from script output")
	require.NotEqual(t, common.Address{}, suite.MasterMinter, "Failed to parse master minter address from script output")

	return suite
}

func extractAddress(line string) common.Address {
	// Look for hex addresses in the line (0x followed by 40 hex chars)
	re := regexp.MustCompile(`0x[a-fA-F0-9]{40}`)
	matches := re.FindStringSubmatch(line)

	if len(matches) > 0 {
		return common.HexToAddress(matches[0])
	}

	return common.Address{}
}
