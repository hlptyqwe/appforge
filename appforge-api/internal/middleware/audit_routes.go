package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"appforge/admin-api/internal/config"
)

type auditRouteSpec struct {
	method  string
	pattern *regexp.Regexp
}

func buildSensitiveAuditRoutes(routes []config.AuditRoute) ([]auditRouteSpec, error) {
	if len(routes) == 0 {
		return nil, errors.New("Audit.Routes must contain at least one sensitive route")
	}

	result := make([]auditRouteSpec, 0, len(routes))
	seen := make(map[string]struct{}, len(routes))
	for i, route := range routes {
		method := strings.ToUpper(strings.TrimSpace(route.Method))
		path := normalizePath(strings.TrimSpace(route.Path))
		if !isSupportedAuditMethod(method) {
			return nil, fmt.Errorf("Audit.Routes[%d].Method %q is not supported", i, route.Method)
		}
		if !strings.HasPrefix(path, "/admin/") {
			return nil, fmt.Errorf("Audit.Routes[%d].Path %q must start with /admin/", i, route.Path)
		}
		key := method + " " + path
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate audit route %s", key)
		}
		seen[key] = struct{}{}

		pattern, _, err := compilePathPattern(path)
		if err != nil {
			return nil, fmt.Errorf("compile audit route %s: %w", key, err)
		}
		result = append(result, auditRouteSpec{method: method, pattern: pattern})
	}
	return result, nil
}

func isSupportedAuditMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}
