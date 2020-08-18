package tendermint

import (
	"math/big"
	"time"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/core/state/staking"
)

type ProposerPolicy uint64

const (
	RoundRobin ProposerPolicy = iota
	Sticky
)

//FaultyMode is the config mode to enable fauty node
type FaultyMode uint64

const (
	// Disabled disables the faulty mode
	Disabled FaultyMode = iota
	// SendFakeProposal sends the proposal with the fake info
	SendFakeProposal
	// RandomlyStopSendingMsg randomly stop message sending
	RandomlyStopSendingMsg
)

func (f FaultyMode) Uint64() uint64 {
	return uint64(f)
}

//Config store all the configuration required for a Tendermint consensus
type Config struct {
	ProposerPolicy        ProposerPolicy   `toml:",omitempty"` // The policy for proposer selection
	Epoch                 uint64           `toml:",omitempty"` // The number of blocks after which to checkpoint and reset the pending votes
	StakingSCAddress      *common.Address  `toml:",omitempty"` // The staking SC address for validating when deploy SC
	BlockPeriod           uint64           `toml:",omitempty"` // Default minimum difference between two consecutive block's timestamps in second
	TimeoutPropose        time.Duration    //Duration waiting a propose
	TimeoutProposeDelta   time.Duration    //Increment if timeout happens at propose step to reach eventually synchronous
	TimeoutPrevote        time.Duration    //Duration waiting for more prevote after 2/3 received
	TimeoutPrevoteDelta   time.Duration    //Increment if timeout happens at prevoteWait to reach eventually synchronous
	TimeoutPrecommit      time.Duration    //Duration waiting for more precommit after 2/3 received
	TimeoutPrecommitDelta time.Duration    //Duration waiting to increase if precommit wait expired to reach eventually synchronous
	TimeoutCommit         time.Duration    //Duration waiting to start round with new height
	FixedValidators       []common.Address // The fixed validators
	BlockReward           *big.Int         //BlockReward for accumulating reward

	FaultyMode uint64 `toml:",omitempty"` // The faulty node indicates the faulty node's behavior

	UseEVMCaller        bool
	IndexStateVariables *staking.IndexConfigs //The index of state variables has stored in stateDB
}

var DefaultConfig = &Config{
	ProposerPolicy:        RoundRobin,
	Epoch:                 30000,
	StakingSCAddress:      &common.Address{},
	BlockPeriod:           1,                       // 1 seconds
	TimeoutPropose:        3000 * time.Millisecond, //This is taken from tendermint. Might need tuning
	TimeoutProposeDelta:   500 * time.Millisecond,  //This is taken from tendermint. Might need tunning
	TimeoutPrevote:        1000 * time.Millisecond,
	TimeoutPrevoteDelta:   500 * time.Millisecond,
	TimeoutPrecommit:      1000 * time.Millisecond,
	TimeoutPrecommitDelta: 500 * time.Millisecond,
	TimeoutCommit:         1000 * time.Millisecond,
	FaultyMode:            Disabled.Uint64(),
	UseEVMCaller:          false,
	IndexStateVariables:   staking.DefaultConfig,
}

//ProposeTimeout return the timeout for a specific round
//The formula is timeout= TimeoutPropose + round*TimeoutProposeDelta
func (cfg Config) ProposeTimeout(round int64) time.Duration {
	return time.Duration(
		cfg.TimeoutPropose.Nanoseconds()+cfg.TimeoutProposeDelta.Nanoseconds()*(round),
	) * time.Nanosecond
}

// PrevoteTimeout returns the amount of time to wait for straggler votes after receiving any +2/3 prevotes
func (cfg *Config) PrevoteTimeout(round int64) time.Duration {
	return time.Duration(
		cfg.TimeoutPrevote.Nanoseconds()+cfg.TimeoutPrevoteDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

//PrevoteCatchupTimeout returns the amount of time to wait for prevote msgs before sending catchup msg
//Notes: if node 1 did not receive propose msg, it will delay max = proposeTimeout before sending prevote
// So node 2 received propose msg, it will entered prevote earlier than node 1 by proposeTimeout
// In here, node 2 sleep about 2 times of proposeTimeout before assuming that sending prevote message of node 1 has problem
func (cfg *Config) PrevoteCatchupTimeout(round int64) time.Duration {
	return time.Duration(cfg.ProposeTimeout(round).Nanoseconds() * int64(2))
}

// PrecommitTimeout returns the amount of time to wait for straggler votes after receiving any +2/3 precommits
func (cfg *Config) PrecommitTimeout(round int64) time.Duration {
	return time.Duration(
		cfg.TimeoutPrecommit.Nanoseconds()+cfg.TimeoutPrecommitDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

//PrecommitCatchupTimeout returns the amount of time to wait for precommit msgs before sending catchup msg
//Notes: if node 1 did not receive a polka of prevote msg, it will delay max = prevoteWaitTimeout before sending precommit
// So node 2 received a polka of prevote msg, it will entered precommit earlier than node 1 by prevoteWaitTimeout
// In here, node 2 sleep about 2 times of prevoteWaitTimeout before assuming that sending precommit message of node 1 has problem
func (cfg *Config) PrecommitCatchupTimeout(round int64) time.Duration {
	return time.Duration(cfg.PrevoteTimeout(round).Nanoseconds() * int64(2))
}

// Commit returns the amount of time to wait for straggler votes after receiving +2/3 precommits for a single block (ie. a commit).
func (cfg *Config) Commit(t time.Time) time.Time {
	return t.Add(cfg.TimeoutCommit)
}
