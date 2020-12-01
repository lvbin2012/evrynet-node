package fconsensus

import (
	"bytes"
	"errors"
	"github.com/Evrynetlabs/evrynet-node/log"
	"golang.org/x/crypto/sha3"
	"io"
	"math/big"
	"sync"
	"time"

	"github.com/Evrynetlabs/evrynet-node/accounts"
	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus"
	"github.com/Evrynetlabs/evrynet-node/core/state"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/evrdb"
	"github.com/Evrynetlabs/evrynet-node/rlp"
	"github.com/Evrynetlabs/evrynet-node/rpc"
	lru "github.com/hashicorp/golang-lru"
)

const (
	inmemorySignatures = 4096
	ExtraVanity        = 32
	SignerAddress      = "EJp8jwQvRs7L74t5XYAH6SfG44koVWUVqv"
)

var (
	uncleHash  = types.CalcUncleHash(nil)
	diffInTurn = big.NewInt(2)
)

var (
	errInvalidHeaderExtra = errors.New("invalid header extra-data")
	errUnknownBlock       = errors.New("unknown block")
	errMissingVanity      = errors.New("extra-data 32 byte vanity prefix missing")
	errInvalidMixDigest   = errors.New("non-zero mix digest")

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
)

type SignerFn func(accounts.Account, string, []byte) ([]byte, error)

type FConExtra struct {
	Seal         []byte
	CurrentBlock common.Hash
	EvilHeader   *types.Header
}

func (fce *FConExtra) EncodeRLP(w io.Writer) error {
	headerRLP, err := rlp.EncodeToBytes(fce.EvilHeader)
	if err != nil {
		return err
	}
	return rlp.Encode(w, []interface{}{
		fce.Seal,
		fce.CurrentBlock,
		headerRLP,
	})
}

func (fce *FConExtra) DecodeRLP(s *rlp.Stream) error {
	var extra struct {
		Seal         []byte
		CurrentBlock common.Hash
		EvilBytes    []byte
	}
	if err := s.Decode(&extra); err != nil {
		return err
	}
	fce.Seal, fce.CurrentBlock = extra.Seal, extra.CurrentBlock

	if len(extra.EvilBytes) > 0 {
		var header types.Header
		if err := rlp.DecodeBytes(extra.EvilBytes, &header); err != nil {
			return err
		}
		fce.EvilHeader = &header
	}
	return nil
}

func ExtractFConExtra(header *types.Header) (*FConExtra, error) {
	if len(header.Extra) < ExtraVanity {
		return nil, errInvalidHeaderExtra
	}
	var extra FConExtra
	if err := rlp.DecodeBytes(header.Extra[ExtraVanity:], &extra); err != nil {
		return nil, err
	}
	return &extra, nil
}

type FConsensus struct {
	db evrdb.Database

	signature *lru.ARCCache

	proposals map[common.Address]bool
	signer    common.Address
	signFn    SignerFn
	lock      sync.RWMutex
}

func New(db evrdb.Database) *FConsensus {
	signatures, _ := lru.NewARC(inmemorySignatures)
	return &FConsensus{db: db, signature: signatures}
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

	return fc.verifySeal(chain, header, parents)
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
	authorizedSigner, err := fc.GetAuthorizedSinger()
	if err != nil {
		return err
	}
	signer, err := ecrecover(header, fc.signature)
	if err != nil {
		return err
	}
	if authorizedSigner != signer {
		return errUnauthorizedSigner
	}
	return nil
}

func (fc *FConsensus) Prepare(chain consensus.FullChainReader, header *types.Header) error {
	panic("implement me Prepare")
}

func (fc *FConsensus) Finalize(chain consensus.FullChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header) error {
	header.Root = state.IntermediateRoot(true)
	header.UncleHash = types.CalcUncleHash(nil)
	return nil
}

func (fc *FConsensus) FinalizeAndAssemble(chain consensus.FullChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	panic("implement me FinalizeAndAssemble")
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
	authorized, _ := fc.GetAuthorizedSinger()
	if signer != authorized {
		return errUnauthorizedSigner
	}
	signHash, err := signFn(accounts.Account{Address: signer}, accounts.MimetypeClique, FConRLP(header))
	if err != nil {
		return nil
	}

	fce, err := ExtractFConExtra(header)
	if err != nil {
		return err
	}

	fce.Seal = signHash
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
	//panic("implement me")
}

func ecrecover(header *types.Header, sigcache *lru.ARCCache) (common.Address, error) {
	hash := header.Hash()
	if address, known := sigcache.Get(hash); known {
		return address.(common.Address), nil
	}

	if len(header.Extra) < ExtraVanity {
		return common.Address{}, errInvalidHeaderExtra
	}
	fce, err := ExtractFConExtra(header)
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
	copy := *header
	if len(header.Extra) <= ExtraVanity {
		panic(errInvalidHeaderExtra)
	}
	fce, err := ExtractFConExtra(header)
	if err != nil {
		panic("can't encode: " + err.Error())
	}

	fce.Seal = nil
	fceBytes, err := rlp.EncodeToBytes(fce)
	if err != nil {
		panic("can't encode: " + err.Error())
	}
	copy.Extra = append(copy.Extra[:ExtraVanity], fceBytes...)
	err = rlp.Encode(w, []interface{}{
		copy.ParentHash,
		copy.UncleHash,
		copy.Coinbase,
		copy.Root,
		copy.TxHash,
		copy.ReceiptHash,
		copy.Bloom,
		copy.Difficulty,
		copy.Number,
		copy.GasLimit,
		copy.GasUsed,
		copy.Time,
		copy.Extra,
		copy.MixDigest,
		copy.Nonce,
	})

	if err != nil {
		panic("can't encode: " + err.Error())
	}

}
