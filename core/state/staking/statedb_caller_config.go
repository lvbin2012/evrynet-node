package staking

import (
	"math/big"

	"github.com/Evrynetlabs/evrynet-node/common"
)

// LayOut represents the Offset and Slot order of a state variable
type LayOut struct {
	Offset uint16
	Slot   uint64
}

func (layOut *LayOut) slotHash() common.Hash {
	return common.BigToHash(new(big.Int).SetUint64(layOut.Slot))
}

// IndexConfigs represents the configuration index of state variables.
type IndexConfigs struct {
	WithdrawsStateLayout    LayOut //1
	CandidateVotersLayout   LayOut //2
	CandidateDataLayout     LayOut //3
	CandidatesLayout        LayOut //4
	StartBlockLayout        LayOut //5
	EpochPeriodLayout       LayOut //6
	MaxValidatorSizeLayout  LayOut //7
	MinValidatorStakeLayout LayOut //8
	MinVoterCapLayout       LayOut //9
	AdminLayout             LayOut //10

	CandidateDataStruct CandidateDataStructIndex
}

// layout inside candidateData struct
type CandidateDataStructIndex struct {
	Owner        LayOut
	TotalStake   LayOut
	VotersStakes LayOut
}

// DefaultConfig represents he default configuration.
var DefaultConfig = &IndexConfigs{
	WithdrawsStateLayout:    NewLayOut(1, 0),
	CandidateVotersLayout:   NewLayOut(2, 0),
	CandidateDataLayout:     NewLayOut(3, 0),
	CandidatesLayout:        NewLayOut(4, 0),
	StartBlockLayout:        NewLayOut(5, 0),
	EpochPeriodLayout:       NewLayOut(6, 0),
	MaxValidatorSizeLayout:  NewLayOut(7, 0),
	MinValidatorStakeLayout: NewLayOut(8, 0),
	MinVoterCapLayout:       NewLayOut(9, 0),
	AdminLayout:             NewLayOut(10, 0),
	CandidateDataStruct: CandidateDataStructIndex{
		TotalStake:   NewLayOut(1, 0),
		Owner:        NewLayOut(2, 0),
		VotersStakes: NewLayOut(3, 0),
	},
}

// NewLayOut returns new instance of a LayOut
func NewLayOut(slot uint64, offset uint16) LayOut {
	return LayOut{Offset: offset, Slot: slot}
}
