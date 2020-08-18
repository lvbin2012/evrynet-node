package staking

import (
	"math/big"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint/utils"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint/validator"
	"github.com/Evrynetlabs/evrynet-node/log"
)

// StakingValidator is implementation of ValidatorSetInfo
type StakingValidator struct {
	Epoch          uint64
	ProposerPolicy tendermint.ProposerPolicy
}

// NewStakingValidatorInfo returns new StakingValidator
func NewStakingValidatorInfo(epoch uint64, proposerPolicy tendermint.ProposerPolicy) *StakingValidator {
	return &StakingValidator{
		Epoch:          epoch,
		ProposerPolicy: proposerPolicy,
	}
}

// GetValSet returns the validators available in the block if it already been created
func (v *StakingValidator) GetValSet(chainReader consensus.ChainReader, number *big.Int) (tendermint.ValidatorSet, error) {
	var (
		// get the checkpoint of block-number
		blockNumber = number.Int64()
		checkPoint  = utils.GetCheckpointNumber(v.Epoch, number.Uint64())
		valSet      = validator.NewSet([]common.Address{}, v.ProposerPolicy, blockNumber)
	)

	header := chainReader.GetHeaderByNumber(checkPoint)
	if (header == nil || header.Hash() == common.Hash{}) {
		return valSet, tendermint.ErrUnknownBlock
	}

	validatorAdds, err := utils.GetValSetAddresses(header)
	if err != nil {
		log.Error("can't get the validators's address from extra-data", "number", blockNumber)
		return valSet, err
	}

	return validator.NewSet(validatorAdds, v.ProposerPolicy, blockNumber), nil
}
