package conv

import "testing"

func TestParseBoundedDecimalField(t *testing.T) {
	valid := []string{"0", "1000.25", "0001.2300", "999999999999999999.999999999999999999"}
	for _, value := range valid {
		if _, err := ParseBoundedDecimalField(value, 18, 18); err != nil {
			t.Fatalf("expected %q to be valid: %v", value, err)
		}
	}

	invalid := []string{
		"abc",
		"1e3",
		"1.2.3",
		"1000000000000000000",
		"1.1234567890123456789",
	}
	for _, value := range invalid {
		if _, err := ParseBoundedDecimalField(value, 18, 18); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestParseDecimalFieldRejectsNonPlainDecimal(t *testing.T) {
	for _, value := range []string{"abc", "1e3", "NaN", "Inf", "1.2.3"} {
		if _, err := ParseDecimalField(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}
