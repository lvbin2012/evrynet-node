package backend

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Evrynetlabs/evrynet-node/accounts/abi"
	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/consensus/staking_contracts"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint/tests_utils"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint/utils"
	"github.com/Evrynetlabs/evrynet-node/core"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/log"
	"github.com/Evrynetlabs/evrynet-node/params"
	"github.com/Evrynetlabs/evrynet-node/rlp"
)

//TestBackend_RewardNoTx this is integration test between core.BlockChain and tendermint.Backend
// this test check the reward of validator without any transactions and voters
func TestBackend_RewardNoTx(t *testing.T) {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlTrace, log.StreamHandler(os.Stderr, log.TerminalFormat(false))))
	_, addr := getValidatorAccounts()
	testFinalize(t, func(i int, gen *core.BlockGen) {
		gen.SetCoinbase(addr[0])
	}, stakingEpoch, func(chain *core.BlockChain) {

		currentState, err := chain.State()
		require.NoError(t, err)
		expectedBalance := new(big.Int).Mul(big.NewInt(stakingEpoch), chain.Config().Tendermint.BlockReward)
		require.Equal(t, currentState.GetBalance(addr[0]), expectedBalance)
	})
}

//TestBackend_RewardNoTx_WithVoter this is integration test between core.BlockChain and tendermint.Backend
// this test check the reward of validators and voters without any transactions
func TestBackend_RewardNoTx_WithVoter(t *testing.T) {
	var (
		nonce uint64 = 0
	)
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlTrace, log.StreamHandler(os.Stderr, log.TerminalFormat(false))))
	_, validatorAddresses := getValidatorAccounts()
	faucetKeys, faucetAddresses := getFaucetAccounts()
	testFinalize(t, func(i int, gen *core.BlockGen) {
		gen.SetCoinbase(validatorAddresses[0])
		if i == stakingEpoch-1 {
			valSetData, err := rlp.EncodeToBytes(validatorAddresses)
			require.NoError(t, err)
			tdm := &types.TendermintExtra{
				ValidatorAdds: valSetData,
			}
			payload, err := rlp.EncodeToBytes(&tdm)
			require.NoError(t, err)
			gen.SetExtra(append(bytes.Repeat([]byte{0x00}, types.TendermintExtraVanity), payload...))
		}
		if i == 0 {
			parsed, err := abi.JSON(strings.NewReader(staking_contracts.StakingContractsABI))
			require.NoError(t, err)
			input, err := parsed.Pack("vote", validatorAddresses[0])
			require.NoError(t, err)
			tx, err := types.SignTx(
				types.NewTransaction(nonce, stakingScAddress, big.NewInt(1), 1000000, big.NewInt(params.GasPriceConfig), input),
				types.NewEIP155Signer(big.NewInt(15)),
				faucetKeys[1],
			)
			require.NoError(t, err)
			nonce++
			gen.AddTx(tx)
		}

	}, stakingEpoch*2, func(chain *core.BlockChain) {
		receipts := chain.GetReceiptsByHash(chain.GetHeaderByNumber(1).Hash())
		require.Equal(t, 1, len(receipts))
		require.Equal(t, types.ReceiptStatusSuccessful, receipts[0].Status)
		// check balance reward for epoch 0
		state0, err := chain.StateAt(chain.GetHeaderByNumber(stakingEpoch).Root)
		require.NoError(t, err)
		expectedBalance0 := new(big.Int).Mul(big.NewInt(stakingEpoch), chain.Config().Tendermint.BlockReward)
		expectedBalance0 = new(big.Int).Add(expectedBalance0, new(big.Int).Mul(big.NewInt(int64(receipts[0].GasUsed)), big.NewInt(params.GasPriceConfig)))
		require.Equal(t, expectedBalance0, state0.GetBalance(validatorAddresses[0]))
		// check balance reward for epoch 1
		// validator[0] stake 1, faucet[1] stake 1 -> reward 75% 25% respectively
		state1, err := chain.StateAt(chain.GetHeaderByNumber(stakingEpoch * 2).Root)
		require.NoError(t, err)
		epoch1Reward := new(big.Int).Sub(state1.GetBalance(validatorAddresses[0]), expectedBalance0)
		expectedTotalReward := new(big.Int).Mul(big.NewInt(stakingEpoch), chain.Config().Tendermint.BlockReward)
		expectedOwnerReward := new(big.Int).Div(new(big.Int).Mul(expectedTotalReward, big.NewInt(75)), big.NewInt(100))
		require.Equal(t, expectedOwnerReward, epoch1Reward)

		expectedVoterReward := new(big.Int).Div(new(big.Int).Mul(expectedTotalReward, big.NewInt(25)), big.NewInt(100))
		require.Equal(t, expectedVoterReward, new(big.Int).Sub(state1.GetBalance(faucetAddresses[1]), state0.GetBalance(faucetAddresses[1])))
	})
}

