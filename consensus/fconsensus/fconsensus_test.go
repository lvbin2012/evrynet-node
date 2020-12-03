package fconsensus

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/common/hexutil"
	"github.com/Evrynetlabs/evrynet-node/consensus/clique"
	"github.com/Evrynetlabs/evrynet-node/core"
	"github.com/Evrynetlabs/evrynet-node/core/rawdb"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/params"
	"github.com/Evrynetlabs/evrynet-node/rlp"
	"math/big"
	"testing"
)

var (
	extraVanity            = 32 // Fixed number of extra-data prefix bytes reserved for signer vanity
	extraSeal              = 65
	randomNumberForBalance = 100000000
)

func toJsonBytes(fce *FConExtra)([]byte, error){
	fceJson :=  struct{
		Seal hexutil.Bytes
		CurrentBlock common.Hash
		EvilHeader *types.Header
	}{
		Seal: fce.Seal,
		CurrentBlock: fce.CurrentBlock,
		EvilHeader: fce.EvilHeader,
	}
	return json.Marshal(&fceJson)
}


func TestRLPFconExtra(t *testing.T) {

	var (
		db     = rawdb.NewMemoryDatabase()
		key, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		addr   = crypto.PubkeyToAddress(key.PublicKey)
		engine = clique.New(params.AllCliqueProtocolChanges.Clique, db)
	)
	config := params.ChainConfig{Clique: params.AllCliqueProtocolChanges.Clique}
	genspec := &core.Genesis{
		Config:    &config,
		ExtraData: make([]byte, extraVanity+common.AddressLength+extraSeal),
		Alloc: map[common.Address]core.GenesisAccount{
			addr: {Balance: new(big.Int).Mul(big.NewInt(int64(randomNumberForBalance)), big.NewInt(params.GasPriceConfig))},
		},
	}
	copy(genspec.ExtraData[extraVanity:], addr[:])
	genesis := genspec.MustCommit(db)
	blocks, _ := core.GenerateChain(params.AllCliqueProtocolChanges, genesis, engine, db, 2, nil)



	fce := FConExtra{CurrentBlock: blocks[1].Hash(), EvilHeader: blocks[1].Header()}
	fce.Seal = make([]byte, 65)
	rand.Read(fce.Seal)
	res, err := rlp.EncodeToBytes(&fce)
	if err != nil {
		t.Fatal(err)
	}
	var fceNew FConExtra
	err = rlp.DecodeBytes(res, &fceNew)
	if err != nil {
		t.Fatal(err)
	}

	jsonPrebytes, err := toJsonBytes(&fce)
	if err != nil {
		t.Fatal(err)
	}
	jsonBytes, err := toJsonBytes(&fceNew)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(jsonBytes, jsonPrebytes){
		t.Error("rlp change value")
	}
}

func TestExtractFConExtra(t *testing.T) {
	extraStr := "d8830105008367657688676f312e31352e348664617277696e00000000000000f87cb841f63e8add5efc97ac894849dfd11b08a8a4b1c42e2b19c214465f354a3245e17232323e38d0cf7911fd64eab7d55f3baef0cfd22d8b48c01d3d21c08342bb678401a05ff77c3f46102ee446007fb59b355d5a46ff2efeee173d501792624e6fee5ce081c0d5941232fda40c3baf755a6a62b15f5c9d5a7faa64b6"
	extra, err := hex.DecodeString(extraStr)
	if err != nil {
		t.Fatal(err)
	}
	if len(extra) <= 32 {
		t.Fatal("wrong length extra")
	}

	var fce FConExtra
	err = rlp.DecodeBytes(extra[32:], &fce)
	if err != nil {
		t.Fatal(err)
	}

	expect := common.HexToHash("0x5ff77c3f46102ee446007fb59b355d5a46ff2efeee173d501792624e6fee5ce0")
	if expect != fce.CurrentBlock {
		t.Errorf("FConExtra.Conrrent not match, expect:%s, but get:%s", expect.String(), fce.CurrentBlock.String())
	}
}
