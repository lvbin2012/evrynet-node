package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Evrynetlabs/evrynet-node/rpc"
)

const (
	RPCEndpoint        = "http://localhost:22001"
	MethodNotFoundCode = -32601 // the code of method not found error
)

func TestAllApis(t *testing.T) {

	rpcClient, err := rpc.Dial(RPCEndpoint)
	require.NoError(t, err)

	var (
		prefix  = "eth_"
		methods = []string{
			"protocolVersion", "syncing", "coinbase", "mining", "hashrate", "gasPrice", "accounts",
			"blockNumber", "getStorageAt", "getTransactionCount", "getBlockTransactionCountByHash",
			"getBlockTransactionCountByNumber", "getUncleCountByBlockHash", "getUncleCountByBlockNumber",
			"getCode", "sign", "providerSignTransaction", "sendTransaction", "sendRawTransaction",
			"estimateGas", "getBlockByHash", "getBlockSignerByHash", "getBlockByNumber",
			"getBlockSignerByNumber", "getTransactionByHash", "getTransactionByBlockHashAndIndex",
			"getTransactionByBlockNumberAndIndex", "getTransactionReceipt", "getUncleByBlockHashAndIndex",
			"newFilter",
			"newBlockFilter", "newPendingTransactionFilter", "uninstallFilter", "getFilterChanges",
			"getFilterLogs", "getLogs",
		}
	)

	for _, element := range methods {
		if element == "" {
			continue
		}
		method := prefix + element
		err = rpcClient.Call(nil, method)

		if err != nil {
			if jsonErr, ok := err.(rpc.Error); ok {
				if jsonErr.ErrorCode() == MethodNotFoundCode {
					t.Error(jsonErr)
				}
			} else {
				t.Error(err)
			}
		}

		time.Sleep(200 * time.Millisecond)
	}
}
