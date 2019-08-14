package tendermint

type ProposerPolicy uint64

const (
	RoundRobin ProposerPolicy = iota
)

type Config struct {
	ProposerPolicy ProposerPolicy `toml:",omitempty"` // The policy for proposer selection
	Epoch          uint64         `toml:",omitempty"` // The number of blocks after which to checkpoint and reset the pending votes
}

var DefaultConfig = &Config{
	ProposerPolicy: RoundRobin,
	Epoch:          30000,
}
