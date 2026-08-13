// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	corepb "appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePlatformVersionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformVersionLogic {
	return &CreatePlatformVersionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformVersionLogic) CreatePlatformVersion(req *types.CreatePlatformVersionReq) (resp *types.PlatformVersionResp, err error) {
	item, err := l.svcCtx.CoreCli.CreateVersion(l.ctx, &corepb.CreateVersionReq{AppId: req.AppId, VersionCode: req.VersionCode, VersionName: req.VersionName, SourceApkUrl: req.SourceApkUrl, SourceApkSha256: req.SourceApkSha256, ReleaseNotes: req.ReleaseNotes, BuildConfigJson: req.BuildConfigJson})
	if err != nil {
		return nil, err
	}
	return &types.PlatformVersionResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformVersion(item.Data)}, nil
}
