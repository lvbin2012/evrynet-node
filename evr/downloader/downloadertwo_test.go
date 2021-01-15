package downloader

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/core"
	"github.com/Evrynetlabs/evrynet-node/core/rawdb"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/event"
	"github.com/Evrynetlabs/evrynet-node/evrdb"
	"github.com/Evrynetlabs/evrynet-node/log"
	"github.com/Evrynetlabs/evrynet-node/trie"
)

func init() {
	maxForkAncestry = 1000
	blockCacheItems = 1024
	fsHeaderContCheck = 500 * time.Millisecond
}

type dbHelp interface {
	getDB() evrdb.Database
}

type testChainInfo struct {
	genesis      *types.Block
	isFinalChain bool
	dbHelp       dbHelp
	ownHashes    []common.Hash
	ownHeaders   map[common.Hash]*types.Header
	ownBlocks    map[common.Hash]*types.Block
	ownReceipts  map[common.Hash]types.Receipts
	ownChainTd   map[common.Hash]*big.Int

	ancientHeaders  map[common.Hash]*types.Header
	ancientBlocks   map[common.Hash]*types.Block
	ancientReceipts map[common.Hash]types.Receipts
	ancientChainTd  map[common.Hash]*big.Int

	evilHeaders  map[common.Hash]*types.Header
	evilBlocks   map[common.Hash]*types.Block
	evilReceipts map[common.Hash]types.Receipts

	lock sync.RWMutex
}

type downloadTwoTester struct {
	downloader *Downloader
	stateDb    evrdb.Database
	peerDb     evrdb.Database
	peers      map[string]*downloadTwoTesterPeer
	chainInfo  *testChainInfo
	fChainInfo *testChainInfo
	lock       sync.RWMutex
}

type downloadTwoTesterPeer struct {
	dlt           *downloadTwoTester
	id            string
	lock          sync.RWMutex
	chain         *testChain
	fchain        *testChain
	missingStates map[common.Hash]bool
}

func (d *downloadTwoTesterPeer) Head() (common.Hash, *big.Int) {
	b := d.chain.headBlock()
	return b.Hash(), d.chain.td(b.Hash())
}
func (d *downloadTwoTesterPeer) FHead() (common.Hash, *big.Int) {
	b := d.fchain.headBlock()
	return b.Hash(), d.fchain.td(b.Hash())
}

func (d *downloadTwoTesterPeer) RequestHeadersByHash(origin common.Hash, amount int, skip int, reverse bool, isFinalChain bool) error {
	if reverse {
		panic("reverse header requests not supported")
	}
	var chain *testChain
	if isFinalChain {
		chain = d.fchain
	} else {
		chain = d.chain
	}
	result := chain.headersByHash(origin, amount, skip)
	log.Debug("RequestHeadersByHash", "origin", origin.String(), "amount", amount, "skip", skip, "reverse", reverse,
		"isFinalChain", isFinalChain, "result", len(result))
	go d.dlt.downloader.DeliverHeaders(d.id, isFinalChain, result)
	return nil
}

func (d *downloadTwoTesterPeer) RequestHeadersByNumber(origin uint64, amount int, skip int, reverse bool, isFinalChain bool) error {
	if reverse {
		panic("reverse header requests not supported")
	}
	var chain *testChain
	if isFinalChain {
		chain = d.fchain
	} else {
		chain = d.chain
	}
	result := chain.headersByNumber(origin, amount, skip)
	log.Debug("RequestHeadersByNumber", "origin", origin, "amount", amount, "skip", skip, "reverse", reverse,
		"isFinalChain", isFinalChain, "result", len(result))
	go d.dlt.downloader.DeliverHeaders(d.id, isFinalChain, result)
	return nil
}

