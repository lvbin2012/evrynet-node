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
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/p2p"
	"github.com/Evrynetlabs/evrynet-node/rlp"
)

var (
	errClosed            = errors.New("Peer set is closed")
	errAlreadyRegistered = errors.New("Peer is already registered")
	errNotRegistered     = errors.New("Peer is not registered")
)

const (
	maxKnownTxs    = 32768 // Maximum transactions hashes to keep in the known list (prevent DOS)
	maxKnownBlocks = 1024  // Maximum block hashes to keep in the known list (prevent DOS)

	// maxQueuedTxs is the maximum number of transaction lists to queue up before
	// dropping broadcasts. This is a sensitive number as a transaction list might
	// contain a single transaction, or thousands.
	maxQueuedTxs = 128

	// maxQueuedProps is the maximum number of block propagations to queue up before
	// dropping broadcasts. There's not much point in queueing stale blocks, so a few
	// that might cover uncles should be enough.
	maxQueuedProps = 4

	// maxQueuedAnns is the maximum number of block announcements to queue up before
	// dropping broadcasts. Similarly to block propagations, there's no point to queue
	// above some healthy uncle limit, so use that.
	maxQueuedAnns = 4

	handshakeTimeout = 5 * time.Second
)

// PeerInfo represents a short summary of the Evrynet sub-protocol metadata known
// about a connected Peer.
type PeerInfo struct {
	Version    int      `json:"version"`    // Evrynet protocol version negotiated
	Difficulty *big.Int `json:"difficulty"` // Total difficulty of the Peer's blockchain
	Head       string   `json:"head"`       // SHA3 hash of the Peer's best owned block
}

// propEvent is a block propagation, waiting for its turn in the broadcast queue.
type propEvent struct {
	block        *types.Block
	td           *big.Int
	isFinalChain bool
}

type annsEvent struct {
	block        *types.Block
	isFinalChain bool
}

type Peer struct {
	id string

	*p2p.Peer
	rw p2p.MsgReadWriter

	version  int         // Protocol version negotiated
	syncDrop *time.Timer // Timed connection dropper if sync progress isn't validated in time

	head common.Hash
	td   *big.Int

	fHead common.Hash
	fTD   *big.Int

	lock sync.RWMutex

	knownTxs     mapset.Set                // Set of transaction hashes known to be known by this Peer
	knownBlocks  mapset.Set                // Set of block hashes known to be known by this Peer
	knownFBlocks mapset.Set                // Set of block hashes known to be known by this Peer
	queuedTxs    chan []*types.Transaction // Queue of transactions to broadcast to the Peer
	queuedProps  chan *propEvent           // Queue of blocks to broadcast to the Peer
	queuedAnns   chan *annsEvent           // Queue of blocks to announce to the Peer
	term         chan struct{}             // Termination channel to stop the broadcaster
}

func newPeer(version int, p *p2p.Peer, rw p2p.MsgReadWriter) *Peer {
	return &Peer{
		Peer:         p,
		rw:           rw,
		version:      version,
		id:           fmt.Sprintf("%x", p.ID().Bytes()[:8]),
		knownTxs:     mapset.NewSet(),
		knownBlocks:  mapset.NewSet(),
		knownFBlocks: mapset.NewSet(),
		queuedTxs:    make(chan []*types.Transaction, maxQueuedTxs),
		queuedProps:  make(chan *propEvent, maxQueuedProps),
		queuedAnns:   make(chan *annsEvent, maxQueuedAnns),
		term:         make(chan struct{}),
	}
}

