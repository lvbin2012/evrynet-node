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

package core

import (
	"bytes"
	"fmt"
	"math/big"
	"math/rand"
	"runtime"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus"
	fconTypes "github.com/Evrynetlabs/evrynet-node/consensus/fconsensus/types"
	"github.com/Evrynetlabs/evrynet-node/core/state"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/core/vm"
	"github.com/Evrynetlabs/evrynet-node/evrdb"
	"github.com/Evrynetlabs/evrynet-node/params"
	"github.com/Evrynetlabs/evrynet-node/rlp"
	"github.com/pkg/errors"
)

// BlockGen creates blocks for testing.
// See GenerateChain for a detailed explanation.
type BlockGen struct {
	i       int
	parent  *types.Block
	chain   []*types.Block
	header  *types.Header
	statedb *state.StateDB

	gasPool  *GasPool
	txs      []*types.Transaction
	receipts []*types.Receipt
	uncles   []*types.Header

	config *params.ChainConfig
	engine consensus.Engine
}

// SetCoinbase sets the coinbase of the generated block.
// It can be called at most once.
func (b *BlockGen) SetCoinbase(addr common.Address) {
	if b.gasPool != nil {
		if len(b.txs) > 0 {
			panic("coinbase must be set before adding transactions")
		}
		panic("coinbase can only be set once")
	}
	b.header.Coinbase = addr
	b.gasPool = new(GasPool).AddGas(b.header.GasLimit)
}

// SetExtra sets the extra data field of the generated block.
func (b *BlockGen) SetExtra(data []byte) {
	b.header.Extra = data
}

// SetNonce sets the nonce field of the generated block.
func (b *BlockGen) SetNonce(nonce types.BlockNonce) {
	b.header.Nonce = nonce
}

// SetDifficulty sets the difficulty field of the generated block. This method is
// useful for Clique tests where the difficulty does not depend on time. For the
// ethash tests, please use OffsetTime, which implicitly recalculates the diff.
func (b *BlockGen) SetDifficulty(diff *big.Int) {
	b.header.Difficulty = diff
}

// AddTx adds a transaction to the generated block. If no coinbase has
// been set, the block's coinbase is set to the zero address.
//
// AddTx panics if the transaction cannot be executed. In addition to
// the protocol-imposed limitations (gas limit, etc.), there are some
// further limitations on the content of transactions that can be
// added. Notably, contract code relying on the BLOCKHASH instruction
// will panic during execution.
func (b *BlockGen) AddTx(tx *types.Transaction) {
	b.AddTxWithChain(nil, tx)
}

// AddTxWithChain adds a transaction to the generated block. If no coinbase has
// been set, the block's coinbase is set to the zero address.
//
// AddTxWithChain panics if the transaction cannot be executed. In addition to
// the protocol-imposed limitations (gas limit, etc.), there are some
// further limitations on the content of transactions that can be
// added. If contract code relies on the BLOCKHASH instruction,
// the block in chain will be returned.
func (b *BlockGen) AddTxWithChain(bc *BlockChain, tx *types.Transaction) {
	if b.gasPool == nil {
		b.SetCoinbase(common.Address{})
	}
	b.statedb.Prepare(tx.Hash(), common.Hash{}, len(b.txs))
	receipt, _, err := ApplyTransaction(b.config, bc, &b.header.Coinbase, b.gasPool, b.statedb, b.header, tx, &b.header.GasUsed, vm.Config{})
	if err != nil {
		panic(err)
	}
	b.txs = append(b.txs, tx)
	b.receipts = append(b.receipts, receipt)
}

// AddUncheckedTx forcefully adds a transaction to the block without any
// validation.
//
// AddUncheckedTx will cause consensus failures when used during real
// chain processing. This is best used in conjunction with raw block insertion.
func (b *BlockGen) AddUncheckedTx(tx *types.Transaction) {
	b.txs = append(b.txs, tx)
}

// Number returns the block number of the block being generated.
func (b *BlockGen) Number() *big.Int {
	return new(big.Int).Set(b.header.Number)
}

// AddUncheckedReceipt forcefully adds a receipts to the block without a
// backing transaction.
//
// AddUncheckedReceipt will cause consensus failures when used during real
// chain processing. This is best used in conjunction with raw block insertion.
func (b *BlockGen) AddUncheckedReceipt(receipt *types.Receipt) {
	b.receipts = append(b.receipts, receipt)
}

// TxNonce returns the next valid transaction nonce for the
// account at addr. It panics if the account does not exist.
func (b *BlockGen) TxNonce(addr common.Address) uint64 {
	if !b.statedb.Exist(addr) {
		panic("account does not exist")
	}
	return b.statedb.GetNonce(addr)
}

