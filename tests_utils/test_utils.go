package tests_utils

import (
	"bytes"
	"crypto/ecdsa"
	"errors"
	"math/big"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/core/rawdb"
	"github.com/Evrynetlabs/evrynet-node/core/state"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/event"
	"github.com/Evrynetlabs/evrynet-node/rlp"
)

func MakeNodeKey() *ecdsa.PrivateKey {
	key, _ := GeneratePrivateKey()
	return key
}

func MakeGenesisHeader(validators []common.Address) *types.Header {
	var header = &types.Header{
		Number:     big.NewInt(int64(0)),
		ParentHash: common.HexToHash("0x01"),
		UncleHash:  types.CalcUncleHash(nil),
		Root:       common.HexToHash("0x0"),
		Difficulty: big.NewInt(1),
		MixDigest:  types.TendermintDigest,
	}
	extra, _ := PrepareExtra(header)

	var buf bytes.Buffer
	buf.Write(extra[:types.TendermintExtraVanity])
	valSetData, _ := rlp.EncodeToBytes(validators)
	tdm := &types.TendermintExtra{
		Seal:          []byte{},
		CommittedSeal: [][]byte{},
		ValidatorAdds: valSetData,
	}
	payload, _ := rlp.EncodeToBytes(&tdm)

	header.Extra = append(buf.Bytes(), payload...)
	return header
}

func GeneratePrivateKey() (*ecdsa.PrivateKey, error) {
	return crypto.GenerateKey()
}

// PrepareExtra returns a extra-data of the given header and validators
func PrepareExtra(header *types.Header) ([]byte, error) {
	var buf bytes.Buffer

	// compensate the lack bytes if header.Extra is not enough TendermintExtraVanity bytes.
	if len(header.Extra) < types.TendermintExtraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, types.TendermintExtraVanity-len(header.Extra))...)
	}
	buf.Write(header.Extra[:types.TendermintExtraVanity])

	tdm := &types.TendermintExtra{}
	payload, err := rlp.EncodeToBytes(&tdm)
	if err != nil {
		return nil, err
	}

	return append(buf.Bytes(), payload...), nil
}

func GenerateMockChainReader() (*MockChainReader, error) {
	var (
		nodePrivateKey = MakeNodeKey()
		nodeAddr       = crypto.PubkeyToAddress(nodePrivateKey.PublicKey)
		validators     = []common.Address{nodeAddr}
		genesisHeader  = MakeGenesisHeader(validators)
		stateDB, err   = state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()))
	)
	if err != nil {
		return nil, errors.New("failed to create stateDB, error: " + err.Error())
	}

	return &MockChainReader{
		GenesisHeader: genesisHeader,
		MockBlockChain: &MockBlockChain{
			Statedb:          stateDB,
			GasLimit:         1000000000,
			ChainHeadFeed:    new(event.Feed),
			MockCurrentBlock: types.NewBlockWithHeader(genesisHeader),
		},
	}, nil
}
