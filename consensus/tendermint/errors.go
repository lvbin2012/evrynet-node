package tendermint

import "errors"

var (
	// ErrStartedEngine is returned if the engine is already started
	ErrStartedEngine = errors.New("engine is already started")
	// ErrStoppedEngine is returned if the engine is stopped
	ErrStoppedEngine = errors.New("engine is already stopped")
	// ErrEmptyCommittedSeals is returned if the field of committed seals is zero.
	ErrEmptyCommittedSeals = errors.New("zero committed seals")
	// ErrEmptyValSet is returned if the field of validator set is zero.
	ErrEmptyValSet = errors.New("zero validator set")
	// ErrMismatchValSet is returned if the field of validator set is mismatch.
	ErrMismatchValSet = errors.New("mismatch validator set")
	// ErrMismatchTxhashes is returned if the TxHash in header is mismatch.
	ErrMismatchTxhashes = errors.New("mismatch transaction hashes")
	// errInvalidSignature is returned when given signature is not signed by given
	// address.
	ErrInvalidSignature = errors.New("invalid signature")
	// errUnknownBlock is returned when the list of validators is requested for a block
	// that is not part of the local blockchain.
	ErrUnknownBlock = errors.New("unknown block")
	// errUnauthorized is returned if a header is signed by a non authorized entity.
	ErrUnauthorized = errors.New("unauthorized")
	// errInvalidDifficulty is returned if the difficulty of a block is not 1
	ErrInvalidDifficulty = errors.New("invalid difficulty")
	// errInvalidExtraDataFormat is returned when the extra data format is incorrect
	ErrInvalidExtraDataFormat = errors.New("invalid extra data format")
	// errInvalidMixDigest is returned if a block's mix digest is not Tendermint digest.
	ErrInvalidMixDigest = errors.New("invalid Tendermint mix digest")
	// errInvalidCommittedSeals is returned if the committed seal is not signed by any of parent validators.
	ErrInvalidCommittedSeals = errors.New("invalid committed seals")
	// errInvalidVotingChain is returned if an authorization list is attempted to
	// be modified via out-of-range or non-contiguous headers.
	ErrInvalidVotingChain = errors.New("invalid voting chain")
	// errCoinBaseInvalid is returned if the value of coin base is not equals proposer's address in header
	ErrCoinBaseInvalid = errors.New("invalid coin base address")
	// errInvalidUncleHash is returned if a block contains an non-empty uncle list.
	ErrInvalidUncleHash = errors.New("non empty uncle hash")
	// errInvalidVote is returned if a nonce value is something else that the two
	// allowed constants of 0x00..0 or 0xff..f.
	ErrInvalidVote = errors.New("vote nonce not 0x0000000000000000 or 0xffffffffffffffff")
	// errInvalidCandidate is return if the extra data's modifiedValidator is empty or nil
	ErrInvalidCandidate = errors.New("candidate for validator is invalid")
	// ErrUnknownParent is return when a proposal is sent with unknown parent hash
	ErrUnknownParent = errors.New("unknown parent")
	// ErrFinalizeZeroBlock is returned if node finalize with block number = 0
	ErrFinalizeZeroBlock = errors.New("finalize zero block")
)
