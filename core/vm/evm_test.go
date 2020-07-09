package vm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/core/rawdb"
	"github.com/Evrynetlabs/evrynet-node/core/state"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/core/vm"
	"github.com/Evrynetlabs/evrynet-node/params"
)

func TestEVM_AddProvider(t *testing.T) {
	var (
		contractAddr    = common.HexToAddress("0x00000000000000000000000000000000deadbeef")
		ownerAddr       = common.HexToAddress("0x560089ab68dc224b250f9588b3db540d87a66b7a")
		providerAddr    = common.HexToAddress("954e4bf2c68f13d97c45db0e02645d145db6911f")
		newProviderAddr = common.HexToAddress("3ca5f11792bad2aa50816726b441fa306ddeab2f")
	)

	db := state.NewDatabase(rawdb.NewMemoryDatabase())
	statedb, _ := state.New(common.Hash{}, db)
	statedb.CreateAccount(contractAddr, types.CreateAccountOption{OwnerAddress: &ownerAddr, ProviderAddress: &providerAddr})
	root, _ := statedb.Commit(false)
	statedb, _ = state.New(root, db)
	evm := vm.NewEVM(vm.Context{}, statedb, params.TestChainConfig, vm.Config{})

	addProviderMsg := types.ModifyProvidersMsg{
		Provider: newProviderAddr,
	}

	require.Error(t, evm.AddProvider(ownerAddr, common.HexToAddress("0x11"), addProviderMsg), vm.ErrOwnerNotFound)
	require.Error(t, evm.AddProvider(common.HexToAddress("0x11"), contractAddr, addProviderMsg), vm.ErrOnlyOwner)

	require.NoError(t, evm.AddProvider(ownerAddr, contractAddr, addProviderMsg))
	require.Equal(t, statedb.GetProviders(contractAddr), []common.Address{providerAddr, newProviderAddr})
}

func TestEVM_RemoveProvider(t *testing.T) {
	var (
		contractAddr = common.HexToAddress("0x00000000000000000000000000000000deadbeef")
		ownerAddr    = common.HexToAddress("0x560089ab68dc224b250f9588b3db540d87a66b7a")
		providerAddr = common.HexToAddress("954e4bf2c68f13d97c45db0e02645d145db6911f")
	)

	db := state.NewDatabase(rawdb.NewMemoryDatabase())
	statedb, _ := state.New(common.Hash{}, db)
	statedb.CreateAccount(contractAddr, types.CreateAccountOption{OwnerAddress: &ownerAddr, ProviderAddress: &providerAddr})
	root, _ := statedb.Commit(false)
	statedb, _ = state.New(root, db)
	evm := vm.NewEVM(vm.Context{}, statedb, params.TestChainConfig, vm.Config{})

	removeProviderMsg := types.ModifyProvidersMsg{
		Provider: providerAddr,
	}

	require.Error(t, evm.RemoveProvider(ownerAddr, common.HexToAddress("0x11"), removeProviderMsg), vm.ErrOwnerNotFound)
	require.Error(t, evm.RemoveProvider(common.HexToAddress("0x11"), contractAddr, removeProviderMsg), vm.ErrOnlyOwner)

	require.NoError(t, evm.RemoveProvider(ownerAddr, contractAddr, removeProviderMsg))
	require.Equal(t, statedb.GetProviders(contractAddr), []common.Address{})
}