// broadcast is a write loop that multiplexes block propagations, announcements
// and transaction broadcasts into the remote Peer. The goal is to have an async
// writer that does not lock up node internals.
func (p *Peer) broadcast() {
	for {
		select {
		case txs := <-p.queuedTxs:
			if err := p.SendTransactions(txs); err != nil {
				return
			}
			p.Log().Trace("Broadcast transactions", "count", len(txs))

		case prop := <-p.queuedProps:
			if err := p.SendNewBlock(prop.block, prop.td, prop.isFinalChain); err != nil {
				return
			}
			p.Log().Trace("Propagated block", "number", prop.block.Number(), "hash", prop.block.Hash(), "td", prop.td)

		case anns := <-p.queuedAnns:
			if err := p.SendNewBlockHashes([]common.Hash{anns.block.Hash()}, []uint64{anns.block.NumberU64()},
				anns.isFinalChain); err != nil {
				return
			}
			p.Log().Trace("Announced block", "number", anns.block.Number(), "hash", anns.block.Hash())

		case <-p.term:
			return
		}
	}
}

// close signals the broadcast goroutine to terminate.
func (p *Peer) close() {
	close(p.term)
}

// Info gathers and returns a collection of metadata known about a Peer.
func (p *Peer) Info() *PeerInfo {
	hash, td := p.Head()

	return &PeerInfo{
		Version:    p.version,
		Difficulty: td,
		Head:       hash.Hex(),
	}
}

// Head retrieves a copy of the current head hash and total difficulty of the
// Peer.
func (p *Peer) Head() (hash common.Hash, td *big.Int) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	copy(hash[:], p.head[:])
	return hash, new(big.Int).Set(p.td)
}

func (p *Peer) FHead() (hash common.Hash, td *big.Int) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	copy(hash[:], p.fHead[:])
	return hash, new(big.Int).Set(p.fTD)
}

// SetHead updates the head hash and total difficulty of the Peer.
func (p *Peer) SetHead(hash common.Hash, td *big.Int) {
	p.lock.Lock()
	defer p.lock.Unlock()

	copy(p.head[:], hash[:])
	p.td.Set(td)
}

func (p *Peer) SetFHead(hash common.Hash, td *big.Int) {
	p.lock.Lock()
	defer p.lock.Unlock()

	copy(p.fHead[:], hash[:])
	p.fTD.Set(td)
}

// MarkBlock marks a block as known for the Peer, ensuring that the block will
// never be propagated to this particular Peer.
func (p *Peer) MarkBlock(hash common.Hash) {
	// If we reached the memory allowance, drop a previously known block hash
	for p.knownBlocks.Cardinality() >= maxKnownBlocks {
		p.knownBlocks.Pop()
	}
	p.knownBlocks.Add(hash)
}

// MarkTransaction marks a transaction as known for the Peer, ensuring that it
// will never be propagated to this particular Peer.
func (p *Peer) MarkTransaction(hash common.Hash) {
	// If we reached the memory allowance, drop a previously known transaction hash
	for p.knownTxs.Cardinality() >= maxKnownTxs {
		p.knownTxs.Pop()
	}
	p.knownTxs.Add(hash)
}

// Send writes an RLP-encoded message with the given code.
// data should encode as an RLP list.
func (p *Peer) Send(msgcode uint64, data interface{}) error {
	return p2p.Send(p.rw, msgcode, data)
}

// SendTransactions sends transactions to the Peer and includes the hashes
// in its transaction hash set for future reference.
func (p *Peer) SendTransactions(txs types.Transactions) error {
	// Mark all the transactions as known, but ensure we don't overflow our limits
	for _, tx := range txs {
		p.knownTxs.Add(tx.Hash())
	}
	for p.knownTxs.Cardinality() >= maxKnownTxs {
		p.knownTxs.Pop()
	}
	return p2p.Send(p.rw, TxMsg, txs)
}

// AsyncSendTransactions queues list of transactions propagation to a remote
// Peer. If the Peer's broadcast queue is full, the event is silently dropped.
func (p *Peer) AsyncSendTransactions(txs []*types.Transaction) {
	select {
	case p.queuedTxs <- txs:
		// Mark all the transactions as known, but ensure we don't overflow our limits
		for _, tx := range txs {
			p.knownTxs.Add(tx.Hash())
		}
		for p.knownTxs.Cardinality() >= maxKnownTxs {
			p.knownTxs.Pop()
		}
	default:
		p.Log().Debug("Dropping transaction propagation", "count", len(txs))
	}
}

