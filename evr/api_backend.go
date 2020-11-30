// Copyright 2015 The evrynet-node Authors
// This file is part of the evrynet-node library.
//
// The evrynet-node library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The evrynet-node library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the evrynet-node library. If not, see <http://www.gnu.org/licenses/>.

package evr

import (
	"context"
	"errors"
	"math/big"

	"github.com/Evrynetlabs/evrynet-node/accounts"
	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/common/math"
	"github.com/Evrynetlabs/evrynet-node/core"
	"github.com/Evrynetlabs/evrynet-node/core/bloombits"
	"github.com/Evrynetlabs/evrynet-node/core/rawdb"
	"github.com/Evrynetlabs/evrynet-node/core/state"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/core/vm"
	"github.com/Evrynetlabs/evrynet-node/event"
	"github.com/Evrynetlabs/evrynet-node/evr/downloader"
	"github.com/Evrynetlabs/evrynet-node/evr/gasprice"
	"github.com/Evrynetlabs/evrynet-node/evrdb"
	"github.com/Evrynetlabs/evrynet-node/params"
	"github.com/Evrynetlabs/evrynet-node/rpc"
)

// EvrAPIBackend implements evrapi.Backend for full nodes
type EvrAPIBackend struct {
	extRPCEnabled bool
	evr           *Evrynet
	gpo           *gasprice.Oracle
}

// ChainConfig returns the active chain configuration.
func (b *EvrAPIBackend) ChainConfig() *params.ChainConfig {
	return b.evr.blockchain.Config()
}

func (b *EvrAPIBackend) CurrentBlock() *types.Block {
	return b.evr.blockchain.CurrentBlock()
}

func (b *EvrAPIBackend) SetHead(number uint64) {
	b.evr.protocolManager.downloader.Cancel()
	b.evr.blockchain.SetHead(number)
}

func (b *EvrAPIBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.evr.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.evr.blockchain.CurrentBlock().Header(), nil
	}
	return b.evr.blockchain.GetHeaderByNumber(uint64(blockNr)), nil
}

// Test by lvbin
func (b *EvrAPIBackend) FHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	//if blockNr == rpc.PendingBlockNumber {
	//	block := b.evr.miner.PendingBlock()
	//	return block.Header(), nil
	//}
	//// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.evr.fBlockchain.CurrentBlock().Header(), nil
	}
	return b.evr.fBlockchain.GetHeaderByNumber(uint64(blockNr)), nil
}


func (b *EvrAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.evr.blockchain.GetHeaderByHash(hash), nil
}

func (b *EvrAPIBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.evr.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.evr.blockchain.CurrentBlock(), nil
	}
	return b.evr.blockchain.GetBlockByNumber(uint64(blockNr)), nil
}


// Test by lvbin
func (b *EvrAPIBackend) FBlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.evr.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.evr.fBlockchain.CurrentBlock(), nil
	}
	return b.evr.fBlockchain.GetBlockByNumber(uint64(blockNr)), nil
}

func (b *EvrAPIBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block, state := b.evr.miner.Pending()
		return state, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, blockNr)
	if err != nil {
		return nil, nil, err
	}
	if header == nil {
		return nil, nil, errors.New("header not found")
	}
	stateDb, err := b.evr.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *EvrAPIBackend) GetBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.evr.blockchain.GetBlockByHash(hash), nil
}

// Test by lvbin
func (b *EvrAPIBackend) FGetBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.evr.fBlockchain.GetBlockByHash(hash), nil
}

func (b *EvrAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.evr.blockchain.GetReceiptsByHash(hash), nil
}

func (b *EvrAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	receipts := b.evr.blockchain.GetReceiptsByHash(hash)
	if receipts == nil {
		return nil, nil
	}
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (b *EvrAPIBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.evr.blockchain.GetTdByHash(blockHash)
}

func (b *EvrAPIBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header) (*vm.EVM, func() error, error) {
	state.SetBalance(msg.From(), math.MaxBig256)
	vmError := func() error { return nil }

	context := core.NewEVMContext(msg, header, b.evr.BlockChain(), nil)
	return vm.NewEVM(context, state, b.evr.blockchain.Config(), *b.evr.blockchain.GetVMConfig()), vmError, nil
}

func (b *EvrAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.evr.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *EvrAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.evr.BlockChain().SubscribeChainEvent(ch)
}

func (b *EvrAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.evr.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *EvrAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.evr.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *EvrAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.evr.BlockChain().SubscribeLogsEvent(ch)
}

func (b *EvrAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.evr.txPool.AddLocal(signedTx)
}

func (b *EvrAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.evr.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *EvrAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.evr.txPool.Get(hash)
}

func (b *EvrAPIBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(b.evr.ChainDb(), txHash, false)
	return tx, blockHash, blockNumber, index, nil
}

func (b *EvrAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.evr.txPool.State().GetNonce(addr), nil
}

func (b *EvrAPIBackend) Stats() (pending int, queued int) {
	return b.evr.txPool.Stats()
}

func (b *EvrAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.evr.TxPool().Content()
}

func (b *EvrAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.evr.TxPool().SubscribeNewTxsEvent(ch)
}

func (b *EvrAPIBackend) Downloader() *downloader.Downloader {
	return b.evr.Downloader()
}

func (b *EvrAPIBackend) ProtocolVersion() int {
	return b.evr.EthVersion()
}

func (b *EvrAPIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}

func (b *EvrAPIBackend) ChainDb() evrdb.Database {
	return b.evr.ChainDb()
}

func (b *EvrAPIBackend) EventMux() *event.TypeMux {
	return b.evr.EventMux()
}

func (b *EvrAPIBackend) AccountManager() *accounts.Manager {
	return b.evr.AccountManager()
}

func (b *EvrAPIBackend) ExtRPCEnabled() bool {
	return b.extRPCEnabled
}

func (b *EvrAPIBackend) RPCGasCap() *big.Int {
	return b.evr.config.RPCGasCap
}

func (b *EvrAPIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.evr.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

func (b *EvrAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.evr.bloomRequests)
	}
}