// TestBackend_RewardWithTx this is integration test between core.BlockChain and tendermint.Backend
// this test check the reward of validators with included transaction fee
func TestBackend_RewardWithTx(t *testing.T) {
	const (
		numberTransaction = 100
	)
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlTrace, log.StreamHandler(os.Stderr, log.TerminalFormat(false))))
	_, addr := getValidatorAccounts()
	faucetKeys, _ := getFaucetAccounts()
	var nonce uint64 = 0
	testFinalize(t, func(i int, gen *core.BlockGen) {
		gen.SetCoinbase(addr[0])
		if i == 0 {
			sign := types.NewEIP155Signer(big.NewInt(15))
			for i := 0; i < numberTransaction; i++ {
				tx, _ := types.SignTx(
					types.NewTransaction(nonce, common.Address{0}, big.NewInt(1000), params.TxGas, big.NewInt(params.GasPriceConfig), nil),
					sign,
					faucetKeys[0],
				)
				nonce++
				gen.AddTx(tx)
			}
		}
	}, stakingEpoch, func(chain *core.BlockChain) {
		currentState, err := chain.State()
		require.NoError(t, err)

		expectedBalance := new(big.Int).Mul(big.NewInt(stakingEpoch), chain.Config().Tendermint.BlockReward)
		expectedBalance = expectedBalance.Add(expectedBalance, new(big.Int).Mul(big.NewInt(params.GasPriceConfig), big.NewInt(int64(numberTransaction*params.TxGas))))
		require.Equal(t, currentState.GetBalance(addr[0]), expectedBalance)
	})
}

func testFinalize(t *testing.T, generate func(int, *core.BlockGen), n int, assertFn func(chain *core.BlockChain)) {
	be, chain, db, err := createBlockchainAndBackendFromGenesis(StakingSC)
	require.NoError(t, err)
	genesis := chain.Genesis()
	require.NotNil(t, genesis)
	pks, addrs := getValidatorAccounts()
	blocks, receipts := core.GenerateChain(chain.Config(), genesis, be, db, n, generate)
	fmt.Println("blocks", len(blocks), "receipts", len(receipts))
	parent := genesis.Header().Hash()
	// sealing
	for i, block := range blocks {
		header := block.Header()
		header.ParentHash = parent
		extra, _ := tests_utils.PrepareExtra(header)
		header.Extra = extra
		if header.Number.Uint64()%be.config.Epoch == 0 { // transition block
			require.NoError(t, utils.WriteValSet(header, addrs))
		}
		tests_utils.AppendSealByPkKey(header, pks[0])
		tests_utils.AppendCommitedSealByPkKeys(header, pks)
		parent = header.Hash()
		blocks[i] = block.WithSeal(header)
	}
	// insert to chain
	insertedBlock, err := chain.InsertChain(blocks)
	require.NoError(t, err)
	require.Equal(t, len(blocks), insertedBlock)
	if assertFn != nil {
		assertFn(chain)
	}
}
