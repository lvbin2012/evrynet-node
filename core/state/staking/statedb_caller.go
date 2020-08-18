package staking

import (
	"math/big"
	"sort"
	"strings"

	"github.com/Evrynetlabs/evrynet-node/accounts/abi/bind"
	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/core/state"
	"github.com/Evrynetlabs/evrynet-node/crypto"
)

const (
	// default element size for address, uint array
	defaultElementSize = 1
)

// stateDBStakingCaller creates a wrapper with statedb to implements ContractCaller
// based on https://solidity.readthedocs.io/en/develop/miscellaneous.html
type stateDBStakingCaller struct {
	stateDB *state.StateDB
	config  *IndexConfigs
}

// NewStateDbStakingCaller return instance of StakingCaller which reads data directly from state DB
func NewStateDbStakingCaller(state *state.StateDB, cfg *IndexConfigs) StakingCaller {
	return &stateDBStakingCaller{
		stateDB: state,
		config:  cfg,
	}
}

// GetCandidates returns list candidate's address
func (c *stateDBStakingCaller) GetCandidates(stakingContractAddr common.Address) ([]common.Address, error) {
	//arrLength := c.getStorageLocation(stakingContractAddr, c.config.CandidatesLayout, common.Hash{}, nil)
	arrLength := c.stateDB.GetState(stakingContractAddr, c.config.CandidatesLayout.slotHash())
	if arrLength.Big().Cmp(big.NewInt(0)) == 0 {
		return nil, ErrEmptyValidatorSet
	}
	var candidates []common.Address
	for i := uint64(0); i < arrLength.Big().Uint64(); i++ {
		ret := c.stateDB.GetState(stakingContractAddr, getElementArrayLoc(c.config.CandidatesLayout.slotHash(), i, defaultElementSize))
		candidates = append(candidates, common.HexToAddress(ret.Hex()))
	}
	return candidates, nil
}

// GetValidators returns validators from stateDB and block number of the caller by smart-contract's address
func (c *stateDBStakingCaller) GetValidators(stakingContractAddr common.Address) ([]common.Address, error) {
	// check if this address is a valid contract, this will help us return better error
	if codes := c.stateDB.GetCode(stakingContractAddr); len(codes) == 0 {
		return nil, bind.ErrNoCode
	}

	candidates, err := c.GetCandidates(stakingContractAddr)
	if err != nil {
		return nil, err
	}

	var (
		validators []common.Address
		stakes     = make(map[common.Address]*big.Int)
	)
	minValStake := c.GetMinValidatorStake(stakingContractAddr)
	for _, candidate := range candidates {
		stake := c.GetCandidateStake(stakingContractAddr, candidate)
		if stake.Cmp(minValStake) < 0 {
			continue
		}

		validators = append(validators, candidate)
		stakes[candidate] = stake
	}

	maxValSize := int(c.GetMaxValidatorSize(stakingContractAddr).Uint64())
	if len(validators) <= maxValSize {
		return validators, nil
	}

	sort.Slice(validators, func(i, j int) bool {
		if stakes[validators[i]].Cmp(stakes[validators[j]]) == 0 {
			return strings.Compare(validators[i].String(), validators[j].String()) > 0
		}
		return stakes[validators[i]].Cmp(stakes[validators[j]]) > 0
	})

	return candidates[:maxValSize], nil
}

// GetValidatorsData return information of validators including owner, totalStake and voterStakes
func (c *stateDBStakingCaller) GetValidatorsData(scAddress common.Address, candidates []common.Address) (map[common.Address]CandidateData, error) {
	allVoterStake := make(map[common.Address]CandidateData)
	for _, candidate := range candidates {
		candidateData := c.GetCandidateData(scAddress, candidate)
		allVoterStake[candidate] = candidateData
	}
	return allVoterStake, nil
}

// GetCandidateData returns current stake of a candidate
func (c *stateDBStakingCaller) GetCandidateData(stakingContractAddr common.Address, candidate common.Address) CandidateData {
	loc := getMappingElementLoc(c.config.CandidateDataLayout.slotHash(), candidate.Hash())
	totalStakeLoc := addOffsetToLoc(loc, new(big.Int).SetUint64(c.config.CandidateDataStruct.TotalStake.Slot))
	totalStake := c.stateDB.GetState(stakingContractAddr, totalStakeLoc).Big()

	ownerLoc := addOffsetToLoc(loc, new(big.Int).SetUint64(c.config.CandidateDataStruct.Owner.Slot))
	owner := common.HexToAddress(c.stateDB.GetState(stakingContractAddr, ownerLoc).Hex())

	voteStakes := make(map[common.Address]*big.Int)
	for _, voter := range c.GetVoters(stakingContractAddr, candidate) {
		voterStakesSlot := addOffsetToLoc(loc, new(big.Int).SetUint64(c.config.CandidateDataStruct.VotersStakes.Slot))
		voterStakeSlot := getMappingElementLoc(voterStakesSlot, voter.Hash())
		stake := c.getBigInt(stakingContractAddr, voterStakeSlot)
		voteStakes[voter] = stake
	}

	return CandidateData{
		Owner:       owner,
		TotalStake:  totalStake,
		VoterStakes: voteStakes,
	}
}