// SendNewBlockHashes announces the availability of a number of blocks through
// a hash notification.
func (p *Peer) SendNewBlockHashes(hashes []common.Hash, numbers []uint64, isFinalChain bool) error {
	// Mark all the block hashes as known, but ensure we don't overflow our limits
	knowBlocks := p.knownBlocks
	if isFinalChain {
		knowBlocks = p.knownFBlocks
	}
	for _, hash := range hashes {
		knowBlocks.Add(hash)
	}
	for knowBlocks.Cardinality() >= maxKnownBlocks {
		knowBlocks.Pop()
	}
	request := make(newBlockHashesData, len(hashes))
	for i := 0; i < len(hashes); i++ {
		request[i].Hash = hashes[i]
		request[i].Number = numbers[i]
	}
	if isFinalChain {
		return p2p.Send(p.rw, NewFBlockHashesMsg, request)
	}
	return p2p.Send(p.rw, NewBlockHashesMsg, request)
}

// AsyncSendNewBlockHash queues the availability of a block for propagation to a
// remote Peer. If the Peer's broadcast queue is full, the event is silently
// dropped.
func (p *Peer) AsyncSendNewBlockHash(block *types.Block, isFinalChain bool) {
	select {
	case p.queuedAnns <- &annsEvent{block: block, isFinalChain: isFinalChain}:
		if isFinalChain {
			// Mark all the block hash as known, but ensure we don't overflow our limits
			p.knownFBlocks.Add(block.Hash())
			for p.knownFBlocks.Cardinality() >= maxKnownBlocks {
				p.knownFBlocks.Pop()
			}
		} else {
			// Mark all the block hash as known, but ensure we don't overflow our limits
			p.knownBlocks.Add(block.Hash())
			for p.knownBlocks.Cardinality() >= maxKnownBlocks {
				p.knownBlocks.Pop()
			}
		}
	default:
		p.Log().Debug("Dropping block announcement", "number", block.NumberU64(), "hash", block.Hash())
	}
}

// SendNewBlock propagates an entire block to a remote Peer.
func (p *Peer) SendNewBlock(block *types.Block, td *big.Int, isFinalChain bool) error {
	// Mark all the block hash as known, but ensure we don't overflow our limits
	p.knownBlocks.Add(block.Hash())
	for p.knownBlocks.Cardinality() >= maxKnownBlocks {
		p.knownBlocks.Pop()
	}
	if isFinalChain {
		return p2p.Send(p.rw, NewFBlockMsg, []interface{}{block, td})
	}
	return p2p.Send(p.rw, NewBlockMsg, []interface{}{block, td})
}

// AsyncSendNewBlock queues an entire block for propagation to a remote Peer. If
// the Peer's broadcast queue is full, the event is silently dropped.
func (p *Peer) AsyncSendNewBlock(block *types.Block, td *big.Int, isFinalChain bool) {
	select {
	case p.queuedProps <- &propEvent{block: block, td: td, isFinalChain: isFinalChain}:
		if isFinalChain {
			// Mark all the block hash as known, but ensure we don't overflow our limits
			p.knownFBlocks.Add(block.Hash())
			for p.knownFBlocks.Cardinality() >= maxKnownBlocks {
				p.knownFBlocks.Pop()
			}
		} else {
			// Mark all the block hash as known, but ensure we don't overflow our limits
			p.knownBlocks.Add(block.Hash())
			for p.knownBlocks.Cardinality() >= maxKnownBlocks {
				p.knownBlocks.Pop()
			}
		}
	default:
		p.Log().Debug("Dropping block propagation", "number", block.NumberU64(), "hash", block.Hash())
	}
}

// SendBlockHeaders sends a batch of block headers to the remote Peer.
func (p *Peer) SendBlockHeaders(headers []*types.Header, isFinalChain bool) error {
	if isFinalChain {
		return p2p.Send(p.rw, FBlockHeadersMsg, headers)
	}
	return p2p.Send(p.rw, BlockHeadersMsg, headers)
}

