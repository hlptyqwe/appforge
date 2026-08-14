// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1InitiateUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1InitiateUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1InitiateUploadLogic {
	return &OpenV1InitiateUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1InitiateUploadLogic) OpenV1InitiateUpload(req *types.InitiatePlatformUploadReq) (resp *types.InitiatePlatformUploadResp, err error) {
	return openV1InitiateUpload(l.ctx, l.svcCtx, req)
}
