package tests

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/stretchr/testify/assert"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

/* These tests are done on a chain with already setup account/ contracts.
To run these test, please deploy your own account/ contract and extract privatekey inorder to get the expected result
Adjust these params to match deployment on local machine:
*/

const (
	normalAddress                  = "0x11c93c29591ba613852ac2c9278faec2d7e7ea59"
	senderPK                       = "112CD7FA616EF6499DA9FA0A227AC73B4B109CC3F7F94C2BEFB3346CCB18CD08"
	senderAddrStr                  = "0xa091e44e0B6Adc71ce1f58B81337343597301FF6"
	contractAddrStrWithoutProvider = "0x0C53C92701a897BC7FAcfc6fa3bB01bEb8459F5E"
	contractAddrStrWithProvider    = "0xc73f6b67fdeE628331d51C783a580804229b8eB1"
	providerPK                     = "E6CFAAD68311D3A873C98750F52B2543F2C3C692A8F11E6B411B390BCD807133"
	invadlidProviderPK             = "5564a4ddd059ba6352aae637812ea6be7d818f92b5aff3564429478fcdfe4e8a"
	providerAddrStr                = "0x8359d8C955DAef81e171C13659bA3Fb0dDa144b4"

	payload = "0x6060604052341561000f57600080fd5b6102e38061001e6000396000f30060606040526004361061004c576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063368b877214610051578063ce6d41de146100ae575b600080fd5b341561005c57600080fd5b6100ac600480803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509190505061013c565b005b34156100b957600080fd5b6100c1610156565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156101015780820151818401526020810190506100e6565b50505050905090810190601f16801561012e5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b80600090805190602001906101529291906101fe565b5050565b61015e61027e565b60008054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156101f45780601f106101c9576101008083540402835291602001916101f4565b820191906000526020600020905b8154815290600101906020018083116101d757829003601f168201915b5050505050905090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061023f57805160ff191683800117855561026d565b8280016001018555821561026d579182015b8281111561026c578251825591602001919060010190610251565b5b50905061027a9190610292565b5090565b602060405190810160405280600081525090565b6102b491905b808211156102b0576000816000905550600101610298565b5090565b905600a165627a7a723058208a6eba9352e080994bc6a1041d71eff20de6686dbafb2341e23c07d938e706d60029"

	testGasLimit   = 1000000
	testGasPrice   = 1000000000
	testAmountSend = 1000000
	ethRPCEndpoint = "http://localhost:8545"
)

/*
	Test Send ETH to a normal address
		- No provider signature is required
*/
func TestSendToNormalAddress(t *testing.T) {
	senderAddr := common.HexToAddress(senderAddrStr)
	normalAddr := common.HexToAddress(normalAddress)
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)

	signer := types.HomesteadSigner{}
	ethClient, err := ethclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), senderAddr)
	assert.NoError(t, err)
	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	assert.NoError(t, err)

	transaction := types.NewTransaction(nonce, normalAddr, big.NewInt(testAmountSend), testGasLimit, gasPrice, nil)
	transaction, err = types.SignTx(transaction, signer, spk)
	err = ethClient.SendTransaction(context.Background(), transaction)
	assert.NoError(t, err)
}

/*
	Test Send ETH to a Smart Contract without provider's signature
		- Provider's signature is not required
*/
func TestSendToNonEnterpriseSmartContractWithoutProviderSignature(t *testing.T) {
	senderAddr := common.HexToAddress(senderAddrStr)
	contractAddr := common.HexToAddress(contractAddrStrWithoutProvider)
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)

	signer := types.HomesteadSigner{}
	ethClient, err := ethclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), senderAddr)
	assert.NoError(t, err)
	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	assert.NoError(t, err)

	transaction := types.NewTransaction(nonce, contractAddr, big.NewInt(testAmountSend), testGasLimit, gasPrice, nil)
	// return newTransaction(nonce, &to, amount, gasLimit, gasPrice, data)
	transaction, err = types.SignTx(transaction, signer, spk)
	err = ethClient.SendTransaction(context.Background(), transaction)
	assert.NoError(t, err)
}

