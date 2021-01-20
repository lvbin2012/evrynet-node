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

// This file contains some shares testing functionality, common to  multiple
// different files and modules being tested.

package evr

import (
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"testing"

	"github.com/Evrynetlabs/evrynet-node/accounts"
	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus"
	"github.com/Evrynetlabs/evrynet-node/consensus/clique"
	"github.com/Evrynetlabs/evrynet-node/consensus/ethash"
	"github.com/Evrynetlabs/evrynet-node/consensus/fconsensus"
	"github.com/Evrynetlabs/evrynet-node/core"
	"github.com/Evrynetlabs/evrynet-node/core/rawdb"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/core/vm"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/event"
	"github.com/Evrynetlabs/evrynet-node/evr/downloader"
	"github.com/Evrynetlabs/evrynet-node/evrdb"
	"github.com/Evrynetlabs/evrynet-node/p2p"
	"github.com/Evrynetlabs/evrynet-node/p2p/enode"
	"github.com/Evrynetlabs/evrynet-node/params"
)

var (
	testBankKey, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	testPublicKey  = crypto.FromECDSAPub(&testBankKey.PublicKey)
	testBank       = crypto.PubkeyToAddress(testBankKey.PublicKey)
)

// newTestProtocolManager creates a new protocol manager for testing purposes,
// with the given number of blocks already known, and potential notification
// channels for different events.
func newTestProtocolManager(mode downloader.SyncMode, blocks int, generator func(int, *core.BlockGen), newtx chan<- []*types.Transaction) (*ProtocolManager, evrdb.Database, error) {
	var (
		evmux  = new(event.TypeMux)
		engine = ethash.NewFaker()
		db     = rawdb.NewMemoryDatabase()
		gspec  = &core.Genesis{
			Config: params.TestChainConfig,
			Alloc: core.GenesisAlloc{
				testBank: {
					Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
				},
			},
		}
		genesis       = gspec.MustCommit(db)
		blockchain, _ = core.NewBlockChain(db, nil, gspec.Config, engine, vm.Config{}, nil)
	)
	chain, _ := core.GenerateChain(gspec.Config, genesis, ethash.NewFaker(), db, blocks, generator)
	if _, err := blockchain.InsertChain(chain); err != nil {
		panic(err)
	}
	pm, err := NewProtocolManager(gspec.Config, nil, mode, DefaultConfig.NetworkId, evmux, &testTxPool{added: newtx}, engine, nil, blockchain, nil, db, 1, nil)
	if err != nil {
		return nil, nil, err
	}
	pm.Start(1000)
	return pm, db, nil
}

func newTestProtocolManagerForTwoChain(mode downloader.SyncMode, n int, k int, seed byte, generator func(int, *core.BlockGen),
	newTx chan<- []*types.Transaction) (*ProtocolManager, evrdb.Database, error) {
	evmux := new(event.TypeMux)
	db := rawdb.NewMemoryDatabase()
	extraData := make([]byte, 32+common.AddressLength+65)
	copy(extraData[32:], testBank[:])

	signFun := func(a accounts.Account, mineType string, data []byte) ([]byte, error) {
		if a.Address != testBank {
			return nil, errors.New("unkown signer")
		}
		return crypto.Sign(crypto.Keccak256(data), testBankKey)
	}

	gspec := &core.Genesis{
		Difficulty: big.NewInt(1),
		ExtraData:  extraData,
		Config:     params.AllCliqueProtocolChanges,
		Alloc: core.GenesisAlloc{
			testBank: {
				Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
			},
		},
	}
	fGspec := &core.Genesis{
		Difficulty: big.NewInt(1),
		ExtraData:  extraData,
		Config:     params.FConsensusChainConfig,
		Alloc: core.GenesisAlloc{
			testBank: {
				Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
			},
		},
	}
	if generator == nil {
		generator = func(i int, block *core.BlockGen) {
			block.SetCoinbase(common.Address{seed})
			// Include transactions to the miner to make blocks more interesting.
			if true {
				signer := types.MakeSigner(params.AllCliqueProtocolChanges, block.Number())
				tx, err := types.SignTx(types.NewTransaction(block.TxNonce(testBank), common.Address{}, big.NewInt(1000), params.TxGas, params.AllCliqueProtocolChanges.GasPrice, nil), signer, testBankKey)
				if err != nil {
					panic(err)
				}
				block.AddTx(tx)
			}
		}
	}

	engine := clique.New(params.AllCliqueProtocolChanges.Clique, db)
	engine.Authorize(testBank, signFun)
	conf := &params.FConConfig{params.FConsensusChainConfig.Clique.Period, params.FConsensusChainConfig.Clique.Epoch}
	fEngine := fconsensus.New(conf, db)
	fEngine.Authorize(testBank, signFun)

	genesis := gspec.MustCommit(db)
	fGenesis := fGspec.MustCommit(db)

	blockChain, _ := core.NewBlockChain(db, nil, gspec.Config, engine, vm.Config{}, nil)
	fBlockChain, _ := core.NewBlockChain(db, nil, fGspec.Config, fEngine, vm.Config{}, nil)

	chain, _, fChain, _, eBlocks, _ := core.GenerateTwoChain(gspec.Config, gspec.Config,
		genesis, fGenesis, engine, fEngine, db, n, k, seed, generator)
	fmt.Println("Insert Block ", len(chain))
	if _, err := blockChain.InsertChain(chain); err != nil {
		panic(err)
	}
	fmt.Println("Insert Final Block ", len(fChain))
	if _, err := fBlockChain.InsertChain(fChain); err != nil {
		panic(err)
	}
	fmt.Println("Insert Evil  eBlocks ", len(eBlocks))
	if _, err := fBlockChain.SaveEvilBlock(eBlocks); err != nil {
		panic(err)
	}

	pm, err := NewProtocolManager(gspec.Config, fGspec.Config, mode, DefaultConfig.NetworkId, evmux, &testTxPool{added: newTx},
		engine, fEngine, blockChain, fBlockChain, db, 1, nil)
	if err != nil {
		return nil, nil, err
	}
	pm.Start(1000)
	return pm, db, nil

}

