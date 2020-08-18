package tests_utils

import (
	"crypto/ecdsa"

	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint/utils"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/crypto"
)

func AppendSealByPkKey(header *types.Header, pk *ecdsa.PrivateKey) {
	hashData := crypto.Keccak256(utils.SigHash(header).Bytes())
	seal, _ := crypto.Sign(hashData, pk)
	_ = utils.WriteSeal(header, seal)
}

func AppendCommitedSealByPkKeys(header *types.Header, pks []*ecdsa.PrivateKey) {
	committedSeals := make([][]byte, len(pks))
	for i, pk := range pks {
		committedSeals[i] = make([]byte, types.TendermintExtraSeal)
		commitHash := utils.PrepareCommittedSeal(header.Hash())
		committedSeal, _ := crypto.Sign(crypto.Keccak256(commitHash), pk)
		copy(committedSeals[i][:], committedSeal[:])
	}
	_ = utils.WriteCommittedSeals(header, committedSeals)
}
