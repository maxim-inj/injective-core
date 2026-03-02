package types

import (
	"regexp"
	"strconv"

	abci "github.com/cometbft/cometbft/abci/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
)

var msgIndexRegex = regexp.MustCompile(`message index:\s*(\d+)`)
var failedEthereumTxMarkerRegex = regexp.MustCompile(`"tx_hash"|"txHash"|"vm_error"|"vmError"|MsgEthereumTx|/injective\.evm\.v1\.MsgEthereumTx|ethereum_tx`)

// PatchTxResponses fills the evm tx index and log indexes in the tx result
func PatchTxResponses(input []*abci.ExecTxResult) []*abci.ExecTxResult {
	var (
		txIndex  uint64
		logIndex uint64
	)
	for _, res := range input {
		ethTxCount := countEthereumTxEvents(res.Events)
		inferredEthTxCount := inferFailedEthereumTxCount(res.Log)
		if inferredEthTxCount > ethTxCount {
			ethTxCount = inferredEthTxCount
		}
		if res.Code != 0 {
			// Failed txs still consume ethereum transaction indexes within the block.
			txIndex += uint64(ethTxCount)
			continue
		}

		var txMsgData sdk.TxMsgData
		if err := proto.Unmarshal(res.Data, &txMsgData); err != nil {
			panic(err)
		}

		var (
			// if the response data is modified and need to be marshaled back
			dataDirty bool
			seenEthTx uint64
		)

		for i, rsp := range txMsgData.MsgResponses {
			var response MsgEthereumTxResponse
			if rsp.TypeUrl != "/"+proto.MessageName(&response) {
				continue
			}
			seenEthTx++

			if err := proto.Unmarshal(rsp.Value, &response); err != nil {
				panic(err)
			}

			if len(response.Logs) > 0 {
				for _, log := range response.Logs {
					log.TxIndex = txIndex
					log.Index = logIndex
					logIndex++
				}

				anyRsp, err := codectypes.NewAnyWithValue(&response)
				if err != nil {
					panic(err)
				}
				txMsgData.MsgResponses[i] = anyRsp

				dataDirty = true
			}

			txIndex++
		}
		if seenEthTx < uint64(ethTxCount) {
			// Keep tx indexes monotonic when response payload has fewer ethereum responses than events.
			txIndex += uint64(ethTxCount) - seenEthTx
		}

		if dataDirty {
			data, err := proto.Marshal(&txMsgData)
			if err != nil {
				panic(err)
			}

			res.Data = data
		}
	}

	return input
}

func countEthereumTxEvents(events []abci.Event) int {
	count := 0
	for _, event := range events {
		if event.Type == EventTypeEthereumTx {
			count++
		}
	}
	return count
}

func inferFailedEthereumTxCount(log string) int {
	match := msgIndexRegex.FindStringSubmatch(log)
	if len(match) != 2 {
		return 0
	}
	if !failedEthereumTxMarkerRegex.MatchString(log) {
		return 0
	}

	msgIndex, err := strconv.Atoi(match[1])
	if err != nil || msgIndex < 0 {
		return 0
	}

	// Message index is 0-based, tx count is index+1.
	return msgIndex + 1
}