//NewTestProtocolManagerWithConsensus return an evr.ProtocolManager with specific consensusEngine
func NewTestProtocolManagerWithConsensus(engine consensus.Engine) (*ProtocolManager, error) {
	var (
		mode   = downloader.FullSync
		blocks = 0
		evmux  = new(event.TypeMux)
		db     = rawdb.NewMemoryDatabase()
		gspec  = &core.Genesis{
			Config: params.TestChainConfig,
			Alloc:  core.GenesisAlloc{testBank: {Balance: big.NewInt(1000000)}},
		}
		genesis       = gspec.MustCommit(db)
		blockchain, _ = core.NewBlockChain(db, nil, gspec.Config, engine, vm.Config{}, nil)
	)
	chain, _ := core.GenerateChain(gspec.Config, genesis, engine, db, blocks, nil)
	if _, err := blockchain.InsertChain(chain); err != nil {
		panic(err)
	}
	pm, err := NewProtocolManager(gspec.Config, nil, mode, DefaultConfig.NetworkId, evmux, &testTxPool{}, engine, nil, blockchain, nil, db, 1, nil)
	if err != nil {
		return nil, err
	}
	pm.Start(1000)
	return pm, nil
}

// newTestProtocolManagerMust creates a new protocol manager for testing purposes,
// with the given number of blocks already known, and potential notification
// channels for different events. In case of an error, the constructor force-
// fails the test.
func newTestProtocolManagerMust(t *testing.T, mode downloader.SyncMode, blocks int, generator func(int, *core.BlockGen), newtx chan<- []*types.Transaction) (*ProtocolManager, evrdb.Database) {
	pm, db, err := newTestProtocolManager(mode, blocks, generator, newtx)
	if err != nil {
		t.Fatalf("Failed to create protocol manager: %v", err)
	}
	return pm, db
}

// testTxPool is a fake, helper transaction pool for testing purposes
type testTxPool struct {
	txFeed event.Feed
	pool   []*types.Transaction        // Collection of all transactions
	added  chan<- []*types.Transaction // Notification channel for new transactions

	lock sync.RWMutex // Protects the transaction pool
}

// AddRemotes appends a batch of transactions to the pool, and notifies any
// listeners if the addition channel is non nil
func (p *testTxPool) AddRemotes(txs []*types.Transaction) []error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.pool = append(p.pool, txs...)
	if p.added != nil {
		p.added <- txs
	}
	return make([]error, len(txs))
}

// Pending returns all the transactions known to the pool
func (p *testTxPool) Pending() (map[common.Address]types.Transactions, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	batches := make(map[common.Address]types.Transactions)
	for _, tx := range p.pool {
		from, _ := types.Sender(types.BaseSigner{}, tx)
		batches[from] = append(batches[from], tx)
	}
	for _, batch := range batches {
		sort.Sort(types.TxByNonce(batch))
	}
	return batches, nil
}

func (p *testTxPool) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return p.txFeed.Subscribe(ch)
}

// newTestTransaction create a new dummy transaction.
func newTestTransaction(from *ecdsa.PrivateKey, nonce uint64, datasize int) *types.Transaction {
	tx := types.NewTransaction(nonce, common.Address{}, big.NewInt(0), 100000, big.NewInt(params.GasPriceConfig), make([]byte, datasize))
	tx, _ = types.SignTx(tx, types.BaseSigner{}, from)
	return tx
}

// testPeer is a simulated Peer to allow testing direct network calls.
type testPeer struct {
	net p2p.MsgReadWriter // Network layer reader/writer to simulate remote messaging
	app *p2p.MsgPipeRW    // Application layer reader/writer to simulate the local side
	*Peer
}

