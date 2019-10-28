package backend

import (
	"crypto/ecdsa"
	"errors"
	"math/big"
	"sync"

	queue "github.com/enriquebris/goconcurrentqueue"

	"github.com/evrynet-official/evrynet-client/common"
	"github.com/evrynet-official/evrynet-client/consensus"
	"github.com/evrynet-official/evrynet-client/consensus/tendermint"
	tendermintCore "github.com/evrynet-official/evrynet-client/consensus/tendermint/core"
	"github.com/evrynet-official/evrynet-client/consensus/tendermint/validator"
	"github.com/evrynet-official/evrynet-client/core/types"
	"github.com/evrynet-official/evrynet-client/crypto"
	"github.com/evrynet-official/evrynet-client/ethdb"
	"github.com/evrynet-official/evrynet-client/event"
	"github.com/evrynet-official/evrynet-client/log"
)

const (
	fetcherID         = "tendermint"
	maxNumberMessages = 64 * 128 * 6 // 64 node * 128 round * 6 messages per round. These number are made higher than expected for safety.
)

var (
	//ErrNoBroadcaster is return when trying to access backend.Broadcaster without SetBroadcaster first
	ErrNoBroadcaster = errors.New("no broadcaster is set")
)

//Option return an optional function for backend's initial behaviour
type Option func(b *backend) error

//WithDB return an option to set backend's db
func WithDB(db ethdb.Database) Option {
	return func(b *backend) error {
		b.db = db
		return nil
	}
}

// New creates an backend for Istanbul core engine.
// The p2p communication, i.e, broadcaster is set separately by calling backend.SetBroadcaster
func New(config *tendermint.Config, privateKey *ecdsa.PrivateKey, opts ...Option) consensus.Tendermint {
	be := &backend{
		config:             config,
		tendermintEventMux: new(event.TypeMux),
		privateKey:         privateKey,
		address:            crypto.PubkeyToAddress(privateKey.PublicKey),
		commitChs:          newCommitChannels(),
		mutex:              &sync.RWMutex{},
		storingMsgs:        queue.NewFIFO(),
	}
	be.core = tendermintCore.New(be, tendermint.DefaultConfig)
	for _, opt := range opts {
		if err := opt(be); err != nil {
			log.Error("error at initialization of backend",
				err)
		}
	}
	return be
}

// SetBroadcaster implements consensus.Handler.SetBroadcaster
func (sb *backend) SetBroadcaster(broadcaster consensus.Broadcaster) {
	sb.broadcaster = broadcaster
}

// ----------------------------------------------------------------------------
type backend struct {
	config             *tendermint.Config
	tendermintEventMux *event.TypeMux
	privateKey         *ecdsa.PrivateKey
	core               tendermintCore.Engine
	db                 ethdb.Database
	broadcaster        consensus.Broadcaster
	address            common.Address

	//once voting finish, the block will be send for commit here
	//it is a map of blocknumber- channels with mutex
	commitChs *commitChannels

	coreStarted bool
	mutex       *sync.RWMutex
	chain       consensus.ChainReader

	//storingMsgs is used to store msg to handler when core stopped
	storingMsgs *queue.FIFO

	currentBlock func() *types.Block
}

// EventMux implements tendermint.Backend.EventMux
func (sb *backend) EventMux() *event.TypeMux {
	return sb.tendermintEventMux
}

// Sign implements tendermint.Backend.Sign
func (sb *backend) Sign(data []byte) ([]byte, error) {
	hashData := crypto.Keccak256(data)
	return crypto.Sign(hashData, sb.privateKey)
}

// Address implements tendermint.Backend.Address
func (sb *backend) Address() common.Address {
	return sb.address
}

// Broadcast implements tendermint.Backend.Broadcast
// It sends message to its validator by calling gossiping, and send message to itself by eventMux
func (sb *backend) Broadcast(valSet tendermint.ValidatorSet, payload []byte) error {
	// send to others
	if err := sb.Gossip(valSet, payload); err != nil {
		return err
	}
	// send to self
	go func() {
		if err := sb.EventMux().Post(tendermint.MessageEvent{
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
// It will return backend.ErrNoBroadcaster if no broadcaster is set for backend
func (sb *backend) Gossip(valSet tendermint.ValidatorSet, payload []byte) error {
	//TODO: check for known message by lru.ARCCache

	targets := make(map[common.Address]bool)

	for _, val := range valSet.List() {
		if val.Address() != sb.address {
			targets[val.Address()] = true
		}
	}
	if sb.broadcaster == nil {
		return ErrNoBroadcaster
	}
	if len(targets) > 0 {
		ps := sb.broadcaster.FindPeers(targets)
		log.Info("prepare to send message to peers", "total_peers", len(ps))
		for _, p := range ps {
			//TODO: check for recent messsages using lru.ARCCache
			go func(p consensus.Peer) {
				if err := p.Send(consensus.TendermintMsg, payload); err != nil {
					log.Error("failed to send message to peer", "error", err)
				}
			}(p)
		}
	}
	return nil
}

// Validators return validator set for a block number
// TODO: revise this function once auth vote is implemented
func (sb *backend) Validators(blockNumber *big.Int) tendermint.ValidatorSet {
	var (
		previousBlock uint64
		header        *types.Header
		err           error
		snap          *Snapshot
	)
	// check if blockNumber is zero
	if blockNumber.Cmp(big.NewInt(0)) == 0 {
		previousBlock = 0
	} else {
		previousBlock = uint64(blockNumber.Int64() - 1)
	}
	header = sb.chain.GetHeaderByNumber(previousBlock)
	if header == nil {
		log.Error("cannot get valSet since previousBlock is not available", "block_number", blockNumber)
	}
	snap, err = sb.snapshot(sb.chain, previousBlock, header.Hash(), nil)
	if err != nil {
		log.Error("cannot load snapshot", "error", err)
	}
	if err == nil {
		return snap.ValSet
	}
	return validator.NewSet(nil, sb.config.ProposerPolicy, int64(0))
}

// FindExistingPeers check validator peers exist or not by address
func (sb *backend) FindExistingPeers(valSet tendermint.ValidatorSet) map[common.Address]consensus.Peer {
	targets := make(map[common.Address]bool)
	for _, val := range valSet.List() {
		if val.Address() != sb.Address() {
			targets[val.Address()] = true
		}
	}
	return sb.broadcaster.FindPeers(targets)
}

//Commit implement tendermint.Backend.Commit()
func (sb *backend) Commit(block *types.Block) {
	ch, ok := sb.commitChs.getCommitChannel(block.Number().String())
	if !ok {
		log.Error("no commit channel available", "block_number", block.Number().String())
		return
	}
	ch <- block

	// if node is not proposer, EnqueueBlock for downloading
	if block.Coinbase() != sb.address {
		sb.EnqueueBlock(block)
	}
}

// EnqueueBlock adds a block returned from consensus into fetcher queue
func (sb *backend) EnqueueBlock(block *types.Block) {
	if sb.broadcaster != nil {
		sb.broadcaster.Enqueue(fetcherID, block)
	}
}

func (sb *backend) CurrentHeadBlock() *types.Block {
	return sb.currentBlock()
}

//ClearStoringMsg will delete all item in queue
func (sb *backend) ClearStoringMsg() {
	log.Info("Clear storing msg queue")
	sb.storingMsgs = queue.NewFIFO()
}
