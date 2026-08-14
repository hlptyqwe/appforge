package middleware

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"appforge/admin-api/internal/svc"
	"appforge/common/utils"
	"appforge/proto/system"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/zeromicro/go-zero/core/logx"
)

type PermissionRule struct {
	AppScope   system.ApplicationScope
	Method     string
	Path       string
	PermKey    string
	Pattern    *regexp.Regexp
	StaticSegs int
}

type RbacMiddleware struct {
	svcCtx *svc.ServiceContext
	mu     sync.RWMutex
	rules  []PermissionRule
}

func NewRbacMiddleware(svcCtx *svc.ServiceContext) *RbacMiddleware {
	m := &RbacMiddleware{
		svcCtx: svcCtx,
	}
	m.refreshRules(context.Background())

	return m
}

func (m *RbacMiddleware) refreshRules(ctx context.Context) {
	rules, err := loadPermissionRules(ctx, m.svcCtx)
	if err != nil {
		logx.Errorf("fetch system permissions failed: %v", err)
		return
	}

	m.mu.Lock()
	m.rules = rules
	m.mu.Unlock()

	logx.Infof("loaded rbac permission rules: %d", len(rules))
}

func loadPermissionRules(ctx context.Context, svcCtx *svc.ServiceContext) ([]PermissionRule, error) {
	result, err := svcCtx.SystemCli.SysPermList(ctx, &system.Empty{})
	if err != nil {
		return nil, err
	}

	rules := make([]PermissionRule, 0)
	if result != nil {
		rules = make([]PermissionRule, 0, len(result.Data))
		for _, item := range result.Data {
			pattern, staticSegs, err := compilePathPattern(item.Path)
			if err != nil {
				logx.Errorf("compile path pattern failed: method=%s path=%s err=%v", item.Method, item.Path, err)
				continue
			}

			method := strings.TrimPrefix(item.Method.String(), "REQUEST_METHOD_")
			if item.Method == system.RequestMethod_REQUEST_METHOD_UNKNOWN {
				method = ""
			}

			rules = append(rules, PermissionRule{
				AppScope:   item.AppScope,
				Method:     strings.ToUpper(strings.TrimSpace(method)),
				Path:       normalizePath(item.Path),
				PermKey:    item.PermKey,
				Pattern:    pattern,
				StaticSegs: staticSegs,
			})
		}
	}

	return rules, nil
}

func (m *RbacMiddleware) requiredPermission(ctx context.Context, appScope system.ApplicationScope, path, method string) string {
	m.mu.RLock()
	requiredPerm := getRequiredPermission(m.rules, appScope, path, method)
	m.mu.RUnlock()
	if requiredPerm != "" {
		return requiredPerm
	}

	m.refreshRules(ctx)

	m.mu.RLock()
	requiredPerm = getRequiredPermission(m.rules, appScope, path, method)
	m.mu.RUnlock()

	return requiredPerm
}

