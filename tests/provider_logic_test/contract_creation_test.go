package test

import (
	"context"
	"errors"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/common/hexutil"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/evrclient"
)

// TODO: create a global nonce provider so tests not need to wait others complete
/* These tests are done on a chain with already setup account/ contracts.
To run these test, please deploy your own account/ contract and extract privatekey inorder to get the expected result
Adjust these params to match deployment on local machine:
*/

/*
	Test Send ETH to a normal address
		- No provider signature is required
*/

func TestMain(m *testing.M) {
	ethClient, err := evrclient.Dial(ethRPCEndpoint)
	if err != nil {
		panic(err)
	}
	// wait until pass byzantium block
	for {
		block, err := ethClient.BlockByNumber(context.Background(), nil)
		if err != nil {
			panic(err)
		}
		if block.Number().Cmp(big.NewInt(2)) > 0 {
			break
		}
	}
	code := m.Run()
	os.Exit(code)
}

func TestCreateContractWithProviderAddress(t *testing.T) {
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)
	sender := common.HexToAddress(senderAddrStr)
	provideraddr := common.HexToAddress(providerAddrStr)
	payLoadBytes, err := hexutil.Decode(payload)
	assert.NoError(t, err)
	var option types.CreateAccountOption
	option.ProviderAddress = &provideraddr

	ethClient, err := evrclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.NonceAt(context.Background(), sender, nil)
	assert.NoError(t, err)
	tx := types.NewContractCreation(nonce, big.NewInt(0), testGasLimit, big.NewInt(testGasPrice), payLoadBytes, option)
	tx, err = types.SignTx(tx, types.HomesteadSigner{}, spk)
	assert.NoError(t, err)
	err = errors.New("owner is required")
	assert.Error(t, err, ethClient.SendTransaction(context.Background(), tx))
}

func TestCreateContractWithProviderAndOwner(t *testing.T) {
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)
	sender := common.HexToAddress(senderAddrStr)
	providerAddr := common.HexToAddress(providerAddrStr)
	payLoadBytes, err := hexutil.Decode(payload)
	assert.NoError(t, err)
	var option types.CreateAccountOption
	option.OwnerAddress = &sender
	option.ProviderAddress = &providerAddr

	ethClient, err := evrclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.NonceAt(context.Background(), sender, nil)
	assert.NoError(t, err)
	tx := types.NewContractCreation(nonce, big.NewInt(0), testGasLimit, big.NewInt(testGasPrice), payLoadBytes, option)
	tx, err = types.SignTx(tx, types.HomesteadSigner{}, spk)
	assert.NoError(t, err)
	require.NoError(t, ethClient.SendTransaction(context.Background(), tx))
	assertTransactionSuccess(t, ethClient, tx.Hash(), true, sender)
}

func TestCreateContractWithoutProviderAddress(t *testing.T) {
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)
	sender := common.HexToAddress(senderAddrStr)
	payLoadBytes, err := hexutil.Decode(payload)
	assert.NoError(t, err)

	ethClient, err := evrclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), sender)
	assert.NoError(t, err)
	tx := types.NewContractCreation(nonce, big.NewInt(0), testGasLimit, big.NewInt(testGasPrice), payLoadBytes)
	tx, err = types.SignTx(tx, types.HomesteadSigner{}, spk)
	assert.NoError(t, err)

	require.NoError(t, ethClient.SendTransaction(context.Background(), tx))
	assertTransactionSuccess(t, ethClient, tx.Hash(), true, sender)
}