func (d *downloadTwoTesterPeer) RequestBodies(hashes []common.Hash, isFinalChain bool) error {

	var chain *testChain
	if isFinalChain {
		chain = d.fchain
	} else {
		chain = d.chain
	}
	txs, uncles := chain.bodies(hashes)
	log.Debug("RequestBodies", "Hashes", len(hashes), "isFinalChain", isFinalChain, "txs", len(txs), "uncles", len(uncles))
	go d.dlt.downloader.DeliverBodies(d.id, isFinalChain, txs, uncles)
	return nil
}

func (d *downloadTwoTesterPeer) RequestReceipts(hashes []common.Hash, isFinalChain bool) error {
	var chain *testChain
	if isFinalChain {
		chain = d.fchain
	} else {
		chain = d.chain
	}
	receipts := chain.receipts(hashes)
	log.Debug("RequestReceipts", "Hashes", len(hashes), "isFinalChain", isFinalChain, "receipts", len(receipts))
	go d.dlt.downloader.DeliverReceipts(d.id, isFinalChain, receipts)
	return nil
}

func (d *downloadTwoTesterPeer) RequestNodeData(hashes []common.Hash, isFinalChain bool) error {
	d.dlt.lock.RLock()
	defer d.dlt.lock.RUnlock()

	results := make([][]byte, 0, len(hashes))
	for _, hash := range hashes {
		if data, err := d.dlt.peerDb.Get(hash.Bytes()); err == nil {
			if !d.missingStates[hash] {
				results = append(results, data)
			}
		}
	}
	log.Debug("RequestNodeData", "Hashes", len(hashes), "isFinalChain", isFinalChain, "results", len(results))
	go d.dlt.downloader.DeliverNodeData(d.id, isFinalChain, results)
	return nil
}

func (d *downloadTwoTesterPeer) RequestEvilBodies(hashes []common.Hash) error {
	txs, uncles := d.fchain.evilBlocks(hashes)
	log.Debug("RequestEvilBodies", "Hashes", len(hashes), "txs", len(txs), "uncles", len(uncles))
	go d.dlt.downloader.DeliverEvilBlocks(d.id, true, txs, uncles)
	return nil
}

func (d *downloadTwoTesterPeer) RequestEvilReceipts(hashes []common.Hash) error {
	panic("implement me later")
}

func (pc *downloadTwoTesterPeer) RequestEvilHeadersByHash(h common.Hash) error {
	panic("implement me later")
}

func (pc *downloadTwoTesterPeer) RequestEvilHeadersByNumber(i uint64) error {
	panic("implement me later")
}

func (dlt *downloadTwoTester) getDB() evrdb.Database {
	return dlt.stateDb
}

func (t *testChainInfo) HasHeader(hash common.Hash, number uint64) bool {
	return t.GetHeaderByHash(hash) != nil
}

func (t *testChainInfo) GetHeaderByHash(hash common.Hash) *types.Header {
	t.lock.RLock()
	defer t.lock.RUnlock()

	header := t.ancientHeaders[hash]
	if header != nil {
		return header
	}
	return t.ownHeaders[hash]
}

func (t *testChainInfo) CurrentHeader() *types.Header {
	t.lock.RLock()
	defer t.lock.RUnlock()

	for i := len(t.ownHashes) - 1; i >= 0; i-- {
		if header := t.ancientHeaders[t.ownHashes[i]]; header != nil {
			return header
		}
		if header := t.ownHeaders[t.ownHashes[i]]; header != nil {
			return header
		}
	}

	return t.genesis.Header()
}

func (t *testChainInfo) GetTd(hash common.Hash, number uint64) *big.Int {
	t.lock.Lock()
	defer t.lock.Unlock()
	if td := t.ancientChainTd[hash]; td != nil {
		return td
	}
	return t.ownChainTd[hash]
}

