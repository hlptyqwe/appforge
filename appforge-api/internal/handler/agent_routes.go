package handler

import (
	"net/http"
	"time"

	authPrivate "appforge/admin-api/internal/handler/auth_private"
	authPublic "appforge/admin-api/internal/handler/auth_public"
	platform "appforge/admin-api/internal/handler/platform_private"
	systemHandler "appforge/admin-api/internal/handler/system"
	"appforge/admin-api/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

// RegisterAgentHandlers 注册与平台管理端隔离的代理端路由。业务处理仍复用
// 同一组租户安全逻辑，但认证作用域、URL 前缀和 RBAC 菜单均独立。
func RegisterAgentHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{Method: http.MethodPost, Path: "/auth/login", Handler: authPublic.AgentLoginHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/core", Handler: authPublic.GetSystemCoreHandler(serverCtx)},
	}, rest.WithPrefix("/agent"))

	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/auth/profile", Handler: authPrivate.ProfileHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/auth/profile", Handler: authPrivate.UpdateProfileHandler(serverCtx)},
	}, rest.WithJwt(serverCtx.Config.Jwt.AccessSecret), rest.WithPrefix("/agent"))

	server.AddRoutes([]rest.Route{
		{Method: http.MethodPost, Path: "/applications", Handler: platform.CreatePlatformApplicationHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/applications", Handler: platform.ListPlatformApplicationsHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/applications/:id", Handler: platform.GetPlatformApplicationHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/versions", Handler: platform.CreatePlatformVersionHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/versions", Handler: platform.ListPlatformVersionsHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/versions/:id", Handler: platform.GetPlatformVersionHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/signing-configs", Handler: platform.CreatePlatformSigningConfigHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/signing-configs", Handler: platform.ListPlatformSigningConfigsHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/signing-configs/:id", Handler: platform.GetPlatformSigningConfigHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/branding-profiles", Handler: platform.CreatePlatformBrandingProfileHandler(serverCtx)},
		{Method: http.MethodPut, Path: "/branding-profiles/:id", Handler: platform.UpdatePlatformBrandingProfileHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/branding-profiles", Handler: platform.ListPlatformBrandingProfilesHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/branding-profiles/:id", Handler: platform.GetPlatformBrandingProfileHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/branding-profiles/:id/status", Handler: platform.ChangePlatformBrandingProfileStatusHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/branding-profiles/:id/preflight", Handler: platform.CreatePlatformBrandingPreflightHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/branding-preflights", Handler: platform.ListPlatformBrandingPreflightsHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/branding-preflights/:id", Handler: platform.GetPlatformBrandingPreflightHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/white-label/templates", Handler: platform.CreatePlatformWhiteLabelTemplateHandler(serverCtx)},
		{Method: http.MethodPut, Path: "/white-label/templates/:id", Handler: platform.UpdatePlatformWhiteLabelTemplateHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/white-label/templates/:id/copy", Handler: platform.CopyPlatformWhiteLabelTemplateHandler(serverCtx)},
		{Method: http.MethodDelete, Path: "/white-label/templates/:id", Handler: platform.DeletePlatformWhiteLabelTemplateHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/white-label/templates", Handler: platform.ListPlatformWhiteLabelTemplatesHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/white-label/templates/:id", Handler: platform.GetPlatformWhiteLabelTemplateHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/white-label/templates/:id/revisions", Handler: platform.CreatePlatformWhiteLabelTemplateRevisionHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/white-label/templates/:id/revisions/:revision", Handler: platform.GetPlatformWhiteLabelTemplateRevisionHandler(serverCtx)},
		{Method: http.MethodPut, Path: "/white-label/templates/:id/revisions/:revision", Handler: platform.UpdatePlatformWhiteLabelTemplateRevisionHandler(serverCtx)},
		{Method: http.MethodDelete, Path: "/white-label/templates/:id/revisions/:revision", Handler: platform.DeletePlatformWhiteLabelTemplateRevisionHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/white-label/templates/:id/revisions", Handler: platform.ListPlatformWhiteLabelTemplateRevisionsHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/white-label/templates/:id/publish", Handler: platform.PublishPlatformWhiteLabelTemplateHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/white-label/templates/:id/status", Handler: platform.ChangePlatformWhiteLabelTemplateStatusHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/white-label/products", Handler: platform.CreatePlatformWhiteLabelProductHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/white-label/products", Handler: platform.ListPlatformWhiteLabelProductsHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/white-label/products/:id", Handler: platform.GetPlatformWhiteLabelProductHandler(serverCtx)},
		{Method: http.MethodPut, Path: "/white-label/products/:id", Handler: platform.UpdatePlatformWhiteLabelProductHandler(serverCtx)},
		{Method: http.MethodDelete, Path: "/white-label/products/:id", Handler: platform.DeletePlatformWhiteLabelProductHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/white-label/products/:id/status", Handler: platform.ChangePlatformWhiteLabelProductStatusHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/white-label/products/:id/preflight", Handler: platform.PreflightPlatformWhiteLabelProductHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/channels", Handler: platform.CreatePlatformChannelHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/channels", Handler: platform.ListPlatformChannelsHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/channels/:id", Handler: platform.GetPlatformChannelHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/build-tasks", Handler: platform.CreatePlatformBuildTaskHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/build-tasks", Handler: platform.ListPlatformBuildTasksHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/build-tasks/:id", Handler: platform.GetPlatformBuildTaskHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/channel-stats", Handler: platform.GetPlatformChannelStatsHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/uploads/initiate", Handler: platform.InitiatePlatformUploadHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/uploads/:id/complete", Handler: platform.CompletePlatformUploadHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/storage/objects/:id/download", Handler: platform.GetPlatformStorageDownloadHandler(serverCtx)},
	}, rest.WithJwt(serverCtx.Config.Jwt.AccessSecret), rest.WithPrefix("/agent/core"), rest.WithTimeout(600000*time.Millisecond))

	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/users", Handler: systemHandler.SysUserListHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/users/:id", Handler: systemHandler.SysUserDetailHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/users", Handler: systemHandler.SysUserCreateHandler(serverCtx)},
		{Method: http.MethodPut, Path: "/users", Handler: systemHandler.SysUserUpdateHandler(serverCtx)},
		{Method: http.MethodDelete, Path: "/users/:id", Handler: systemHandler.SysUserDeleteHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/users/status", Handler: systemHandler.ChangeUserStatusHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/users/resetPwd", Handler: systemHandler.ResetUserPwdHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/users/assignRoles", Handler: systemHandler.AssignUserRolesHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/roles", Handler: systemHandler.SysRoleListHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/roles", Handler: systemHandler.SysRoleCreateHandler(serverCtx)},
		{Method: http.MethodPut, Path: "/roles", Handler: systemHandler.SysRoleUpdateHandler(serverCtx)},
		{Method: http.MethodDelete, Path: "/roles/:id", Handler: systemHandler.SysRoleDeleteHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/roles/:id/grant", Handler: systemHandler.SysRoleGrantDetailHandler(serverCtx)},
		{Method: http.MethodPost, Path: "/roles/grant", Handler: systemHandler.SysRoleGrantHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/perms", Handler: systemHandler.SysPermListHandler(serverCtx)},
		{Method: http.MethodGet, Path: "/menus/tree/:roleId", Handler: systemHandler.SysMenuTreeHandler(serverCtx)},
	}, rest.WithJwt(serverCtx.Config.Jwt.AccessSecret), rest.WithPrefix("/agent/team"))
}
