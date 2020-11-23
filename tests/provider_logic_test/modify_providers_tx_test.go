package test

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/evrclient"
	"github.com/Evrynetlabs/evrynet-node/params"
)

func TestModifyProviders(t *testing.T) {
	var (
		senderAddr, _   = common.EvryAddressStringToAddressCheck(senderAddrStr)
		ownerAddr, _    = common.EvryAddressStringToAddressCheck(ownerAddrStr)
		providerAddr, _ = common.EvryAddressStringToAddressCheck(providerAddrStr)
		ownerKey, _     = crypto.HexToECDSA(ownerPK)
		senderKey, _    = crypto.HexToECDSA(senderPK)
		providerKey, _  = crypto.HexToECDSA(providerPK)
		gasPrice        = big.NewInt(params.GasPriceConfig)
		signer          = types.NewOmahaSigner(big.NewInt(chainId))
	)
	contractAddr := prepareNewContract(true)
	require.NotNil(t, contractAddr)

	evrClient, err := evrclient.Dial(evrRPCEndpoint)
	require.NoError(t, err)

	nonce, err := evrClient.PendingNonceAt(context.Background(), ownerAddr)
	require.NoError(t, err)
	removeProviderTx, err := types.NewModifyProvidersTransaction(nonce, *contractAddr, 21000, gasPrice, providerAddr, false)
	require.NoError(t, err)
	removeProviderTx, err = types.SignTx(removeProviderTx, signer, ownerKey)
	require.NoError(t, err)
	require.NoError(t, evrClient.SendTransaction(context.Background(), removeProviderTx))
	assertTransactionSuccess(t, evrClient, removeProviderTx.Hash(), false, ownerAddr)
	nonce++

	// create a contract interaction tx to enterprise contract from owner
	// expected to failed
	senderNonce, err := evrClient.PendingNonceAt(context.Background(), senderAddr)
	require.NoError(t, err)
	dataBytes := []byte("0x3fb5c1cb0000000000000000000000000000000000000000000000000000000000000002")
	tx := types.NewTransaction(senderNonce, *contractAddr, big.NewInt(0), testGasLimit, gasPrice, dataBytes)
	tx, err = types.SignTx(tx, signer, senderKey)
	require.NoError(t, err)
	tx, err = types.ProviderSignTx(tx, signer, ownerKey)
	require.NoError(t, err)
	require.Error(t, evrClient.SendTransaction(context.Background(), tx))

	addProviderTx, err := types.NewModifyProvidersTransaction(nonce, *contractAddr, 21000, big.NewInt(params.GasPriceConfig), providerAddr, true)
	require.NoError(t, err)
	addProviderTx, err = types.SignTx(addProviderTx, types.NewOmahaSigner(big.NewInt(chainId)), ownerKey)
	require.NoError(t, err)
	require.NoError(t, evrClient.SendTransaction(context.Background(), addProviderTx))
	assertTransactionSuccess(t, evrClient, addProviderTx.Hash(), false, ownerAddr)
	nonce++

	tx = types.NewTransaction(senderNonce, *contractAddr, big.NewInt(0), testGasLimit, gasPrice, dataBytes)
	tx, err = types.SignTx(tx, signer, senderKey)
	require.NoError(t, err)
	tx, err = types.ProviderSignTx(tx, signer, providerKey)
	require.NoError(t, err)
	require.NoError(t, evrClient.SendTransaction(context.Background(), tx))
	assertTransactionSuccess(t, evrClient, tx.Hash(), false, providerAddr)
}
