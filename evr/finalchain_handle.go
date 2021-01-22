package evr

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus"
	"github.com/Evrynetlabs/evrynet-node/consensus/fconsensus"
	fconTypes "github.com/Evrynetlabs/evrynet-node/consensus/fconsensus/types"
	"github.com/Evrynetlabs/evrynet-node/core"
	"github.com/Evrynetlabs/evrynet-node/core/state"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/core/vm"
	"github.com/Evrynetlabs/evrynet-node/event"
	"github.com/Evrynetlabs/evrynet-node/log"
	"github.com/Evrynetlabs/evrynet-node/rlp"
)

const (
	M = uint64(2)
	K = uint64(2)
)

type FBManager struct {
	mux                *event.TypeMux
	engine             consensus.Engine
	blockchain         *core.BlockChain
	finaliseBlockchain *core.BlockChain
	chainHeadCh        chan core.ChainHeadEvent
	abort              chan struct{}
	signer             common.Address      // Evrynet address of the signing key
	signFn             fconsensus.SignerFn // Signer function to authorize hashes with
}

var AuthorSinger common.Address

//type SignerFn func(accounts.Account, string, []byte) ([]byte, error)

func NewFBManager(bc, fbc *core.BlockChain, engine consensus.Engine, mux *event.TypeMux) *FBManager {
	fb := &FBManager{
		engine:             engine,
		blockchain:         bc,
		finaliseBlockchain: fbc,
		chainHeadCh:        make(chan core.ChainHeadEvent, 10),
		abort:              make(chan struct{}),
		mux:                mux,
	}

	fb.blockchain.SubscribeChainHeadEvent(fb.chainHeadCh)
	return fb
}

func (fb *FBManager) Authorize(signer common.Address, signFn fconsensus.SignerFn) {
	fb.signer = signer
	fb.signFn = signFn
	if fcon, ok := fb.engine.(*fconsensus.FConsensus); ok {
		fcon.Authorize(signer, signFn)
	}
}

func (fb *FBManager) GetBlockSections(newBlock *types.Block) (uint64, uint64, bool) {
	number := newBlock.Number().Uint64()
	currentBlock := fb.finaliseBlockchain.CurrentBlock()
	packedBlockNumber := uint64(0)
	if currentBlock.Number().Uint64() > 0 {
		fce, err := fconTypes.ExtractFConExtra(currentBlock.Header())
		if err != nil {
			log.Error("ExtractFConExtra failed", "err", err)
			return 0, 0, false
		}
		packBlock := fb.blockchain.GetBlockByHash(fce.CurrentBlock)
		packedBlockNumber = packBlock.Number().Uint64()
	}

	if packedBlockNumber+M+K > number {
		return 0, 0, false
	}

	end := packedBlockNumber + M
	if end < number-K {
		end = number - K
	}

	return packedBlockNumber + 1, end, true

}

func (fb *FBManager) PrepareHeader() (*types.Header, error) {
	extra := makeExtraData(nil)
	if len(extra) < 32 {
		extra = append(extra, bytes.Repeat([]byte{0x00}, 32-len(extra))...)
	}
	parent := fb.finaliseBlockchain.CurrentBlock()
	timestamp := time.Now().Unix()

	if parent.Time() >= uint64(timestamp) {
		timestamp = int64(parent.Time() + 1)
	}
	// this will ensure we're not going off too far in the future
	if now := time.Now().Unix(); timestamp > now+1 {
		wait := time.Duration(timestamp-now) * time.Second
		log.Info("Mining too far in the future", "wait", common.PrettyDuration(wait))
		time.Sleep(wait)
	}

	num := parent.Number()

	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		GasLimit:   core.CalcGasLimit(parent, 8000000, 8000000),
		Time:       uint64(timestamp),
		Coinbase:   common.Address{},
		Nonce:      types.BlockNonce{},
		Extra:      extra,
		Difficulty: new(big.Int).SetInt64(2),
	}

	err := fb.engine.Prepare(fb.finaliseBlockchain, header)

	return header, err
}

func (fb *FBManager) VerifyBlock(block *types.Block, statedb *state.StateDB, fheader *types.Header, tcount *int, gasUsed *uint64) (types.Transactions, types.Receipts, uint64, error) {
	var (
		receipts types.Receipts
		//header   = block.Header()
		gp = new(core.GasPool).AddGas(block.GasLimit())
	)
	gasUsedPre := *gasUsed
	txs := block.Transactions()
	for _, tx := range txs {
		fb.finaliseBlockchain.GetVMConfig()
		statedb.Prepare(tx.Hash(), common.Hash{}, *tcount)
		receipt, _, err := core.ApplyTransaction(fb.finaliseBlockchain.Config(), fb.finaliseBlockchain, nil, gp,
			statedb, fheader, tx, gasUsed, vm.Config{})
		if err != nil {
			log.Error("FBManager Apply transactions failed", "err", err.Error())
			return nil, nil, 0, err
		}
		receipts = append(receipts, receipt)
		*tcount ++
	}
	root := statedb.IntermediateRoot(true)

	if root != block.Root() {
		errStr := fmt.Sprintf("block: %s, number: %s  stateRoot is not equal, we get: %s, expect: %s", block.Hash().String(),
			block.Number().String(), root.String(), block.Root().String())
		log.Error("FBManager Apply transactions failed", "err", errStr)
		return nil, nil, 0, errors.New(errStr)
	}

	if (*gasUsed - gasUsedPre) != block.GasUsed() {
		errStr := fmt.Sprintf("block: %s, number: %s  gasUsed is not equal, we get: %d, expect: %d", block.Hash().String(),
			block.Number().String(), *gasUsed, block.GasUsed())
		log.Error("FBManager Apply transactions failed", "err", errStr)
		return nil, nil, 0, errors.New(errStr)
	}
	return txs, receipts, *gasUsed, nil

}

