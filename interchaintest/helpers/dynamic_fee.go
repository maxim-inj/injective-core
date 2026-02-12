package helpers

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	txfeestypes "github.com/InjectiveLabs/sdk-go/chain/txfees/types"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GetDynamicGasPrice returns the dynamic gas price for a EIP-1559 compatible chain using gRPC.
func GetDynamicGasPrice(
	t *testing.T,
	ctx context.Context,
	chain *cosmos.CosmosChain,
) sdkmath.LegacyDec {
	t.Helper()

	// Create gRPC connection
	conn, err := grpc.NewClient(chain.GetHostGRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "failed to create gRPC connection")
	defer conn.Close()

	// Create txfees query client and execute query
	queryClient := txfeestypes.NewQueryClient(conn)
	resp, err := QueryRPC(ctx, queryClient.GetEipBaseFee, &txfeestypes.QueryEipBaseFeeRequest{})
	require.NoError(t, err, "error querying EIP base fee")
	require.NotNil(t, resp.BaseFee, "base fee is nil")

	return resp.BaseFee.BaseFee
}

// GetTxFeesParams returns the parameters of the txfees module using gRPC.
func GetTxFeesParams(
	t *testing.T,
	ctx context.Context,
	chain *cosmos.CosmosChain,
) *txfeestypes.Params {
	t.Helper()

	// Create gRPC connection
	conn, err := grpc.NewClient(chain.GetHostGRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "failed to create gRPC connection")
	defer conn.Close()

	// Create txfees query client and execute query
	queryClient := txfeestypes.NewQueryClient(conn)
	resp, err := QueryRPC(ctx, queryClient.Params, &txfeestypes.QueryParamsRequest{})
	require.NoError(t, err, "error querying txfees params")

	return &resp.Params
}
