package evr

import (
	"bytes"
	"github.com/Evrynetlabs/evrynet-node/accounts"
	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus/clique"
	"github.com/Evrynetlabs/evrynet-node/core"
	"github.com/Evrynetlabs/evrynet-node/core/state"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/core/vm"
	"github.com/Evrynetlabs/evrynet-node/log"
	"math/big"
	"time"
)

type FBManager struct {
	blockchain         *core.BlockChain
	finaliseBlockchain *core.BlockChain
	chainHeadCh        chan core.ChainHeadEvent
	abort              chan struct{}
	signer             common.Address // Evrynet address of the signing key
	signFn             SignerFn       // Signer function to authorize hashes with
}

type SignerFn func(accounts.Account, string, []byte) ([]byte, error)

func NewFBManager(bc, fbc *core.BlockChain) *FBManager {
	fb := &FBManager{
		blockchain:         bc,
		finaliseBlockchain: fbc,
		chainHeadCh:        make(chan core.ChainHeadEvent, 10),
		abort:              make(chan struct{})}

	fb.blockchain.SubscribeChainHeadEvent(fb.chainHeadCh)
	return fb
}

func (fb *FBManager) Authorize(signer common.Address, signFn SignerFn) {
	fb.signer = signer
	fb.signFn = signFn
}

func (fb *FBManager) CreateFinaliseBlock(epoch int64, b *types.Block) *types.Block {
	number := b.Number().Int64()
	log.Info("FBManager: Receive a new block from fast chain", "number", number, "hash", b.Hash())
	if number%epoch != 0 {
		log.Info("FBManager: not trigger to create block")
		return nil
	}
	end := number - epoch
	start := end - epoch
	if start < 0 {
		log.Info("FBManager: not trigger to create block")
		return nil
	}

	extra := makeExtraData(nil)
	if len(extra) < 32 {
		extra = append(extra, bytes.Repeat([]byte{0x00}, 97-len(extra))...)
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
	// 考虑之后只存区块hash更加合适
	var txs []*types.Transaction
	for start < end {
		b := fb.blockchain.GetBlockByNumber(uint64(end))
		term := b.Transactions()
		for i := len(term) - 1; i >= 0; i-- {
			txs = append(txs, term[i])
		}
		end--
	}
	length := len(txs)
	for i := 0; i < length/2; i++ {
		txs[i], txs[length-1-i] = txs[length-1-i], txs[i]
	}

	statedb, err := state.New(parent.Root(), fb.finaliseBlockchain.StateCache())
	if err != nil {
		log.Error("FBManage Create stateDB failed", "err", err.Error())
		return nil
	}
	var (
		receipts []*types.Receipt
		gp       = new(core.GasPool).AddGas(header.GasLimit)
	)
	log.Info("FBManagerFinish: pack txs", "len", len(txs))
	for i, tx := range txs {
		log.Info("FBManagerFinish: pack txs info", "hash", tx.Hash().String())
		statedb.Prepare(tx.Hash(), common.Hash{}, i)
		fb.finaliseBlockchain.GetVMConfig()
		receipt, _, err := core.ApplyTransaction(fb.finaliseBlockchain.Config(), fb.finaliseBlockchain, nil, gp, statedb, header, tx, &header.GasUsed, vm.Config{})
		if err != nil {
			log.Error("FBManager Apply transactions failed", "err", err.Error())
			return nil
		}
		receipts = append(receipts, receipt)
	}
	header.Root = statedb.IntermediateRoot(true)
	block := types.NewBlock(header, txs, nil, receipts)
	hash := block.Hash()

	header = block.Header()
	sighash, err := fb.signFn(accounts.Account{Address: fb.signer}, accounts.MimetypeClique, clique.CliqueRLP(header))
	if err != nil {
		log.Error("FBManager Sign block failed", "err", err.Error())
		return nil
	}
	copy(header.Extra[32:], sighash)
	block = block.WithSeal(header)

	for i, receipt := range receipts {
		receipt.BlockHash = hash
		receipt.BlockNumber = block.Number()
		receipt.TransactionIndex = uint(i)
		receipts[i] = new(types.Receipt)
		*receipts[i] = *receipt
		// Update the block hash in all logs since it is now available and not when the
		// receipt/log of individual transactions were created.
		for _, log := range receipt.Logs {
			log.BlockHash = hash
		}
	}
	log.Info("FBManagerFinish creating block", "number", block.Number().String(), "hash", block.Hash().String())
	return block
}

func (fb *FBManager) Start() {
	epoch := int64(5)
	go func() {
		for {
			select {
			case <-fb.abort:
				log.Info("FBManager receive stop message")
				return
			case ev := <-fb.chainHeadCh:
				block := fb.CreateFinaliseBlock(epoch, ev.Block)
				if block != nil {
					fb.finaliseBlockchain.InsertChain(types.Blocks{block})
				}
			}
		}
	}()

}

func (fb *FBManager) Stop() {
	close(fb.abort)

}
