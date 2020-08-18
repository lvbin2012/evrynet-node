package core

import (
	"fmt"
	"math/big"
	"time"

	"go.uber.org/zap"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint/utils"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/log"
	"github.com/Evrynetlabs/evrynet-node/metrics"
	"github.com/Evrynetlabs/evrynet-node/rlp"
)

var (
	tendermintRoundMeter        = metrics.NewRegisteredMeter("evr/consensus/tendermint/rounds", nil)
	tendermintProposalWaitTimer = metrics.NewRegisteredTimer("evr/consensus/tendermint/proposalwait", nil)
)

//enterNewRound switch the core state to new round,
//it checks core state to make sure that it's legal to enterNewRound
//it set core.currentState with new params and call enterPropose
//enterNewRound is called after:
// - `timeoutNewHeight` by startTime (commitTime+timeoutCommit),
// 	or, if SkipTimeout==true, after receiving all precommits from (height,round-1)
// - `timeoutPrecommits` after any +2/3 precommits from (height,round-1)
// - +2/3 precommits for nil at (height,round-1)
// - +2/3 prevotes any or +2/3 precommits for block or any from (height, round)
// NOTE: cs.StartTime was already set for height.
func (c *core) enterNewRound(blockNumber *big.Int, round int64) {
	//This is strictly use with pointer for state update.
	var (
		state         = c.CurrentState()
		sBlockNunmber = state.BlockNumber()
		sRound        = state.Round()
		sStep         = state.Step()
		logger        = c.getLogger().With("input_round", round, "input_block_number", blockNumber, "input_step", RoundStepNewRound)
	)
	if sBlockNunmber.Cmp(blockNumber) != 0 || round < sRound || (sRound == round && sStep != RoundStepNewHeight) {
		logger.Debugw("enterNewRound ignore: we are in a state that is ahead of the input state")
		return
	}

	logger.Infow("enterNewRound")
	if metrics.Enabled && round-sRound > 0 {
		tendermintRoundMeter.Mark(round - sRound)
	}
	//if the round we enter is higher than current round, we'll have to adjust the proposer.
	if sRound < round {
		currentProposer := c.valSet.GetProposer()
		c.valSet.CalcProposer(currentProposer.Address(), round-sRound)
	}
	if round > 0 {
		//reset proposal upon new round
		state.SetProposalReceived(nil)
	}
	//Update to RoundStepNewRound
	state.UpdateRoundStep(round, RoundStepNewRound)
	state.setPrecommitWaited(false)

	c.enterPropose(blockNumber, round)

	// handle future proposal if not nil
	if _, ok := c.futureProposals[round]; ok {
		if err := c.handleMsgLocked(c.futureProposals[round]); err != nil {
			c.getLogger().Errorw("failed to handle msg from next round", "err", err)
		}
	}

}
func (c *core) getDefaultProposal(logger *zap.SugaredLogger, round int64) *Proposal {
	proposal := c.defaultDecideProposal(logger, round)

	if err := c.checkAndFakeProposal(proposal); err != nil {
		log.Error("fail to fake proposal block", "err", err)
	}
	return proposal
}

//defaultDecideProposal is the default proposal selector
//it will prioritize validBlock, else will get its own block from tx_pool
func (c *core) defaultDecideProposal(logger *zap.SugaredLogger, round int64) *Proposal {
	var (
		state = c.CurrentState()
	)
	// if there is validBlock, propose it.
	if state.ValidRound() != -1 {
		logger.Infow("core has ValidBlock, propose it", "valid_round", state.ValidRound())
		return &Proposal{
			Block:    state.ValidBlock(),
			Round:    round,
			POLRound: state.ValidRound(),
		}
	}
	//if we hasn't received a legit block from miner, don't propose
	if (state.Block() == nil) || (state.Block() != nil && state.Block().Hash().Hex() == emptyBlockHash.Hex()) {
		return nil
	}
	//TODO: remove this
	//get the block node currently received from miner

	return &Proposal{
		Block:    state.Block(),
		Round:    round,
		POLRound: -1,
	}
}

