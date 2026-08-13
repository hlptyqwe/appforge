package system

import (
	"testing"

	systempb "appforge/proto/system"
)

func TestMapOpLogItemMapsRequestMethod(t *testing.T) {
	item := mapOpLogItem(&systempb.OpLogItem{
		Id:     1,
		Method: systempb.RequestMethod_REQUEST_METHOD_PUT,
		Path:   "/admin/member/users/1/status",
	})
	if item.Method != "PUT" {
		t.Fatalf("method=%q want PUT", item.Method)
	}
}

func TestToRequestMethodMapsFilter(t *testing.T) {
	if got := toRequestMethod("post"); got != systempb.RequestMethod_REQUEST_METHOD_POST {
		t.Fatalf("method=%v want POST", got)
	}
}
