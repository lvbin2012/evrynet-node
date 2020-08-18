package backend

import (
	"math/big"

	"github.com/Evrynetlabs/evrynet-node/consensus"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint"
)

//ValidatorSetInfo keep tracks of validator set in associate with blockNumber
type ValidatorSetInfo interface {
	GetValSet(chainReader consensus.ChainReader, blockNumber *big.Int) (tendermint.ValidatorSet, error)
}
