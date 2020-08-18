// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.
package types

import (
	"errors"
	"io"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/rlp"
)

var (
	// TendermintDigest
	// to identify whether the block is from Tendermint consensus engine
	//TODO: Add digest for our consensus, Digest represents a hash of ""
	TendermintDigest = common.HexToHash("0x0")

	// TendermintExtraVanity Fixed number of extra-data bytes reserved for validator vanity
	// A Tendermint's Header.Extra is : <normal ExtraData> + 0x00 til first 32 bytes + n. Seal for each 65 bytes after
	TendermintExtraVanity = 32
	// TendermintExtraSeal Fixed number of extra-data bytes reserved for validator seal
	// Each seal is exactly 65 bytes.
	TendermintExtraSeal = 65

	// ErrInvalidTendermintHeaderExtra is returned if the length of extra-data is less than 32 bytes
	ErrInvalidTendermintHeaderExtra = errors.New("invalid tendermint header extra-data")
)

// TendermintExtra extra data for Tendermint consensus
type TendermintExtra struct {
	Seal []byte
	// CommittedSeal is list seals of validators that committed the block, 65 * len(Validators) bytes
	CommittedSeal [][]byte
	// Set of authorized validators at this moment
	ValidatorAdds []byte
}

// EncodeRLP serializes ist into the Evrynet RLP format.
func (te *TendermintExtra) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		te.Seal,
		te.CommittedSeal,
		te.ValidatorAdds,
	})
}

// DecodeRLP implements rlp.Decoder, and load the tendermint fields from a RLP stream.
func (te *TendermintExtra) DecodeRLP(s *rlp.Stream) error {
	var tendermintExtra struct {
		Seal          []byte
		CommittedSeal [][]byte
		ValidatorAdds []byte
	}
	if err := s.Decode(&tendermintExtra); err != nil {
		return err
	}
	te.Seal, te.CommittedSeal, te.ValidatorAdds = tendermintExtra.Seal, tendermintExtra.CommittedSeal, tendermintExtra.ValidatorAdds
	return nil
}

// ExtractTendermintExtra extracts all values of the TendermintExtra from the header. It returns an
// error if the length of the given extra-data is less than 32 bytes or the extra-data can not
// be decoded.
func ExtractTendermintExtra(h *Header) (*TendermintExtra, error) {
	if len(h.Extra) < TendermintExtraVanity {
		return nil, ErrInvalidTendermintHeaderExtra
	}

	var tendermintExtra *TendermintExtra
	err := rlp.DecodeBytes(h.Extra[TendermintExtraVanity:], &tendermintExtra)
	if err != nil {
		return nil, err
	}
	return tendermintExtra, nil
}

// TendermintFilteredHeader returns a filtered header which some information (like seal, committed seals)
// are clean to fulfill the Tendermint hash rules. It returns nil if the extra-data cannot be
// decoded/encoded by rlp.
func TendermintFilteredHeader(h *Header, keepSeal bool) *Header {
	newHeader := CopyHeader(h)
	tendermintExtra, err := ExtractTendermintExtra(newHeader)
	if err != nil {
		return nil
	}

	if !keepSeal {
		tendermintExtra.Seal = []byte{}
	}
	tendermintExtra.CommittedSeal = [][]byte{}
	tendermintExtra.ValidatorAdds = []byte{}

	payload, err := rlp.EncodeToBytes(&tendermintExtra)
	if err != nil {
		return nil
	}

	newHeader.Extra = append(newHeader.Extra[:TendermintExtraVanity], payload...)

	return newHeader
}
