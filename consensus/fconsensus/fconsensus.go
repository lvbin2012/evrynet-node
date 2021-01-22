package fconsensus

import (
	"bytes"
	"errors"
	"io"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/crypto/sha3"

	"github.com/Evrynetlabs/evrynet-node/accounts"
	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/common/hexutil"
	"github.com/Evrynetlabs/evrynet-node/consensus"
	fconTypes "github.com/Evrynetlabs/evrynet-node/consensus/fconsensus/types"
	"github.com/Evrynetlabs/evrynet-node/core/state"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/evrdb"
	"github.com/Evrynetlabs/evrynet-node/log"
	"github.com/Evrynetlabs/evrynet-node/params"
	"github.com/Evrynetlabs/evrynet-node/rlp"
	"github.com/Evrynetlabs/evrynet-node/rpc"
	lru "github.com/hashicorp/golang-lru"
)

const (
	checkpointInterval = 1024 // Number of blocks after which to save the vote snapshot to the database
	inmemorySignatures = 4096
	inmemorySnapshots  = 128 // Number of recent vote snapshots to keep in memory
	ExtraVanity        = 32
	SignerAddress      = "EJp8jwQvRs7L74t5XYAH6SfG44koVWUVqv"
)

var (
	epochLength   = uint64(30000) // Default number of blocks after which to checkpoint and reset the pending votes
	uncleHash     = types.CalcUncleHash(nil)
	diffInTurn    = big.NewInt(2)
	nonceAuthVote = hexutil.MustDecode("0xffffffffffffffff") // Magic nonce number to vote on adding a new signer
	nonceDropVote = hexutil.MustDecode("0x0000000000000000") // Magic nonce number to vote on removing a signer.
)

var (
	errInvalidHeaderExtra = errors.New("invalid header extra-data")
	errUnknownBlock       = errors.New("unknown block")
	errMissingVanity      = errors.New("extra-data 32 byte vanity prefix missing")
	errInvalidMixDigest   = errors.New("non-zero mix digest")

	errInvalidCheckpointBeneficiary = errors.New("beneficiary in checkpoint block non-zero")
	errInvalidVote                  = errors.New("vote nonce not 0x00..0 or 0xff..f")
	errInvalidCheckpointVote        = errors.New("vote nonce in checkpoint block non-zero")
	// errInvalidUncleHash is returned if a block contains an non-empty uncle list.
	errInvalidUncleHash = errors.New("non empty uncle hash")

	// errInvalidDifficulty is returned if the difficulty of a block neither 1 or 2.
	errInvalidDifficulty = errors.New("invalid difficulty")

	// errWrongDifficulty is returned if the difficulty of a block doesn't match the
	// turn of the signer.
	errWrongDifficulty = errors.New("wrong difficulty")

	// ErrInvalidTimestamp is returned if the timestamp of a block is lower than
	// the previous block's timestamp + the minimum block period.
	ErrInvalidTimestamp = errors.New("invalid timestamp")

	// errInvalidVotingChain is returned if an authorization list is attempted to
	// be modified via out-of-range or non-contiguous headers.
	errInvalidVotingChain = errors.New("invalid voting chain")

	// errUnauthorizedSigner is returned if a header is signed by a non-authorized entity.
	errUnauthorizedSigner = errors.New("unauthorized signer")

	errRecentlySigned               = errors.New("recently signed")
	errSignersNumberWrong           = errors.New("wrong number of signers")
	errMismatchingCheckpointSigners = errors.New("mismatching signer list on checkpoint block")
)

type SignerFn func(accounts.Account, string, []byte) ([]byte, error)

type FConsensus struct {
	config *params.FConConfig
	db     evrdb.Database

	recents   *lru.ARCCache
	signature *lru.ARCCache

	proposals map[common.Address]bool
	signer    common.Address
	signFn    SignerFn
	lock      sync.RWMutex
}

func New(config *params.FConConfig, db evrdb.Database) *FConsensus {
	conf := *config
	if conf.Epoch == 0 {
		conf.Epoch = epochLength
	}
	recents, _ := lru.NewARC(inmemorySnapshots)
	signatures, _ := lru.NewARC(inmemorySignatures)

	return &FConsensus{
		db:        db,
		config:    &conf,
		recents:   recents,
		signature: signatures,
		proposals: make(map[common.Address]bool),
	}
}