// SendBlockBodies sends a batch of block contents to the remote Peer.
func (p *Peer) SendBlockBodies(bodies []*blockBody, isFinalChain bool) error {
	if isFinalChain {
		return p2p.Send(p.rw, FBlockBodiesMsg, blockBodiesData(bodies))
	}
	return p2p.Send(p.rw, BlockBodiesMsg, blockBodiesData(bodies))
}

// SendBlockBodiesRLP sends a batch of block contents to the remote Peer from
// an already RLP encoded format.
func (p *Peer) SendBlockBodiesRLP(bodies []rlp.RawValue, isFinalChain bool) error {
	if isFinalChain {
		return p2p.Send(p.rw, FBlockBodiesMsg, bodies)
	}
	return p2p.Send(p.rw, BlockBodiesMsg, bodies)
}

func (p *Peer) SendEvilBlockRLP(evilBlocks []rlp.RawValue) error {
	return p2p.Send(p.rw, FEvilBlockMsg, evilBlocks)
}

// SendNodeDataRLP sends a batch of arbitrary internal data, corresponding to the
// hashes requested.
func (p *Peer) SendNodeData(data [][]byte, isFinalChain bool) error {
	if isFinalChain {
		return p2p.Send(p.rw, FNodeDataMsg, data)
	}
	return p2p.Send(p.rw, NodeDataMsg, data)
}

// SendReceiptsRLP sends a batch of transaction receipts, corresponding to the
// ones requested from an already RLP encoded format.
func (p *Peer) SendReceiptsRLP(receipts []rlp.RawValue, isFinalChain bool) error {
	if isFinalChain {
		return p2p.Send(p.rw, FReceiptsMsg, receipts)
	}
	return p2p.Send(p.rw, ReceiptsMsg, receipts)
}

// RequestOneHeader is a wrapper around the header query functions to fetch a
// single header. It is used solely by the fetcher.
func (p *Peer) RequestOneHeader(hash common.Hash, isFinalChain bool) error {
	p.Log().Debug("Fetching single header", "isFinalChain", isFinalChain, "hash", hash)
	if isFinalChain {
		return p2p.Send(p.rw, GetFBlockHeadersMsg, &getBlockHeadersData{Origin: hashOrNumber{Hash: hash}, Amount: uint64(1), Skip: uint64(0), Reverse: false})
	}
	return p2p.Send(p.rw, GetBlockHeadersMsg, &getBlockHeadersData{Origin: hashOrNumber{Hash: hash}, Amount: uint64(1), Skip: uint64(0), Reverse: false})
}

// RequestHeadersByHash fetches a batch of blocks' headers corresponding to the
// specified header query, based on the hash of an origin block.
func (p *Peer) RequestHeadersByHash(origin common.Hash, amount int, skip int, reverse bool, isFinalChain bool) error {
	p.Log().Debug("Fetching batch of headers", "isFinalChain", isFinalChain, "count", amount, "fromhash", origin, "skip", skip, "reverse", reverse)
	if isFinalChain {
		return p2p.Send(p.rw, GetFBlockHeadersMsg, &getBlockHeadersData{Origin: hashOrNumber{Hash: origin}, Amount: uint64(amount), Skip: uint64(skip), Reverse: reverse})
	}
	return p2p.Send(p.rw, GetBlockHeadersMsg, &getBlockHeadersData{Origin: hashOrNumber{Hash: origin}, Amount: uint64(amount), Skip: uint64(skip), Reverse: reverse})
}

// RequestHeadersByNumber fetches a batch of blocks' headers corresponding to the
// specified header query, based on the number of an origin block.
func (p *Peer) RequestHeadersByNumber(origin uint64, amount int, skip int, reverse bool, isFinalChain bool) error {
	p.Log().Debug("Fetching batch of headers", "isFinalChain", isFinalChain, "count", amount, "fromnum", origin, "skip", skip, "reverse", reverse)
	if isFinalChain {
		return p2p.Send(p.rw, GetFBlockHeadersMsg, &getBlockHeadersData{Origin: hashOrNumber{Number: origin}, Amount: uint64(amount), Skip: uint64(skip), Reverse: reverse})
	}
	return p2p.Send(p.rw, GetBlockHeadersMsg, &getBlockHeadersData{Origin: hashOrNumber{Number: origin}, Amount: uint64(amount), Skip: uint64(skip), Reverse: reverse})
}

