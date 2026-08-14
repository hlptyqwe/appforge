// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/middleware"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OpenV1ListApplicationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1ListApplicationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1ListApplicationsLogic {
	return &OpenV1ListApplicationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1ListApplicationsLogic) OpenV1ListApplications(req *types.OpenListApplicationsReq) (resp *types.PlatformApplicationListResp, err error) {
	principal, ok := middleware.OpenApiPrincipalFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "API credential is required")
	}
	if !middleware.RequireOpenApiScope(l.ctx, core.OpenApiScope_OPEN_API_SCOPE_APPS_READ, 0) {
		return nil, status.Error(codes.PermissionDenied, "apps:read scope is required")
	}
	middleware.SetOpenApiResource(l.ctx, "application", 0)
	appIDs := make([]int64, 0, len(principal.AppIDs))
	for appID := range principal.AppIDs {
		appIDs = append(appIDs, appID)
	}
	item, err := l.svcCtx.CoreCli.ListApplications(l.ctx, &core.ApplicationListReq{
		Page:    platformlogic.PlatformPage(req.PageReq),
		Keyword: req.Keyword,
		Status:  core.ApplicationStatus(req.Status),
		AppIds:  appIDs,
	})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformApplication, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, platformlogic.MapPlatformApplication(value))
	}
	return &types.PlatformApplicationListResp{
		RespBase: platformlogic.PlatformRespBase(item.Base),
		Data:     data,
	}, nil
}
