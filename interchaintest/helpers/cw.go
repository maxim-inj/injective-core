package helpers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	wasmStoreGas       = 3_000_000
	wasmInstantiateGas = 500_000
)

// WasmSetupContract stores and instantiates a contract using gRPC message broadcasting.
func WasmSetupContract(
	t *testing.T,
	ctx context.Context,
	chain *cosmos.CosmosChain,
	user ibc.Wallet,
	fileLoc string,
	message string,
) (codeId, contract string) {
	t.Helper()

	// Read and compress wasm file
	wasmCode, err := os.ReadFile(fileLoc)
	require.NoError(t, err, "failed to read wasm file: %s", fileLoc)

	// Compress the wasm code using gzip
	var compressedCode bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedCode)
	_, err = gzipWriter.Write(wasmCode)
	require.NoError(t, err, "failed to write wasm code to gzip writer")
	err = gzipWriter.Close()
	require.NoError(t, err, "failed to close gzip writer")

	// Store the contract
	storeMsg := &wasmtypes.MsgStoreCode{
		Sender:       user.FormattedAddress(),
		WASMByteCode: compressedCode.Bytes(),
	}

	storeResp := BroadcastTxBlock(t, ctx, chain, user, []cosmos.FactoryOpt{WithGas(wasmStoreGas)}, storeMsg)
	require.EqualValues(t, uint32(0), storeResp.Code, "store contract tx failed: %s", storeResp.RawLog)

	// Extract code ID from events
	var codeID uint64
	for _, event := range storeResp.Events {
		if event.Type == "store_code" {
			for _, attr := range event.Attributes {
				if attr.Key == "code_id" {
					codeID, err = strconv.ParseUint(attr.Value, 10, 64)
					require.NoError(t, err, "failed to parse code_id from event")
					break
				}
			}
		}
	}
	require.NotZero(t, codeID, "code_id not found in store_code event")

	// Instantiate the contract
	instantiateMsg := &wasmtypes.MsgInstantiateContract{
		Sender: user.FormattedAddress(),
		Admin:  user.FormattedAddress(),
		CodeID: codeID,
		Label:  "contract",
		Msg:    wasmtypes.RawContractMessage(message),
		Funds:  nil,
	}

	instantiateResp := BroadcastTxBlock(t, ctx, chain, user, []cosmos.FactoryOpt{WithGas(wasmInstantiateGas)}, instantiateMsg)
	require.EqualValues(t, uint32(0), instantiateResp.Code, "instantiate contract tx failed: %s", instantiateResp.RawLog)

	// Extract contract address from events
	var contractAddr string
	for _, event := range instantiateResp.Events {
		if event.Type == "instantiate" {
			for _, attr := range event.Attributes {
				if attr.Key == "_contract_address" {
					contractAddr = attr.Value
					break
				}
			}
		}
	}
	require.NotEmpty(t, contractAddr, "contract_address not found in instantiate event")

	return strconv.FormatUint(codeID, 10), contractAddr
}

// WasmExecuteMsgWithAmount executes a contract with a given amount.
// NOTE: this method is commented out because it executes CLI commands instead of gRPC queries, and has no callers
// CLI commands have a big impact in the performance
// func WasmExecuteMsgWithAmount(
// 	t *testing.T,
// 	ctx context.Context,
// 	chain *cosmos.CosmosChain,
// 	user ibc.Wallet,
// 	contractAddr, amount, message string,
// ) (txHash string) {
// 	cmd := []string{
// 		"wasm", "execute", contractAddr, message,
// 		"--gas", "500000",
// 		"--amount", amount,
// 	}

// 	chainNode := chain.Nodes()[0]
// 	txHash, err := chainNode.ExecTx(ctx, user.KeyName(), cmd...)
// 	require.NoError(t, err)

// 	stdout, _, err := chainNode.ExecQuery(ctx, "tx", txHash)
// 	require.NoError(t, err)

// 	debugOutput(t, string(stdout))

// 	return txHash
// }

const wasmExecuteGas = 500_000

// WasmExecuteMsgWithFee executes a contract with a given fee using gRPC.
// This function broadcasts the transaction via gRPC and waits for it to be included in a block.
func WasmExecuteMsgWithFee(
	t *testing.T,
	ctx context.Context,
	chain *cosmos.CosmosChain,
	user ibc.Wallet,
	contractAddr string,
	funds, fees sdk.Coin,
	message string,
) (txHash string) {
	t.Helper()

	// Build the wasm execute message
	msg := &wasmtypes.MsgExecuteContract{
		Sender:   user.FormattedAddress(),
		Contract: contractAddr,
		Msg:      wasmtypes.RawContractMessage(message),
		Funds:    sdk.NewCoins(funds),
	}

	// Configure factory options with gas and fees
	// When providing explicit fees, gas prices must be set to empty to avoid conflict
	factoryOpts := []cosmos.FactoryOpt{WithFees(fees.String())}

	// Broadcast transaction and wait for it to be included in a block
	resp := BroadcastTxBlock(t, ctx, chain, user, factoryOpts, msg)
	require.Equal(t, uint32(0), resp.Code, "wasm execute tx failed: %s", resp.RawLog)

	debugOutput(t, resp.RawLog)

	return resp.TxHash
}

// WasmQueryContractState queries a contract using gRPC and unmarshals the response into the given response container pointer.
// E.g. WasmQueryContractState(t, ctx, chain, contract, &GetTotalAmountLockedQuery{}, &GetTotalAmountLockedResponse{})
func WasmQueryContractState(
	t *testing.T,
	ctx context.Context,
	chain *cosmos.CosmosChain,
	contract string,
	queryMsg, response any,
) {
	t.Helper()

	// Marshal the query message to JSON bytes
	queryData, err := json.Marshal(queryMsg)
	require.NoError(t, err, "failed to marshal query message")

	// Create gRPC connection
	conn, err := grpc.NewClient(chain.GetHostGRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "failed to create gRPC connection")
	defer conn.Close()

	// Create wasm query client and execute query
	queryClient := wasmtypes.NewQueryClient(conn)
	resp, err := QueryRPC(ctx, queryClient.SmartContractState, &wasmtypes.QuerySmartContractStateRequest{
		Address:   contract,
		QueryData: queryData,
	})
	require.NoError(t, err, "error querying contract (%s) state", contract)

	debugOutput(t, string(resp.Data))

	// Unmarshal the response data into the provided response container
	err = json.Unmarshal(resp.Data, response)
	require.NoError(t, err, "failed to unmarshal contract response")
}