/*
	Test send ETH to a Non-enterprise Smart Contract with provider's signature
		- Provider's signature is not required
*/
func TestSendToNonEnterpriseSmartContractWithProviderSignature(t *testing.T) {
	senderAddr := common.HexToAddress(senderAddrStr)
	contractAddr := common.HexToAddress(contractAddrStrWithoutProvider)
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)

	assert.NoError(t, err)

	signer := types.HomesteadSigner{}
	ethClient, err := ethclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), senderAddr)
	assert.NoError(t, err)
	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	assert.NoError(t, err)

	transaction := types.NewTransaction(nonce, contractAddr, big.NewInt(testAmountSend), testGasLimit, gasPrice, nil)
	// return newTransaction(nonce, &to, amount, gasLimit, gasPrice, data)
	transaction, err = types.SignTx(transaction, signer, spk)
	err = ethClient.SendTransaction(context.Background(), transaction)
	assert.NoError(t, err)
}

/*
	Test interact with Non-Enterprise Smart Contract
		- Update value inside Smart Contract and expect to get no error (skip provider check)
	Note: Please change data to your own function data
*/
func TestInteractWithNonEnterpriseSmartContractWithoutProviderSignature(t *testing.T) {
	//This should be a contract with provider address
	senderAddr := common.HexToAddress(senderAddrStr)
	contractAddr := common.HexToAddress(contractAddrStrWithoutProvider)
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)

	signer := types.HomesteadSigner{}
	ethClient, err := ethclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), senderAddr)
	assert.NoError(t, err)
	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	assert.NoError(t, err)

	// data to interact with a function of this contract
	dataBytes := []byte("0x552410770000000000000000000000000000000000000000000000000000000000000002")
	transaction := types.NewTransaction(nonce, contractAddr, big.NewInt(testAmountSend), testGasLimit, gasPrice, dataBytes)
	transaction, err = types.SignTx(transaction, signer, spk)
	err = ethClient.SendTransaction(context.Background(), transaction)
	assert.NoError(t, err)
}

/*
	Test Send ETH to an Enterprise Smart Contract with invalid provider's signature
*/
func TestSendToEnterPriseSmartContractWithInvalidProviderSignature(t *testing.T) {
	senderAddr := common.HexToAddress(senderAddrStr)
	contractAddr := common.HexToAddress(contractAddrStrWithProvider)
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)

	ppk, err := crypto.HexToECDSA(invadlidProviderPK)
	assert.NoError(t, err)

	signer := types.HomesteadSigner{}
	ethClient, err := ethclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), senderAddr)
	assert.NoError(t, err)
	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	assert.NoError(t, err)

	transaction := types.NewTransaction(nonce, contractAddr, big.NewInt(testAmountSend), testGasLimit, gasPrice, nil)
	transaction, err = types.SignTx(transaction, signer, spk)
	assert.NoError(t, err)
	transaction, err = types.ProviderSignTx(transaction, signer, ppk)
	assert.NoError(t, err)

	assert.NotEqual(t, nil, ethClient.SendTransaction(context.Background(), transaction))
}

/*
	Test Send ETH to an enterprise Smart Contract with valid provider's signature
*/
func TestSendToEnterPriseSmartContractWithValidProviderSignature(t *testing.T) {
	senderAddr := common.HexToAddress(senderAddrStr)
	contractAddr := common.HexToAddress(contractAddrStrWithProvider)
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)

	ppk, err := crypto.HexToECDSA(providerPK)
	assert.NoError(t, err)

	signer := types.HomesteadSigner{}
	ethClient, err := ethclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), senderAddr)
	assert.NoError(t, err)
	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	assert.NoError(t, err)

	transaction := types.NewTransaction(nonce, contractAddr, big.NewInt(testAmountSend), testGasLimit, gasPrice, nil)
	transaction, err = types.SignTx(transaction, signer, spk)
	assert.NoError(t, err)
	transaction, err = types.ProviderSignTx(transaction, signer, ppk)
	assert.NoError(t, err)

	assert.NoError(t, ethClient.SendTransaction(context.Background(), transaction))
}

