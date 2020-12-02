package fconsensus

import (
	"encoding/hex"
	"fmt"
	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus/clique"
	"github.com/Evrynetlabs/evrynet-node/core"
	"github.com/Evrynetlabs/evrynet-node/core/rawdb"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/params"
	"github.com/Evrynetlabs/evrynet-node/rlp"
	"math/big"
	"testing"
)
var(
	extraVanity = 32 // Fixed number of extra-data prefix bytes reserved for signer vanity
	extraSeal   = 65
	randomNumberForBalance = 100000000
)

func TestRLPFconExtra(t *testing.T){

	var (
		db     = rawdb.NewMemoryDatabase()
		key, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		addr   = crypto.PubkeyToAddress(key.PublicKey)
		engine = clique.New(params.AllCliqueProtocolChanges.Clique, db)
	)
	config := params.ChainConfig{Clique: params.AllCliqueProtocolChanges.Clique}
	genspec := &core.Genesis{
		Config: &config,
		ExtraData: make([]byte, extraVanity+common.AddressLength+extraSeal),
		Alloc: map[common.Address]core.GenesisAccount{
			addr: {Balance: new(big.Int).Mul(big.NewInt(int64(randomNumberForBalance)), big.NewInt(params.GasPriceConfig))},
		},
	}
	copy(genspec.ExtraData[extraVanity:], addr[:])
	genesis := genspec.MustCommit(db)
	blocks, _ := core.GenerateChain(params.AllCliqueProtocolChanges, genesis, engine, db, 2, nil)

	fce := FConExtra{CurrentBlock: blocks[1].Hash(), EvilHeader: blocks[1].Header()}

	res, err := rlp.EncodeToBytes(&fce)
	if err != nil{
		t.Fatal(err)
	}else{
		fmt.Println(hex.EncodeToString(res))
	}

	var fceNew FConExtra
	err = rlp.DecodeBytes(res, &fceNew)
	if err != nil{
		t.Fatal(err)
	}
	fmt.Println(fceNew.EvilHeader.Hash().String(), fceNew.CurrentBlock.String())





}