func (fc *FConsensus) Authorize(signer common.Address, signFn SignerFn) {
	fc.lock.Lock()
	defer fc.lock.Unlock()

	fc.signer = signer
	fc.signFn = signFn
}

func (fc *FConsensus) Author(header *types.Header) (common.Address, error) {
	return ecrecover(header, fc.signature)
}

func (fc *FConsensus) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	return fc.verifyHeader(chain, header, nil)
}

func (fc *FConsensus) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			err := fc.verifyHeader(chain, header, headers[:i])
			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results

}

func (fc *FConsensus) verifyHeader(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {
	if header.Number == nil {
		return errUnknownBlock
	}

	number := header.Number.Uint64()

	if header.Time > uint64(time.Now().Unix()) {
		return consensus.ErrFutureBlock
	}

	checkpoint := (number % fc.config.Epoch) == 0
	if checkpoint && header.Coinbase != (common.Address{}) {
		return errInvalidCheckpointBeneficiary
	}

	if !bytes.Equal(header.Nonce[:], nonceDropVote) && !bytes.Equal(header.Nonce[:], nonceAuthVote) {
		return errInvalidVote
	}

	if checkpoint && !bytes.Equal(header.Nonce[:], nonceDropVote) {
		return errInvalidCheckpointVote
	}
	if len(header.Extra) < ExtraVanity {
		return errMissingVanity
	}

	if header.MixDigest != (common.Hash{}) {
		return errInvalidMixDigest
	}

	if header.UncleHash != uncleHash {
		return errInvalidUncleHash
	}

	if number > 0 {
		if header.Difficulty == nil || header.Difficulty.Cmp(diffInTurn) != 0 {
			return errInvalidDifficulty
		}
	}
	return fc.verifyCascadingFields(chain, header, parents)
}

func (fc *FConsensus) verifyCascadingFields(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {
	number := header.Number.Uint64()
	if number == 0 {
		return nil
	}
	var parent *types.Header
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(header.ParentHash, number-1)
	}
	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}
	fsnap, err := fc.fsnapshot(chain, number-1, header.ParentHash, parents)
	if err != nil {
		return err
	}
	if number%fc.config.Epoch == 0 {
		fce, err := fconTypes.ExtractFConExtra(header)
		if err != nil {
			return err
		}
		signers := fsnap.signers()
		if len(fce.Signers) != len(signers) {
			return errSignersNumberWrong
		}
		for i := 0; i < len(signers); i++ {
			if signers[i] != fce.Signers[i] {
				return errMismatchingCheckpointSigners
			}
		}
	}

	return fc.verifySeal(chain, header, parents)
}

func (fc *FConsensus) fsnapshot(chain consensus.ChainReader, number uint64, hash common.Hash,
	parents []*types.Header) (*FSnapshot, error) {
	var (
		headers []*types.Header
		fsnap   *FSnapshot
	)
	for fsnap == nil {
		if s, ok := fc.recents.Get(hash); ok {
			fsnap = s.(*FSnapshot)
			break
		}
		if number%checkpointInterval == 0 {
			if fs, err := loadFSnapshot(fc.config, fc.signature, fc.db, hash); err == nil {
				fsnap = fs
				break
			}
		}

		if number == 0 || (number%fc.config.Epoch == 0 && (len(headers) > params.ImmutabilityThreshold ||
			chain.GetHeaderByNumber(number-1) == nil)) {
			checkpoint := chain.GetHeaderByNumber(number)
			var signers []common.Address
			if checkpoint != nil {
				hash := checkpoint.Hash()
				if number == 0 {
					// TODO change genesisfile
					signers = make([]common.Address, (len(checkpoint.Extra)-97)/common.AddressLength)
					for i := 0; i < len(signers); i++ {
						copy(signers[i][:], checkpoint.Extra[32+i*common.AddressLength:])
					}
				} else {
					fce, err := fconTypes.ExtractFConExtra(checkpoint)
					if err != nil {
						return nil, err
					}
					signers = fce.Signers
				}
				fsnap = newFSnapshot(fc.config, fc.signature, number, hash, signers)
				if err := fsnap.store(fc.db); err != nil {
					return nil, err
				}

				log.Info("FConsensus Stored checkpoint snapshot to disk", "number", number, "hash", hash)
				break
			}
		}
		var header *types.Header
		if len(parents) > 0 {
			header = parents[len(parents)-1]
			if header.Hash() != hash || header.Number.Uint64() != number {
				return nil, consensus.ErrUnknownAncestor
			}
			parents = parents[:len(parents)-1]
		} else {
			header = chain.GetHeader(hash, number)
			if header == nil {
				return nil, consensus.ErrUnknownAncestor
			}
		}
		headers = append(headers, header)
		number, hash = number-1, header.ParentHash
	}

	for i := 0; i < len(headers)/2; i++ {
		headers[i], headers[len(headers)-1-i] = headers[len(headers)-1-i], headers[i]
	}
	fsnap, err := fsnap.apply(headers)
	if err != nil {
		return nil, err
	}
	fc.recents.Add(fsnap.Hash, fsnap)

	if fsnap.Number%checkpointInterval == 0 && len(headers) > 0 {
		if err = fsnap.store(fc.db); err != nil {
			return nil, err
		}
		log.Trace("FConsensus Stored voting snapshot to disk", "number", fsnap.Number, "hash", fsnap.Hash)
	}
	return fsnap, err
}

