// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1CompleteUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1CompleteUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1CompleteUploadLogic {
	return &OpenV1CompleteUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1CompleteUploadLogic) OpenV1CompleteUpload(req *types.CompletePlatformUploadReq) (resp *types.CompletePlatformUploadResp, err error) {
	return openV1CompleteUpload(l.ctx, l.svcCtx, req)
}