// AddUncle adds an uncle header to the generated block.
func (b *BlockGen) AddUncle(h *types.Header) {
	b.uncles = append(b.uncles, h)
}

// PrevBlock returns a previously generated block by number. It panics if
// num is greater or equal to the number of the block being generated.
// For index -1, PrevBlock returns the parent block given to GenerateChain.
func (b *BlockGen) PrevBlock(index int) *types.Block {
	if index >= b.i {
		panic(fmt.Errorf("block index %d out of range (%d,%d)", index, -1, b.i))
	}
	if index == -1 {
		return b.parent
	}
	return b.chain[index]
}

// OffsetTime modifies the time instance of a block, implicitly changing its
// associated difficulty. It's useful to test scenarios where forking is not
// tied to chain length directly.
func (b *BlockGen) OffsetTime(seconds int64) {
	b.header.Time += uint64(seconds)
	if b.header.Time <= b.parent.Header().Time {
		panic("block time out of range")
	}
	chainreader := &fakeChainReader{config: b.config}
	b.header.Difficulty = b.engine.CalcDifficulty(chainreader, b.header.Time, b.parent.Header())
}

// GenerateChain creates a chain of n blocks. The first block's
// parent will be the provided parent. db is used to store
// intermediate states and should contain the parent's state trie.
//
// The generator function is called with a new block generator for
// every block. Any transactions and uncles added to the generator
// become part of the block. If gen is nil, the blocks will be empty
// and their coinbase will be the zero address.
//
// Blocks created by GenerateChain do not contain valid proof of work
// values. Inserting them into BlockChain requires use of FakePow or
// a similar non-validating proof of work implementation.
func GenerateChain(config *params.ChainConfig, parent *types.Block, engine consensus.Engine, db evrdb.Database, n int, gen func(int, *BlockGen)) ([]*types.Block, []types.Receipts) {
	if config == nil {
		config = params.TestChainConfig
	}
	blocks, receipts := make(types.Blocks, n), make([]types.Receipts, n)
	statedb, err := state.New(parent.Root(), state.NewDatabase(db))
	if err != nil {
		panic(err)
	}
	chainreader := &fakeChainReader{
		config: config,
		blocksByNumber: map[uint64]*types.Block{
			parent.NumberU64(): parent,
		},
		stateByHash: map[common.Hash]*state.StateDB{
			parent.Root(): statedb,
		},
	}
	genblock := func(i int, parent *types.Block, statedb *state.StateDB) (*types.Block, types.Receipts) {
		b := &BlockGen{i: i, chain: blocks, parent: parent, statedb: statedb, config: config, engine: engine}
		b.header = makeHeader(chainreader, parent, statedb, b.engine, 0)

		// Execute any user modifications to the block
		if gen != nil {
			gen(i, b)
		}
		if b.engine != nil {
			// Finalize and seal the block
			block, err := b.engine.FinalizeAndAssemble(chainreader, b.header, statedb, b.txs, b.uncles, b.receipts)
			if err != nil {
				panic(fmt.Sprintf("FinalizeAndAssemble error: %v", err))
			}

			// Write state changes to db
			root, err := statedb.Commit(true)
			if err != nil {
				panic(fmt.Sprintf("state write error: %v", err))
			}
			if err := statedb.Database().TrieDB().Commit(root, false); err != nil {
				panic(fmt.Sprintf("trie write error: %v", err))
			}
			chainreader.blocksByNumber[block.NumberU64()] = block
			chainreader.stateByHash[statedb.IntermediateRoot(true)] = statedb
			return block, b.receipts
		}
		return nil, nil
	}
	for i := 0; i < n; i++ {
		statedb, err := state.New(parent.Root(), state.NewDatabase(db))
		if err != nil {
			panic(err)
		}
		block, receipt := genblock(i, parent, statedb)
		blocks[i] = block
		receipts[i] = receipt
		parent = block
	}
	return blocks, receipts
}