func (fc *FConsensus) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed")
	}
	return nil
}

func (fc *FConsensus) GetAuthorizedSinger() (common.Address, error) {
	return common.EvryAddressStringToAddressCheck(SignerAddress)
}

func (fc *FConsensus) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	return fc.verifyHeader(chain, header, nil)
}

func (fc *FConsensus) verifySeal(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {
	number := header.Number.Uint64()
	if number == 0 {
		return nil
	}
	fsnap, err := fc.fsnapshot(chain, number-1, header.ParentHash, parents)
	if err != nil {
		return err
	}
	//authorizedSigner, err := fc.GetAuthorizedSinger()
	if err != nil {
		return err
	}
	signer, err := ecrecover(header, fc.signature)
	if err != nil {
		return err
	}
	if _, ok := fsnap.Signers[signer]; !ok {
		return errUnauthorizedSigner
	}
	for seen, recent := range fsnap.Recents {
		if recent == signer {
			if limit := uint64(len(fsnap.Signers)/2 + 1); seen > number-limit {
				return errRecentlySigned
			}
		}
	}
	return nil
}

func (fc *FConsensus) Prepare(chain consensus.FullChainReader, header *types.Header) error {
	header.Coinbase = common.Address{}
	header.Nonce = types.BlockNonce{}

	number := header.Number.Uint64()

	fsnap, err := fc.fsnapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return err
	}
	if number%fc.config.Epoch != 0 {
		fc.lock.RLock()
		addresses := make([]common.Address, 0, len(fc.proposals))
		for address, authorize := range fc.proposals {
			if fsnap.validVate(address, authorize) {
				addresses = append(addresses, address)
			}
		}
		if len(addresses) > 0 {
			header.Coinbase = addresses[rand.Intn(len(addresses))]
			if fc.proposals[header.Coinbase] {
				copy(header.Nonce[:], nonceAuthVote)
			} else {
				copy(header.Nonce[:], nonceDropVote)
			}
		}
		fc.lock.RUnlock()
	}

	header.Difficulty = diffInTurn

	if len(header.Extra) < ExtraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, ExtraVanity-len(header.Extra))...)
	}

	fce := fconTypes.FConExtra{}
	if number%fc.config.Epoch == 0 {
		fce.Signers = fsnap.signers()
	}

	byteBuffer := new(bytes.Buffer)
	err = rlp.Encode(byteBuffer, &fce)
	if err != nil {
		return err
	}
	header.Extra = append(header.Extra[:ExtraVanity], byteBuffer.Bytes()...)
	return nil
}

func (fc *FConsensus) Finalize(chain consensus.FullChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header) error {
	header.Root = state.IntermediateRoot(true)
	header.UncleHash = types.CalcUncleHash(nil)
	return nil
}

func (fc *FConsensus) FinalizeAndAssemble(chain consensus.FullChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	header.Root = state.IntermediateRoot(true)
	header.UncleHash = types.CalcUncleHash(nil)
	return types.NewBlock(header, txs, nil, receipts), nil
}

func (fc *FConsensus) SealForTest(block *types.Block) (*types.Block, error) {
	header := block.Header()
	if len(header.Extra) < ExtraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, ExtraVanity-len(header.Extra))...)
	}
	signHash, err := fc.signFn(accounts.Account{Address: fc.signer}, accounts.MimetypeClique, FConRLP(header))
	if err != nil {
		return nil, err
	}
	fce, err := fconTypes.ExtractFConExtra(header)
	if err != nil {
		return nil, err
	}

	fce.Seal = append(fce.Seal[:0], signHash[:]...)
	byteBuffer := new(bytes.Buffer)
	err = rlp.Encode(byteBuffer, &fce)
	if err != nil {
		return nil, err
	}
	header.Extra = append(header.Extra[:ExtraVanity], byteBuffer.Bytes()...)
	return block.WithSeal(header), nil

}