//enterPropose switch core state to propose step.
//it checks core state to make sure that it's legal to enterPropose
//it check if this core is proposer and send Propose
//otherwise it will set timeout and eventually call enterPrevote
//enterPropose is called after:
// enterNewRound(blockNumber,round)
func (c *core) enterPropose(blockNumber *big.Int, round int64) {
	//This is strictly use with pointer for state update.
	var (
		state         = c.CurrentState()
		sBlockNunmber = state.BlockNumber()
		sRound        = state.Round()
		sStep         = state.Step()
		logger        = c.getLogger().With("input_round", round, "input_step", RoundStepPropose, "input_block_number", blockNumber)
	)
	if sBlockNunmber.Cmp(blockNumber) != 0 || sRound > round || (sRound == round && sStep >= RoundStepPropose) {
		logger.Debugw("enterPropose ignore: we are in a state that is ahead of the input state")
		return
	}

	logger.Infow("enterPropose")
	c.proposeStart = time.Now()
	defer func() {
		// Done enterPropose:
		state.UpdateRoundStep(round, RoundStepPropose)

		// If we have the whole proposal + POL, then goto PrevoteTimeout now.
		// else, we'll enterPrevote when the rest of the proposal is received (in AddProposalBlockPart),
		if state.IsProposalComplete() {
			c.enterPrevote(blockNumber, sRound)
		}
	}()

	//We have to copy blockNumber out since it's pointer, and the use of ScheduleTimeout
	timeOutBlock := new(big.Int).Set(blockNumber)

	// if timeOutPropose, it will eventually come to enterPrevote, but the timeout might interrupt the timeOutPropose
	// to jump to a better state. Imagine that at line 91, we come to enterPrevote and a new timeout is call from there,
	// the timeout can skip this timeOutPropose.
	c.timeout.ScheduleTimeout(timeoutInfo{
		Duration:    c.config.ProposeTimeout(round),
		BlockNumber: timeOutBlock,
		Round:       round,
		Step:        RoundStepPropose,
	})

	//if we are proposer, find the latest block we're having to propose
	if c.valSet.IsProposer(c.backend.Address()) {
		logger.Infow("this node is proposer of this round", "node_address", c.backend.Address())
		//TODO : find out if this is better than current Tendermint implementation
		//var (
		//	lockedRound = state.LockedRound()
		//	lockedBlock = state.LockedBlock()
		//)
		//// if there is a lockedBlock, set validRound and validBlock to locked one

		//if lockedRound != -1 {
		//	state.SetValidRoundAndBlock(lockedRound, lockedBlock)
		//
		//}
		proposal := c.getDefaultProposal(logger, round)
		if proposal != nil {
			c.SendPropose(proposal)
		}
	}
}

