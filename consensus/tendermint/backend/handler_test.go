package backend

import (
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/evrynet-official/evrynet-client/common"
	"github.com/evrynet-official/evrynet-client/consensus"
	"github.com/evrynet-official/evrynet-client/consensus/tendermint"
	"github.com/evrynet-official/evrynet-client/consensus/tendermint/tests_utils"
	"github.com/evrynet-official/evrynet-client/core/types"
	"github.com/evrynet-official/evrynet-client/crypto"
	"github.com/evrynet-official/evrynet-client/event"
	"github.com/evrynet-official/evrynet-client/log"
	"github.com/evrynet-official/evrynet-client/p2p"
	"github.com/evrynet-official/evrynet-client/rlp"
)

func TestHandleMsg(t *testing.T) {
	var (
		nodePrivateKey = tests_utils.MakeNodeKey()
		nodeAddr       = crypto.PubkeyToAddress(nodePrivateKey.PublicKey)
		validators     = []common.Address{
			nodeAddr,
		}
		genesisHeader = tests_utils.MakeGenesisHeader(validators)
	)

	//create New test backend and newMockChain
	be := mustCreateAndStartNewBackend(t, nodePrivateKey, genesisHeader)

	// generate one msg
	data := []byte("data1")
	msg := makeMsg(consensus.TendermintMsg, data)
	addr := tests_utils.GetAddress()

	// 2. this message should be in cache after we handle it
	handled, err := be.HandleMsg(addr, msg)
	if err != nil {
		t.Errorf("expected message being handled successfully but got %s", err)
	}
	if !handled {
		t.Errorf("expected message not being handled")
	}
}

func makeMsg(msgcode uint64, data interface{}) p2p.Msg {
	size, r, _ := rlp.EncodeToReader(data)
	return p2p.Msg{Code: msgcode, Size: uint32(size), Payload: r}
}

// mockCore is similar to real core with fixed time for processing each request
// mockCore also has 'numMsg' variable for testing
type mockCore struct {
	be        tendermint.Backend
	handlerWg sync.WaitGroup
	events    *event.TypeMuxSubscription
	numMsg    int64
}

func NewMockCore(be tendermint.Backend) *mockCore {
	return &mockCore{
		be: be,
	}
}

func (m *mockCore) Start() error {

	log.Debug("core start")
	m.events = m.be.EventMux().Subscribe(tendermint.MessageEvent{})
	go m.handleEvents()
	return nil
}

func (m *mockCore) handleEvents() {
	defer func() {
		m.handlerWg.Done()
	}()
	m.handlerWg.Add(1)
	for event := range m.events.Chan() {
		switch ev := event.Data.(type) {
		case tendermint.MessageEvent:
			_ = ev
			log.Debug("handling event", "payload", string(ev.Payload))
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&m.numMsg, 1)
		default:
			panic("unexpected type")
		}
	}
	log.Debug("exit loop")
}

func (m *mockCore) Stop() error {
	m.events.Unsubscribe()
	m.handlerWg.Wait()
	return nil
}

func (m *mockCore) SetBlockForProposal(block *types.Block) {
	panic("implement me")
}

// This test case is when user start miner then stop it before core handles all msg in storingMsgs
func TestBackend_HandleMsg(t *testing.T) {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlTrace, log.StreamHandler(os.Stderr, log.TerminalFormat(false))))

	be, _, blockchain, err := createBlockchainAndBackendFromGenesis()
	require.NoError(t, err)
	mockCore := NewMockCore(be)
	be.core = mockCore

	count := 0
	// send msg when core is not started
	numMsg := 10
	for i := 0; i < numMsg; i++ {
		_, err := be.HandleMsg(common.Address{}, makeMsg(consensus.TendermintMsg, []byte(strconv.FormatInt(int64(count), 10))))
		count += 1
		require.NoError(t, err)
	}
	// start core
	require.NoError(t, be.Start(blockchain, blockchain.CurrentBlock))
	// trigger to  dequeue and replay msg
	_, err = be.HandleMsg(common.Address{}, makeMsg(consensus.TendermintMsg, []byte(strconv.FormatInt(int64(count), 10))))
	count += 1
	require.NoError(t, err)
	time.Sleep(time.Millisecond)
	// immediately stop core
	require.NoError(t, be.Stop())

	require.NoError(t, be.Start(blockchain, blockchain.CurrentBlock))
	_, err = be.HandleMsg(common.Address{}, makeMsg(consensus.TendermintMsg, []byte(strconv.FormatInt(int64(count), 10))))
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 16)
	require.Equal(t, int64(numMsg+2), mockCore.numMsg)
}