func (c *stateDBStakingCaller) GetVoters(stakingAddr common.Address, candidate common.Address) []common.Address {
	votersSlot := getMappingElementLoc(c.config.CandidateVotersLayout.slotHash(), candidate.Hash())
	votersLength := c.getBigInt(stakingAddr, votersSlot).Uint64()
	var voters []common.Address
	for i := uint64(0); i < votersLength; i++ {
		voterSlot := getElementArrayLoc(votersSlot, i, defaultElementSize)
		voters = append(voters, c.getAddress(stakingAddr, voterSlot))
	}
	return voters
}

// GetCandidateStake returns current stake of a candidate
func (c *stateDBStakingCaller) GetCandidateStake(scAddress common.Address, candidate common.Address) *big.Int {
	loc := getMappingElementLoc(c.config.CandidateDataLayout.slotHash(), candidate.Hash())
	loc = addOffsetToLoc(loc, new(big.Int).SetUint64(c.config.CandidateDataStruct.TotalStake.Slot))
	return c.getBigInt(scAddress, loc)
}

// GetStartBlock returns the startblock
func (c *stateDBStakingCaller) GetStartBlock(scAddress common.Address) *big.Int {
	return c.getBigInt(scAddress, c.config.StartBlockLayout.slotHash())
}

// GetEpochPeriod returns the epochPeriod
func (c *stateDBStakingCaller) GetEpochPeriod(scAddress common.Address) *big.Int {
	return c.getBigInt(scAddress, c.config.EpochPeriodLayout.slotHash())
}

// GetMaxValidatorSize returns maximum validators allowed
func (c *stateDBStakingCaller) GetMaxValidatorSize(scAddress common.Address) *big.Int {
	return c.getBigInt(scAddress, c.config.MaxValidatorSizeLayout.slotHash())
}

// GetMinValidatorStake returns the min stake of a validator
func (c *stateDBStakingCaller) GetMinValidatorStake(scAddress common.Address) *big.Int {
	return c.getBigInt(scAddress, c.config.MinValidatorStakeLayout.slotHash())
}

// GetMinVoterCap returns the MinVoterCap
func (c *stateDBStakingCaller) GetMinVoterCap(scAddress common.Address) *big.Int {
	return c.getBigInt(scAddress, c.config.MinVoterCapLayout.slotHash())
}

// GetAdmin returns admin's address
func (c *stateDBStakingCaller) GetAdmin(scAddress common.Address) common.Address {
	return c.getAddress(scAddress, c.config.AdminLayout.slotHash())
}

// GetCandidateOwner returns current owner of a candidate
func (c *stateDBStakingCaller) GetCandidateOwner(stakingContractAddr common.Address, candidate common.Address) common.Address {
	loc := getMappingElementLoc(c.config.CandidateDataLayout.slotHash(), candidate.Hash())
	loc = addOffsetToLoc(loc, new(big.Int).SetUint64(c.config.CandidateDataStruct.Owner.Slot))
	return c.getAddress(stakingContractAddr, loc)
}

func (c *stateDBStakingCaller) getAddress(contractAddr common.Address, hash common.Hash) common.Address {
	return common.HexToAddress(c.stateDB.GetState(contractAddr, hash).Hex())
}

func (c *stateDBStakingCaller) getBigInt(contractAddr common.Address, hash common.Hash) *big.Int {
	return c.stateDB.GetState(contractAddr, hash).Big()
}

/**
 * Array data is located at keccak256(p)
 *  So to get the location of element we add a offset = index * elementSize
 */
func getElementArrayLoc(slotHash common.Hash, index uint64, elementSize uint64) common.Hash {
	slotKecBig := crypto.Keccak256Hash(slotHash.Bytes()).Big()
	arrBig := slotKecBig.Add(slotKecBig, new(big.Int).SetUint64(index*elementSize))
	return common.BigToHash(arrBig)
}

/**
 * The value to a mapping key k at position p is located at keccak256(k . p) where . is concatenation.
 */
func getMappingElementLoc(slotHash common.Hash, key common.Hash) common.Hash {
	return common.BytesToHash(crypto.Keccak256(key.Bytes(), slotHash.Bytes()))
}

/**
 * Get the position for a field inside a struct
 */
func addOffsetToLoc(slotHash common.Hash, slot *big.Int) common.Hash {
	rootBig := slotHash.Big()
	arrBig := new(big.Int).Add(rootBig, slot)
	return common.BigToHash(arrBig)
}