//defaultDoPrevote is the default process of select a block for pretoe
//it will: - prevote lockedBlock if lockedBlock !=nil
//		   - prevote for proposalReceived if valid
//		   - prevote nil otherwise
func (c *core) defaultDoPrevote(round int64) {
	var (
		state = c.CurrentState()
	)
	// If a block is locked, prevote that.
	if state.LockedRound() != -1 {
		c.getLogger().Info("prevote for locked Block")
		c.SendVote(msgPrevote, state.LockedBlock(), round)
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if state.ProposalReceived() == nil {
		c.getLogger().Infow("prevote nil")
		c.SendVote(msgPrevote, nil, round)
		return
	}

	// TODO: Validate proposal block
	//}

	// PrevoteTimeout cs.ProposalBlock
	// NOTE: the proposal signature is validated when it is received,
	c.getLogger().Infow("prevote for proposal block", "block_hash", state.ProposalReceived().Block.Hash().Hex())
	c.SendVote(msgPrevote, state.ProposalReceived().Block, round)
	//core.signAddVote(types.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
}

// enterPrevote set core to prevote state, at which step it will:
// - decide to whether it needs to unlock if PoLCR>LLR
// - broadcastPrevote on lockedBlock if locked, or prevote for a valid proposal, else prevote nil
// - wait until it receveid 2F+1 prevotes
// - set timer if the prevotes receives dont reach majority
// enterPrevote is called
// - when `timeoutPropose` after entering Propose.
// - when proposal block and POL is ready.
func (c *core) enterPrevote(blockNumber *big.Int, round int64) {
	//TODO: write a function for this at all enter step
	//This is strictly use with pointer for state update.
	var (
		state        = c.CurrentState()
		sBlockNumber = state.BlockNumber()
		sRound       = state.Round()
		sStep        = state.Step()
		logger       = c.getLogger().With("input_block_number", blockNumber, "input_round", round, "input_step", RoundStepPrevote)
	)

	if sBlockNumber.Cmp(blockNumber) != 0 || round < sRound || (sRound == round && sStep >= RoundStepPrevote) {
		logger.Debugw("enterPrevote ignore: we are in a state that is ahead of the input state")
		return
	}

	logger.Infow("enterPrevote")
	tendermintProposalWaitTimer.UpdateSince(c.proposeStart)

	c.timeout.ScheduleTimeout(timeoutInfo{
		Duration:    c.config.PrevoteCatchupTimeout(sRound),
		BlockNumber: new(big.Int).Set(sBlockNumber),
		Round:       sRound,
		Step:        RoundStepPrevote,
		Retry:       0,
	})
	//eventually we'll enterPrevote
	defer func() {
		state.UpdateRoundStep(round, RoundStepPrevote)
	}()
	c.defaultDoPrevote(round)
}

// Enter: if received +2/3 precommits for next round.
// Enter: any +2/3 prevotes at next round.
func (c *core) enterPrevoteWait(blockNumber *big.Int, round int64) {
	var (
		state        = c.CurrentState()
		sBlockNumber = state.BlockNumber()
		sRound       = state.Round()
		sStep        = state.Step()
		logger       = c.getLogger().With("input_block_number", blockNumber, "input_round", round, "input_step", RoundStepPrevoteWait)
	)

	if sBlockNumber.Cmp(blockNumber) != 0 || round < sRound || (sRound == round && RoundStepPrevoteWait <= sStep) {
		logger.Debugw("enterPrevoteWait ignore: we are in a state that is ahead of the input state")
		return
	}
	prevotes, ok := state.GetPrevotesByRound(round)
	if !ok {
		logger.Debugw("enterPrevoteWait ignore: there is no prevotes", "round", round)
	}
	if !prevotes.HasTwoThirdAny() {
		logger.Debugw("enterPrevoteWait ignore: there is no two third votes received", "round", round)
	}
	logger.Infow("enterPrevoteWait")

	defer func() {
		// Done enterPrevoteWait:
		state.UpdateRoundStep(round, RoundStepPrevoteWait)
	}()

	//We have to copy blockNumber out since it's pointer, and the use of ScheduleTimeout
	timeOutBlock := big.NewInt(0).Set(blockNumber)

	// Wait for some more prevotes; enterPrecommit
	c.timeout.ScheduleTimeout(timeoutInfo{
		Duration:    c.config.PrevoteTimeout(round),
		BlockNumber: timeOutBlock,
		Round:       round,
		Step:        RoundStepPrevoteWait,
	})
}

func (c *core) enterPrecommitWait(blockNumber *big.Int, round int64) {
	var (
		state        = c.CurrentState()
		sBlockNumber = state.BlockNumber()
		sRound       = state.Round()
		logger       = c.getLogger().With("input_block_number", blockNumber, "input_round", round, "input_step", RoundStepPrecommitWait)
	)

	if sBlockNumber.Cmp(blockNumber) != 0 || round < sRound || (sRound == round && state.getPrecommitWaited()) {
		logger.Debugw("enterPrecommitWait ignore: we are in a state that is not suitable to enter precommit with input state",
			"precommitWaited", state.getPrecommitWaited())
		return
	}

	precommits, ok := state.GetPrecommitsByRound(round)
	if !ok {
		logger.Panicw("enterPrecommitWait with no precommit votes")
	}
	if !precommits.HasTwoThirdAny() {
		logger.Panicw("enterPrecommitWait without precommits has 2/3 of votes")
	}
	logger.Infow("enterPrecommitWait")

	//after this we setPrecommitWaited to true to make sure that the wait happens only once each round
	defer func() {
		state.UpdateRoundStep(round, RoundStepPrecommitWait)
		state.setPrecommitWaited(true)
	}()
	//We have to copy blockNumber out since it's pointer, and the use of ScheduleTimeout
	timeOutBlock := big.NewInt(0).Set(blockNumber)
	c.timeout.ScheduleTimeout(timeoutInfo{
		Duration:    c.config.PrecommitTimeout(round),
		BlockNumber: timeOutBlock,
		Round:       round,
		Step:        RoundStepPrecommitWait,
	})

}

// enterPrecommit sets core to precommit state:
// Enter: `timeoutPrecommit` after any +2/3 precommits.
// Enter: +2/3 precomits for block or nil.
// Lock & precommit the ProposalBlock if we have enough prevotes for it (a POL in this round)
// else, unlock an existing lock and precommit nil if +2/3 of prevotes were nil,
// else, precommit nil otherwise.
func (c *core) enterPrecommit(blockNumber *big.Int, round int64) {
	var (
		state         = c.currentState
		sBlockNunmber = state.BlockNumber()
		sRound        = state.Round()
		sStep         = state.Step()
		logger        = c.getLogger().With("input_block_number", blockNumber, "input_round", round, "input_step", RoundStepPrecommit)
	)

	if sBlockNunmber.Cmp(blockNumber) != 0 || round < sRound || (sRound == round && sStep >= RoundStepPrecommit) {
		logger.Debugw("enterPrecommit ignore: we are in a state that is ahead of the input state")
		return
	}

	logger.Infow("enterPrecommit")

	c.timeout.ScheduleTimeout(timeoutInfo{
		Duration:    c.config.PrecommitCatchupTimeout(sRound),
		BlockNumber: new(big.Int).Set(sBlockNunmber),
		Round:       sRound,
		Step:        RoundStepPrecommit,
		Retry:       0,
	})

	defer func() {
		// Done enterPrecommit:
		state.UpdateRoundStep(round, RoundStepPrecommit)
	}()

	var blockHash = common.Hash{}
	prevotes, ok := state.GetPrevotesByRound(round)
	if ok {
		blockHash, ok = prevotes.TwoThirdMajority()
	}

	// if we don't have polka, must precommit nil
	if !ok {
		if state.LockedBlock() != nil {
			logger.Infow("enterPrecommit: No +2/3 prevotes during enterPrecommit while we're locked. Precommitting nil")
		} else {
			logger.Infow("enterPrecommit: No +2/3 prevotes during enterPrecommit. Precommitting nil.")
		}
		c.SendVote(msgPrecommit, nil, round)
		return
	}

	// The last PoLR should be this round
	polRound, _ := state.POLInfo()
	if polRound < round {
		logger.Panicw("wrong POLRound", "expected_pol", round, "received_pol", polRound)
	}

	// +2/3 prevoted nil. Unlock and precommit nil.
	if blockHash.Hex() == emptyBlockHash.Hex() {
		if state.LockedBlock() == nil {
			logger.Infow("enterPrecommit: +2/3 prevoted for nil.")
		} else {
			logger.Infow("enterPrecommit: +2/3 prevoted for nil. Unlocking")
			state.Unlock()
		}
		c.SendVote(msgPrecommit, nil, round)
		return
	}

	// At this point, +2/3 prevoted for a particular block.
	// If we're already locked on that block, precommit it, and update the LockedRound
	if state.LockedBlock() != nil && state.LockedBlock().Hash().Hex() == blockHash.Hex() {
		logger.Infow("enterPrecommit: +2/3 prevoted locked block. Relocking")
		state.SetLockedRoundAndBlock(round, state.LockedBlock())
		c.SendVote(msgPrecommit, state.LockedBlock(), round)
		return
	}

	// If +2/3 prevoted for proposal block, stage and precommit it
	if state.ProposalReceived() != nil && state.ProposalReceived().Block.Hash().Hex() == blockHash.Hex() {
		logger.Infow("enterPrecommit: +2/3 prevoted proposal block. Locking", "hash", blockHash)
		// TODO: Validate the block before locking and precommit
		state.SetLockedRoundAndBlock(round, state.ProposalReceived().Block)
		c.SendVote(msgPrecommit, state.ProposalReceived().Block, round)
		return
	}

	// There was a polka in this round for a block we don't have.
	// TODO: Fetch that block, unlock, and precommit nil.
	// The +2/3 prevotes for this round is the POL for our unlock.
	logger.Infow("enterPrecommit: +2/3 prevoted a block we don't have. Fetch. Unlock and Precommit nil", "hash", blockHash.Hex())
	state.Unlock()
	c.SendVote(msgPrecommit, nil, round)
}

func (c *core) enterCommit(blockNumber *big.Int, commitRound int64) {
	var (
		state  = c.currentState
		logger = c.getLogger().With("input_block_number", blockNumber, "input_round", commitRound, "input_step", RoundStepCommit)
	)
	if state.BlockNumber().Cmp(blockNumber) != 0 || state.Step() >= RoundStepCommit {
		logger.Debugw("enterCommit ignore: we are in a state that is ahead of the input state")
		return
	}

	defer func() {
		// Done enterCommit:
		// keep state.Round the same, commitRound points to the right Precommits set.
		state.UpdateRoundStep(state.Round(), RoundStepCommit)
		state.commitRound = commitRound
		state.commitTime = time.Now()

		c.finalizeCommit(blockNumber)
	}()

	precommits, ok := state.GetPrecommitsByRound(commitRound)

	if !ok {
		logger.Panicw("commit round must have a set of precommits")
	}

	blockHash, ok := precommits.TwoThirdMajority()

	if !ok {
		logger.Panicw("commit round must has a majority block")
	}
	var (
		lockedBlock = state.LockedBlock()
	)
	//if lockBlock is the same as the hash, move it to Proposal
	//it will be cleared upon entering newHeight
	if lockedBlock != nil && lockedBlock.Hash().Hex() == blockHash.Hex() {
		logger.Infow("Commit is for locked block. Set ProposalBlock=LockedBlock", "blockHash", blockHash.Hex())
		state.SetProposalReceived(&Proposal{
			Block:    lockedBlock,
			Round:    commitRound,
			POLRound: state.LockedRound(),
		})
	}
	var (
		proposalReceived = state.ProposalReceived()
	)
	// If we don't have the block being commit, we set proposalReceived to nil and wait
	if proposalReceived != nil && proposalReceived.Block.Hash().Hex() != blockHash.Hex() {
		state.SetProposalReceived(nil)
	}

}

func (c *core) finalizeCommit(blockNumber *big.Int) {
	var (
		state  = c.CurrentState()
		logger = c.getLogger().With("input_block_number", blockNumber, "input_step", RoundStepPrecommitWait)
	)
	if state.BlockNumber().Cmp(blockNumber) != 0 {
		logger.Panicw("finalize a commit at different state block number")
	}
	if state.Step() != RoundStepCommit {
		logger.Errorw("finalizeCommit invalid: we are in a state that is invalid for commit")
		return
	}
	precommits, ok := state.GetPrecommitsByRound(state.commitRound)
	if !ok {
		logger.Errorw("no precommits at commitRound")
		return
	}
	blockHash, ok := precommits.TwoThirdMajority()
	if !ok {
		logger.Errorw("no 2/3 majority for a block at commitRound")
		return
	}
	if blockHash.Hex() == emptyBlockHash.Hex() {
		logger.Errorw("nil majority at commitRound")
		return
	}
	proposal := state.ProposalReceived()
	if proposal == nil {
		logger.Infow("empty proposal at finalizeCommit: no proposal has been received")
		return
	}
	if proposal.Block != nil && proposal.Block.Hash().Hex() != blockHash.Hex() {
		logger.Infow("the proposal received was not the commit hash. Finalize failed")
		return
	}

	//TODO: do we need revalidating block at this step?

	logger.Infow("committing: write seals onto Block", "block_hash", blockHash.Hex())

	block, err := c.FinalizeBlock(state.ProposalReceived())
	if err != nil {
		logger.Panicw("block committing failed", "error", err)
	}

	c.backend.Commit(block)
}

//FinalizeBlock will fill extradata with signature and return the ready to store block
func (c *core) FinalizeBlock(proposal *Proposal) (*types.Block, error) {
	var (
		state           = c.currentState
		round           = state.commitRound
		totalPrecommits = 0
		commitSeals     = [][]byte{}
		header          = proposal.Block.Header()
		minMajority     = c.valSet.MinMajority()
	)
	precommits, ok := state.GetPrecommitsByRound(round)
	if !ok {
		c.getLogger().Panicw("no precommits at commitRound")
	}

	votes, ok := precommits.voteByBlock[header.Hash()]
	if !ok || votes == nil {
		c.getLogger().Panicw("no votes for the committing block", "block_hash", header.Hash())
	}
	if votes.totalReceived < minMajority {
		return nil, fmt.Errorf("not enough precommits received expect at least %d received %d", minMajority, totalPrecommits)
	}

	for _, vote := range votes.votes {
		if vote == nil {
			continue
		}
		commitSeals = append(commitSeals, vote.Seal)
		totalPrecommits++
		//TODO: is it fair to always take the first 2F+1 seals?
		if totalPrecommits >= minMajority {
			break
		}
	}

	if totalPrecommits < minMajority {
		return nil, fmt.Errorf("not enough precommits received expect at least %d received %d", minMajority, totalPrecommits)
	}
	//writeCommitSeals
	if err := utils.WriteCommittedSeals(header, commitSeals); err != nil {
		return nil, err
	}
	return proposal.Block.WithSeal(header), nil
}

func (c *core) startNewRound() {
	var (
		state                 = c.CurrentState()
		lastBlockNumber       = c.backend.CurrentHeadBlock().Number()
		expectedBlock         = big.NewInt(0).Add(lastBlockNumber, big.NewInt(1))
		needInitializeTimeout = true
		duration              time.Duration
	)

	if state.BlockNumber().Cmp(expectedBlock) == 0 {
		c.getLogger().Infow("Catch up with the latest block")
	} else {
		//special case:core is stopped and the network has already consensus newer block
		// update new round with lastKnownHeight
		c.getLogger().Infow("New height is not catch up with the latest block, update height to lastest block + 1")
		state.SetView(&tendermint.View{
			Round:       0,
			BlockNumber: expectedBlock,
		})
		state.clearPreviousRoundData()
		c.sentMsgStorage.truncateMsgStored(c.getLogger())
		c.valSet = c.backend.Validators(state.BlockNumber())
	}

	//TODO: the timeout must account for the stopped time that core wasn't
	switch state.Step() {
	case RoundStepNewHeight:
		duration = time.Until(state.startTime)
	case RoundStepPropose:
		duration = c.config.ProposeTimeout(state.Round())
	case RoundStepPrevote:
		duration = c.config.PrevoteCatchupTimeout(state.Round())
	case RoundStepPrevoteWait:
		duration = c.config.PrevoteTimeout(state.Round())
	case RoundStepPrecommit:
		duration = c.config.PrecommitCatchupTimeout(state.Round())
	case RoundStepPrecommitWait:
		duration = c.config.PrecommitTimeout(state.Round())
	default:
		needInitializeTimeout = false
	}
	// if this is not a validator, core stays at NewHeightStep
	if i, _ := c.valSet.GetByAddress(c.backend.Address()); i == -1 {
		c.getLogger().Warnw("this node is not a validator of this round, skipping consensus process", "address", c.backend.Address())
		needInitializeTimeout = false
	}

	if needInitializeTimeout {
		//We have to copy blockNumber out since it's pointer, and the use of ScheduleTimeout
		timeOutBlock := big.NewInt(0).Set(state.BlockNumber())
		c.timeout.ScheduleTimeout(timeoutInfo{
			Duration:    duration,
			BlockNumber: timeOutBlock,
			Round:       state.Round(),
			Step:        state.Step(),
			Retry:       0,
		})
	}
}

// processFutureMessages dequeue and runs all msgs for the next block (caller should lock the mutex to ensure thread-safe)
func (c *core) processFutureMessages(logger *zap.SugaredLogger) (done bool, err error) {
	var (
		state          = c.CurrentState()
		msgBlockNumber *big.Int
		msgRound       int64
	)
	for {
		if c.futureMessages.Len() == 0 {
			return true, nil
		}
		// get at position 0, check if it is current block number
		data := c.futureMessages.Peek()
		msg, ok := data.(*msgItem).message.(message)
		if !ok {
			logger.Errorw("Failed to decode data to message")
			return false, err
		}
		switch msg.Code {
		case msgPrecommit, msgPrevote:
			var vote Vote
			if err := rlp.DecodeBytes(msg.Msg, &vote); err != nil {
				logger.Errorw("Failed to decode vote from message", "error", err)
				return false, err
			}
			msgBlockNumber = vote.BlockNumber
			msgRound = vote.Round
		case msgPropose:
			var proposal Proposal
			if err := rlp.DecodeBytes(msg.Msg, &proposal); err != nil {
				logger.Errorw("Failed to decode vote from message", "error", err)
				return false, err
			}
			msgBlockNumber = proposal.Block.Number()
			msgRound = proposal.Round
		default:
			return false, fmt.Errorf("unknown msg code %d", msg.Code)
		}
		if msgBlockNumber.Cmp(state.BlockNumber()) < 0 {
			logger.Infow("vote from older block number, ignore")
			// Ignore vote from older block, remove element at position 0 and continue
			if _, err := c.futureMessages.Get(1); err != nil {
				logger.Warn("failed to remove from future msgs", "err", err)
			}
			continue
		}
		if msgBlockNumber.Cmp(state.BlockNumber()) > 0 {
			// It is future block, stop processing
			break
		}
		// at here vote block number should be equal state block number
		// remove message and handle it
		logger.Infow("handle vote message in future message queue",
			"msg_block", msgBlockNumber, "msg_round", msgRound, "from", msg.Address)
		if _, err := c.futureMessages.Get(1); err != nil {
			logger.Warn("failed to remove from future msgs", "err", err)
		}
		if err := c.handleMsgLocked(msg); err != nil {
			logger.Warn("failed to handle msg", "err", err)
		}
	}
	return true, nil
}
