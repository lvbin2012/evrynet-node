// Copyright 2015 The evrynet-node Authors
// This file is part of the evrynet-node library.
//
// The evrynet-node library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The evrynet-node library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the evrynet-node library. If not, see <http://www.gnu.org/licenses/>.

package tests

import (
	"testing"
)

func TestBlockchain(t *testing.T) {
	t.Parallel()

	bt := new(testMatcher)
	// General state tests are 'exported' as blockchain tests, but we can run them natively.
	bt.skipLoad(`^GeneralStateTests/`)
	// Skip random failures due to selfish mining test.
	bt.skipLoad(`^bcForgedTest/bcForkUncle\.json`)
	bt.skipLoad(`^bcMultiChainTest/(ChainAtoChainB_blockorder|CallContractFromNotBestBlock)`)
	bt.skipLoad(`^bcTotalDifficultyTest/(lotsOfLeafs|lotsOfBranches|sideChainWithMoreTransactions)`)
	// Slow tests
	bt.slow(`^bcExploitTest/DelegateCallSpam.json`)
	bt.slow(`^bcExploitTest/ShanghaiLove.json`)
	bt.slow(`^bcExploitTest/SuicideIssue.json`)
	bt.slow(`^bcForkStressTest/`)
	bt.slow(`^bcGasPricerTest/RPC_API_Test.json`)
	bt.slow(`^bcWalletTest/`)

	bt.walk(t, blockTestDir, func(t *testing.T, name string, test *BlockTest) {
		if err := bt.checkFailure(t, name, test.Run()); err != nil {
			t.Error(err)
		}
	})
}