func GenerateTwoChain(config, fConfig *params.ChainConfig, parent, fParent *types.Block, engine, fEngine consensus.Engine,
	db evrdb.Database, n, k int, seed byte, gen func(int, *BlockGen)) ([]*types.Block, []types.Receipts, []*types.Block, []types.Receipts, []*types.Block, []types.Receipts) {
	if n < k {
		panic("n shoud big than k")
	}

	if config == nil {
		config = params.TestChainConfig
	}
	if fConfig == nil {
		fConfig = params.TestChainConfig
	}
	fn := n / k
	blocks, fBlocks, receipts, fReceipts := make(types.Blocks, n), make(types.Blocks, fn), make([]types.Receipts, n), make([]types.Receipts, fn)
	evilBlocks, evilReceipts := make(types.Blocks, 0), make([]types.Receipts, 0)
	stateDB, err := state.New(parent.Root(), state.NewDatabase(db))
	if err != nil {
		panic(err)
	}
	fStateDB, err := state.New(fParent.Root(), state.NewDatabase(db))
	if err != nil {
		panic(err)
	}

	chainreader := &fakeChainReader{
		config: config,
		blocksByNumber: map[uint64]*types.Block{
			parent.NumberU64(): parent,
		},
		stateByHash: map[common.Hash]*state.StateDB{
			parent.Root(): stateDB,
		},
	}

	fChainreader := &fakeChainReader{
		config: fConfig,
		blocksByNumber: map[uint64]*types.Block{
			fParent.NumberU64(): parent,
		},
		stateByHash: map[common.Hash]*state.StateDB{
			fParent.Root(): fStateDB,
		},
	}

	sealBlock := func(engine consensus.Engine, header *types.Header, state *state.StateDB, txs []*types.Transaction,
		uncles []*types.Header, receipts []*types.Receipt, chainreader *fakeChainReader, fixed func()) *types.Block {
		block, err := engine.FinalizeAndAssemble(chainreader, header, state, txs, uncles, receipts)
		if err != nil {
			panic(fmt.Sprintf("FinalizeAndAssemble error: %v", err))
		}
		if fixed != nil {
			fixed()
			header.Root = state.IntermediateRoot(true)
			block = types.NewBlock(header, txs, uncles, receipts)
		}
		// Write state changes to db
		root, err := state.Commit(true)
		if err != nil {
			panic(fmt.Sprintf("state write error: %v", err))
		}
		if err := state.Database().TrieDB().Commit(root, false); err != nil {
			panic(fmt.Sprintf("trie write error: %v", err))
		}

		if chainTest, ok := engine.(consensus.TwoChainTest); ok {
			block, err = chainTest.SealForTest(block)
			if err != nil {
				panic(err)
			}
		}
		if chainTest, ok := fEngine.(consensus.TwoChainTest); ok {
			block, err = chainTest.SealForTest(block)
			if err != nil {
				panic(err)
			}
		}

		chainreader.blocksByNumber[block.NumberU64()] = block
		chainreader.stateByHash[state.IntermediateRoot(true)] = state
		return block
	}

	genblock := func(i int, parent *types.Block, statedb *state.StateDB) (*types.Block, types.Receipts) {
		b := &BlockGen{i: i, chain: blocks, parent: parent, statedb: statedb, config: config, engine: engine}
		b.header = makeHeader(chainreader, parent, statedb, b.engine, 0)
		var evilHeader *types.Header
		// Execute any user modifications to the block
		if gen != nil {
			gen(i, b)
		}
		if b.engine != nil {
			block := sealBlock(b.engine, b.header, statedb, b.txs, b.uncles, b.receipts, chainreader, nil)
			if (i+1)%k != 0 {
				return block, b.receipts
			}
			// random make evil block
			if isEvilBlock() {
				evilHeader = block.Header()
				evilBlocks = append(evilBlocks, block)
				evilReceipts = append(evilReceipts, b.receipts)
				statedb, err = state.New(parent.Root(), state.NewDatabase(db))
				if err != nil {
					panic(err)
				}
				b = &BlockGen{i: i, chain: blocks, parent: parent, statedb: statedb, config: config, engine: engine}
				b.header = makeHeader(chainreader, parent, statedb, b.engine, 1)
				if gen != nil {
					gen(i, b)
				}
				block = sealBlock(b.engine, b.header, statedb, b.txs, b.uncles, b.receipts, chainreader, nil)
			}

			fParentNumber := (i+1)/k - 1
			fParent := fChainreader.blocksByNumber[uint64(fParentNumber)]
			fStateDB, err := state.New(fParent.Root(), state.NewDatabase(db))
			if err != nil {
				panic(err)
			}

			fb := &BlockGen{i: fParentNumber, chain: fBlocks, parent: fParent, statedb: fStateDB, config: fConfig, engine: fEngine}
			fb.header = makeHeader(fChainreader, fParent, fStateDB, fEngine, 0)
			// Create Extra
			extra := makeHeaderExtra(block.Hash(), evilHeader)
			fb.header.Extra = extra
			// Add Txs
			fb.SetCoinbase(common.Address{0x00})

			for j := i + 2 - k; j < i+1; j++ {
				txs := chainreader.blocksByNumber[uint64(j)].Transactions()
				for _, tx := range txs {
					fb.AddTx(tx)
				}
			}
			for _, tx := range block.Transactions() {
				fb.AddTx(tx)
			}
			fBlock := sealBlock(fb.engine, fb.header, fStateDB, fb.txs, fb.uncles, fb.receipts, fChainreader, func() {
				balance := statedb.GetBalance(common.Address{seed})
				fBalance := fStateDB.GetBalance(common.Address{seed})
				fStateDB.AddBalance(common.Address{0x00}, balance.Sub(balance, fBalance))
			})
			fBlocks[fBlock.NumberU64()-1] = fBlock
			fReceipts[fBlock.NumberU64()-1] = fb.receipts
			//fmt.Println("========>", (i + 2 - k), i+1, fBlock.Number().String(), block.Number().String(), fBlock.Root().String(), block.Root().String())
			return block, b.receipts
		}
		return nil, nil
	}
	for i := 0; i < n; i++ {
		statedb, err := state.New(parent.Root(), state.NewDatabase(db))
		if err != nil {
			panic(err)
		}
		block, receipt := genblock(i, parent, statedb)
		blocks[i] = block
		receipts[i] = receipt
		parent = block
	}

	return blocks, receipts, fBlocks, fReceipts, evilBlocks, evilReceipts
}

