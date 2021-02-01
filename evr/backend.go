// Copyright 2014 The evrynet-node Authors
// This file is part of the evrynet-node library.
//
// The evrynet-node library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The evrynet-node library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the evrynet-node library. If not, see <http://www.gnu.org/licenses/>.

// Package evr implements the Evrynet protocol.
package evr

import (
	"errors"
	"fmt"
	"github.com/Evrynetlabs/evrynet-node/consensus/fconsensus"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/Evrynetlabs/evrynet-node/accounts"
	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/common/hexutil"
	"github.com/Evrynetlabs/evrynet-node/consensus"
	"github.com/Evrynetlabs/evrynet-node/consensus/clique"
	"github.com/Evrynetlabs/evrynet-node/consensus/ethash"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint"
	tendermintBackend "github.com/Evrynetlabs/evrynet-node/consensus/tendermint/backend"
	"github.com/Evrynetlabs/evrynet-node/core"
	"github.com/Evrynetlabs/evrynet-node/core/bloombits"
	"github.com/Evrynetlabs/evrynet-node/core/rawdb"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/core/vm"
	"github.com/Evrynetlabs/evrynet-node/event"
	"github.com/Evrynetlabs/evrynet-node/evr/downloader"
	"github.com/Evrynetlabs/evrynet-node/evr/filters"
	"github.com/Evrynetlabs/evrynet-node/evr/gasprice"
	"github.com/Evrynetlabs/evrynet-node/evrdb"
	"github.com/Evrynetlabs/evrynet-node/internal/evrapi"
	"github.com/Evrynetlabs/evrynet-node/log"
	"github.com/Evrynetlabs/evrynet-node/miner"
	"github.com/Evrynetlabs/evrynet-node/node"
	"github.com/Evrynetlabs/evrynet-node/p2p"
	"github.com/Evrynetlabs/evrynet-node/params"
	"github.com/Evrynetlabs/evrynet-node/rlp"
	"github.com/Evrynetlabs/evrynet-node/rpc"
)

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	APIs() []rpc.API
	Protocols() []p2p.Protocol
	SetBloomBitsIndexer(bbIndexer *core.ChainIndexer)
}

// Evrynet implements the Evrynet full node service.
type Evrynet struct {
	config *Config

	// Channel for shutting down the service
	shutdownChan chan bool // Channel for shutting down the Evrynet

	// Handlers
	txPool      *core.TxPool
	blockchain  *core.BlockChain
	fBlockchain *core.BlockChain
	//fb              *FBManager
	protocolManager *ProtocolManager
	lesServer       LesServer

	// DB interfaces
	chainDb evrdb.Database // Block chain database

	eventMux *event.TypeMux
	engine   consensus.Engine
	fEngine  consensus.Engine

	accountManager *accounts.Manager

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	APIBackend *EvrAPIBackend

	miner     *miner.Miner
	gasPrice  *big.Int
	etherbase common.Address

	networkID     uint64
	netRPCService *evrapi.PublicNetAPI

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and etherbase)
}

func (s *Evrynet) AddLesServer(ls LesServer) {
	s.lesServer = ls
	ls.SetBloomBitsIndexer(s.bloomIndexer)
}