// Just for Test, fix later
func (fb *FBManager) IsAuthorizedSinger() bool {
	if (AuthorSinger != common.Address{}) {
		return bytes.Equal(AuthorSinger[:], fb.signer[:])
	}

	header := fb.finaliseBlockchain.GetHeaderByNumber(0)
	if len(header.Extra) < 97 {
		return false
	}

	AuthorSinger.SetBytes(header.Extra[32:52])
	return bytes.Equal(AuthorSinger[:], fb.signer[:])
}

func (fb *FBManager) CreateFinaliseBlock(newBlock *types.Block) *types.Block {
	// fix later
	if !fb.IsAuthorizedSinger() {
		return nil
	}

	start, end, trigger := fb.GetBlockSections(newBlock)
	if !trigger {
		log.Info("FBManager: not trigger to create block")
		return nil
	}
	log.Info("FBManager: pack section", "start", start, "end", end)
	header, err := fb.PrepareHeader()
	if err != nil {
		log.Error("FBManager: PrepareHeader failed", "err", err)
		return nil
	}
	parent := fb.finaliseBlockchain.CurrentBlock()

	statedb, err := state.New(parent.Root(), fb.finaliseBlockchain.StateCache())
	var (
		txsSum      types.Transactions
		receiptsSum types.Receipts
		evilHeader  *types.Header
		txCount     = 0
		gasUsedSum  = new(uint64)
	)
	for start <= end {
		blockTerm := fb.blockchain.GetBlockByNumber(start)
		start++
		txs, receipts, _, err := fb.VerifyBlock(blockTerm, statedb, header, &txCount, gasUsedSum)
		if err != nil {
			evilHeader = blockTerm.Header()
			break
		}
		txsSum = append(txsSum, txs...)
		receiptsSum = append(receiptsSum, receipts...)

	}

	packBlock := fb.blockchain.GetBlockByNumber(start - 1)
	log.Info("FBManager: latest package block", "hash", packBlock.Hash().String(), "number", packBlock.Number().String())
	log.Info("FBManager: pack transactions", "len", len(txsSum), "gasUsed", gasUsedSum)

	currentHash := packBlock.Hash()
	latestRoot := packBlock.Root()

	copy(header.Root[:], latestRoot[:])
	header.GasUsed = *gasUsedSum

	fce, err := fconTypes.ExtractFConExtra(header)
	if err != nil {
		log.Error("FBManager ExtractFConExtra  failed", "err", err.Error())
		return nil
	}
	fce.EvilHeader = evilHeader
	fce.CurrentBlock = currentHash
	rlpbytes, err := rlp.EncodeToBytes(&fce)
	if err != nil {
		log.Error("FBManager rlp extra failed", "err", err.Error())
		return nil
	}
	header.Extra = append(header.Extra[:fconsensus.ExtraVanity], rlpbytes...)
	block := types.NewBlock(header, txsSum, nil, receiptsSum)

	results := make(chan *types.Block)

	go func(b *types.Block) {
		fb.engine.Seal(fb.finaliseBlockchain, b, results, fb.abort)
	}(block)

	select {
	case block = <-results:
	case <-fb.abort:
		return nil
	}
	hash := block.Hash()

	for i, receipt := range receiptsSum {
		receipt.BlockHash = hash
		receipt.BlockNumber = block.Number()
		receipt.TransactionIndex = uint(i)
		receiptsSum[i] = new(types.Receipt)
		*receiptsSum[i] = *receipt
		// Update the block hash in all logs since it is now available and not when the
		// receipt/log of individual transactions were created.
		for _, log := range receipt.Logs {
			log.BlockHash = hash
		}
	}
	log.Info("FBManagerFinish creating block", "number", block.Number().String(), "hash", block.Hash().String(), "parent", block.ParentHash().String())
	return block
}

func (fb *FBManager) Start() {
	go func() {
		for {
			select {
			case <-fb.abort:
				log.Info("FBManager receive stop message")
				return
			case ev := <-fb.chainHeadCh:
				continue
				block := fb.CreateFinaliseBlock(ev.Block)
				if block != nil {
					fb.finaliseBlockchain.InsertChain(types.Blocks{block})
					fb.mux.Post(core.NewMinedBlockEvent{Block: block, IsFinalChain: true})
				}
			}
		}
	}()

}

func (fb *FBManager) Stop() {
	close(fb.abort)

}
