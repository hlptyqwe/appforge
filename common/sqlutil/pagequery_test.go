package sqlutil

import "testing"

func TestKnownCount(t *testing.T) {
	tests := []struct {
		name   string
		counts []int64
		want   int64
	}{
		{name: "missing", want: 0},
		{name: "zero", counts: []int64{0}, want: 0},
		{name: "negative", counts: []int64{-1}, want: 0},
		{name: "positive", counts: []int64{128}, want: 128},
		{name: "first value only", counts: []int64{128, 256}, want: 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KnownCount(tt.counts...); got != tt.want {
				t.Fatalf("KnownCount(%v) = %d, want %d", tt.counts, got, tt.want)
			}
		})
	}
}
