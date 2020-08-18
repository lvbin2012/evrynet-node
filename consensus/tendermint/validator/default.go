package validator

import (
	"math"
	"sort"
	"sync"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint"
)

type defaultValidator struct {
	address common.Address
}

// Address will return address of defaultValidator
func (val *defaultValidator) Address() common.Address {
	return val.address
}

// String will parse address of defaultValidator to string and return it
func (val *defaultValidator) String() string {
	return val.Address().String()
}

// ----------------------------------------------------------------------------

// defaultSet stores list of validator,
// proposer, proposer policy, proposer selector for voting
// and validator mutex to handle the conflict of the reader/writer between goroutines
type defaultSet struct {
	validators tendermint.Validators
	policy     tendermint.ProposerPolicy

	proposer    tendermint.Validator
	validatorMu sync.RWMutex
	selector    tendermint.ProposalSelector

	height int64 // current height when backend init validator set
}

func newDefaultSet(addrs []common.Address, policy tendermint.ProposerPolicy, height int64) *defaultSet {
	valSet := &defaultSet{}

	valSet.policy = policy
	// init validators
	valSet.validators = make([]tendermint.Validator, len(addrs))
	for i, addr := range addrs {
		valSet.validators[i] = New(addr)
	}

	// sort validator
	sort.Sort(valSet.validators)

	// init proposer
	if valSet.Size() > 0 {
		// this ensure first validator in array can propose block height 1
		shiftHeight := height
		if shiftHeight > 0 {
			shiftHeight = shiftHeight - 1
		}
		index := shiftHeight % int64(valSet.Size())
		valSet.proposer = valSet.GetByIndex(index)
	}
	if policy == tendermint.Sticky {
		valSet.selector = stickyProposer
	} else {
		valSet.selector = roundRobinProposer
	}

	valSet.height = height

	return valSet
}

// Size will return the length of validators in defaultSet
func (valSet *defaultSet) Size() int {
	valSet.validatorMu.RLock()
	defer valSet.validatorMu.RUnlock()
	return len(valSet.validators)
}

// List will return validators in defaultSet
func (valSet *defaultSet) List() []tendermint.Validator {
	valSet.validatorMu.RLock()
	defer valSet.validatorMu.RUnlock()
	return valSet.validators
}

// GetByIndex will return validator by index in defaultSet
// If the index >= size of validators, return nil
func (valSet *defaultSet) GetByIndex(i int64) tendermint.Validator {
	valSet.validatorMu.RLock()
	defer valSet.validatorMu.RUnlock()
	if i < int64(valSet.Size()) {
		return valSet.validators[i]
	}
	return nil
}

// GetByAddress will return the validator & its index of the validator set by the address
// If the address does not exist in the validator set, it will return -1
func (valSet *defaultSet) GetByAddress(addr common.Address) (int, tendermint.Validator) {
	for i, val := range valSet.List() {
		if addr == val.Address() {
			return i, val
		}
	}
	return -1, nil
}

func calcSeed(valSet tendermint.ValidatorSet, proposer common.Address, roundDiff int64) int64 {
	offset := 0
	if idx, val := valSet.GetByAddress(proposer); val != nil {
		offset = idx
	}
	return int64(offset) + roundDiff
}

func emptyAddress(addr common.Address) bool {
	return addr == common.Address{}
}

func roundRobinProposer(valSet tendermint.ValidatorSet, proposer common.Address, roundDiff int64) tendermint.Validator {
	if valSet.Size() == 0 {
		return nil
	}
	var seed int64
	if emptyAddress(proposer) {
		seed = roundDiff
	} else {
		seed = calcSeed(valSet, proposer, roundDiff)
	}
	pick := seed % int64(valSet.Size())
	return valSet.GetByIndex(pick)
}

func stickyProposer(valSet tendermint.ValidatorSet, proposer common.Address, roundDiff int64) tendermint.Validator {
	if valSet.Size() == 0 {
		return nil
	}
	var seed int64
	if emptyAddress(proposer) {
		seed = roundDiff
	} else {
		seed = calcSeed(valSet, proposer, roundDiff)
	}
	pick := seed % int64(valSet.Size())
	return valSet.GetByIndex(pick)
}

// AddValidator will add a validator to validators collection
func (valSet *defaultSet) AddValidator(address common.Address) bool {
	valSet.validatorMu.Lock()
	defer valSet.validatorMu.Unlock()
	for _, v := range valSet.validators {
		if v.Address() == address {
			return false
		}
	}
	valSet.validators = append(valSet.validators, New(address))
	return true
}

// RemoveValidator will remove a validator from validatorset
func (valSet *defaultSet) RemoveValidator(address common.Address) bool {
	valSet.validatorMu.Lock()
	defer valSet.validatorMu.Unlock()

	for i, v := range valSet.validators {
		if v.Address() == address {
			valSet.validators = append(valSet.validators[:i], valSet.validators[i+1:]...)
			return true
		}
	}
	return false
}

// Copy allows copy all items from A to B
func (valSet *defaultSet) Copy() tendermint.ValidatorSet {
	valSet.validatorMu.RLock()
	defer valSet.validatorMu.RUnlock()

	addresses := make([]common.Address, 0, len(valSet.validators))
	for _, v := range valSet.validators {
		addresses = append(addresses, v.Address())
	}
	return NewSet(addresses, valSet.policy, valSet.height)
}

// Get the minimum number of peers to archive consensus
func (valSet *defaultSet) MinPeers() int {
	return valSet.Size() - valSet.F() - 1
}

// Get the minimum number of votes for a polka
func (valSet *defaultSet) MinMajority() int {
	return valSet.Size() - valSet.F()
}

// F get the maximum number of faulty nodes
func (valSet *defaultSet) F() int { return int(math.Ceil(float64(valSet.Size())/3)) - 1 }

// V get the minimum number of vote nodes
func (valSet *defaultSet) V() int { return int(math.Ceil(float64(valSet.Size()) / 2)) }

// Policy get proposal policy
func (valSet *defaultSet) Policy() tendermint.ProposerPolicy { return valSet.policy }

// GetNeighbors returns address of neighbor to rebroadcast tendermint message
func (valSet *defaultSet) GetNeighbors(addr common.Address) map[common.Address]bool {
	i, _ := valSet.GetByAddress(addr)
	if i == -1 {
		return nil
	}

	var neighbors = make(map[common.Address]bool)
	for j := 0; j < int(math.Ceil(math.Sqrt(float64(valSet.Size())))); j++ {
		neighborIndex := i + j + 1
		if neighborIndex >= valSet.Size() {
			neighborIndex -= valSet.Size()
		}

		if neighborIndex == i {
			continue
		}
		neighbors[valSet.GetByIndex(int64(neighborIndex)).Address()] = true
	}
	return neighbors
}

//CalcProposer implement valSet.CalcProposer. Based on the proposer selection scheme,
//it will set valSet.proposer to the address of the pre-determined round.
func (valSet *defaultSet) CalcProposer(lastProposer common.Address, roundDiff int64) {
	valSet.validatorMu.RLock()
	defer valSet.validatorMu.RUnlock()
	valSet.proposer = valSet.selector(valSet, lastProposer, roundDiff)
}

//GetProposer return the current proposer of this valSet
func (valSet *defaultSet) GetProposer() tendermint.Validator {
	return valSet.proposer
}

// Height return block height when valSet is init
func (valSet *defaultSet) Height() int64 {
	return valSet.height
}
