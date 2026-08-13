package adminlogic

import (
	"testing"

	"appforge/proto/system"
	"appforge/services/system/models"
)

func TestBuildProfileMenuTree(t *testing.T) {
	items := []models.SysMenu{
		{Id: 1, ParentId: 0, Name: "业务", MenuType: int64(system.MenuType_MENU_TYPE_DIR), Sort: 1},
		{Id: 2, ParentId: 1, Name: "应用", MenuType: int64(system.MenuType_MENU_TYPE_MENU), Sort: 1},
		{Id: 3, ParentId: 2, Name: "新增应用", MenuType: int64(system.MenuType_MENU_TYPE_BUTTON), Sort: 1},
	}

	tree := buildProfileMenuTree(items)
	if len(tree) != 1 {
		t.Fatalf("expected one root menu, got %d", len(tree))
	}
	if tree[0].Id != 1 || len(tree[0].Children) != 1 {
		t.Fatalf("unexpected menu tree: %#v", tree[0])
	}
	if tree[0].Children[0].Id != 2 {
		t.Fatalf("expected menu 2 as child, got %d", tree[0].Children[0].Id)
	}
	if len(tree[0].Children[0].Children) != 0 {
		t.Fatal("button permissions must not be returned as navigation menus")
	}
}

func TestRequestMethod(t *testing.T) {
	tests := map[string]system.RequestMethod{
		"GET":    system.RequestMethod_REQUEST_METHOD_GET,
		"post":   system.RequestMethod_REQUEST_METHOD_POST,
		"PUT":    system.RequestMethod_REQUEST_METHOD_PUT,
		"delete": system.RequestMethod_REQUEST_METHOD_DELETE,
		"":       system.RequestMethod_REQUEST_METHOD_UNKNOWN,
	}
	for input, expected := range tests {
		if actual := requestMethod(input); actual != expected {
			t.Errorf("requestMethod(%q) = %v, want %v", input, actual, expected)
		}
	}
}
