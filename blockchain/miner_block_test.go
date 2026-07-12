package blockchain

import "testing"

func TestAddressValidationLookupTopoUsesCandidateBlockWindow(t *testing.T) {
	tests := []struct {
		name          string
		chainTopo     int64
		candidateTopo int64
		want          int64
	}{
		{name: "candidate 29 sees registration at topo 4", chainTopo: 28, candidateTopo: 29, want: 4},
		{name: "candidate 30 sees registration at topo 5", chainTopo: 29, candidateTopo: 30, want: 5},
		{name: "early chain clamps to current state", chainTopo: 3, candidateTopo: 4, want: 3},
		{name: "candidate topo below maturity clamps to current state", chainTopo: 20, candidateTopo: 21, want: 20},
		{name: "negative candidate clamps to zero", chainTopo: 0, candidateTopo: -1, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addressValidationLookupTopo(tt.chainTopo, tt.candidateTopo); got != tt.want {
				t.Fatalf("lookup topo mismatch: got %d want %d", got, tt.want)
			}
		})
	}
}
