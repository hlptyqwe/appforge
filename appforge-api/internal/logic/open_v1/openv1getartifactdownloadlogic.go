// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1GetArtifactDownloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1GetArtifactDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1GetArtifactDownloadLogic {
	return &OpenV1GetArtifactDownloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1GetArtifactDownloadLogic) OpenV1GetArtifactDownload(req *types.PlatformStorageObjectIdReq) (resp *types.PlatformStorageDownloadResp, err error) {
	return openV1GetArtifactDownload(l.ctx, l.svcCtx, req)
}
