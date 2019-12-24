// Copyright 2018 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// This file contains a miner stress test based on the Clique consensus engine.
package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/evrynet-official/evrynet-client"
	"github.com/evrynet-official/evrynet-client/accounts/keystore"
	"github.com/evrynet-official/evrynet-client/common"
	"github.com/evrynet-official/evrynet-client/common/fdlimit"
	"github.com/evrynet-official/evrynet-client/common/hexutil"
	"github.com/evrynet-official/evrynet-client/core"
	"github.com/evrynet-official/evrynet-client/core/types"
	"github.com/evrynet-official/evrynet-client/crypto"
	"github.com/evrynet-official/evrynet-client/eth"
	"github.com/evrynet-official/evrynet-client/eth/downloader"
	"github.com/evrynet-official/evrynet-client/ethclient"
	"github.com/evrynet-official/evrynet-client/log"
	"github.com/evrynet-official/evrynet-client/miner"
	"github.com/evrynet-official/evrynet-client/node"
	"github.com/evrynet-official/evrynet-client/p2p"
	"github.com/evrynet-official/evrynet-client/p2p/enode"
	"github.com/evrynet-official/evrynet-client/params"
)

func main() {
	var (
		err          error
		genesisFile  = "./genesis_testnet.json"
		configFile   = "stress_config.json"
		sendSCTxFlag = true                        //For sending SC Tx
		rpcEndpoint  = "http://34.206.41.188:8545" //For sending SC Tx
	)
	if len(os.Args) == 3 {
		fmt.Println("overwrite default config")
		genesisFile = os.Args[1]
		configFile = os.Args[2]
	}
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))
	fdlimit.Raise(2048)

	enodes, faucets := parseTestConfig(configFile)

	nodePriKey, _ := crypto.GenerateKey()
	// Create a Clique network based off of the Rinkeby config
	genesis, err := makeGenesis(genesisFile)
	if err != nil {
		panic(err)
	}

	//make node
	node, err := makeNode(genesis)
	if err != nil {
		panic(err)
	}
	defer node.Close()

	for node.Server().NodeInfo().Ports.Listener == 0 {
		time.Sleep(250 * time.Millisecond)
	}
	// Connect the node to al the previous ones
	for _, n := range enodes {
		node.Server().AddPeer(n)
	}

	// Inject the signer key and start sealing with it
	store := node.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
	signer, err := store.ImportECDSA(nodePriKey, "")
	if err != nil {
		panic(err)
	}
	if err := store.Unlock(signer, ""); err != nil {
		panic(err)
	}

	// wait until node is synced
	time.Sleep(3 * time.Second)
	var ethereum *eth.Ethereum
	if err := node.Service(&ethereum); err != nil {
		panic(err)
	}
	bc := ethereum.BlockChain()
	for !ethereum.Synced() {
		log.Warn("node is not synced, sleeping", "current_block", bc.CurrentHeader().Number)
		time.Sleep(3 * time.Second)
	}

	nonces := make([]uint64, len(faucets))
	// wait for nonce is not change
	for {
		for i, faucet := range faucets {
			addr := crypto.PubkeyToAddress(*(faucet.Public().(*ecdsa.PublicKey)))
			log.Info("faucet addr", "addr", addr)
			nonces[i] = ethereum.TxPool().State().GetNonce(addr)
		}
		time.Sleep(time.Second * 10)
		var diff = false
		for i, faucet := range faucets {
			addr := crypto.PubkeyToAddress(*(faucet.Public().(*ecdsa.PublicKey)))
			tmp := ethereum.TxPool().State().GetNonce(addr)
			if tmp != nonces[i] {
				diff = true
			}
		}
		if !diff {
			break
		}
	}

	contractAddr := &common.Address{}
	if sendSCTxFlag {
		if contractAddr, err = prepareNewContract(rpcEndpoint, faucets[0], nonces[0]); err != nil {
			panic(err)
		}
		nonces[0] = ethereum.TxPool().State().GetNonce(crypto.PubkeyToAddress(faucets[0].PublicKey))
	}

	maxBlockNumber := ethereum.BlockChain().CurrentHeader().Number.Uint64()
	numTxs := 0
	start := time.Now()
	preNumTxs := 0
	prevTime := time.Now()
	// Start injecting transactions from the faucet like crazy
	go func() {
		for {
			currentBlk := bc.CurrentHeader().Number.Uint64()
			for currentBlk > maxBlockNumber {
				maxBlockNumber++
				numTxs += len(bc.GetBlockByNumber(maxBlockNumber).Body().Transactions)
				log.Info("new_block", "txs", len(bc.GetBlockByNumber(maxBlockNumber).Body().Transactions), "number", maxBlockNumber)
			}
			log.Warn("num tx info", "usingSC", sendSCTxFlag, "txs", numTxs, "duration", time.Since(start),
				"avg_tps", float64(numTxs)/time.Since(start).Seconds(), "current_tps", float64(numTxs-preNumTxs)/time.Since(prevTime).Seconds(),
				"block", currentBlk)

			preNumTxs = numTxs
			prevTime = time.Now()
			time.Sleep(2 * time.Second)
		}
	}()

	for {
		var txs types.Transactions
		// Create a batch of transaction and inject into the pool
		// Note: if we add a single transaction one by one, the queue for broadcast txs might be full
		for i := 0; i < 1024; i++ {
			var (
				tx  *types.Transaction
				err error
			)
			index := rand.Intn(len(faucets))
			if sendSCTxFlag {
				tx, err = types.SignTx(
					types.NewTransaction(nonces[index], *contractAddr, big.NewInt(0),
						40000, big.NewInt(params.GasPriceConfig),
						[]byte("0x3fb5c1cb0000000000000000000000000000000000000000000000000000000000000002")),
					types.HomesteadSigner{},
					faucets[index],
				)
			} else {
				tx, err = types.SignTx(
					types.NewTransaction(nonces[index], crypto.PubkeyToAddress(faucets[index].PublicKey), new(big.Int),
						21000, big.NewInt(params.GasPriceConfig), nil),
					types.HomesteadSigner{},
					faucets[index],
				)
			}

			if err != nil {
				panic(err)
			}
			nonces[index]++
			txs = append(txs, tx)
		}
		errs := ethereum.TxPool().AddLocals(txs)
		for _, err := range errs {
			if err != nil {
				panic(err)
			}
		}

		// Wait if we're too saturated
		rebroardcast := false
	waitLoop:
		for epoch := 0; ; epoch++ {
			pend, _ := ethereum.TxPool().Stats()
			switch {
			case pend < 40960:
				break waitLoop
			default:
				if !rebroardcast {
					forceBroadcastPendingTxs(ethereum)
					rebroardcast = true
				}
				log.Info("tx pool is full, sleeping", "pending", pend)
				time.Sleep(time.Second)
			}
		}
	}
}