// New creates a new Evrynet object (including the
// initialisation of the common Evrynet object)
func New(ctx *node.ServiceContext, config *Config) (*Evrynet, error) {
	// Ensure configuration values are compatible and sane
	if config.SyncMode == downloader.LightSync {
		return nil, errors.New("can't run evr.Evrynet in light sync mode, use les.LightEvrynet")
	}
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}
	if config.NoPruning && config.TrieDirtyCache > 0 {
		config.TrieCleanCache += config.TrieDirtyCache
		config.TrieDirtyCache = 0
	}
	log.Info("Allocated trie memory caches", "clean", common.StorageSize(config.TrieCleanCache)*1024*1024, "dirty", common.StorageSize(config.TrieDirtyCache)*1024*1024)

	// Assemble the Evrynet object
	chainDb, err := ctx.OpenDatabaseWithFreezer("chaindata", config.DatabaseCache, config.DatabaseHandles, config.DatabaseFreezer, "evr/db/chaindata/")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis, false)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	fchainConfig, fgenesisHash, fgenesisErr := core.SetupGenesisBlockWithOverride(chainDb, nil, true)
	if _, ok := fgenesisErr.(*params.ConfigCompatError); fgenesisErr != nil && !ok {
		return nil, fgenesisErr
	}

	log.Info("Initialised chain configuration", "config", chainConfig)

	evr := &Evrynet{
		config:         config,
		chainDb:        chainDb,
		eventMux:       ctx.EventMux,
		accountManager: ctx.AccountManager,
		engine:         CreateConsensusEngine(ctx, chainConfig, config, config.Miner.Notify, config.Miner.Noverify, chainDb),
		shutdownChan:   make(chan bool),
		networkID:      config.NetworkId,
		gasPrice:       chainConfig.GasPrice,
		etherbase:      config.Miner.Etherbase,
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   NewBloomIndexer(chainDb, params.BloomBitsBlocks, params.BloomConfirms, chainConfig.IsFinalChain),
	}

	bcVersion := rawdb.ReadDatabaseVersion(chainDb)
	var dbVer = "<nil>"
	if bcVersion != nil {
		dbVer = fmt.Sprintf("%d", *bcVersion)
	}
	log.Info("Initialising Evrynet protocol", "versions", ProtocolVersions, "network", config.NetworkId, "dbversion", dbVer)

	if !config.SkipBcVersionCheck {
		if bcVersion != nil && *bcVersion > core.BlockChainVersion {
			return nil, fmt.Errorf("database version is v%d, Gev %s only supports v%d", *bcVersion, params.VersionWithMeta, core.BlockChainVersion)
		} else if bcVersion == nil || *bcVersion < core.BlockChainVersion {
			log.Warn("Upgrade blockchain database version", "from", dbVer, "to", core.BlockChainVersion)
			rawdb.WriteDatabaseVersion(chainDb, core.BlockChainVersion)
		}
	}
	var (
		vmConfig = vm.Config{
			EnablePreimageRecording: config.EnablePreimageRecording,
			EWASMInterpreter:        config.EWASMInterpreter,
			EVMInterpreter:          config.EVMInterpreter,
		}
		cacheConfig = &core.CacheConfig{
			TrieCleanLimit:      config.TrieCleanCache,
			TrieCleanNoPrefetch: config.NoPrefetch,
			TrieDirtyLimit:      config.TrieDirtyCache,
			TrieDirtyDisabled:   config.NoPruning,
			TrieTimeLimit:       config.TrieTimeout,
		}
	)
	evr.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, chainConfig, evr.engine, vmConfig, evr.shouldPreserve)
	if err != nil {
		return nil, err
	}

	conf := &params.FConConfig{}
	if fchainConfig.Clique != nil {
		conf.Epoch = fchainConfig.Clique.Epoch
		conf.Period = fchainConfig.Clique.Period
	}
	fEngin := fconsensus.New(conf, chainDb)
	evr.fEngine = fEngin
	//coinbase, _ := evr.Etherbase()
	//wallet, err := evr.accountManager.Find(accounts.Account{Address: coinbase})
	//if wallet == nil || err != nil {
	//	log.Error("Etherbase account unavailable locally", "err", err)
	//	return nil, fmt.Errorf("signer missing: %v", err)
	//}
	//fEngin.Authorize(coinbase, wallet.SignData)

	evr.fBlockchain, err = core.NewBlockChain(chainDb, cacheConfig, fchainConfig, fEngin, vmConfig, evr.shouldPreserve)
	evr.blockchain.SubscribeAssistChainEvent(evr.blockchain)

	if err != nil {
		return nil, err
	}
	//evr.fb = NewFBManager(evr.blockchain, evr.fBlockchain, fEngin, evr.EventMux())
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		evr.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}

	if compat, ok := fgenesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		evr.fBlockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, fgenesisHash, fchainConfig)
	}

	evr.bloomIndexer.Start(evr.blockchain)

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = ctx.ResolvePath(config.TxPool.Journal)
	}
	evr.txPool = core.NewTxPool(config.TxPool, chainConfig, evr.blockchain)

	// Permit the downloader to use the trie cache allowance during fast sync
	cacheLimit := cacheConfig.TrieCleanLimit + cacheConfig.TrieDirtyLimit
	if evr.protocolManager, err = NewProtocolManager(chainConfig, fchainConfig, config.SyncMode, config.NetworkId,
		evr.eventMux, evr.txPool, evr.engine, fEngin, evr.blockchain, evr.fBlockchain, chainDb, cacheLimit,
		config.Whitelist); err != nil {
		return nil, err
	}
	evr.miner = miner.New(evr, &config.Miner, chainConfig, fchainConfig, evr.EventMux(), evr.engine, evr.fEngine, evr.isLocalBlock)
	evr.miner.SetExtra(makeExtraData(config.Miner.ExtraData))

	evr.APIBackend = &EvrAPIBackend{ctx.ExtRPCEnabled(), evr, nil}
	gpoParams := config.GPO
	evr.APIBackend.gpo = gasprice.NewOracle(evr.APIBackend, gpoParams)

	return evr, nil
}

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]interface{}{
			uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
			"gev",
			runtime.Version(),
			runtime.GOOS,
		})
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.MaximumExtraDataSize)
		extra = nil
	}
	return extra
}

