package staking

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/Evrynetlabs/evrynet-node/common"
)

var (
	// ErrEmptyValidatorSet returns when valset from smart-contract is empty
	ErrEmptyValidatorSet = errors.New("empty validator set")
	// ErrLengthOfCandidatesAndStakesMisMatch returns when lengths stakes and candidates are not match
	ErrLengthOfCandidatesAndStakesMisMatch = errors.New("length of stakes is not equal to length of candidates")
	// ErrLengthOfVotesAndStakesMisMatch returns when lengths voters and stakes are not match
	ErrLengthOfVotesAndStakesMisMatch = errors.New("length of voters is not equal to length of stakes")

	maxGasGetValSet uint64 = 500000000
)

type StakingCaller interface {
	// GetValidators returns list of validators, calculate from current stateDB
	GetValidators(common.Address) ([]common.Address, error)
	// GetValidatorsData return information of validators including owner, totalStake and voterStakes
	GetValidatorsData(common.Address, []common.Address) (map[common.Address]CandidateData, error)
}

type CandidateData struct {
	Owner       common.Address
	VoterStakes map[common.Address]*big.Int
	TotalStake  *big.Int
}
