package pageutil

import (
	"testing"

	"appforge/proto/common"
)

func TestCountAndOutputPreserveKnownTotal(t *testing.T) {
	page := &common.PageReq{Cursor: 12, Limit: 20, Count: 345}
	if got := Count(page); got != 345 {
		t.Fatalf("Count()=%d want=345", got)
	}

	out := Output(page, 50)
	if out.Cursor != 12 || out.Limit != 50 || out.Count != 345 {
		t.Fatalf("Output()=%+v", out)
	}
}

func TestCountNilPage(t *testing.T) {
	if got := Count(nil); got != 0 {
		t.Fatalf("Count(nil)=%d want=0", got)
	}
}
