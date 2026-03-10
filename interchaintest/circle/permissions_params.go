package circle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"

	"github.com/InjectiveLabs/injective-core/interchaintest/helpers"
)

// proposalJSON matches the cosmos-sdk v0.50 JSON format accepted by
// `injectived tx gov submit-proposal <file.json>`.
type proposalJSON struct {
	Messages []json.RawMessage `json:"messages"`
	Metadata string            `json:"metadata"`
	Deposit  string            `json:"deposit"`
	Title    string            `json:"title"`
	Summary  string            `json:"summary"`
}

// SetEnforcedRestrictionsEvmContracts submits a governance proposal via the CLI
// to update the permissions module params, registering the given EVM contract
// as an enforced restrictions contract. This enables the permissions module's
// PostTxProcessing hook to detect Pause/Unpause/Blacklist/Unblacklist events
// and notify registered listeners (e.g. the exchange keeper force-pauses
// derivative markets whose quote denom matches the paused token).
func SetEnforcedRestrictionsEvmContracts(
	t *testing.T,
	ctx context.Context,
	chain *cosmos.CosmosChain,
	proposer ibc.Wallet,
	contractAddress common.Address,
) {
	t.Helper()

	// Build the MsgUpdateParams as raw JSON — avoids importing the local
	// permissions types which cause init() registration conflicts with sdk-go.
	govAuthority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	msgUpdateParams := map[string]interface{}{
		"@type":     "/injective.permissions.v1beta1.MsgUpdateParams",
		"authority": govAuthority,
		"params": map[string]interface{}{
			"contract_hook_max_gas": "1000000",
			"enforced_restrictions_evm_contracts": []map[string]string{
				{
					"contract_address":            contractAddress.Hex(),
					"pause_event_signature":       "Pause()",
					"unpause_event_signature":     "Unpause()",
					"blacklist_event_signature":   "Blacklisted(address)",
					"unblacklist_event_signature": "UnBlacklisted(address)",
				},
			},
		},
	}

	msgJSON, err := json.Marshal(msgUpdateParams)
	require.NoError(t, err)

	proposal := proposalJSON{
		Messages: []json.RawMessage{msgJSON},
		Metadata: "",
		Deposit:  "10000000000000000000inj", // 10 INJ (18 decimals)
		Title:    "Set enforced restrictions EVM contracts",
		Summary:  "Register USDC proxy contract for enforced restrictions (pause/blacklist)",
	}

	proposalBytes, err := json.MarshalIndent(proposal, "", "  ")
	require.NoError(t, err)

	// Write to temp file and copy into the container
	tmpFile, err := os.CreateTemp("", "proposal-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write(proposalBytes)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	node := chain.Nodes()[0]
	containerPath := fmt.Sprintf("%s/proposal-permissions-params.json", node.HomeDir())
	err = node.CopyFile(ctx, tmpFile.Name(), "proposal-permissions-params.json")
	require.NoError(t, err, "failed to copy proposal file to container")

	// Submit the proposal
	txResult, err := node.ExecTx(ctx, proposer.KeyName(),
		"gov", "submit-proposal", containerPath,
		"--gas", "auto",
	)
	require.NoError(t, err, "failed to submit permissions params proposal")
	t.Logf("Proposal submitted, tx: %s", txResult)

	// Extract proposal ID from the tx result
	proposalID := extractProposalIDFromTxHash(t, ctx, node, txResult)

	// Vote yes with all validators
	_, err = helpers.VoteOnProposalAllValidatorsRPC(t, ctx, chain, proposalID, govv1.VoteOption_VOTE_OPTION_YES)
	require.NoError(t, err, "failed to vote on permissions params proposal")

	// Wait for proposal to pass
	_, err = helpers.WaitForProposalStatusByTime(ctx, chain, proposalID, govv1beta1.StatusPassed, 45*time.Second, 500*time.Millisecond)
	require.NoError(t, err, "permissions params proposal did not pass")

	t.Logf("Successfully set enforced restrictions EVM contracts param for contract: %s", contractAddress.Hex())
}

func extractProposalIDFromTxHash(t *testing.T, ctx context.Context, node *cosmos.ChainNode, txHash string) uint64 {
	t.Helper()

	proposalTx, err := helpers.QueryProposalTx(ctx, node, txHash)
	require.NoError(t, err, "failed to query proposal tx")

	proposalID, err := strconv.ParseUint(proposalTx.ProposalID, 10, 64)
	require.NoError(t, err, "failed to parse proposal ID")

	t.Logf("Proposal ID: %d", proposalID)
	return proposalID
}