// CreateConsensusEngine creates the required type of consensus engine instance for an Evrynet service
func CreateConsensusEngine(ctx *node.ServiceContext, chainConfig *params.ChainConfig, config *Config, notify []string, noverify bool, db evrdb.Database) consensus.Engine {
	// If proof-of-authority is requested, set it up
	if chainConfig.Clique != nil {
		return clique.New(chainConfig.Clique, db)
	}
	// If Tendermint is requested, set it up
	if chainConfig.Tendermint != nil {
		config.Tendermint.ProposerPolicy = tendermint.ProposerPolicy(chainConfig.Tendermint.ProposerPolicy)
		config.Tendermint.Epoch = chainConfig.Tendermint.Epoch
		config.Tendermint.StakingSCAddress = chainConfig.Tendermint.StakingSCAddress
		config.Tendermint.FixedValidators = chainConfig.Tendermint.FixedValidators
		config.Tendermint.BlockReward = chainConfig.Tendermint.BlockReward
		log.Info("Create Tendermint consensus engine")
		return tendermintBackend.New(&config.Tendermint, ctx.NodeKey())
	}

	// Otherwise assume proof-of-work
	switch config.Ethash.PowMode {
	case ethash.ModeFake:
		log.Warn("Ethash used in fake mode")
		return ethash.NewFaker()
	case ethash.ModeTest:
		log.Warn("Ethash used in test mode")
		return ethash.NewTester(nil, noverify)
	case ethash.ModeShared:
		log.Warn("Ethash used in shared mode")
		return ethash.NewShared()
	default:
		engine := ethash.New(ethash.Config{
			CacheDir:       ctx.ResolvePath(config.Ethash.CacheDir),
			CachesInMem:    config.Ethash.CachesInMem,
			CachesOnDisk:   config.Ethash.CachesOnDisk,
			DatasetDir:     config.Ethash.DatasetDir,
			DatasetsInMem:  config.Ethash.DatasetsInMem,
			DatasetsOnDisk: config.Ethash.DatasetsOnDisk,
		}, notify, noverify)
		engine.SetThreads(-1) // Disable CPU mining
		return engine
	}
}

// APIs return the collection of RPC services the evrynetNode package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *Evrynet) APIs() []rpc.API {
	apis := evrapi.GetAPIs(s.APIBackend)

	// Append any APIs exposed explicitly by the les server
	if s.lesServer != nil {
		apis = append(apis, s.lesServer.APIs()...)
	}
	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, s.engine.APIs(s.BlockChain())...)

	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "evr",
			Version:   "1.0",
			Service:   NewPublicEvrynetAPI(s),
			Public:    true,
		},
		{
			Namespace: "evr",
			Version:   "1.0",
			Service:   NewPublicMinerAPI(s),
			Public:    true,
		},
		{
			Namespace: "evr",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		},
		{
			Namespace: "miner",
			Version:   "1.0",
			Service:   NewPrivateMinerAPI(s),
			Public:    false,
		},
		{
			Namespace: "evr",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.APIBackend, false),
			Public:    true,
		}, {
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(s),
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(s),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPrivateDebugAPI(s),
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)
}

