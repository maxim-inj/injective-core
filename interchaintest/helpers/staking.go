package helpers

import (
	"context"
	"testing"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// QueryAllValidators lists all validators using gRPC.
func QueryAllValidators(t *testing.T, ctx context.Context, chain *cosmos.CosmosChain) []stakingtypes.Validator {
	t.Helper()

	conn, err := grpc.NewClient(chain.GetHostGRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "failed to create gRPC connection")
	defer conn.Close()

	queryClient := stakingtypes.NewQueryClient(conn)
	resp, err := QueryRPC(ctx, queryClient.Validators, &stakingtypes.QueryValidatorsRequest{})
	require.NoError(t, err, "error querying validators")

	return resp.Validators
}

// QueryValidator gets info about particular validator using gRPC.
func QueryValidator(
	t *testing.T,
	ctx context.Context,
	chain *cosmos.CosmosChain,
	valoperAddr string,
) stakingtypes.Validator {
	t.Helper()

	conn, err := grpc.NewClient(chain.GetHostGRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "failed to create gRPC connection")
	defer conn.Close()

	queryClient := stakingtypes.NewQueryClient(conn)
	resp, err := QueryRPC(ctx, queryClient.Validator, &stakingtypes.QueryValidatorRequest{
		ValidatorAddr: valoperAddr,
	})
	require.NoError(t, err, "error querying validator")

	return resp.Validator
}

// QueryDelegation gets info about particular delegation using gRPC.
func QueryDelegation(
	t *testing.T,
	ctx context.Context,
	chain *cosmos.CosmosChain,
	delegatorAddr string,
	valoperAddr string,
) stakingtypes.Delegation {
	t.Helper()

	conn, err := grpc.NewClient(chain.GetHostGRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "failed to create gRPC connection")
	defer conn.Close()

	queryClient := stakingtypes.NewQueryClient(conn)
	resp, err := QueryRPC(ctx, queryClient.Delegation, &stakingtypes.QueryDelegationRequest{
		DelegatorAddr: delegatorAddr,
		ValidatorAddr: valoperAddr,
	})
	require.NoError(t, err, "error querying delegation")
	require.NotNil(t, resp.DelegationResponse, "delegation response is nil")

	return resp.DelegationResponse.Delegation
}
