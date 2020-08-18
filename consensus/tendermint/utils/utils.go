package utils

import (
	"bytes"
	"errors"

	"golang.org/x/crypto/sha3"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/rlp"
)

var (
	ErrInvalidSealLength = errors.New("seal is expected to be multiplication of 65")
)

const (
	msgCommit uint64 = iota
)

// sigHash returns the hash
// signing. It is the hash of the entire header apart from the 65 byte signature
// contained at the end of the extra data.
func SigHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.New256()

	// Clean seal is required for calculating proposer seal.
	_ = rlp.Encode(hasher, types.TendermintFilteredHeader(header, false))
	hasher.Sum(hash[:0])

	return hash
}

// WriteSeal writes the extra-data field of the given header with the given seals.
// suggest to rename to writeSeal.
func WriteSeal(h *types.Header, seal []byte) error {
	if len(seal)%types.TendermintExtraSeal != 0 {
		return ErrInvalidSealLength
	}

	tendermintExtra, err := types.ExtractTendermintExtra(h)
	if err != nil {
		return err
	}

	tendermintExtra.Seal = seal
	payload, err := rlp.EncodeToBytes(&tendermintExtra)
	if err != nil {
		return err
	}

	h.Extra = append(h.Extra[:types.TendermintExtraVanity], payload...)
	return nil
}

// WriteValSet writes the extra-data field of the given header with the given val-sets address.
func WriteValSet(h *types.Header, validators []common.Address) error {
	tendermintExtra, err := types.ExtractTendermintExtra(h)
	if err != nil {
		return err
	}

	// RLP encode validator's address to bytes
	valSetData, err := rlp.EncodeToBytes(validators)
	if err != nil {
		return err
	}
	tendermintExtra.ValidatorAdds = valSetData

	payload, err := rlp.EncodeToBytes(&tendermintExtra)
	if err != nil {
		return err
	}

	h.Extra = append(h.Extra[:types.TendermintExtraVanity], payload...)
	return nil
}

// WriteCommittedSeals writes the extra-data field of a block header with given committed seals.
func WriteCommittedSeals(h *types.Header, committedSeals [][]byte) error {
	if len(committedSeals) == 0 {
		return ErrInvalidSealLength
	}

	for _, seal := range committedSeals {
		if len(seal) != types.TendermintExtraSeal {
			return ErrInvalidSealLength
		}
	}

	tendermintExtra, err := types.ExtractTendermintExtra(h)
	if err != nil {
		return err
	}

	tendermintExtra.CommittedSeal = make([][]byte, len(committedSeals))
	copy(tendermintExtra.CommittedSeal, committedSeals)

	payload, err := rlp.EncodeToBytes(&tendermintExtra)
	if err != nil {
		return err
	}

	h.Extra = append(h.Extra[:types.TendermintExtraVanity], payload...)
	return nil
}

// GetSignatureAddress gets the signer address from the signature
func GetSignatureAddress(data []byte, sig []byte) (common.Address, error) {
	// 1. Keccak data
	hashData := crypto.Keccak256(data)
	// 2. Recover public key
	pubkey, err := crypto.SigToPub(hashData, sig)
	if err != nil {
		return common.Address{}, err
	}
	return crypto.PubkeyToAddress(*pubkey), nil
}

// PrepareCommittedSeal returns a committed seal for the given hash
func PrepareCommittedSeal(hash common.Hash) []byte {
	var buf bytes.Buffer
	buf.Write(hash.Bytes())
	buf.Write([]byte{byte(msgCommit)})
	return buf.Bytes()
}

// GetCheckpointNumber returns check-point block where header contains valset of current epoch
func GetCheckpointNumber(epochDuration uint64, blockNumber uint64) uint64 {
	if blockNumber == 0 || blockNumber < epochDuration {
		return 0
	}
	return epochDuration * ((blockNumber - 1) / epochDuration)
}

// GetValSetAddresses returns the address of validators from the extra-data field.
func GetValSetAddresses(h *types.Header) ([]common.Address, error) {
	tdmExtra, err := types.ExtractTendermintExtra(h)
	if err != nil {
		return nil, err
	}
	if len(tdmExtra.ValidatorAdds) == 0 {
		return nil, tendermint.ErrEmptyValSet
	}

	// RLP decode validator's address from bytes
	var validators []common.Address
	err = rlp.DecodeBytes(tdmExtra.ValidatorAdds, &validators)
	if err != nil {
		return nil, err
	}

	return validators, nil
}
