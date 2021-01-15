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

package downloader

import (
	"fmt"

	"github.com/Evrynetlabs/evrynet-node/core/types"
)

// peerDropFn is a callback type for dropping a peer detected as malicious.
type peerDropFn func(id string)

// dataPack is a data message returned by a peer for some query.
type dataPack interface {
	PeerId() string
	IsFinalChain() bool
	Items() int
	Stats() string
}

// headerPack is a batch of block headers returned by a peer.
type headerPack struct {
	peerID       string
	isFinalChain bool
	headers      []*types.Header
}

func (p *headerPack) PeerId() string     { return p.peerID }
func (p *headerPack) IsFinalChain() bool { return p.isFinalChain }
func (p *headerPack) Items() int         { return len(p.headers) }
func (p *headerPack) Stats() string      { return fmt.Sprintf("%d", len(p.headers)) }

type headerProcEvent struct {
	headers      []*types.Header
	isFinalChain bool
}

// evilBlockPack is batch of evil block returned by a peer
type evilBlockPack struct {
	peerID       string
	isFinalChain bool
	transactions [][]*types.Transaction
	uncles       [][]*types.Header
}

func (p *evilBlockPack) PeerId() string     { return p.peerID }
func (p *evilBlockPack) IsFinalChain() bool { return p.isFinalChain }
func (p *evilBlockPack) Items() int {
	if len(p.transactions) <= len(p.uncles) {
		return len(p.transactions)
	}
	return len(p.uncles)
}
func (p *evilBlockPack) Stats() string {
	return fmt.Sprintf("%d:%d", len(p.transactions), len(p.uncles))
}

// bodyPack is a batch of block bodies returned by a peer.
type bodyPack struct {
	peerID       string
	isFinalChain bool
	transactions [][]*types.Transaction
	uncles       [][]*types.Header
}

func (p *bodyPack) PeerId() string     { return p.peerID }
func (p *bodyPack) IsFinalChain() bool { return p.isFinalChain }
func (p *bodyPack) Items() int {
	if len(p.transactions) <= len(p.uncles) {
		return len(p.transactions)
	}
	return len(p.uncles)
}
func (p *bodyPack) Stats() string { return fmt.Sprintf("%d:%d", len(p.transactions), len(p.uncles)) }

// receiptPack is a batch of receipts returned by a peer.
type receiptPack struct {
	peerID       string
	isFinalChain bool
	receipts     [][]*types.Receipt
}

func (p *receiptPack) PeerId() string     { return p.peerID }
func (p *receiptPack) IsFinalChain() bool { return p.isFinalChain }
func (p *receiptPack) Items() int         { return len(p.receipts) }
func (p *receiptPack) Stats() string      { return fmt.Sprintf("%d", len(p.receipts)) }

// statePack is a batch of states returned by a peer.
type statePack struct {
	peerID       string
	isFinalChain bool
	states       [][]byte
}

func (p *statePack) PeerId() string     { return p.peerID }
func (p *statePack) IsFinalChain() bool { return p.isFinalChain }
func (p *statePack) Items() int         { return len(p.states) }
func (p *statePack) Stats() string      { return fmt.Sprintf("%d", len(p.states)) }
