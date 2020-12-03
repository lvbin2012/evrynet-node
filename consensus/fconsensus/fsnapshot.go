package fconsensus

import (
	"bytes"
	"encoding/json"
	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/evrdb"
	"github.com/Evrynetlabs/evrynet-node/log"
	"github.com/Evrynetlabs/evrynet-node/params"
	lru "github.com/hashicorp/golang-lru"
	"sort"
	"time"
)

type FVote struct {
	Signer    common.Address `json:"signer"`
	Block     uint64         `json:"block"`
	Address   common.Address `json:"address"`
	Authorize bool           `json:"authorize"`
}

type FTally struct {
	Authorize bool `json:"authorize"`
	Votes     int  `json:"votes"`
}

type FSnapshot struct {
	config    *params.FConConfig
	signCache *lru.ARCCache

	Number  uint64                      `json:"number"`
	Hash    common.Hash                 `json:"hash"`
	Signers map[common.Address]struct{} `json:"signers"`
	Recents map[uint64]common.Address   `json:"recents"`
	FVotes  []*FVote                    `json:"f_votes"`
	FTallys map[common.Address]FTally   `json:"f_tally"`
}

type signersAscending []common.Address

func (s signersAscending) Len() int           { return len(s) }
func (s signersAscending) Less(i, j int) bool { return bytes.Compare(s[i][:], s[j][:]) < 0 }
func (s signersAscending) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func newFSnapshot(config *params.FConConfig, sigCache *lru.ARCCache, number uint64, hash common.Hash,
	signers []common.Address) *FSnapshot {
	fsnap := &FSnapshot{
		config:    config,
		signCache: sigCache,
		Number:    number,
		Hash:      hash,
		Signers:   make(map[common.Address]struct{}),
		Recents:   make(map[uint64]common.Address),
		FTallys:   make(map[common.Address]FTally),
	}
	for _, signer := range signers {
		fsnap.Signers[signer] = struct{}{}
	}
	return fsnap
}
func loadFSnapshot(config *params.FConConfig, sigCache *lru.ARCCache, db evrdb.Database, hash common.Hash) (*FSnapshot, error) {
	blob, err := db.Get(append([]byte("fconse-"), hash[:]...))
	if err != nil {
		return nil, err
	}
	fsnap := new(FSnapshot)
	if err := json.Unmarshal(blob, fsnap); err != nil {
		return nil, err
	}
	fsnap.config = config
	fsnap.signCache = sigCache
	return fsnap, nil
}

func (fs *FSnapshot) store(db evrdb.Database) error {
	blob, err := json.Marshal(fs)
	if err != nil {
		return err
	}
	return db.Put(append([]byte("fconse-"), fs.Hash[:]...), blob)
}

func (fs *FSnapshot) copy() *FSnapshot {
	cpy := &FSnapshot{
		config:    fs.config,
		signCache: fs.signCache,
		Number:    fs.Number,
		Hash:      fs.Hash,
		Signers:   make(map[common.Address]struct{}),
		Recents:   make(map[uint64]common.Address),
		FVotes:    make([]*FVote, len(fs.FVotes)),
		FTallys:   make(map[common.Address]FTally),
	}

	for signer := range fs.Signers {
		cpy.Signers[signer] = struct{}{}
	}

	for block, signer := range fs.Recents {
		cpy.Recents[block] = signer
	}
	for address, ftally := range fs.FTallys {
		cpy.FTallys[address] = ftally
	}
	copy(cpy.FVotes, fs.FVotes)
	return cpy
}

func (fs *FSnapshot) validVate(address common.Address, authorize bool) bool {
	_, signer := fs.Signers[address]
	return (signer && !authorize) || (!signer && authorize)
}

func (fs *FSnapshot) cast(address common.Address, authorize bool) bool {
	if !fs.validVate(address, authorize) {
		return false
	}
	if old, ok := fs.FTallys[address]; ok {
		if ok {
			old.Votes++
			fs.FTallys[address] = old
		} else {
			fs.FTallys[address] = FTally{Authorize: authorize, Votes: 1}
		}
	}
	return true
}