func (t *testChainInfo) InsertHeaderChain(headers []*types.Header, checkFreq int) (int, error) {
	t.lock.Lock()
	defer func(n int, isFinalChain bool) {
		t.lock.Unlock()
		currentHeight := t.CurrentHeader().Number.Uint64()
		shouldHeight := headers[len(headers)-1].Number.Uint64()
		n = n - int(shouldHeight-currentHeight)
		log.Debug("Coming in InsertHeaderChain", "insert headers", n, "currentHeight",
			t.CurrentHeader().Number.String(), "isFinalChain", isFinalChain)
		if n == 0 {
			log.Error("Coming in InsertHeaderChain", "currentHash", t.CurrentHeader().Hash().String(),
				"Input first parent", headers[0].ParentHash.String(), "isFinalChain", isFinalChain)
		}

	}(len(headers), t.isFinalChain)

	_, inOwnHeaders := t.ownHeaders[headers[0].ParentHash]
	_, inAncientHeaders := t.ancientHeaders[headers[0].ParentHash]
	if !(inAncientHeaders || inOwnHeaders) {
		return 0, errors.New("unknown parent")
	}
	for i, header := range headers[1:] {
		if headers[i].Hash() != header.ParentHash {
			return i, errors.New("unknown parent")
		}
	}

	for i, header := range headers {
		if _, ok := t.ownHeaders[header.Hash()]; ok {
			continue
		}
		parentTd, ok := t.ownChainTd[header.ParentHash]
		if !ok {
			parentTd, ok = t.ancientChainTd[header.ParentHash]
		}
		if !ok {
			return i, errors.New("unknown parent")
		}
		t.ownHashes = append(t.ownHashes, header.Hash())
		t.ownHeaders[header.Hash()] = header
		t.ownChainTd[header.Hash()] = new(big.Int).Add(parentTd, header.Difficulty)

	}

	return len(headers), nil
}

func (t *testChainInfo) Rollback(hashes []common.Hash) {
	t.lock.Lock()
	defer t.lock.Unlock()
	for i := len(hashes) - 1; i >= 0; i-- {
		if t.ownHashes[len(t.ownHashes)-1] == hashes[i] {
			t.ownHashes = t.ownHashes[:len(t.ownHashes)-1]
		}
		delete(t.ownChainTd, hashes[i])
		delete(t.ownHeaders, hashes[i])
		delete(t.ownReceipts, hashes[i])
		delete(t.ownBlocks, hashes[i])
		delete(t.ancientChainTd, hashes[i])
		delete(t.ancientHeaders, hashes[i])
		delete(t.ancientReceipts, hashes[i])
		delete(t.ancientBlocks, hashes[i])
	}
}

func (t *testChainInfo) HasBlock(hash common.Hash, number uint64) bool {
	return t.GetBlockByHash(hash) != nil
}

func (t *testChainInfo) HasFastBlock(hash common.Hash, number uint64) bool {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if _, ok := t.ancientReceipts[hash]; ok {
		return true
	}
	_, ok := t.ownReceipts[hash]
	return ok
}

func (t *testChainInfo) GetBlockByHash(hash common.Hash) *types.Block {
	t.lock.RLock()
	defer t.lock.RUnlock()

	block := t.ancientBlocks[hash]
	if block != nil {
		return block
	}
	return t.ownBlocks[hash]
}

func (t *testChainInfo) CurrentBlock() *types.Block {
	t.lock.RLock()
	defer t.lock.RUnlock()
	for i := len(t.ownHashes) - 1; i >= 0; i-- {
		if block := t.ancientBlocks[t.ownHashes[i]]; block != nil {
			if _, err := t.dbHelp.getDB().Get(block.Root().Bytes()); err == nil {
				return block
			}
			return block
		}
		if block := t.ownBlocks[t.ownHashes[i]]; block != nil {
			if _, err := t.dbHelp.getDB().Get(block.Root().Bytes()); err == nil {
				return block
			}
		}
	}
	return t.genesis
}

func (t *testChainInfo) CurrentFastBlock() *types.Block {
	t.lock.RLock()
	defer t.lock.RUnlock()
	for i := len(t.ownHashes) - 1; i >= 0; i-- {
		if block := t.ancientBlocks[t.ownHashes[i]]; block != nil {
			return block
		}
		if block := t.ownBlocks[t.ownHashes[i]]; block != nil {
			return block
		}
	}
	return t.genesis
}

