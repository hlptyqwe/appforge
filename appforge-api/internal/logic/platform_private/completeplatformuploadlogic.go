// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type CompletePlatformUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCompletePlatformUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompletePlatformUploadLogic {
	return &CompletePlatformUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CompletePlatformUploadLogic) CompletePlatformUpload(req *types.CompletePlatformUploadReq) (resp *types.CompletePlatformUploadResp, err error) {
	item, err := l.svcCtx.CoreCli.GetStorageObject(l.ctx, &core.StorageObjectIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	store, err := platformlogic.LoadObjectStore(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	size, sha256Value, err := platformlogic.VerifyStorageObject(l.ctx, store, item.Data)
	if err != nil {
		platformlogic.CleanupFailedUpload(l.ctx, l.svcCtx, store, item.Data)
		return nil, err
	}
	completed, err := l.svcCtx.CoreCli.CompleteStorageObject(l.ctx, &core.CompleteStorageObjectReq{
		Id: req.Id, SizeBytes: size, Sha256: sha256Value,
	})
	if err != nil {
		return nil, err
	}
	return &types.CompletePlatformUploadResp{
		RespBase: platformlogic.PlatformRespBase(completed.Base),
		Data:     platformlogic.MapPlatformStorageObject(completed.Data),
	}, nil
}