func makeHeaderExtra(hash common.Hash, evilHeader *types.Header) []byte {
	extra, _ := rlp.EncodeToBytes([]interface{}{
		uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
		"gev",
		runtime.Version(),
		runtime.GOOS,
	})
	if len(extra) < 32 {
		extra = append(extra, bytes.Repeat([]byte{0x00}, 32-len(extra))...)
	}
	fce := fconTypes.FConExtra{}
	fce.CurrentBlock = hash
	fce.EvilHeader = evilHeader
	byteBuffer := new(bytes.Buffer)
	err := rlp.Encode(byteBuffer, &fce)
	if err != nil {
		panic(err)
	}
	extra = append(extra[:32], byteBuffer.Bytes()...)
	return extra
}

func isEvilBlock() bool {
	return rand.Intn(100)%2 == 1
}

func makeHeader(chain consensus.ChainReader, parent *types.Block, state *state.StateDB, engine consensus.Engine, index uint64) *types.Header {
	var time uint64
	if parent.Time() == 0 {
		time = 10 + index
	} else {
		time = parent.Time() + 10 + index // block time is fixed at 10 seconds
	}

	return &types.Header{
		Root:       state.IntermediateRoot(true),
		ParentHash: parent.Hash(),
		Coinbase:   parent.Coinbase(),
		Difficulty: big.NewInt(2),
		GasLimit:   CalcGasLimit(parent, parent.GasLimit(), parent.GasLimit()),
		Number:     new(big.Int).Add(parent.Number(), common.Big1),
		Time:       time,
	}
}

// makeHeaderChain creates a deterministic chain of headers rooted at parent.
func makeHeaderChain(parent *types.Header, n int, engine consensus.Engine, db evrdb.Database, seed int) []*types.Header {
	blocks := makeBlockChain(types.NewBlockWithHeader(parent), n, engine, db, seed)
	headers := make([]*types.Header, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}
	return headers
}

// makeBlockChain creates a deterministic chain of blocks rooted at parent.
func makeBlockChain(parent *types.Block, n int, engine consensus.Engine, db evrdb.Database, seed int) []*types.Block {
	blocks, _ := GenerateChain(params.TestChainConfig, parent, engine, db, n, func(i int, b *BlockGen) {
		b.SetCoinbase(common.Address{0: byte(seed), 19: byte(i)})
	})
	return blocks
}

type fakeChainReader struct {
	config         *params.ChainConfig
	genesis        *types.Block
	blocksByNumber map[uint64]*types.Block
	stateByHash    map[common.Hash]*state.StateDB
}

func (cr *fakeChainReader) StateAt(hash common.Hash) (*state.StateDB, error) {
	if state, ok := cr.stateByHash[hash]; !ok {
		return nil, errors.New("state not found")
	} else {
		return state, nil
	}
}

// Config returns the chain configuration.
func (cr *fakeChainReader) Config() *params.ChainConfig {
	return cr.config
}

func (cr *fakeChainReader) CurrentHeader() *types.Header { return nil }
func (cr *fakeChainReader) GetHeaderByNumber(number uint64) *types.Header {
	if blk, ok := cr.blocksByNumber[number]; ok {
		return blk.Header()
	}
	return nil
}
func (cr *fakeChainReader) GetHeaderByHash(hash common.Hash) *types.Header          { return nil }
func (cr *fakeChainReader) GetHeader(hash common.Hash, number uint64) *types.Header { return nil }
func (cr *fakeChainReader) GetBlock(hash common.Hash, number uint64) *types.Block   { return nil }