func (t *testChainInfo) FastSyncCommitHead(hash common.Hash) error {
	if block := t.GetBlockByHash(hash); block != nil {
		db := t.dbHelp.getDB()
		_, err := trie.NewSecure(block.Root(), trie.NewDatabase(db))
		return err
	}
	return fmt.Errorf("non existent block: %x", hash[:4])
}

func (t *testChainInfo) InsertChain(blocks types.Blocks) (int, error) {
	t.lock.Lock()
	defer func(n int, isFinalChain bool) {
		t.lock.Unlock()
		log.Debug("Coming in InsertChain", "insert blocks", n, "current", t.CurrentBlock().NumberU64(),
			"currentFast", t.CurrentFastBlock().NumberU64(), "isFinalChain", isFinalChain)
	}(len(blocks), t.isFinalChain)

	for i, block := range blocks {
		if parent, ok := t.ownBlocks[block.ParentHash()]; !ok {
			return i, errors.New("unknown parent")
		} else if _, err := t.dbHelp.getDB().Get(parent.Root().Bytes()); err != nil {
			return i, fmt.Errorf("unknown parent state %x: %v", parent.Root(), err)
		}
		if _, ok := t.ownHeaders[block.Hash()]; !ok {
			t.ownHashes = append(t.ownHashes, block.Hash())
			t.ownHeaders[block.Hash()] = block.Header()
		}

		t.ownBlocks[block.Hash()] = block
		t.ownReceipts[block.Hash()] = make(types.Receipts, 0)
		t.dbHelp.getDB().Put(block.Root().Bytes(), []byte{0x00})
		t.ownChainTd[block.Hash()] = new(big.Int).Add(t.ownChainTd[block.ParentHash()], block.Difficulty())
	}

	return len(blocks), nil
}

func (t *testChainInfo) InsertReceiptChain(blocks types.Blocks, receipts []types.Receipts, ancientLimit uint64) (int, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	log.Debug("Coming in InsertReceiptChain", "insert blocks", len(blocks),
		"Receipts", len(receipts), "isFinalChain", t.isFinalChain, "ancientLimit", ancientLimit)

	for i := 0; i < len(blocks) && i < len(receipts); i++ {
		if _, ok := t.ownHeaders[blocks[i].Hash()]; !ok {
			return i, errors.New("unknown owner")
		}
		if _, ok := t.ancientBlocks[blocks[i].ParentHash()]; !ok {
			if _, ok := t.ownBlocks[blocks[i].ParentHash()]; !ok {
				return i, errors.New("unknown parent")
			}
		}
		if blocks[i].NumberU64() <= ancientLimit {
			t.ancientBlocks[blocks[i].Hash()] = blocks[i]
			t.ancientReceipts[blocks[i].Hash()] = receipts[i]

			t.ancientHeaders[blocks[i].Hash()] = blocks[i].Header()
			t.ancientChainTd[blocks[i].Hash()] = new(big.Int).Add(t.ancientChainTd[blocks[i].ParentHash()], blocks[i].Difficulty())
			delete(t.ownHeaders, blocks[i].Hash())
			delete(t.ownChainTd, blocks[i].Hash())
		} else {
			t.ownBlocks[blocks[i].Hash()] = blocks[i]
			t.ownReceipts[blocks[i].Hash()] = receipts[i]
		}
	}
	return len(blocks), nil
}

func (t *testChainInfo) IsFinalChain() bool {
	return t.isFinalChain
}

func (t *testChainInfo) SaveEvilBlock(blocks types.Blocks) (int, error) {
	t.lock.Lock()
	defer func(n int, isFinalChain bool) {
		t.lock.Unlock()
		log.Debug("Coming SaveEvilBlock", "insert evil blocks", n, "isFinalChain", isFinalChain)
	}(len(blocks), t.isFinalChain)

	for _, block := range blocks {
		hash := block.Hash()
		t.evilBlocks[hash] = block
		t.evilHeaders[hash] = block.Header()
	}
	return len(blocks), nil
}

