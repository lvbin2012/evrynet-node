package core

import (
	"go.uber.org/zap"

	"github.com/Evrynetlabs/evrynet-node/rlp"
)

func (c *core) reBroadcastMsg(msg message, logger *zap.SugaredLogger) {
	if !c.rebroadcast {
		return
	}
	if msg.Address.Hex() == c.getAddress().Hex() {
		return
	}

	payload, err := rlp.EncodeToBytes(&msg)
	if err != nil {
		logger.Error("failed to encode msg", "error", err)
		return
	}
	if err := c.backend.Multicast(c.valSet.GetNeighbors(c.getAddress()), payload); err != nil {
		logger.Error("failed to re-gossip the vote received", "error", err)
	}
}
