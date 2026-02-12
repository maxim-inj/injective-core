package helpers

import (
	"context"
	"fmt"
	"time"

	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// type Tally struct {
// 	AbstainCount    math.Int `json:"abstain_count"`
// 	NoCount         math.Int `json:"no_count"`
// 	NoWithVetoCount math.Int `json:"no_with_veto_count"`
// 	YesCount        math.Int `json:"yes_count"`
// }

// QueryProposalTally gets tally results for a proposal
// NOTE: this method is commented out because it executes CLI commands instead of gRPC queries, and has no callers
// CLI commands have a big impact in the performance
// func QueryProposalTally(t *testing.T, ctx context.Context, chainNode *cosmos.ChainNode, proposalID string) Tally {
// 	stdout, _, err := chainNode.ExecQuery(ctx, "gov", "tally", proposalID)
// 	require.NoError(t, err)

// 	debugOutput(t, string(stdout))

// 	var tally Tally
// 	err = json.Unmarshal(stdout, &tally)
// 	require.NoError(t, err)

// 	return tally
// }

// QueryProposalTx reads results of a proposal Tx, useful to get the ProposalID.
// Uses gRPC to query the transaction.
func QueryProposalTx(ctx context.Context, chainNode *cosmos.ChainNode, txHash string) (tx cosmos.TxProposal, _ error) {
	resultTx, err := getTxResponseRPC(ctx, chainNode, txHash)
	if err != nil {
		return tx, fmt.Errorf("failed to get transaction %s: %w", txHash, err)
	}

	if resultTx.Code != 0 {
		return tx, fmt.Errorf("proposal transaction error: code %d %s", resultTx.Code, resultTx.RawLog)
	}

	tx.Height = resultTx.Height
	tx.TxHash = txHash
	// In cosmos, user is charged for entire gas requested, not the actual gas used.
	tx.GasSpent = resultTx.GasWanted
	events := resultTx.Events

	tx.DepositAmount = cometAttributeValue(events, "proposal_deposit", "amount")

	evtSubmitProp := "submit_proposal"
	tx.ProposalID = cometAttributeValue(events, evtSubmitProp, "proposal_id")
	tx.ProposalType = cometAttributeValue(events, evtSubmitProp, "proposal_type")

	return tx, nil
}

// QueryProposalRPC queries a proposal via gRPC
func QueryProposalRPC(ctx context.Context, chain *cosmos.CosmosChain, proposalID uint64) (*govv1beta1.Proposal, error) {
	conn, err := grpc.NewClient(chain.GetHostGRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	queryClient := govv1beta1.NewQueryClient(conn)

	resp, err := QueryRPC(ctx, queryClient.Proposal, &govv1beta1.QueryProposalRequest{
		ProposalId: proposalID,
	})
	if err != nil {
		return nil, err
	}

	return &resp.Proposal, nil
}

// WaitForProposalStatusByTime polls the gov module via RPC until the proposal reaches the expected status or timeouts.
func WaitForProposalStatusByTime(
	ctx context.Context,
	chain *cosmos.CosmosChain,
	proposalID uint64,
	expectedStatus govv1beta1.ProposalStatus,
	timeout time.Duration,
	pollInterval time.Duration,
) (*govv1beta1.Proposal, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be positive")
	}
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastProposal *govv1beta1.Proposal
	var lastErr error

	for {
		proposal, err := QueryProposalRPC(ctx, chain, proposalID)
		if err != nil {
			lastErr = err
		} else {
			lastProposal = proposal
			if proposal.Status == expectedStatus {
				return proposal, nil
			}
		}

		select {
		case <-ctx.Done():
			switch {
			case lastProposal != nil:
				return lastProposal, fmt.Errorf(
					"timed out waiting for proposal %d to reach status %s (last status: %s)",
					proposalID,
					expectedStatus.String(),
					lastProposal.Status.String(),
				)
			case lastErr != nil:
				return nil, fmt.Errorf(
					"timed out waiting for proposal %d to reach status %s: last error: %w",
					proposalID,
					expectedStatus.String(),
					lastErr,
				)
			default:
				return nil, fmt.Errorf(
					"timed out waiting for proposal %d to reach status %s: %w",
					proposalID,
					expectedStatus.String(),
					ctx.Err(),
				)
			}
		case <-ticker.C:
			continue
		}
	}
}