func newTwoTester() *downloadTwoTester {
	tester := &downloadTwoTester{
		peerDb: testDB,
		peers:  make(map[string]*downloadTwoTesterPeer),
	}
	chainGenesis := core.GenesisBlockForNewTesting(testDB, testAddress, big.NewInt(1000000000), false)
	fchainGenesis := core.GenesisBlockForNewTesting(testDB, testAddress, big.NewInt(1000000000), false)
	tester.stateDb = rawdb.NewMemoryDatabase()
	tester.stateDb.Put(chainGenesis.Root().Bytes(), []byte{0x00})
	tester.stateDb.Put(fchainGenesis.Root().Bytes(), []byte{0x00})

	tester.chainInfo = &testChainInfo{
		genesis:      chainGenesis,
		isFinalChain: false,
		dbHelp:       tester,
		ownHashes:    []common.Hash{chainGenesis.Hash()},
		ownHeaders:   map[common.Hash]*types.Header{chainGenesis.Hash(): chainGenesis.Header()},
		ownBlocks:    map[common.Hash]*types.Block{chainGenesis.Hash(): chainGenesis},
		ownReceipts:  map[common.Hash]types.Receipts{chainGenesis.Hash(): nil},
		ownChainTd:   map[common.Hash]*big.Int{chainGenesis.Hash(): chainGenesis.Difficulty()},

		ancientHeaders:  map[common.Hash]*types.Header{chainGenesis.Hash(): chainGenesis.Header()},
		ancientBlocks:   map[common.Hash]*types.Block{chainGenesis.Hash(): chainGenesis},
		ancientReceipts: map[common.Hash]types.Receipts{chainGenesis.Hash(): nil},
		ancientChainTd:  map[common.Hash]*big.Int{chainGenesis.Hash(): chainGenesis.Difficulty()},
	}
	tester.fChainInfo = &testChainInfo{
		genesis:      fchainGenesis,
		isFinalChain: true,
		dbHelp:       tester,
		ownHashes:    []common.Hash{fchainGenesis.Hash()},
		ownHeaders:   map[common.Hash]*types.Header{fchainGenesis.Hash(): fchainGenesis.Header()},
		ownBlocks:    map[common.Hash]*types.Block{fchainGenesis.Hash(): fchainGenesis},
		ownReceipts:  map[common.Hash]types.Receipts{fchainGenesis.Hash(): nil},
		ownChainTd:   map[common.Hash]*big.Int{fchainGenesis.Hash(): fchainGenesis.Difficulty()},

		ancientHeaders:  map[common.Hash]*types.Header{fchainGenesis.Hash(): fchainGenesis.Header()},
		ancientBlocks:   map[common.Hash]*types.Block{fchainGenesis.Hash(): fchainGenesis},
		ancientReceipts: map[common.Hash]types.Receipts{fchainGenesis.Hash(): nil},
		ancientChainTd:  map[common.Hash]*big.Int{fchainGenesis.Hash(): fchainGenesis.Difficulty()},

		evilHeaders:  map[common.Hash]*types.Header{},
		evilBlocks:   map[common.Hash]*types.Block{},
		evilReceipts: map[common.Hash]types.Receipts{},
	}

	tester.downloader = NewTwoChain(0, tester.stateDb, trie.NewSyncBloom(1, tester.stateDb),
		new(event.TypeMux), tester.chainInfo, nil, tester.fChainInfo, nil, tester.dropPeer)
	return tester
}

func (dlt *downloadTwoTester) terminate() {
	dlt.downloader.Terminate()
}

