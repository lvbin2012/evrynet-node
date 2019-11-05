package core

import (
	"math/big"
	"math/rand"

	"github.com/pkg/errors"

	"github.com/evrynet-official/evrynet-client/common"
	"github.com/evrynet-official/evrynet-client/common/random"
	"github.com/evrynet-official/evrynet-client/consensus/tendermint"
	"github.com/evrynet-official/evrynet-client/consensus/tendermint/utils"
	"github.com/evrynet-official/evrynet-client/core/types"
	"github.com/evrynet-official/evrynet-client/crypto"
	"github.com/evrynet-official/evrynet-client/log"
	"github.com/evrynet-official/evrynet-client/params"
)

func (c *core) fakeProposalBlock(proposal *tendermint.Proposal) error {
	// Check faulty mode to inject fake block
	if c.config.FaultyMode == tendermint.SendFakeProposal.Uint64() {
		fakeHeader := *proposal.Block.Header()
		switch rand.Intn(2) {
		case 0:
			log.Warn("send fake proposal with fake parent hash")
			fakeHeader.ParentHash = common.HexToHash(random.Hex(32))
		case 1:
			log.Warn("send fake proposal with fake transaction")
			if err := c.fakeTxsForProposalBlock(&fakeHeader, proposal); err != nil {
				return errors.Errorf("fail to fake transactions", "err", err)
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

func (c *core) fakeTxsForProposalBlock(header *types.Header, proposal *tendermint.Proposal) error {
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
	extra, err := utils.PrepareExtra(header)
	if err != nil {
		return errors.Errorf("fail to fake proposal", "err", err)
	}
	header.Extra = extra

	// addProposalSeal
	seal, err := c.backend.Sign(utils.SigHash(header).Bytes())
	if err != nil {
		return errors.Errorf("fail to sign fake header", "err", err)
	}

	if err := utils.WriteSeal(header, seal); err != nil {
		return errors.Errorf("fail to write seal for fake header", "err", err)
	}
	return nil
}