func (fc *FConsensus) Seal(chain consensus.ChainReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := block.Header()

	number := header.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}

	fc.lock.RLock()
	signer, signFn := fc.signer, fc.signFn
	fc.lock.RUnlock()

	fsnap, err := fc.fsnapshot(chain, number-1, header.ParentHash, nil)
	if err != nil {
		return err
	}
	if _, authorized := fsnap.Signers[signer]; !authorized {
		return errUnauthorizedSigner
	}

	for seen, recent := range fsnap.Recents {
		if recent == signer {
			// Signer is among recents, only wait if the current block doesn't shift it out
			if limit := uint64(len(fsnap.Signers)/2 + 1); number < limit || seen > number-limit {
				log.Info("Signed recently, must wait for others")
				return nil
			}
		}
	}

	signHash, err := signFn(accounts.Account{Address: signer}, accounts.MimetypeClique, FConRLP(header))
	if err != nil {
		return err
	}

	fce, err := fconTypes.ExtractFConExtra(header)
	if err != nil {
		return err
	}

	fce.Seal = append(fce.Seal[:0], signHash[:]...)
	byteBuffer := new(bytes.Buffer)
	err = rlp.Encode(byteBuffer, &fce)
	if err != nil {
		return err
	}
	header.Extra = append(header.Extra[:ExtraVanity], byteBuffer.Bytes()...)
	go func() {
		select {
		case <-stop:
			return
		default:

		}
		select {
		case results <- block.WithSeal(header):
		default:
			log.Warn("Sealing result is not read by miner", "sealhash", SealHash(header))
		}
	}()
	return nil
}

func (fc *FConsensus) SealHash(header *types.Header) common.Hash {
	return SealHash(header)
}

func (fc *FConsensus) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	return diffInTurn
}

func (fc *FConsensus) APIs(chain consensus.ChainReader) []rpc.API {
	return nil
}

func (fc *FConsensus) Close() error {
	return nil
}

func ecrecover(header *types.Header, sigcache *lru.ARCCache) (common.Address, error) {
	hash := header.Hash()
	if address, known := sigcache.Get(hash); known {
		return address.(common.Address), nil
	}

	if len(header.Extra) < ExtraVanity {
		return common.Address{}, errInvalidHeaderExtra
	}
	fce, err := fconTypes.ExtractFConExtra(header)
	if err != nil {
		return common.Address{}, err
	}
	pubkey, err := crypto.Ecrecover(SealHash(header).Bytes(), fce.Seal)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])
	sigcache.Add(hash, signer)
	return signer, nil
}

func FConRLP(header *types.Header) []byte {
	b := new(bytes.Buffer)
	encodeSigHeader(b, header)
	return b.Bytes()
}

func SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	encodeSigHeader(hasher, header)
	hasher.Sum(hash[:0])
	return hash
}

func encodeSigHeader(w io.Writer, header *types.Header) {
	cpy := types.CopyHeader(header)
	if len(header.Extra) <= ExtraVanity {
		panic(errInvalidHeaderExtra)
	}
	fce, err := fconTypes.ExtractFConExtra(header)
	if err != nil {
		panic("can't encode: " + err.Error())
	}

	fce.Seal = nil
	fceBytes, err := rlp.EncodeToBytes(fce)
	if err != nil {
		panic("can't encode: " + err.Error())
	}
	cpy.Extra = append(cpy.Extra[:ExtraVanity], fceBytes...)
	err = rlp.Encode(w, []interface{}{
		cpy.ParentHash,
		cpy.UncleHash,
		cpy.Coinbase,
		cpy.Root,
		cpy.TxHash,
		cpy.ReceiptHash,
		cpy.Bloom,
		cpy.Difficulty,
		cpy.Number,
		cpy.GasLimit,
		cpy.GasUsed,
		cpy.Time,
		cpy.Extra,
		cpy.MixDigest,
		cpy.Nonce,
	})
	if err != nil {
		panic("can't encode: " + err.Error())
	}
}