func TestCreateContractWithProviderSignature(t *testing.T) {
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)
	ppk, err := crypto.HexToECDSA(providerPK)
	assert.NoError(t, err)
	sender := common.HexToAddress(senderAddrStr)
	payLoadBytes, err := hexutil.Decode(payload)
	assert.NoError(t, err)

	ethClient, err := evrclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), sender)
	assert.NoError(t, err)
	tx := types.NewContractCreation(nonce, big.NewInt(0), testGasLimit, big.NewInt(testGasPrice), payLoadBytes)
	tx, err = types.SignTx(tx, types.HomesteadSigner{}, spk)
	assert.NoError(t, err)
	tx, err = types.ProviderSignTx(tx, types.HomesteadSigner{}, ppk)
	assert.NoError(t, err)
	require.Error(t, ethClient.SendTransaction(context.Background(), tx), "Must return error: redundant provider's signature")
}

func TestCreateContractWithProviderAddressWithoutGas(t *testing.T) {
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)
	sender := common.HexToAddress(senderAddrStr)
	provideraddr := common.HexToAddress(providerWithoutGasAddr)
	var option types.CreateAccountOption
	option.ProviderAddress = &provideraddr
	option.OwnerAddress = &sender
	payLoadBytes, err := hexutil.Decode(payload)
	assert.NoError(t, err)

	ethClient, err := evrclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), sender)
	assert.NoError(t, err)
	tx := types.NewContractCreation(nonce, big.NewInt(0), testGasLimit, big.NewInt(testGasPrice), payLoadBytes, option)
	tx, err = types.SignTx(tx, types.HomesteadSigner{}, spk)
	assert.NoError(t, err)
	require.NoError(t, ethClient.SendTransaction(context.Background(), tx))
	assertTransactionSuccess(t, ethClient, tx.Hash(), true, sender)
}

func TestCreateContractWithProviderAddressMustHaveOwnerAddress(t *testing.T) {
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)
	sender := common.HexToAddress(senderAddrStr)
	provideraddr := common.HexToAddress(providerAddrStr)
	payLoadBytes, err := hexutil.Decode(payload)
	assert.NoError(t, err)
	var option types.CreateAccountOption
	option.ProviderAddress = &provideraddr
	option.OwnerAddress = &sender

	ethClient, err := evrclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), sender)
	assert.NoError(t, err)
	tx := types.NewContractCreation(nonce, big.NewInt(0), testGasLimit, big.NewInt(testGasPrice), payLoadBytes, option)
	tx, err = types.SignTx(tx, types.HomesteadSigner{}, spk)
	assert.NoError(t, err)
	assert.Equal(t, strings.ToLower(senderAddrStr), strings.ToLower(tx.Owner().Hex()))
	assert.Equal(t, strings.ToLower(providerAddrStr), strings.ToLower(tx.Provider().Hex()))
}

func TestCreateNormalContractMustHaveNoOwnerAndProviderAddress(t *testing.T) {
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)
	sender := common.HexToAddress(senderAddrStr)
	payLoadBytes, err := hexutil.Decode(payload)
	assert.NoError(t, err)

	ethClient, err := evrclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), sender)
	assert.NoError(t, err)
	tx := types.NewContractCreation(nonce, big.NewInt(0), testGasLimit, big.NewInt(testGasPrice), payLoadBytes)
	tx, err = types.SignTx(tx, types.HomesteadSigner{}, spk)
	assert.NoError(t, err)
	assert.Nil(t, tx.Owner())
	assert.Nil(t, tx.Provider())
}

func assertTransactionSuccess(t *testing.T, client *evrclient.Client, txHash common.Hash, contractCreation bool, gasPayer common.Address) {
	for i := 0; i < getReceiptMaxRetries; i++ {
		var receipt *types.Receipt
		receipt, err := client.TransactionReceipt(context.Background(), txHash)
		if err == nil {
			assert.Equal(t, uint64(1), receipt.Status)
			if contractCreation {
				assert.NotEqual(t, receipt.ContractAddress, common.Address{}, "not contract creation")
			}
			assert.Equal(t, gasPayer, receipt.GasPayer, "unexpected gas payer")
			return
		}
		time.Sleep(1 * time.Second)
	}
	t.Errorf("transaction %s not found", txHash.Hex())
}
