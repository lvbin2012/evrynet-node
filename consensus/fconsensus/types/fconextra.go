package types

import (
	"bytes"
	"errors"
	"io"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/rlp"
)

const (
	ExtraVanity = 32
)

type FConExtra struct {
	Seal          []byte
	CurrentBlock  common.Hash
	CurrentHeight uint64
	EvilHeader    *types.Header
	Signers       []common.Address
}

func (fce *FConExtra) EncodeRLP(w io.Writer) error {
	headerRLP, err := rlp.EncodeToBytes(fce.EvilHeader)
	if err != nil {
		return err
	}
	return rlp.Encode(w, []interface{}{
		fce.Seal,
		fce.CurrentBlock,
		fce.CurrentHeight,
		headerRLP,
		fce.Signers,
	})
}

func (fce *FConExtra) DecodeRLP(s *rlp.Stream) error {
	var extra struct {
		Seal          []byte
		CurrentBlock  common.Hash
		CurrentHeight uint64
		EvilBytes     []byte
		Signers       []common.Address
	}
	if err := s.Decode(&extra); err != nil {
		return err
	}
	fce.Seal, fce.CurrentBlock, fce.CurrentHeight, fce.Signers = extra.Seal, extra.CurrentBlock, extra.CurrentHeight, extra.Signers

	if len(extra.EvilBytes) > 1 {
		var header types.Header
		if err := rlp.Decode(bytes.NewReader(extra.EvilBytes), &header); err != nil {
			return err
		}
		fce.EvilHeader = &header
	}
	return nil
}

func ExtractFConExtra(header *types.Header) (*FConExtra, error) {
	if len(header.Extra) < ExtraVanity {
		return nil, errors.New("invalid header extra-data")
	}
	var extra FConExtra
	if err := rlp.Decode(bytes.NewReader(header.Extra[ExtraVanity:]), &extra); err != nil {
		return nil, err
	}
	return &extra, nil
}