func (m *RbacMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		method := r.Method

		if isPublicPath(path) {
			next(w, r)
			return
		}
		routeScope := applicationScopeForPath(path)
		if routeScope == system.ApplicationScope_APPLICATION_SCOPE_UNKNOWN {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		userId, err := utils.GetUserIdFromCtx(r.Context())
		if err != nil {
			logx.Errorf("invalid token: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		resp, err := m.svcCtx.SystemCli.LoginUserPerms(r.Context(), &system.LoginUserPermsReq{
			UserId: userId,
		})
		if err != nil {
			logx.Errorf("get profile failed, userId=%d err=%v", userId, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		userResp, err := m.svcCtx.SystemCli.SysUserDetail(r.Context(), &system.SysUserDetailReq{
			Id: userId,
		})
		if err != nil || userResp == nil || userResp.Data == nil {
			logx.Errorf("get user detail failed, userId=%d err=%v", userId, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		setAuditActor(r.Context(), userResp.Data.Id, userResp.Data.Username, userResp.Data.TenantId)
		if userResp.Data.AppScope != routeScope {
			logx.Errorf("application scope mismatch, userId=%d userScope=%s routeScope=%s path=%s",
				userId, userResp.Data.AppScope, routeScope, path)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), utils.CtxKeyUid, userResp.Data.Id)
		ctx = context.WithValue(ctx, utils.CtxKeyUsername, userResp.Data.Username)
		ctx = context.WithValue(ctx, utils.CtxKeyTenantId, userResp.Data.TenantId)
		ctx = context.WithValue(ctx, utils.CtxKeyUserType, int64(userResp.Data.UserType))
		ctx = context.WithValue(ctx, utils.CtxKeyAppScope, int64(userResp.Data.AppScope))
		r = r.WithContext(ctx)

		if isProfilePath(path) {
			next(w, r)
			return
		}

		requiredPerm := m.requiredPermission(r.Context(), routeScope, path, method)
		if requiredPerm == "" {
			logx.Errorf("permission route not found, method=%s path=%s", method, path)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		setAuditPermission(r.Context(), requiredPerm)

		enforcer, err := newUserPermEnforcer(fmt.Sprintf("%d", userId), resp.Perms)
		if err != nil {
			logx.Errorf("create casbin enforcer failed: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		obj, act, ok := parsePerm(requiredPerm)
		if !ok {
			logx.Errorf("invalid required permission format: %s", requiredPerm)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		allowed, err := enforcer.Enforce(fmt.Sprintf("%d", userId), obj, act)
		if err != nil {
			logx.Errorf("casbin enforce failed, userId=%d perm=%s err=%v", userId, requiredPerm, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if !allowed {
			logx.Errorf("forbidden, userId=%d method=%s path=%s required=%s userPerms=%v",
				userId, method, path, requiredPerm, resp.Perms)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

// newUserPermEnforcer 创建“用户直接权限”模式的 Enforcer
func newUserPermEnforcer(userID string, perms []string) (*casbin.Enforcer, error) {
	modelText := `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
`

	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, err
	}

	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}

	for _, perm := range perms {
		obj, act, ok := parsePerm(perm)
		if !ok {
			logx.Errorf("skip invalid perm: %s", perm)
			continue
		}

		_, err = enforcer.AddPolicy(userID, obj, act)
		if err != nil {
			return nil, err
		}
	}

	return enforcer, nil
}

func parsePerm(perm string) (obj string, act string, ok bool) {
	perm = strings.TrimSpace(perm)
	if perm == "" {
		return "", "", false
	}

	parts := strings.Split(perm, ":")
	if len(parts) < 2 {
		return "", "", false
	}

	act = strings.TrimSpace(parts[len(parts)-1])
	obj = strings.TrimSpace(strings.Join(parts[:len(parts)-1], ":"))

	if obj == "" || act == "" {
		return "", "", false
	}

	return obj, act, true
}

func isPublicPath(path string) bool {
	if strings.HasPrefix(path, "/d/") {
		return true
	}
	whiteList := map[string]struct{}{
		"/admin/system/core":       {},
		"/admin/system/auth/login": {},
		"/agent/core":              {},
		"/agent/auth/login":        {},
		"/api/install/report":      {},
		"/api/channel/event":       {},
		"/health":                  {},
	}

	_, ok := whiteList[path]
	return ok
}

func isProfilePath(path string) bool {
	return path == "/admin/system/auth/profile" || path == "/agent/auth/profile"
}

func applicationScopeForPath(path string) system.ApplicationScope {
	switch {
	case path == "/admin" || strings.HasPrefix(path, "/admin/"):
		return system.ApplicationScope_APPLICATION_SCOPE_ADMIN
	case path == "/agent" || strings.HasPrefix(path, "/agent/"):
		return system.ApplicationScope_APPLICATION_SCOPE_AGENT
	default:
		return system.ApplicationScope_APPLICATION_SCOPE_UNKNOWN
	}
}

func getRequiredPermission(rules []PermissionRule, appScope system.ApplicationScope, path, method string) string {
	path = strings.TrimSpace(path)

	if strings.HasPrefix(path, "/admin/") {
		path = strings.TrimPrefix(path, "/admin")
	} else if path == "/admin" {
		path = "/"
	} else if strings.HasPrefix(path, "/agent/") {
		path = strings.TrimPrefix(path, "/agent")
	} else if path == "/agent" {
		path = "/"
	}

	path = normalizePath(path)
	method = strings.ToUpper(strings.TrimSpace(method))

	var matched *PermissionRule

	for i := range rules {
		rule := &rules[i]
		if rule.AppScope != appScope {
			continue
		}
		if rule.Method != method {
			continue
		}
		if !rule.Pattern.MatchString(path) {
			continue
		}

		if matched == nil || rule.StaticSegs > matched.StaticSegs {
			matched = rule
		}
	}

	if matched == nil {
		return ""
	}

	return matched.PermKey
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}

	return path
}

// compilePathPattern
// /member/users/{id}            -> ^/member/users/[^/]+$
// /member/users/{id}/status     -> ^/member/users/[^/]+/status$
// /dept/{deptId}/users/{userId} -> ^/dept/[^/]+/users/[^/]+$
func compilePathPattern(route string) (*regexp.Regexp, int, error) {
	route = normalizePath(route)

	parts := strings.Split(route, "/")
	staticSegs := 0

	for i, part := range parts {
		if part == "" {
			continue
		}

		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			parts[i] = `[^/]+`
		} else if strings.HasPrefix(part, ":") {
			parts[i] = `[^/]+`
		} else {
			staticSegs++
		}
	}

	pattern := "^" + strings.Join(parts, "/") + "$"
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return nil, 0, err
	}

	return reg, staticSegs, nil
}