/*
	Test interact with Enterprise Smart Contract
		- Update value inside Smart Contract and expect to get error with invalid provider signature
	Note: Please change data to your own function data
*/
func TestInteractToEnterpriseSmartContractWithInvalidProviderSignature(t *testing.T) {
	senderAddr := common.HexToAddress(senderAddrStr)
	contractAddr := common.HexToAddress(contractAddrStrWithProvider)
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)

	ppk, err := crypto.HexToECDSA(invadlidProviderPK)
	assert.NoError(t, err)

	signer := types.HomesteadSigner{}
	ethClient, err := ethclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), senderAddr)
	assert.NoError(t, err)
	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	assert.NoError(t, err)

	// data to interact with a function of this contract
	dataBytes := []byte("0x552410770000000000000000000000000000000000000000000000000000000000000004")
	transaction := types.NewTransaction(nonce, contractAddr, big.NewInt(testAmountSend), testGasLimit, gasPrice, dataBytes)
	transaction, err = types.SignTx(transaction, signer, spk)
	assert.NoError(t, err)
	transaction, err = types.ProviderSignTx(transaction, signer, ppk)
	assert.NoError(t, err)

	assert.NotEqual(t, nil, ethClient.SendTransaction(context.Background(), transaction))
}

/*
	Test interact with Enterprise Smart Contract
		- Update value inside Smart Contract and expect to successfully update data with valid provider signature
	Note: Please change data to your own function data
*/
func TestInteractToEnterpriseSmartContractWithValidProviderSignature(t *testing.T) {
	senderAddr := common.HexToAddress(senderAddrStr)
	contractAddr := common.HexToAddress(contractAddrStrWithProvider)
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)

	ppk, err := crypto.HexToECDSA(providerPK)
	assert.NoError(t, err)

	signer := types.HomesteadSigner{}
	ethClient, err := ethclient.Dial(ethRPCEndpoint)
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), senderAddr)
	assert.NoError(t, err)
	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	assert.NoError(t, err)

	// data to interact with a function of this contract
	dataBytes := []byte("0x552410770000000000000000000000000000000000000000000000000000000000000004")
	transaction := types.NewTransaction(nonce, contractAddr, big.NewInt(testAmountSend), testGasLimit, gasPrice, dataBytes)
	transaction, err = types.SignTx(transaction, signer, spk)
	assert.NoError(t, err)
	transaction, err = types.ProviderSignTx(transaction, signer, ppk)
	assert.NoError(t, err)

	assert.NoError(t, ethClient.SendTransaction(context.Background(), transaction))
}

func TestInteractToEnterpriseSmartContractWithValidProviderSignatureFromAccountWithoutGas(t *testing.T) {
	const (
		contractAddrStr = "0x6d88d80c9AC4bB26DaC4c4Bb09a61200F9Cb8d75"
		providerPK      = "E6CFAAD68311D3A873C98750F52B2543F2C3C692A8F11E6B411B390BCD807133"
		senderPK        = "CD79C18795A866C4A7FA8D3A88494F618AB0E69B1493382D638A6483538EEA97"
		senderAddrStr   = "0xBBD9e63B95308358AAfb20d6606701A4b6429f5e"

		testBal1     = 1000000 //1e6
		testBal2     = 2000000 //2e6
		testGasLimit = 1500000
	)
	senderAddr := common.HexToAddress(senderAddrStr)
	contractAddr := common.HexToAddress(contractAddrStr)
	spk, err := crypto.HexToECDSA(senderPK)
	assert.NoError(t, err)

	ppk, err := crypto.HexToECDSA(providerPK)
	assert.NoError(t, err)

	signer := types.HomesteadSigner{}
	ethClient, err := ethclient.Dial("http://localhost:8545")
	assert.NoError(t, err)
	nonce, err := ethClient.PendingNonceAt(context.Background(), senderAddr)
	assert.NoError(t, err)
	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	assert.NoError(t, err)

	// data to interact with a function of this contract
	dataBytes := []byte("0x552410770000000000000000000000000000000000000000000000000000000000000004")
	transaction := types.NewTransaction(nonce, contractAddr, big.NewInt(1000000), testGasLimit, gasPrice, dataBytes)
	transaction, err = types.SignTx(transaction, signer, spk)
	transaction, err = types.ProviderSignTx(transaction, signer, ppk)
	err = ethClient.SendTransaction(context.Background(), transaction)
	assert.NoError(t, err)
}