func forceBroadcastPendingTxs(ethereum *eth.Ethereum) {
	// force rebroadcast
	var txs types.Transactions
	pendings, err := ethereum.TxPool().Pending()
	if err != nil {
		panic(err)
	}
	for _, pendingTxs := range pendings {
		ethereum.TxPool().State()
		txs = append(txs, pendingTxs...)
	}
	go func() {
		ethereum.GetPm().ReBroadcastTxs(txs)
	}()
}

type stressConfig struct {
	EnodeStrings  []string `json:"enodes"`
	FaucetStrings []string `json:"faucets"`
}

func parseTestConfig(fileName string) ([]*enode.Node, []*ecdsa.PrivateKey) {
	var cfg stressConfig
	f, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&cfg); err != nil {
		panic(err)
	}

	var (
		enodes  []*enode.Node
		faucets []*ecdsa.PrivateKey
	)
	for _, enodeS := range cfg.EnodeStrings {
		enodes = append(enodes, enode.MustParse(enodeS))
	}

	for _, faucetS := range cfg.FaucetStrings {
		faucetPriKey, err := crypto.HexToECDSA(faucetS)
		if err != nil {
			panic(err)
		}
		faucets = append(faucets, faucetPriKey)
	}
	return enodes, faucets
}

// makeGenesis creates a custom Clique genesis block based on some pre-defined
// signer and faucet accounts.
func makeGenesis(fileName string) (*core.Genesis, error) {
	// Create a Clique network based off of the Rinkeby config
	// Read file genesis generated from pupeth
	genesisFile, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	config := &core.Genesis{}
	err = json.Unmarshal(genesisFile, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func makeNode(genesis *core.Genesis) (*node.Node, error) {
	// Define the basic configurations for the Ethereum node
	datadir := "./test_data"

	config := &node.Config{
		Name:    "geth",
		Version: params.Version,
		DataDir: datadir,

		P2P: p2p.Config{
			ListenAddr:  "0.0.0.0:0",
			NoDiscovery: true,
			MaxPeers:    25,
		},
		NoUSB:    true,
		HTTPHost: "127.0.0.1",
		HTTPPort: 22001,
		HTTPModules: []string{"admin", "db", "eth", "debug", "miner", "net", "shh", "txpool",
			"personal", "web3", "tendermint"},
	}
	// Start the node and configure a full Ethereum node on it
	stack, err := node.New(config)
	if err != nil {
		return nil, err
	}
	if err := stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
		return eth.New(ctx, &eth.Config{
			Genesis:         genesis,
			NetworkId:       genesis.Config.ChainID.Uint64(),
			GasPrice:        big.NewInt(params.GasPriceConfig),
			SyncMode:        downloader.FullSync,
			DatabaseCache:   256,
			DatabaseHandles: 256,
			TxPool: core.TxPoolConfig{
				Journal:   "transactions.rlp",
				Rejournal: time.Hour,

				PriceLimit: 1,
				PriceBump:  10,

				AccountSlots: 16,
				GlobalSlots:  40960,
				AccountQueue: 64,
				GlobalQueue:  10240,

				Lifetime: 3 * time.Hour,
			},
			GPO: eth.DefaultConfig.GPO,
			Miner: miner.Config{
				GasFloor: genesis.GasLimit * 9 / 10,
				GasCeil:  genesis.GasLimit * 11 / 10,
				GasPrice: genesis.Config.GasPrice,
				Recommit: time.Second,
			},
		})
	}); err != nil {
		return nil, err
	}
	// Start the node and return if successful
	return stack, stack.Start()
}