// newTestPeer creates a new Peer registered at the given protocol manager.
func newTestPeer(name string, version int, pm *ProtocolManager, shake bool) (*testPeer, <-chan error) {
	// Create a message pipe to communicate through
	app, net := p2p.MsgPipe()

	// Generate a random id and create the Peer
	var id enode.ID
	rand.Read(id[:])

	peer := pm.NewPeer(version, p2p.NewPeer(id, name, nil), net)

	// Start the Peer on a new thread
	errc := make(chan error, 1)
	go func() {
		select {
		case pm.newPeerCh <- peer:
			errc <- pm.handle(peer)
		case <-pm.quitSync:
			errc <- p2p.DiscQuitting
		}
	}()
	tp := &testPeer{app: app, net: net, Peer: peer}
	// Execute any implicitly requested handshakes and return
	if shake {
		var (
			genesis = pm.blockchain.Genesis()
			head    = pm.blockchain.CurrentHeader()
			td      = pm.blockchain.GetTd(head.Hash(), head.Number.Uint64())
		)
		tp.handshake(nil, td, head.Hash(), genesis.Hash())
	}
	return tp, errc
}

func newTestPeerForTwoChain(name string, version int, pm *ProtocolManager, shake bool) (*testPeer, <-chan error) {
	app, net := p2p.MsgPipe()

	var id enode.ID
	rand.Read(id[:])

	peer := pm.NewPeer(version, p2p.NewPeer(id, name, nil), net)
	errc := make(chan error, 1)
	go func() {
		select {
		case pm.newPeerCh <- peer:
			errc <- pm.handle(peer)
		case <-pm.quitSync:
			errc <- p2p.DiscQuitting
		}
	}()

	tp := &testPeer{app: app, net: net, Peer: peer}
	if shake {
		var (
			genesis  = pm.blockchain.Genesis()
			head     = pm.blockchain.CurrentHeader()
			td       = pm.blockchain.GetTd(head.Hash(), head.Number.Uint64())
			fGenesis = pm.fblockchain.Genesis()
			fHead    = pm.fblockchain.CurrentHeader()
			fTd      = pm.fblockchain.GetTd(fHead.Hash(), fHead.Number.Uint64())
		)
		tp.handshakeForTwoChain(nil, td, fTd, head.Hash(), fHead.Hash(), genesis.Hash(), fGenesis.Hash())
	}
	return tp, errc
}

// newTestPeerFromNode creates a new Peer from a node registered at the given protocol manager.
// Its used in TestFindPeers
func newTestPeerFromNode(name string, version int, pm *ProtocolManager, shake bool, node *enode.Node) (*testPeer, <-chan error) {
	// Create a message pipe to communicate through
	app, net := p2p.MsgPipe()

	peer := pm.NewPeer(version, p2p.NewPeerFromNode(node, name, nil), net)

	// Start the Peer on a new thread
	errc := make(chan error, 1)
	go func() {
		select {
		case pm.newPeerCh <- peer:
			errc <- pm.handle(peer)
		case <-pm.quitSync:
			errc <- p2p.DiscQuitting
		}
	}()
	tp := &testPeer{app: app, net: net, Peer: peer}
	// Execute any implicitly requested handshakes and return
	if shake {
		var (
			genesis = pm.blockchain.Genesis()
			head    = pm.blockchain.CurrentHeader()
			td      = pm.blockchain.GetTd(head.Hash(), head.Number.Uint64())
		)
		tp.handshake(nil, td, head.Hash(), genesis.Hash())
	}
	return tp, errc
}

// handshake simulates a trivial handshake that expects the same state from the
// remote side as we are simulating locally.
func (p *testPeer) handshake(t *testing.T, td *big.Int, head common.Hash, genesis common.Hash) {
	msg := &statusData{
		ProtocolVersion: uint32(p.version),
		NetworkId:       DefaultConfig.NetworkId,
		TD:              td,
		CurrentBlock:    head,
		GenesisBlock:    genesis,
	}
	if err := p2p.ExpectMsg(p.app, StatusMsg, msg); err != nil {
		t.Fatalf("status recv: %v", err)
	}
	if err := p2p.Send(p.app, StatusMsg, msg); err != nil {
		t.Fatalf("status send: %v", err)
	}
}

func (p *testPeer) handshakeForTwoChain(t *testing.T, td, ftd *big.Int, head, fHead, genesis, fGenesis common.Hash) {
	msg := &statusData{
		ProtocolVersion: uint32(p.version),
		NetworkId:       1,
		TD:              td,
		CurrentBlock:    head,
		GenesisBlock:    genesis,
		FTD:             ftd,
		FCurrentBlock:   fHead,
		FGenesisBlock:   fGenesis,
	}
	if err := p2p.ExpectMsg(p.app, StatusMsg, msg); err != nil {
		t.Fatalf("status recv: %v", err)
	}
	if err := p2p.Send(p.app, StatusMsg, msg); err != nil {
		t.Fatalf("status send: %v", err)
	}
}

// close terminates the local side of the Peer, notifying the remote protocol
// manager of termination.
func (p *testPeer) close() {
	p.app.Close()
}

func mustGeneratePrivateKey(t *testing.T) *ecdsa.PrivateKey {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fail()
	}
	return privateKey
}

func RegisterNewPeer(pm *ProtocolManager, p *Peer) error {
	if err := pm.peers.Register(p); err != nil {
		return err
	}
	return nil
}
