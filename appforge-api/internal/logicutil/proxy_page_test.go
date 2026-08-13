package logicutil

import (
	"reflect"
	"testing"

	"appforge/admin-api/internal/types"
	"appforge/proto/common"
)

func TestCopyValueMapsPageCount(t *testing.T) {
	type sourceRequest struct {
		types.PageReq
	}
	type targetRequest struct {
		Page *common.PageReq
	}

	source := &sourceRequest{PageReq: types.PageReq{Cursor: 11, Limit: 20, Count: 321}}
	var target targetRequest
	if err := copyValue(reflect.ValueOf(&target), reflect.ValueOf(source)); err != nil {
		t.Fatal(err)
	}
	if target.Page == nil || target.Page.Cursor != 11 || target.Page.Limit != 20 || target.Page.Count != 321 {
		t.Fatalf("page was not mapped: %+v", target.Page)
	}
}