func (dlt *downloadTwoTester) newPeer(id string, version int, chain, fchain *testChain) error {
	dlt.lock.Lock()
	defer dlt.lock.Unlock()
	peer := &downloadTwoTesterPeer{dlt: dlt, id: id, chain: chain, fchain: fchain}
	dlt.peers[id] = peer
	return dlt.downloader.RegisterPeer(id, version, peer)

}

func (dlt *downloadTwoTester) dropPeer(id string) {
	dlt.lock.Lock()
	defer dlt.lock.Unlock()
	delete(dlt.peers, id)
	dlt.downloader.UnregisterPeer(id)
}

func (dlt *downloadTwoTester) sync(id string, td *big.Int, ftd *big.Int, mode SyncMode) error {
	dlt.lock.RLock()
	hash := dlt.peers[id].chain.headBlock().Hash()
	if td == nil {
		td = dlt.peers[id].chain.td(hash)
	}

	fHash := dlt.peers[id].fchain.headBlock().Hash()
	if ftd == nil {
		ftd = dlt.peers[id].fchain.td(fHash)
	}
	dlt.lock.RUnlock()
	// tag start one
	//hash = common.Hash{}

	err := dlt.downloader.synchroniseTwoChain(id, hash, td, fHash, ftd, mode)
	select {
	case <-dlt.downloader.cancelCh:
	default:
		panic("downloader active post sync cycle")
	}
	return err
}

func testSynchronisation(t *testing.T, protocol int, mode SyncMode) {
	t.Parallel()
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlDebug, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))

	tester := newTwoTester()
	defer tester.terminate()
	//chain := newTestChain(blockCacheItems+200, tester.chainInfo.genesis)
	//fChain := newTestChain((blockCacheItems+200)*9/9, tester.fChainInfo.genesis)
	chain, fChain := newTwoTestChain(blockCacheItems+200, 2, tester.chainInfo.genesis, tester.fChainInfo.genesis)
	fmt.Println("two chain info", chain.headBlock().Number().String(), fChain.headBlock().Number().String())

	tester.newPeer("peer", protocol, chain, fChain)
	if err := tester.sync("peer", nil, nil, mode); err != nil {
		t.Fatalf("failed to synchronise blocks: %v", err)
	}
	assertMOwnChain(t, tester, chain.len(), fChain.len())
}

func assertMOwnChain(t *testing.T, tester *downloadTwoTester, chainLen, fchainLen int) {
	t.Helper()
	assertMOwnForkedChain(t, tester, 1, []int{chainLen, fchainLen})
}

func assertMOwnForkedChain(t *testing.T, tester *downloadTwoTester, common int, lengths []int) {
	t.Helper()

	funcChack := func(headerLen, blockLen, receiptLen int, chain *testChainInfo) {
		if tester.downloader.mode == LightSync {
			blockLen, receiptLen = 1, 1
		}
		if hs := len(chain.ownHeaders) + len(chain.ancientHeaders) - 1; hs != headerLen {
			t.Fatalf("synchronised headers mismatch: have %v, want %v", hs, headerLen)
		}
		if bs := len(chain.ownBlocks) + len(chain.ancientBlocks) - 1; bs != blockLen {
			t.Fatalf("synchronised blocks mismatch: have %v, want %v", bs, blockLen)
		}
		if rs := len(chain.ownReceipts) + len(chain.ancientReceipts) - 1; rs != receiptLen {
			t.Fatalf("synchronised receipts mismatch: have %v, want %v", rs, receiptLen)
		}
	}
	funcChack(lengths[0], lengths[0], lengths[0], tester.chainInfo)
	funcChack(lengths[1], lengths[1], lengths[1], tester.fChainInfo)
}

func TestCanonicalSynchronisation65Full(t *testing.T) {
	testSynchronisation(t, 65, FullSync)
}
func TestCanonicalSynchronisation65Light(t *testing.T) {
	testSynchronisation(t, 65, LightSync)
}

func TestCanonicalSynchronisation65Fast(t *testing.T) {
	testSynchronisation(t, 65, FastSync)
}
