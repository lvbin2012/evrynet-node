package core

import (
	"math/big"
	"time"

	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint"
	"github.com/Evrynetlabs/evrynet-node/core/types"
)

//getInitializedState init core with the last known roundState
//if there is no state in storage, init a new state.
func (c *core) getInitializedState() *roundState {
	var (
		//these are default config at block 1 round 0
		rs *roundState

		prevotesReceived  = make(map[int64]*messageSet)
		precommitReceived = make(map[int64]*messageSet)
		block             = types.NewBlockWithHeader(&types.Header{})
		view              = tendermint.View{
			Round:       0,
			BlockNumber: big.NewInt(1),
		}
		lockedRound      int64 = -1
		lockedBlock      *types.Block
		validRound       int64 = -1
		validBlock       *types.Block
		commitRound      int64 = -1
		proposalReceived *Proposal
		step             = RoundStepNewHeight
	)

	//to continue from a stored State, get the last known block height
	lastKnownHeight := c.backend.CurrentHeadBlock().Number()

	// Increase block number to 1 block
	view.BlockNumber = new(big.Int).Add(lastKnownHeight, big.NewInt(1))

	rs = newRoundState(&view, prevotesReceived, precommitReceived, block,
		lockedRound, lockedBlock,
		validRound, validBlock,
		proposalReceived,
		step, commitRound,
	)

	//TODO: timeout setup
	return rs
}

func (c *core) updateStateForNewblock() {
	var (
		state  = c.CurrentState()
		logger = c.getLogger()
	)

	if state.commitRound > -1 {
		// having commit round, should have seen +2/3 precommits
		precommits, ok := state.GetPrecommitsByRound(state.commitRound)
		if !ok {
			logger.Errorw("updateStateForNewblock(): Can not found the message set")
			return
		}
		_, ok = precommits.TwoThirdMajority()
		if !ok {
			logger.Errorw("updateStateForNewblock(): Having commitRound with no +2/3 precommits")
			return
		}
	}

	// Update all roundState's fields
	height := state.BlockNumber()
	state.SetView(&tendermint.View{
		Round:       0,
		BlockNumber: height.Add(height, big.NewInt(1)),
	})

	if state.commitTime.IsZero() {
		// "Now" makes it easier to sync up dev nodes.
		// We add timeoutCommit to allow transactions
		// to be gathered for the first block.
		// And alternative solution that relies on clocks:
		state.startTime = c.config.Commit(time.Now())
	} else {
		state.startTime = c.config.Commit(state.commitTime)
	}

	state.clearPreviousRoundData()
	c.currentState = state
	c.valSet = c.backend.Validators(c.CurrentState().BlockNumber())
	c.futureProposals = make(map[int64]message)
	logger.Infow("updated to new block", "new_block_number", state.BlockNumber())
}
