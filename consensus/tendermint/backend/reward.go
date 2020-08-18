package backend

import (
	"math/big"
	"time"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint/utils"
	"github.com/Evrynetlabs/evrynet-node/core/state"
	"github.com/Evrynetlabs/evrynet-node/core/state/staking"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/log"
)

// AccumulateRewards credits the coinbase of the given block with the proposing
// reward.
func (sb *Backend) accumulateRewards(chainReader consensus.FullChainReader, state *state.StateDB, header *types.Header) error {
	// If fixed validators (test) then return
	if chainReader.Config().Tendermint.FixedValidators != nil {
		reward := new(big.Int).Set(chainReader.Config().Tendermint.BlockReward)
		state.AddBalance(header.Coinbase, reward)
		return nil
	}
	var (
		currentBlock = header.Number.Uint64()
		epoch        = chainReader.Config().Tendermint.Epoch
		start        = time.Now()
	)

	if currentBlock == 0 {
		return tendermint.ErrFinalizeZeroBlock
	}

	if currentBlock%epoch != 0 {
		return nil
	}

	validatorsRewards := calculateTotalValidatorsRewards(chainReader, epoch, header)
	transitionHeader := chainReader.GetHeaderByNumber(currentBlock - epoch)
	validatorAdds, err := utils.GetValSetAddresses(transitionHeader)
	if err != nil {
		return err
	}
	stateDB, err := chainReader.StateAt(transitionHeader.Root)
	if err != nil {
		return err
	}
	stakingCaller := sb.getStakingCaller(chainReader, stateDB, header)
	validatorsData, err := stakingCaller.GetValidatorsData(*sb.config.StakingSCAddress, validatorAdds)
	if err != nil {
		return err
	}

	finalReward := calculateReward(validatorsData, validatorsRewards)
	for addr, value := range finalReward {
		state.AddBalance(addr, value)
	}
	log.Debug("accumulateRewards", "number", currentBlock, "elapsed", common.PrettyDuration(time.Since(start)))
	return nil
}

// calculateTotalValidatorsRewards gets reward from chainReader and current header (from finalize)
// reward includes block rewards and tx fee from block number currentBlock - epoch +1
func calculateTotalValidatorsRewards(chainReader consensus.ChainReader, epoch uint64, header *types.Header) map[common.Address]*big.Int {
	var currentBlock = header.Number.Uint64()
	validatorsRewards := make(map[common.Address]*big.Int)
	for i := currentBlock - epoch + 1; i <= currentBlock; i++ {
		var currentHeader *types.Header
		if i != currentBlock {
			currentHeader = chainReader.GetHeaderByNumber(i)
		} else {
			currentHeader = header
		}
		txFee := new(big.Int).Mul(big.NewInt(int64(currentHeader.GasUsed)), chainReader.Config().GasPrice)
		reward := new(big.Int).Add(chainReader.Config().Tendermint.BlockReward, txFee)
		if current, ok := validatorsRewards[currentHeader.Coinbase]; ok {
			validatorsRewards[currentHeader.Coinbase] = new(big.Int).Add(current, reward)
		} else {
			validatorsRewards[currentHeader.Coinbase] = reward
		}
	}
	return validatorsRewards
}

// calculateReward divides rewards into 50% to owner and 50% among voters
// rewards for voters is proportional to voters'stake
func calculateReward(validatorsData map[common.Address]staking.CandidateData, validatorsReward map[common.Address]*big.Int) map[common.Address]*big.Int {
	finalReward := make(map[common.Address]*big.Int)
	addReward := func(addr common.Address, value *big.Int) {
		if current, ok := finalReward[addr]; ok {
			finalReward[addr] = new(big.Int).Add(current, value)
		} else {
			finalReward[addr] = new(big.Int).Set(value)
		}
	}
	for addr, validatorData := range validatorsData {
		totalReward, ok := validatorsReward[addr]
		if !ok {
			continue
		}
		// remainingReward to ensure the total reward for the voters and owner is equals to the wei validator earns
		remainingReward := new(big.Int).Set(totalReward)
		totalVoterReward := new(big.Int).Mul(totalReward, big.NewInt(voterRewardPercentage))
		totalVoterReward = new(big.Int).Div(totalVoterReward, big.NewInt(100))
		for voter, voterStake := range validatorData.VoterStakes {
			voterReward := new(big.Int).Mul(totalVoterReward, voterStake)
			voterReward = new(big.Int).Div(voterReward, validatorData.TotalStake)
			addReward(voter, voterReward)
			remainingReward.Sub(remainingReward, voterReward)
		}
		validatorReward := new(big.Int).Mul(totalReward, big.NewInt(validatorRewardPercentage))
		validatorReward = new(big.Int).Div(validatorReward, big.NewInt(100))
		addReward(validatorData.Owner, remainingReward)
	}
	return finalReward
}
