package generate

import "testing"

func TestCompactGeneratedNo(t *testing.T) {
	t.Run("short value unchanged", func(t *testing.T) {
		const value = "FLOW20260728000001_ORDER-1"
		if got := compactGeneratedNo(value, 64); got != value {
			t.Fatalf("compactGeneratedNo() = %q, want %q", got, value)
		}
	})

	t.Run("long value is bounded and stable", func(t *testing.T) {
		value := "FLOW20260728000001_ACCEPT-FILL-INVERSE-PERP-20260728-3-MARGIN_RELEASE"
		got := compactGeneratedNo(value, 64)
		if len(got) != 64 {
			t.Fatalf("len(compactGeneratedNo()) = %d, want 64: %q", len(got), got)
		}
		if again := compactGeneratedNo(value, 64); again != got {
			t.Fatalf("compactGeneratedNo() is not stable: %q != %q", got, again)
		}
	})

	t.Run("different long values retain distinct hashes", func(t *testing.T) {
		base := "FLOW20260728000001_ACCEPT-FILL-INVERSE-PERP-20260728-3-"
		left := compactGeneratedNo(base+"MARGIN_RELEASE", 64)
		right := compactGeneratedNo(base+"PNL_PROFIT", 64)
		if left == right {
			t.Fatalf("different values compacted to the same number: %q", left)
		}
	})
}
