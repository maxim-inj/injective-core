package helpers

import (
	"context"
	"testing"

	ibchookskeeper "github.com/cosmos/ibc-apps/modules/ibc-hooks/v8/keeper"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/stretchr/testify/require"
)

// GetIBCHooksUserAddress derives the intermediate sender address for IBC hooks.
// This uses the ibchookskeeper.DeriveIntermediateSender function from the ibc-hooks library,
// which deterministically derives an address from the channel ID and original sender.
func GetIBCHooksUserAddress(
	t *testing.T,
	_ context.Context,
	chain *cosmos.CosmosChain,
	channel, uaddr string,
) string {
	t.Helper()

	bech32Prefix := chain.Config().Bech32Prefix
	intermediateAddr, err := ibchookskeeper.DeriveIntermediateSender(channel, uaddr, bech32Prefix)
	require.NoError(t, err, "failed to derive intermediate sender address")

	return intermediateAddr
}

// GetIBCHookTotalFunds queries the total funds for an address using gRPC.
func GetIBCHookTotalFunds(t *testing.T, ctx context.Context, chain *cosmos.CosmosChain, contract string, uaddr string) IbcHooksGetTotalFundsResponse {
	t.Helper()

	// Query returns the raw contract response without a "data" wrapper
	var data IbcHooksGetTotalFundsObj
	WasmQueryContractState(t, ctx, chain, contract,
		IbcHooksQueryMsg{
			GetTotalFunds: &IbcHooksGetTotalFundsQuery{
				Addr: uaddr,
			},
		},
		&data)

	return IbcHooksGetTotalFundsResponse{Data: &data}
}

// GetIBCHookCount queries the count for an address using gRPC.
func GetIBCHookCount(t *testing.T, ctx context.Context, chain *cosmos.CosmosChain, contract string, uaddr string) IbcHooksGetCountResponse {
	t.Helper()

	// Query returns the raw contract response without a "data" wrapper
	var data IbcHooksGetCountObj
	WasmQueryContractState(t, ctx, chain, contract,
		IbcHooksQueryMsg{
			GetCount: &IbcHooksGetCountQuery{
				Addr: uaddr,
			},
		},
		&data)

	return IbcHooksGetCountResponse{Data: data}
}
