package backend

import (
	"math/big"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus"
)

// TendermintAPI is a user facing RPC API to dump tendermint state
type TendermintAPI struct {
	chain consensus.ChainReader
	be    *Backend
}

// GetValidators returns the list of validators by block's number
func (api *TendermintAPI) GetValidators(number *uint64) []common.Address {
	var (
		blockNumber *big.Int
	)
	if number == nil {
		blockNumber = api.chain.CurrentHeader().Number
	} else {
		blockNumber = new(big.Int).SetUint64(*number)
	}
	valSet := api.be.ValidatorsByChainReader(blockNumber, api.chain)
	validators := make([]common.Address, 0, valSet.Size())
	for _, validator := range valSet.List() {
		validators = append(validators, validator.Address())
	}
	return validators
}
