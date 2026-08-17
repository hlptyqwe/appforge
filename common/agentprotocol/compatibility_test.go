package agentprotocol

import "testing"

func TestCompatibilityWindow(t *testing.T) {
	tests := []struct {
		protocol  int32
		supported bool
		canClaim  bool
	}{
		{protocol: 1, supported: false, canClaim: false},
		{protocol: 2, supported: true, canClaim: false},
		{protocol: 3, supported: true, canClaim: true},
		{protocol: 4, supported: false, canClaim: false},
	}
	for _, test := range tests {
		if actual := Supported(test.protocol); actual != test.supported {
			t.Fatalf("Supported(%d)=%v, want %v", test.protocol, actual, test.supported)
		}
		if actual := CanClaimTaskBundle(test.protocol); actual != test.canClaim {
			t.Fatalf("CanClaimTaskBundle(%d)=%v, want %v", test.protocol, actual, test.canClaim)
		}
	}
}

func TestValidateReleaseWindow(t *testing.T) {
	if err := ValidateReleaseWindow(Minimum, Current); err != nil {
		t.Fatal(err)
	}
	for _, window := range [][2]int32{{1, 3}, {2, 4}, {3, 3}} {
		if err := ValidateReleaseWindow(window[0], window[1]); err == nil {
			t.Fatalf("mismatched window %v was accepted", window)
		}
	}
}
