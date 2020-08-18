package tests_utils

import (
	"math/big"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus"
	"github.com/Evrynetlabs/evrynet-node/core/rawdb"
	"github.com/Evrynetlabs/evrynet-node/core/state"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/params"
)

//MockChainReader is mock struct for chain reader,
//it serves basic block/ state reading for testing purposes
type MockChainReader struct {
	GenesisHeader *types.Header
	*MockBlockChain
	Address common.Address
	Trigger *bool
}

func (c *MockChainReader) Config() *params.ChainConfig {
	return &params.ChainConfig{
		Tendermint: &params.TendermintConfig{
			Epoch: params.EpochDuration,
		},
	}
}

func (c *MockChainReader) CurrentHeader() *types.Header {
	return c.CurrentBlock().Header()
}

//GetHeader implement a mock version of chainReader.GetHeader
//It returns correctParentHeader with Number set to input blockNumber
func (c *MockChainReader) GetHeader(hash common.Hash, blockNumber uint64) *types.Header {
	if c.GenesisHeader.Hash() == hash {
		return c.GenesisHeader
	}
	return nil
}

//GetHeaderByNumber implement a mock version of chainReader.GetHeaderByNumber
//It returns genesis Header if blockNumber is 0, else return an empty Header
func (c *MockChainReader) GetHeaderByNumber(blockNumber uint64) *types.Header {
	return c.GenesisHeader
}

func (c *MockChainReader) GetHeaderByHash(hash common.Hash) *types.Header {
	panic("implement me")
}

//State is used multiple times to reset the pending state.
// when simulate is true it will create a state that indicates
// that tx0 and tx1 are included in the chain.
func (c *MockChainReader) State() (*state.StateDB, error) {
	// delay "state change" by one. The tx pool fetches the
	// state multiple times and by delaying it a bit we simulate
	// a state change between those fetches.
	stdb := c.Statedb
	if *c.Trigger {
		c.Statedb, _ = state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()))
		c.Statedb.SetNonce(c.Address, 0)
		c.Statedb.SetBalance(c.Address, new(big.Int).SetUint64(params.Ether))
		*c.Trigger = false
	}
	return stdb, nil
}

//MockChainReader is mock struct for chain reader,
//it serves basic header for testing purposes
type headersMockChainReader struct {
	headers []*types.Header
}

func (c *headersMockChainReader) Config() *params.ChainConfig {
	panic("implement me")
}

func (c *headersMockChainReader) CurrentHeader() *types.Header {
	return c.headers[len(c.headers)-1]
}

func (c *headersMockChainReader) GetHeader(hash common.Hash, number uint64) *types.Header {
	if int(number) > len(c.headers)-1 {
		return nil
	}
	header := c.headers[number]
	if header.Hash() != hash {
		return nil
	}
	return header
}

func (c *headersMockChainReader) GetHeaderByNumber(number uint64) *types.Header {
	if int(number) > len(c.headers)-1 {
		return nil
	}
	return c.headers[number]
}

func (c *headersMockChainReader) GetHeaderByHash(hash common.Hash) *types.Header {
	panic("implement me")
}

func (c *headersMockChainReader) GetBlock(hash common.Hash, number uint64) *types.Block {
	panic("implement me")
}

func NewHeadersMockChainReader(headers []*types.Header) consensus.ChainReader {
	return &headersMockChainReader{
		headers: headers,
	}
}