func (s *Evrynet) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *Evrynet) Etherbase() (eb common.Address, err error) {
	s.lock.RLock()
	etherbase := s.etherbase
	s.lock.RUnlock()

	if tendermint, ok := s.engine.(consensus.Tendermint); ok {
		eb = tendermint.Address()
		if eb == (common.Address{}) {
			return eb, errors.New("etherbase is missing from tendermint")
		}
		return eb, nil
	}

	if etherbase != (common.Address{}) {
		return etherbase, nil
	}
	if wallets := s.AccountManager().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			etherbase := accounts[0].Address

			s.lock.Lock()
			s.etherbase = etherbase
			s.lock.Unlock()

			log.Info("Etherbase automatically configured", "address", etherbase)
			return etherbase, nil
		}
	}
	return common.Address{}, fmt.Errorf("etherbase must be explicitly specified")
}

// isLocalBlock checks whether the specified block is mined
// by local miner accounts.
//
// We regard two types of accounts as local miner account: etherbase
// and accounts specified via `txpool.locals` flag.
func (s *Evrynet) isLocalBlock(block *types.Block, isFinalChain bool) bool {
	engine := s.engine
	if isFinalChain {
		engine = s.fEngine
	}
	author, err := engine.Author(block.Header())
	if err != nil {
		log.Warn("Failed to retrieve block author", "number", block.NumberU64(), "hash", block.Hash(), "err", err)
		return false
	}
	// Check whether the given address is etherbase.
	s.lock.RLock()
	etherbase := s.etherbase
	s.lock.RUnlock()
	if author == etherbase {
		return true
	}
	// Check whether the given address is specified by `txpool.local`
	// CLI flag.
	for _, account := range s.config.TxPool.Locals {
		if account == author {
			return true
		}
	}
	return false
}

// shouldPreserve checks whether we should preserve the given block
// during the chain reorg depending on whether the author of block
// is a local account.
func (s *Evrynet) shouldPreserve(block *types.Block, isFinalChain bool) bool {
	// The reason we need to disable the self-reorg preserving for clique
	// is it can be probable to introduce a deadlock.
	//
	// e.g. If there are 7 available signers
	//
	// r1   A
	// r2     B
	// r3       C
	// r4         D
	// r5   A      [X] F G
	// r6    [X]
	//
	// In the round5, the inturn signer E is offline, so the worst case
	// is A, F and G sign the block of round5 and reject the block of opponents
	// and in the round6, the last available signer B is offline, the whole
	// network is stuck.
	if _, ok := s.engine.(*clique.Clique); ok {
		return false
	}
	if _, ok := s.engine.(*fconsensus.FConsensus); ok {
		return false
	}
	return s.isLocalBlock(block, isFinalChain)
}

// SetEtherbase sets the mining reward address.
func (s *Evrynet) SetEtherbase(etherbase common.Address) {
	s.lock.Lock()
	s.etherbase = etherbase
	s.lock.Unlock()

	s.miner.SetEtherbase(etherbase)
}

// StartMining starts the miner with the given number of CPU threads. If mining
// is already running, this method adjust the number of threads allowed to use
// and updates the minimum price required by the transaction pool.
func (s *Evrynet) StartMining(threads int) error {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}
	if th, ok := s.engine.(threaded); ok {
		log.Info("Updated mining threads", "threads", threads)
		if threads == 0 {
			threads = -1 // Disable the miner from within
		}
		th.SetThreads(threads)
	}
	// If the miner was not running, initialize it
	if !s.IsMining() {
		// Propagate the initial price point to the transaction pool
		s.lock.RLock()
		price := s.gasPrice
		s.lock.RUnlock()
		s.txPool.SetGasPrice(price)

		// Configure the local mining address
		eb, err := s.Etherbase()
		if err != nil {
			log.Error("Cannot start mining without etherbase", "err", err)
			return fmt.Errorf("etherbase missing: %v", err)
		}
		if clique, ok := s.engine.(*clique.Clique); ok {
			wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
			if wallet == nil || err != nil {
				log.Error("Etherbase account unavailable locally", "err", err)
				return fmt.Errorf("signer missing: %v", err)
			}
			clique.Authorize(eb, wallet.SignData)
		}

		// If mining is started, we can disable the transaction rejection mechanism
		// introduced to speed sync times.
		atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)

		go s.miner.Start(eb)
	}
	return nil
}