func prepareNewContract(rpcEndpoint string, acc *ecdsa.PrivateKey, nonce uint64) (*common.Address, error) {
	log.Info("Creating Smart Contract ...")

	evrClient, err := ethclient.Dial(rpcEndpoint)
	if err != nil {
		return nil, err
	}

	// payload to create a smart contract
	payload := "0x608060405260d0806100126000396000f30060806040526004361060525763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416633fb5c1cb811460545780638381f58a14605d578063f2c9ecd8146081575b005b60526004356093565b348015606857600080fd5b50606f6098565b60408051918252519081900360200190f35b348015608c57600080fd5b50606f609e565b600055565b60005481565b600054905600a165627a7a723058209573e4f95d10c1e123e905d720655593ca5220830db660f0641f3175c1cdb86e0029"
	payLoadBytes, err := hexutil.Decode(payload)
	if err != nil {
		return nil, err
	}

	accAddr := crypto.PubkeyToAddress(acc.PublicKey)
	msg := evrynet.CallMsg{
		From:  accAddr,
		Value: common.Big0,
		Data:  payLoadBytes,
	}
	estGas, err := evrClient.EstimateGas(context.Background(), msg)
	if err != nil {
		return nil, err
	}

	tx := types.NewContractCreation(nonce, big.NewInt(0), estGas, big.NewInt(params.GasPriceConfig), payLoadBytes)
	tx, err = types.SignTx(tx, types.HomesteadSigner{}, acc)

	err = evrClient.SendTransaction(context.Background(), tx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create SC from %s", accAddr.Hex())
	}

	// Wait to get SC address
	for i := 0; i < 10; i++ {
		var receipt *types.Receipt
		receipt, err = evrClient.TransactionReceipt(context.Background(), tx.Hash())
		if err == nil && receipt.Status == uint64(1) {
			log.Info("Creating Smart Contract successfully!")
			return &receipt.ContractAddress, nil
		}
		time.Sleep(1 * time.Second)
	}
	return nil, errors.New("Can not get SC address")
}
