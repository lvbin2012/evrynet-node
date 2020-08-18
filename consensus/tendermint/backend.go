package tendermint

import (
	"math/big"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/event"
)

// Backend provides application specific functions for Tendermint core
type Backend interface {
	// Address returns the Evrynet address of the node running this backend
	Address() common.Address

	// EventMux returns the event mux used for Core to subscribe/ send events back to Backend.
	// Think of it as pub/sub models
	EventMux() *event.TypeMux

	// Sign signs input data with the backend's private key
	Sign([]byte) ([]byte, error)

	// Gossip sends a message to all validators (exclude self)
	// these message are send via p2p network interface.
	Gossip(valSet ValidatorSet, blockNumber *big.Int, round int64, msgType uint64, payload []byte) error

	// Broadcast sends a message to all validators (including self)
	// It will call gossip and post an identical event to its EventMux().
	Broadcast(valSet ValidatorSet, blockNumber *big.Int, round int64, msgType uint64, payload []byte) error

	// Multicast sends a message to a group of given address
	// returns error if sending is failed, or not found the targets address
	Multicast(targets map[common.Address]bool, payload []byte) error

	// Validators returns the validator set
	// we should only use this method when core is started.
	Validators(blockNumber *big.Int) ValidatorSet

	// CurrentHeadBlock get the current block of from the canonical chain.
	CurrentHeadBlock() *types.Block

	// FindExistingPeers check validator peers exist or not by address
	FindExistingPeers(targets ValidatorSet) map[common.Address]consensus.Peer

	//Commit send the consensus block back to miner, it should also handle the logic after a block get enough vote to be the next block in chain
	Commit(block *types.Block)

	//Cancel send the consensus block back to miner if it is invalid for consensus.
	Cancel(block *types.Block)

	// VerifyProposalHeader checks whether a header conforms to the consensus rules of a
	// given engine. Verifying the seal may be done optionally here, or explicitly
	// via the VerifySeal method.
	VerifyProposalHeader(header *types.Header) error

	// VerifyProposalBlock verify post-processor state of proposal block (txs, Root, receipt).
	// If success, the result will be send to the pending tasks of miner
	VerifyProposalBlock(block *types.Block) error
}