func (s *Evrynet) StartFMining() error {
	// If the miner was not running, initialize it
	if !s.IsFMining() {
		// Configure the local mining address
		eb, err := s.Etherbase()
		if err != nil {
			log.Error("Cannot start mining without etherbase", "err", err)
			return fmt.Errorf("etherbase missing: %v", err)
		}

		if fconse, ok := s.fEngine.(*fconsensus.FConsensus); ok {
			wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
			if wallet == nil || err != nil {
				log.Error("Etherbase account unavailable locally", "err", err)
				return fmt.Errorf("signer missing: %v", err)
			}
			fconse.Authorize(eb, wallet.SignData)
		}
		go s.miner.FStart(eb)
	}
	return nil
}

// StopMining terminates the miner, both at the consensus engine level as well as
// at the block creation level.
func (s *Evrynet) StopMining() {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}
	if th, ok := s.engine.(threaded); ok {
		th.SetThreads(-1)
	}
	// Stop the block creating itself
	s.miner.Stop()
}

func (s *Evrynet) StopFMining() {
	// Stop the block creating itself
	s.miner.FStop()
}

func (s *Evrynet) IsMining() bool      { return s.miner.Mining() }
func (s *Evrynet) IsFMining() bool     { return s.miner.FMining() }
func (s *Evrynet) Miner() *miner.Miner { return s.miner }

func (s *Evrynet) AccountManager() *accounts.Manager  { return s.accountManager }
func (s *Evrynet) BlockChain() *core.BlockChain       { return s.blockchain }
func (s *Evrynet) FBlockChain() *core.BlockChain      { return s.fBlockchain }
func (s *Evrynet) TxPool() *core.TxPool               { return s.txPool }
func (s *Evrynet) EventMux() *event.TypeMux           { return s.eventMux }
func (s *Evrynet) Engine() consensus.Engine           { return s.engine }
func (s *Evrynet) ChainDb() evrdb.Database            { return s.chainDb }
func (s *Evrynet) IsListening() bool                  { return true } // Always listening
func (s *Evrynet) EthVersion() int                    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Evrynet) NetVersion() uint64                 { return s.networkID }
func (s *Evrynet) GasPrice() *big.Int                 { return s.gasPrice }
func (s *Evrynet) Downloader() *downloader.Downloader { return s.protocolManager.downloader }
func (s *Evrynet) Synced() bool                       { return atomic.LoadUint32(&s.protocolManager.acceptTxs) == 1 }
func (s *Evrynet) ArchiveMode() bool                  { return s.config.NoPruning }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *Evrynet) Protocols() []p2p.Protocol {
	if s.lesServer == nil {
		return s.protocolManager.SubProtocols
	}
	return append(s.protocolManager.SubProtocols, s.lesServer.Protocols()...)
}

// Start implements node.Service, starting all internal goroutines needed by the
// Evrynet protocol implementation.
func (s *Evrynet) Start(srvr *p2p.Server) error {
	// Start the bloom bits servicing goroutines
	s.startBloomHandlers(params.BloomBitsBlocks)

	// Start the RPC service
	s.netRPCService = evrapi.NewPublicNetAPI(srvr, s.NetVersion())

	// Figure out a max peers count based on the server limits
	maxPeers := srvr.MaxPeers
	if s.config.LightServ > 0 {
		if s.config.LightPeers >= srvr.MaxPeers {
			return fmt.Errorf("invalid Peer config: light Peer count (%d) >= total Peer count (%d)", s.config.LightPeers, srvr.MaxPeers)
		}
		maxPeers -= s.config.LightPeers
	}
	// Start the networking layer and the light server if requested
	s.protocolManager.Start(maxPeers)
	if s.lesServer != nil {
		s.lesServer.Start(srvr)
	}
	//s.fb.Start()
	return nil
}

func (s *Evrynet) GetPm() *ProtocolManager {
	return s.protocolManager
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Evrynet protocol.
func (s *Evrynet) Stop() error {
	//s.fb.Stop()
	s.bloomIndexer.Close()
	s.fBlockchain.Stop()
	s.blockchain.Stop()
	s.engine.Close()
	s.protocolManager.Stop()
	if s.lesServer != nil {
		s.lesServer.Stop()
	}
	s.txPool.Stop()
	s.miner.Stop()
	s.eventMux.Stop()

	s.chainDb.Close()
	close(s.shutdownChan)
	return nil
}
