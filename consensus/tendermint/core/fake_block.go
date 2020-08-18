package core

import (
	"math/big"
	"math/rand"

	"github.com/pkg/errors"

	"github.com/Evrynetlabs/evrynet-node/common"
	"github.com/Evrynetlabs/evrynet-node/common/random"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint/tests_utils"
	"github.com/Evrynetlabs/evrynet-node/consensus/tendermint/utils"
	"github.com/Evrynetlabs/evrynet-node/core/types"
	"github.com/Evrynetlabs/evrynet-node/crypto"
	"github.com/Evrynetlabs/evrynet-node/log"
	"github.com/Evrynetlabs/evrynet-node/params"
)

func (c *core) checkAndFakeProposal(proposal *Proposal) error {
	if proposal == nil {
		return nil
	}
	// Check faulty mode to inject fake block
	if c.config.FaultyMode == tendermint.SendFakeProposal.Uint64() {
		fakeHeader := *proposal.Block.Header()
		switch rand.Intn(2) {
		case 0:
			log.Warn("send fake proposal with fake parent hash", "number", proposal.Block.Number())
			fakeHeader.ParentHash = common.HexToHash(random.Hex(32))
		case 1:
			log.Warn("send fake proposal with fake transaction", "number", proposal.Block.Number())
			if err := fakeTxsForProposalBlock(&fakeHeader, proposal); err != nil {
				return errors.Errorf("fail to fake transactions. Error: %s", err)
			}
		}

		// To bypass validation coinbase
		if err := c.fakeExtraAndSealHeader(&fakeHeader); err != nil {
			return err
		}
		proposal.Block = proposal.Block.WithSeal(&fakeHeader)
	}
	return nil
}

func fakeTxsForProposalBlock(header *types.Header, proposal *Proposal) error {
	var (
		fakePrivateKey, _ = crypto.GenerateKey()
		nodeAddr          = crypto.PubkeyToAddress(fakePrivateKey.PublicKey)
	)
	fakeTx, err := types.SignTx(types.NewTransaction(0, nodeAddr, big.NewInt(10), 800000, big.NewInt(params.GasPriceConfig), nil),
		types.HomesteadSigner{}, fakePrivateKey)
	if err != nil {
		return err
	}
	header.TxHash = types.DeriveSha(types.Transactions([]*types.Transaction{fakeTx}))
	fakeBlock := types.NewBlock(header, []*types.Transaction{fakeTx}, []*types.Header{}, []*types.Receipt{})
	proposal.Block = fakeBlock

	return nil
}

// FakeHeader update fake info to block
func (c *core) fakeExtraAndSealHeader(header *types.Header) error {
	// prepare extra data without validators
	extra, err := tests_utils.PrepareExtra(header)
	if err != nil {
		return errors.Errorf("fail to fake proposal. Error: %s", err)
	}
	header.Extra = extra

	// addProposalSeal
	seal, err := c.backend.Sign(utils.SigHash(header).Bytes())
	if err != nil {
		return errors.Errorf("fail to sign fake header. Error: %s", err)
	}

	if err := utils.WriteSeal(header, seal); err != nil {
		return errors.Errorf("fail to write seal for fake header. Error: %s", err)
	}
	return nil
}
