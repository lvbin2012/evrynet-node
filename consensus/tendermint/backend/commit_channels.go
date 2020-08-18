package backend

import (
	"sync"

	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/log"
)

type commitChannels struct {
	chs   map[string]chan *types.Block
	mutex *sync.RWMutex
}

func newCommitChannels() *commitChannels {
	return &commitChannels{
		chs:   make(map[string]chan *types.Block),
		mutex: &sync.RWMutex{},
	}
}

func (cc *commitChannels) sendBlock(block *types.Block) bool {
	cc.mutex.RLock()
	defer cc.mutex.RUnlock()
	ch, ok := cc.chs[block.Number().String()]
	if !ok {
		log.Error("no commit channel available", "block_number", block.Number().String())
		return false
	}
	select { // allow only 1 block send to the channel
	case ch <- block:
	default:
	}
	return true
}

//createCommitChannelAndCloseIfExist creates the channel and if the channel is exist then close it and replace with new one
func (cc *commitChannels) createCommitChannelAndCloseIfExist(blockNumberStr string) <-chan *types.Block {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()
	ch, avail := cc.chs[blockNumberStr]
	if avail {
		close(ch)
	}
	cc.chs[blockNumberStr] = make(chan *types.Block, 1)
	return cc.chs[blockNumberStr]
}

//closeAndRemoveCommitChannel remove the commitChannel
func (cc *commitChannels) closeAndRemoveCommitChannel(blockNumberStr string) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()
	ch, avail := cc.chs[blockNumberStr]
	if !avail {
		return
	}
	close(ch)
	delete(cc.chs, blockNumberStr)
}

func (cc *commitChannels) closeAndRemoveAllChannels() {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()
	for blockNumberStr, ch := range cc.chs {
		close(ch)
		delete(cc.chs, blockNumberStr)
	}
}
