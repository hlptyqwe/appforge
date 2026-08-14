// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ImportPlatformSourceArtifactLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewImportPlatformSourceArtifactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ImportPlatformSourceArtifactLogic {
	return &ImportPlatformSourceArtifactLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ImportPlatformSourceArtifactLogic) ImportPlatformSourceArtifact(req *types.ImportPlatformSourceArtifactReq) (resp *types.PlatformSourceArtifactImportResp, err error) {
	return importPlatformSourceArtifact(l.ctx, l.svcCtx, req)
}