// specified.
func (p *Peer) RequestBodies(hashes []common.Hash, isFinalChain bool) error {
	p.Log().Debug("Fetching batch of block bodies", "isFinalChain", isFinalChain, "count", len(hashes))
	if isFinalChain {
		return p2p.Send(p.rw, GetFBlockBodiesMsg, hashes)
	}
	return p2p.Send(p.rw, GetBlockBodiesMsg, hashes)
}

// RequestNodeData fetches a batch of arbitrary data from a node's known state
// data, corresponding to the specified hashes.
func (p *Peer) RequestNodeData(hashes []common.Hash, isFinalChain bool) error {
	p.Log().Debug("Fetching batch of state data", "isFinalChain", isFinalChain, "count", len(hashes))
	if isFinalChain {
		return p2p.Send(p.rw, GetFNodeDataMsg, hashes)
	}
	return p2p.Send(p.rw, GetNodeDataMsg, hashes)
}

func (p *Peer) RequestEvilBodies(hashes []common.Hash) error {
	p.Log().Debug("Fetching batch of block bodies", "count", len(hashes))
	return p2p.Send(p.rw, GetFEvilBlockMsg, hashes)
}

func (p *Peer) RequestEvilReceipts(hashes []common.Hash) error {
	panic("implement me later")
}

func (p *Peer) RequestEvilHeadersByHash(h common.Hash) error {
	panic("implement me later")
}

func (p *Peer) RequestEvilHeadersByNumber(i uint64) error {
	panic("implement me later")
}

// RequestReceipts fetches a batch of transaction receipts from a remote node.
func (p *Peer) RequestReceipts(hashes []common.Hash, isFinalChain bool) error {
	p.Log().Debug("Fetching batch of receipts", "isFinalChain", isFinalChain, "count", len(hashes))
	if isFinalChain {
		return p2p.Send(p.rw, GetFReceiptsMsg, hashes)
	}
	return p2p.Send(p.rw, GetReceiptsMsg, hashes)
}

// Handshake executes the evr protocol handshake, negotiating version number,
// network IDs, difficulties, head and genesis blocks.
func (p *Peer) Handshake(network uint64, td *big.Int, head common.Hash, genesis common.Hash, fTD *big.Int, fHead common.Hash, fGenesis common.Hash) error {
	// Send out own handshake in a new thread
	errc := make(chan error, 2)
	var status statusData // safe to read after two values have been received from errc

	go func() {
		errc <- p2p.Send(p.rw, StatusMsg, &statusData{
			ProtocolVersion: uint32(p.version),
			NetworkId:       network,
			TD:              td,
			CurrentBlock:    head,
			GenesisBlock:    genesis,
			FTD:             fTD,
			FCurrentBlock:   fHead,
			FGenesisBlock:   fGenesis,
		})
	}()
	go func() {
		errc <- p.readStatus(network, &status, genesis, fGenesis)
	}()
	timeout := time.NewTimer(handshakeTimeout)
	defer timeout.Stop()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errc:
			if err != nil {
				return err
			}
		case <-timeout.C:
			return p2p.DiscReadTimeout
		}
	}
	p.td, p.head, p.fTD, p.fHead = status.TD, status.CurrentBlock, status.FTD, status.FCurrentBlock
	return nil
}

