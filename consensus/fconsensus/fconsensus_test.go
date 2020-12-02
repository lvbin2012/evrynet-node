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
	extraStr := "d8830105008367657688676f312e31352e348664617277696e00000000000000f866b8410ed05e7efb80a6bb5e04abe48fb8d3a0b2c3c2ad2737301b21a8aeb762bfe0930327ce81f5ebeef58e91f356d7815e50d0ddb8ed5e9f7498825fac600bfb972400a036b13fd969212f319e5530049d69289813fe08628bceb59e35d4b3910a33699981c0"
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

	expect := common.HexToHash("0x36b13fd969212f319e5530049d69289813fe08628bceb59e35d4b3910a336999")
	if expect != fce.CurrentBlock {
		t.Errorf("FConExtra.Conrrent not match, expect:%s, but get:%s", expect.String(), fce.CurrentBlock.String())
	}
}
