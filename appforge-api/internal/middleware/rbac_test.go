package middleware

import "testing"

func TestGetRequiredPermissionStripsAdminPrefix(t *testing.T) {
	rules := []PermissionRule{
		mustRule(t, "GET", "/system/users", "sys:user:list"),
	}

	got := getRequiredPermission(rules, "/admin/system/users", "GET")
	if got != "sys:user:list" {
		t.Fatalf("expected sys:user:list, got %q", got)
	}
}

func TestGetRequiredPermissionMatchesPathParams(t *testing.T) {
	rules := []PermissionRule{
		mustRule(t, "DELETE", "/system/users/{id}", "sys:user:delete"),
		mustRule(t, "DELETE", "/system/roles/:roleId", "sys:role:delete"),
	}

	tests := []struct {
		name   string
		path   string
		method string
		want   string
	}{
		{
			name:   "brace param",
			path:   "/admin/system/users/101",
			method: "DELETE",
			want:   "sys:user:delete",
		},
		{
			name:   "colon param",
			path:   "/admin/system/roles/202",
			method: "DELETE",
			want:   "sys:role:delete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRequiredPermission(rules, tt.path, tt.method)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestGetRequiredPermissionRequiresMethodMatch(t *testing.T) {
	rules := []PermissionRule{
		mustRule(t, "POST", "/system/users", "sys:user:add"),
	}

	got := getRequiredPermission(rules, "/admin/system/users", "GET")
	if got != "" {
		t.Fatalf("expected no permission, got %q", got)
	}
}

func mustRule(t *testing.T, method, path, permKey string) PermissionRule {
	t.Helper()

	pattern, staticSegs, err := compilePathPattern(path)
	if err != nil {
		t.Fatalf("compilePathPattern(%q): %v", path, err)
	}

	return PermissionRule{
		Method:     method,
		Path:       normalizePath(path),
		PermKey:    permKey,
		Pattern:    pattern,
		StaticSegs: staticSegs,
	}
}