func (fs *FSnapshot) apply(headers []*types.Header) (*FSnapshot, error) {
	if len(headers) == 0 {
		return fs, nil
	}
	for i := 0; i < len(headers)-1; i++ {
		if headers[i+1].Number.Uint64() != headers[i].Number.Uint64()+1 {
			return nil, errInvalidVotingChain
		}
	}
	if headers[0].Number.Uint64() != fs.Number+1 {
		return nil, errInvalidVotingChain
	}
	fsnap := fs.copy()

	var (
		start  = time.Now()
		logged = time.Now()
	)
	for i, header := range headers {
		number := header.Number.Uint64()
		if number%fs.config.Epoch == 0 {
			fsnap.FVotes = nil
			fsnap.FTallys = make(map[common.Address]FTally)
		}
		if limit := uint64(len(fsnap.Signers)/2 + 1); number >= limit {
			delete(fsnap.Recents, number-limit)
		}
		signer, err := ecrecover(header, fs.signCache)
		if err != nil {
			return nil, err
		}
		if _, ok := fsnap.Signers[signer]; !ok {
			return nil, errUnauthorizedSigner
		}
		for _, recent := range fsnap.Recents {
			if recent == signer {
				return nil, errRecentlySigned
			}
		}
		fsnap.Recents[number] = signer

		for i, vote := range fsnap.FVotes {
			if vote.Signer == signer && vote.Address == header.Coinbase {
				fsnap.uncast(vote.Address, vote.Authorize)
				fsnap.FVotes = append(fsnap.FVotes[:i], fsnap.FVotes[i+1:]...)
				break
			}
		}

		var authorize bool
		switch {
		case bytes.Equal(header.Nonce[:], nonceAuthVote):
			authorize = true
		case bytes.Equal(header.Nonce[:], nonceDropVote):
			authorize = false
		default:
			return nil, errInvalidVote
		}
		if fsnap.cast(header.Coinbase, authorize) {
			fsnap.FVotes = append(fsnap.FVotes, &FVote{
				Signer:    signer,
				Authorize: authorize,
				Address:   header.Coinbase,
				Block:     number,
			})
		}

		if ftally := fsnap.FTallys[header.Coinbase]; ftally.Votes > len(fsnap.Signers)/2 {
			if ftally.Authorize {
				fsnap.Signers[header.Coinbase] = struct{}{}
			} else {
				delete(fsnap.Signers, header.Coinbase)
				if limit := uint64(len(fsnap.Signers)/2 + 1); number >= limit {
					delete(fsnap.Recents, number-limit)
				}
				for i := 0; i < len(fsnap.FVotes); i++ {
					if fsnap.FVotes[i].Signer == header.Coinbase {
						fsnap.uncast(fsnap.FVotes[i].Signer, fsnap.FVotes[i].Authorize)
						fsnap.FVotes = append(fsnap.FVotes[:i], fsnap.FVotes[i+1:]...)
						i--
					}
				}
			}

			for i := 0; i < len(fsnap.FVotes); i++ {
				if fsnap.FVotes[i].Address == header.Coinbase {
					fsnap.FVotes = append(fsnap.FVotes[:i], fsnap.FVotes[i+1:]...)
					i--
				}
			}
			delete(fsnap.FTallys, header.Coinbase)
		}

		if time.Since(logged) > 8*time.Second {
			log.Info("Reconstructing voting history", "processed", i, "total", len(headers), "elapsed",
				common.PrettyDuration(time.Since(start)))
			logged = time.Now()
		}
	}
	fsnap.Number += uint64(len(headers))
	fsnap.Hash = headers[len(headers)-1].Hash()
	return fsnap, nil
}

func (fs *FSnapshot) uncast(address common.Address, authorize bool) bool {
	ftally, ok := fs.FTallys[address]
	if !ok {
		return false
	}
	if ftally.Authorize != authorize {
		return false
	}
	if ftally.Votes > 1 {
		ftally.Votes--
		fs.FTallys[address] = ftally
	} else {
		delete(fs.FTallys, address)
	}
	return true
}

func (fs *FSnapshot) signers() []common.Address {
	sigs := make([]common.Address, 0, len(fs.Signers))
	for sig := range fs.Signers {
		sigs = append(sigs, sig)
	}
	sort.Sort(signersAscending(sigs))
	return sigs
}

func (fs *FSnapshot) inturn(number uint64, signer common.Address) bool {
	signers, offset := fs.signers(), 0
	for offset < len(signers) && signers[offset] != signer {
		offset++
	}
	return (number % uint64(len(signers))) == uint64(offset)
}
