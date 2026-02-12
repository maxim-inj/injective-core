package circle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

	"github.com/ethereum/go-ethereum/common"
)

// Local types for JSON marshaling to avoid importing cosmos-sdk types that
// conflict with sdk-go
type namespaceConfig struct {
	Denom           string        `json:"denom"`
	EvmHook         string        `json:"evm_hook"`
	RolePermissions []*role       `json:"role_permissions"`
	ActorRoles      []*actorRoles `json:"actor_roles"`
}

type role struct {
	Name        string `json:"name"`
	RoleId      uint32 `json:"role_id"`
	Permissions uint32 `json:"permissions"`
}

type actorRoles struct {
	Actor string   `json:"actor"`
	Roles []string `json:"roles"`
}

// CreatePermissionsNamespace creates a permissions namespace on-chain using CLI
// The transaction has to be signed by the ERC20 token Owner
func CreatePermissionsNamespace(
	t *testing.T,
	ctx context.Context,
	chain *cosmos.CosmosChain,
	ownerWallet ibc.Wallet,
	denom string,
	hookAddress common.Address,
) {
	ownerAddress := ownerWallet.FormattedAddress()

	t.Logf("Creating permissions namespace on-chain...")
	t.Logf("Denom: %s", denom)
	t.Logf("Hook Address: %s", hookAddress.Hex())
	t.Logf("Admin Address: %s", ownerAddress)

	// Create namespace configuration
	// Permission values are bitmasks:
	// MINT = 1, RECEIVE = 2, BURN = 4, SEND = 8, SUPER_BURN = 16
	// Admin permissions = 2013265920 (all modify permissions + basic operations)
	// EVERYONE = 10 (RECEIVE + SEND)
	config := namespaceConfig{
		Denom:   denom,
		EvmHook: hookAddress.Hex(),
		RolePermissions: []*role{
			{
				Name:        "EVERYONE",
				RoleId:      0,
				Permissions: 10, // RECEIVE + SEND
			},
			{
				Name:        "admin",
				RoleId:      1,
				Permissions: 2013265920, // All admin permissions
			},
		},
		ActorRoles: []*actorRoles{
			{
				Actor: ownerAddress,
				Roles: []string{"admin"},
			},
		},
	}

	// Serialize to JSON
	configJSON, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err, "Failed to marshal namespace config")

	// Create temporary file
	tempFile, err := os.CreateTemp("", "namespace-*.json")
	require.NoError(t, err, "Failed to create temp file")
	defer os.Remove(tempFile.Name())

	// Write config to file
	_, err = tempFile.Write(configJSON)
	require.NoError(t, err, "Failed to write config to file")
	require.NoError(t, tempFile.Close(), "Failed to close temp file")

	t.Logf("Created namespace config file: %s", tempFile.Name())

	// Copy file to container working directory where it can be accessed
	// Use the node's home directory instead of chain ID
	nodeHomeDir := chain.Nodes()[0].HomeDir()
	containerPath := fmt.Sprintf("%s/namespace.json", nodeHomeDir)
	t.Logf("Copying file from %s to container path %s (node home: %s)", tempFile.Name(), containerPath, nodeHomeDir)
	err = chain.Nodes()[0].CopyFile(ctx, tempFile.Name(), "namespace.json")
	require.NoError(t, err, "Failed to copy namespace file to container")
	t.Logf("File copied successfully to container")

	// Verify file exists in container by checking if we can cat it
	t.Logf("Verifying file exists in container...")
	catResult, _, err := chain.Nodes()[0].Exec(ctx, []string{"cat", containerPath}, nil)
	t.Logf("File verification result: %s", string(catResult))
	if err != nil {
		t.Logf("File verification error: %v", err)
		// Try listing the directory to see what's there
		dirResult, _, dirErr := chain.Nodes()[0].Exec(ctx, []string{"ls", "-la", "/var/cosmos-chain/"}, nil)
		t.Logf("Directory listing: %s", string(dirResult))
		if dirErr != nil {
			t.Logf("Directory listing error: %v", dirErr)
		}
	}

	// Execute namespace creation command
	t.Logf("Executing namespace creation command...")
	txResult, err := chain.Nodes()[0].ExecTx(ctx, ownerWallet.KeyName(),
		"permissions", "create-namespace", containerPath,
		"--gas", "auto",
	)

	require.NoError(t, err, "Failed to create namespace")
	t.Logf("Permissions namespace created successfully!")
	t.Logf("Transaction result: %s", txResult)
}
