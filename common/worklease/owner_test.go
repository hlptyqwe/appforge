package worklease

import "testing"

func TestNewOwnerIsNonEmptyUniqueAndBounded(t *testing.T) {
	first, second := NewOwner("outbox"), NewOwner("outbox")
	if first == "" || second == "" || first == second {
		t.Fatalf("invalid lease owners first=%q second=%q", first, second)
	}
	if len(first) > 128 || len(second) > 128 {
		t.Fatalf("lease owner exceeds claimed_by capacity")
	}
}