func (p *Peer) readStatus(network uint64, status *statusData, genesis common.Hash, fGenesis common.Hash) (err error) {
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Code != StatusMsg {
		return errResp(ErrNoStatusMsg, "first msg has code %x (!= %x)", msg.Code, StatusMsg)
	}
	if msg.Size > ProtocolMaxMsgSize {
		return errResp(ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
	}
	// Decode the handshake and make sure everything matches
	if err := msg.Decode(&status); err != nil {
		return errResp(ErrDecode, "msg %v: %v", msg, err)
	}
	if status.GenesisBlock != genesis {
		return errResp(ErrGenesisBlockMismatch, "%x (!= %x)", status.GenesisBlock[:8], genesis[:8])
	}
	if status.FGenesisBlock != fGenesis {
		return errResp(ErrGenesisBlockMismatch, " fGenesis %x (!= %x)", status.GenesisBlock[:8], genesis[:8])
	}
	if status.NetworkId != network {
		return errResp(ErrNetworkIdMismatch, "%d (!= %d)", status.NetworkId, network)
	}
	if int(status.ProtocolVersion) != p.version {
		return errResp(ErrProtocolVersionMismatch, "%d (!= %d)", status.ProtocolVersion, p.version)
	}
	return nil
}

// String implements fmt.Stringer.
func (p *Peer) String() string {
	return fmt.Sprintf("Peer %s [%s]", p.id,
		fmt.Sprintf("evr/%2d", p.version),
	)
}

// Address return Evrynet Address of a Peer
func (p *Peer) Address() common.Address {
	pubKey := p.Node().Pubkey()
	if pubKey != nil {
		return (crypto.PubkeyToAddress(*pubKey))
	}
	return common.Address{}
}

// peerSet represents the collection of active peers currently participating in
// the Evrynet sub-protocol.
type peerSet struct {
	peers  map[string]*Peer
	lock   sync.RWMutex
	closed bool
}

// newPeerSet creates a new Peer set to track the active participants.
func newPeerSet() *peerSet {
	return &peerSet{
		peers: make(map[string]*Peer),
	}
}

// Register injects a new Peer into the working set, or returns an error if the
// Peer is already known. If a new Peer it registered, its broadcast loop is also
// started.
func (ps *peerSet) Register(p *Peer) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if ps.closed {
		return errClosed
	}
	if _, ok := ps.peers[p.id]; ok {
		return errAlreadyRegistered
	}
	ps.peers[p.id] = p
	go p.broadcast()

	return nil
}

// Unregister removes a remote Peer from the active set, disabling any further
// actions to/from that particular entity.
func (ps *peerSet) Unregister(id string) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	p, ok := ps.peers[id]
	if !ok {
		return errNotRegistered
	}
	delete(ps.peers, id)
	p.close()

	return nil
}

// Peers returns all registered peers
func (ps *peerSet) Peers() map[string]*Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	set := make(map[string]*Peer)
	for id, p := range ps.peers {
		set[id] = p
	}
	return set
}

// Peer retrieves the registered Peer with the given id.
func (ps *peerSet) Peer(id string) *Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.peers[id]
}

// Len returns if the current number of peers in the set.
func (ps *peerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.peers)
}

// PeersWithoutBlock retrieves a list of peers that do not have a given block in
// their set of known hashes.
func (ps *peerSet) PeersWithoutBlock(hash common.Hash) []*Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*Peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.knownBlocks.Contains(hash) {
			list = append(list, p)
		}
	}
	return list
}

// PeersWithoutTx retrieves a list of peers that do not have a given transaction
// in their set of known hashes.
func (ps *peerSet) PeersWithoutTx(hash common.Hash) []*Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*Peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.knownTxs.Contains(hash) {
			list = append(list, p)
		}
	}
	return list
}

// BestPeer retrieves the known Peer with the currently highest total difficulty.
func (ps *peerSet) BestPeer() *Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	var (
		bestPeer *Peer
		bestTd   *big.Int
	)
	for _, p := range ps.peers {
		if _, td := p.Head(); bestPeer == nil || td.Cmp(bestTd) > 0 {
			bestPeer, bestTd = p, td
		}
	}
	return bestPeer
}

// Close disconnects all peers.
// No new peers can be registered after Close has returned.
func (ps *peerSet) Close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	for _, p := range ps.peers {
		p.Disconnect(p2p.DiscQuitting)
	}
	ps.closed = true
}
