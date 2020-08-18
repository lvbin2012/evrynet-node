package tests

import (
	"encoding/hex"
	"fmt"

	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint/backend"
	core2 "github.com/Evrynetlabs/evrynet-node/consensus/tendermint/core"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint/tests_utils"
	"github.com/Evrynetlabs/evrynet-node/core"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/evr"
	"github.com/Evrynetlabs/evrynet-node/log"
	"github.com/Evrynetlabs/evrynet-node/p2p"
	"github.com/Evrynetlabs/evrynet-node/p2p/enode"
)

const (
	pkey2 = "AEC5EB6A80CC094363D206949C3ED475C2C5060A23049150310D4FD39F95AF99"
	pkey1 = "CAB57E606531AF83BFD023F55E1673713DA7E161D2612570A0ABAAA9507AACDF"
)

//TestStartingTendermint setup a test to with actual running components of a tendermint consensus
//The test is not finished yet but by running it, the procedure of a tendermint in implementation can be seens
//Current Expectation: if backend is created with nodePk1, it will be come proposer of the round and try to send propose message
// 					   if backend is created with nodePk2, it will wait for propose message and timeout
// 					   other logs are printed to indicate flow logic of core's consensus.
func TestStartingTendermint(t *testing.T) {
	//TODO fix this test
	t.Skip()
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlTrace, log.StreamHandler(os.Stderr, log.TerminalFormat(false))))

	var (
		nodePk1    = tests_utils.MustGeneratePrivateKey(pkey1)
		nodePk2    = tests_utils.MustGeneratePrivateKey(pkey2)
		tbe1       = backend.New(tendermint.DefaultConfig, nodePk1)
		totalPeers = 2
		n1         = enode.MustParseV4("enode://" + hex.EncodeToString(crypto.FromECDSAPub(&nodePk1.PublicKey)[1:]) + "@33.4.2.1:30303")
		n2         = enode.MustParseV4("enode://" + hex.EncodeToString(crypto.FromECDSAPub(&nodePk2.PublicKey)[1:]) + "@33.4.2.1:30304")
		header     = &types.Header{
			Number: big.NewInt(1),
		}
		block = types.NewBlockWithHeader(header)
	)
	pm1, err := evr.NewTestProtocolManagerWithConsensus(tbe1)
	//Create 2 Pipe for read and write. These are full duplex
	io1, io2 := p2p.MsgPipe()
	//p1 will write to io2, p2 will receive from io1 and vice versal.
	p1 := pm1.NewPeer(63, p2p.NewPeerFromNode(n1, fmt.Sprintf("peer %d", 0), nil), io2)
	p2 := pm1.NewPeer(63, p2p.NewPeerFromNode(n2, fmt.Sprintf("peer %d", 1), nil), io1)
	assert.NoError(t, evr.RegisterNewPeer(pm1, p1))
	assert.NoError(t, evr.RegisterNewPeer(pm1, p2))
	headHash := pm1.NodeInfo().Head
	// Must Handshake for peer to init peer.td, peer.head
	genesisHash := core.DefaultGenesisBlock().ToBlock(nil).Hash()
	go func() {
		err := p1.Handshake(evr.DefaultConfig.NetworkId, big.NewInt(0), headHash, genesisHash)
		assert.NoError(t, err)
	}()
	go func() {
		err := p2.Handshake(evr.DefaultConfig.NetworkId, big.NewInt(0), headHash, genesisHash)
		assert.NoError(t, err)
	}()
	time.Sleep(2 * time.Second) // Wait for handshaking
	//Making sure that the handlingMsg is done by calling pm.handleMsg
	var (
		errCh         = make(chan error, totalPeers)
		doneCh        = make(chan struct{}, totalPeers)
		receivedCount int
		expectedCount = 10
	)
	timeout := time.After(30 * time.Second)

	for _, p := range []*evr.Peer{p1, p2} {
		go func(p *evr.Peer) {
			for {
				if err := pm1.HandleMsg(p); err != nil {
					errCh <- err
				} else {
					doneCh <- struct{}{}
				}
			}
		}(p)
	}
	assert.NoError(t, err)
	assert.NoError(t, tbe1.Start(nil, nil, nil))

	be, ok := tbe1.(tendermint.Backend)
	assert.Equal(t, true, ok)
	//This is unsafe (it might send new block after core get into propose
	//but repeated run will get a correct case. It is the easiest way to inject a valid block for proposal
	//nolint:errcheck
	go be.EventMux().Post(core2.Proposal{
		Block: block,
	})

outer:
	for {
		select {
		case err = <-errCh:
			fmt.Printf("handling error %v\n", err)
			break outer
		case <-doneCh:
			receivedCount++
			if receivedCount >= expectedCount {
				fmt.Printf("handling done ")
				break outer
			}

		case <-timeout:
			fmt.Printf("timdeout")

			t.Fail()
			break outer
		}

	}
	if err != nil {
		t.Errorf("error handling msg by peer: %v", err)
	}
}
