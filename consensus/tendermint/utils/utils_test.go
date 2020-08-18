package utils

import "testing"

func TestGetCheckpointNumber(t *testing.T) {
	type args struct {
		epochDuration uint64
		blockNumber   uint64
	}
	tests := []struct {
		name string
		args args
		want uint64
	}{
		{
			name: "the block's number is lower than epoch duration",
			args: args{
				epochDuration: 1000,
				blockNumber:   400,
			},
			want: 0,
		},
		{
			name: "the block's number is greater than epoch duration",
			args: args{
				epochDuration: 1000,
				blockNumber:   1999,
			},
			want: 1000,
		},
		{
			name: "the block's number is a checkpoint",
			args: args{
				epochDuration: 1000,
				blockNumber:   2000,
			},
			want: 1000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCheckpointNumber(tt.args.epochDuration, tt.args.blockNumber); got != tt.want {
				t.Errorf("GetCheckpointNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}
