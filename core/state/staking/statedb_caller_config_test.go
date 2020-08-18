package staking_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/tidwall/gjson"

	"github.com/stretchr/testify/require"

	"github.com/Evrynetlabs/evrynet-node/core/state/staking"
)

// Constants represents the configuration name of all state variables.
const (
	storageLayoutPath = "../../../consensus/staking_contracts/storage-layout.json"
	gjsonPath         = `contracts.\./EvrynetStaking\.sol.EvrynetStaking.storageLayout`

	WithdrawsStateIndexName    = "withdrawsState"
	CandidateVotersIndexName   = "candidateVoters"
	CandidateDataIndexName     = "candidateData"
	CandidatesIndexName        = "candidates"
	StartBlockIndexName        = "startBlock"
	EpochPeriodIndexName       = "epochPeriod"
	MaxValidatorSizeIndexName  = "maxValidatorSize"
	MinValidatorStakeIndexName = "minValidatorStake"
	MinVoterCapIndexName       = "minVoterCap"
	AdminIndexName             = "admin"

	candidateStructName = "struct EvrynetStaking.CandidateData"
	TotalStakeField     = "totalStake"
	OwnerField          = "owner"
	VoterStakeField     = "voterStake"
)

type variableConfig struct {
	Label  string `json:"label"`
	Slot   uint64 `json:"slot,string"`
	Offset uint64 `json:"offset"`
}

type storageLayout struct {
	Storage       []variableConfig `json:"Storage"`
	StructConfigs map[string]struct {
		Label   string           `json:"label"`
		Members []variableConfig `json:"members"`
	} `json:"types"`
}

func TestDefaultConfig(t *testing.T) {
	f, err := os.Open(storageLayoutPath)
	require.NoError(t, err)

	data, err := ioutil.ReadAll(f)
	require.NoError(t, err)

	storageJson := gjson.Get(string(data), gjsonPath)
	var storageLayout storageLayout
	require.NoError(t, json.Unmarshal([]byte(storageJson.Raw), &storageLayout))
	// test layout config with generated layout
	for _, layout := range storageLayout.Storage {
		switch layout.Label {
		case WithdrawsStateIndexName:
			require.Equal(t, staking.DefaultConfig.WithdrawsStateLayout.Slot, layout.Slot)
			require.Equal(t, uint64(0), layout.Offset)
		case CandidateVotersIndexName:
			require.Equal(t, staking.DefaultConfig.CandidateVotersLayout.Slot, layout.Slot)
			require.Equal(t, uint64(0), layout.Offset)
		case CandidateDataIndexName:
			require.Equal(t, staking.DefaultConfig.CandidateDataLayout.Slot, layout.Slot)
			require.Equal(t, uint64(0), layout.Offset)
		case CandidatesIndexName:
			require.Equal(t, staking.DefaultConfig.CandidatesLayout.Slot, layout.Slot)
			require.Equal(t, uint64(0), layout.Offset)
		case StartBlockIndexName:
			require.Equal(t, staking.DefaultConfig.StartBlockLayout.Slot, layout.Slot)
			require.Equal(t, uint64(0), layout.Offset)
		case EpochPeriodIndexName:
			require.Equal(t, staking.DefaultConfig.EpochPeriodLayout.Slot, layout.Slot)
			require.Equal(t, uint64(0), layout.Offset)
		case MaxValidatorSizeIndexName:
			require.Equal(t, staking.DefaultConfig.MaxValidatorSizeLayout.Slot, layout.Slot)
			require.Equal(t, uint64(0), layout.Offset)
		case MinValidatorStakeIndexName:
			require.Equal(t, staking.DefaultConfig.MinValidatorStakeLayout.Slot, layout.Slot)
			require.Equal(t, uint64(0), layout.Offset)
		case MinVoterCapIndexName:
			require.Equal(t, staking.DefaultConfig.MinVoterCapLayout.Slot, layout.Slot)
			require.Equal(t, uint64(0), layout.Offset)
		case AdminIndexName:
			require.Equal(t, staking.DefaultConfig.AdminLayout.Slot, layout.Slot)
			require.Equal(t, uint64(0), layout.Offset)
		}
	}

	//test layout position inside struct
	for _, structCfg := range storageLayout.StructConfigs {
		if structCfg.Label != candidateStructName {
			continue
		}

		for _, member := range structCfg.Members {
			switch member.Label {
			case TotalStakeField:
				require.Equal(t, staking.DefaultConfig.CandidateDataStruct.TotalStake.Slot, member.Slot)
				require.Equal(t, uint64(0), member.Offset)
			case OwnerField:
				require.Equal(t, staking.DefaultConfig.CandidateDataStruct.Owner.Slot, member.Slot)
				require.Equal(t, uint64(0), member.Offset)
			case VoterStakeField:
				require.Equal(t, staking.DefaultConfig.CandidateDataStruct.VotersStakes.Slot, member.Slot)
				require.Equal(t, uint64(0), member.Offset)
			}
		}
	}

}
