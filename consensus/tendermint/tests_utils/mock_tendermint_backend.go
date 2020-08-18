package tests_utils

import (
	"crypto/ecdsa"
	"math/big"
	"sync"
	"testing"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint/validator"
	"github.com/Evrynetlabs/evrynet-node/core"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/event"
	"github.com/Evrynetlabs/evrynet-node/log"
	"github.com/Evrynetlabs/evrynet-node/params"
)

type MockBackend struct {
	config             *tendermint.Config
	tendermintEventMux *event.TypeMux
	privateKey         *ecdsa.PrivateKey
	address            common.Address
	validators         []common.Address

	//once voting finish, the block will be send for commit here
	//it is a map of blocknumber- channels with mutex
	mutex *sync.RWMutex
	chain consensus.ChainReader

	//storingMsgs is used to store msg to handler when core stopped
	currentBlock func() *types.Block
	// SendEventMux is used for receiving output msg from core
	SendEventMux *event.TypeMux
}

//SentMsgEvent represents an action send to an peer
type SentMsgEvent struct {
	Target  common.Address
	Payload []byte
}

func NewMockBackend(privateKey *ecdsa.PrivateKey, blockchain *MockChainReader, validators []common.Address) tendermint.Backend {
	return &MockBackend{
		config:             tendermint.DefaultConfig,
		tendermintEventMux: new(event.TypeMux),
		privateKey:         privateKey,
		address:            crypto.PubkeyToAddress(privateKey.PublicKey),
		mutex:              &sync.RWMutex{},
		chain:              blockchain,
		currentBlock:       blockchain.CurrentBlock,
		validators:         validators,
		SendEventMux:       new(event.TypeMux),
	}
}

func (mb *MockBackend) VerifyProposalHeader(header *types.Header) error {
	log.Warn("mocked backend always return nil when verifyProposalHeader")
	return nil
}

func (mb *MockBackend) VerifyProposalBlock(block *types.Block) error {
	var (
		txs     = block.Transactions()
		txsHash = types.DeriveSha(txs)
	)

	// Verify txs hash
	if txsHash != block.Header().TxHash {
		return tendermint.ErrMismatchTxhashes
	}
	return nil
}

// EventMux implements tendermint.Backend.EventMux
func (mb *MockBackend) EventMux() *event.TypeMux {
	return mb.tendermintEventMux
}

// Sign implements tendermint.Backend.Sign
func (mb *MockBackend) Sign(data []byte) ([]byte, error) {
	hashData := crypto.Keccak256(data)
	return crypto.Sign(hashData, mb.privateKey)
}

// Address implements tendermint.Backend.Address
func (mb *MockBackend) Address() common.Address {
	return mb.address
}

// Broadcast implements tendermint.Backend.Broadcast
// It sends message to its validator by calling gossiping, and send message to itself by eventMux
func (mb *MockBackend) Broadcast(valSet tendermint.ValidatorSet, blockNumber *big.Int, round int64, msgType uint64, payload []byte) error {
	// send to others
	if err := mb.Gossip(valSet, blockNumber, round, msgType, payload); err != nil {
		return err
	}
	// send to self
	go func() {
		if err := mb.EventMux().Post(tendermint.MessageEvent{
			Payload: payload,
		}); err != nil {
			log.Error("failed to post event to self", "error", err)
		}
	}()
	return nil
}

// Gossip implements tendermint.Backend.Gossip
// It sends message to its validators only, not itself.
// The validators must be able to connected through Peer.
// It will return MockBackend.ErrNoBroadcaster if no broadcaster is set for MockBackend
func (mb *MockBackend) Gossip(valSet tendermint.ValidatorSet, _ *big.Int, _ int64, _ uint64, payload []byte) error {
	for _, validator := range valSet.List() {
		if validator.Address() == mb.address {
			continue
		}
		if err := mb.SendEventMux.Post(SentMsgEvent{Target: validator.Address(), Payload: payload}); err != nil {
			return err
		}
	}
	return nil
}

// Multicast implements tendermint.Backend.Multicast
func (mb *MockBackend) Multicast(targets map[common.Address]bool, payload []byte) error {
	for addr := range targets {
		if addr == mb.address {
			continue
		}
		if err := mb.SendEventMux.Post(SentMsgEvent{Target: addr, Payload: payload}); err != nil {
			return err
		}
	}
	return nil
}

// Validators return validator set for a block number
// TODO: revise this function once auth vote is implemented
func (mb *MockBackend) Validators(blockNumber *big.Int) tendermint.ValidatorSet {
	log.Error("not implemented")
	return validator.NewSet(mb.validators, mb.config.ProposerPolicy, int64(0))
}

// FindExistingPeers check validator peers exist or not by address
func (mb *MockBackend) FindExistingPeers(valSet tendermint.ValidatorSet) map[common.Address]consensus.Peer {
	log.Error("not implemented")
	return make(map[common.Address]consensus.Peer)
}

//Commit implement tendermint.Backend.Commit()
func (mb *MockBackend) Commit(block *types.Block) {
	log.Error("not implemented")
}

func (mb *MockBackend) CurrentHeadBlock() *types.Block {
	return mb.currentBlock()
}

func (mb *MockBackend) Cancel(block *types.Block) {
	log.Error("not implemented")
}

func MustCreateAndStartNewBackend(t *testing.T, nodePrivateKey *ecdsa.PrivateKey, genesisHeader *types.Header, validators []common.Address) (tendermint.Backend, *core.TxPool) {
	var (
		address = crypto.PubkeyToAddress(nodePrivateKey.PublicKey)
		trigger = false
		statedb = MustCreateStateDB(t)

		testTxPoolConfig core.TxPoolConfig
		blockchain       = &MockChainReader{
			GenesisHeader: genesisHeader,
			MockBlockChain: &MockBlockChain{
				Statedb:       statedb,
				GasLimit:      1000000000,
				ChainHeadFeed: new(event.Feed),
			},
			Address: address,
			Trigger: &trigger,
		}
		pool = core.NewTxPool(testTxPoolConfig, params.TendermintTestChainConfig, blockchain)
		be   = NewMockBackend(nodePrivateKey, blockchain, validators)
	)
	statedb.SetBalance(address, new(big.Int).SetUint64(params.Ether))
	return be, pool
}
