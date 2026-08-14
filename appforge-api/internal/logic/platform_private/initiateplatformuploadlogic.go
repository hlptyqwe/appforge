// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"
	"time"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type InitiatePlatformUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewInitiatePlatformUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InitiatePlatformUploadLogic {
	return &InitiatePlatformUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *InitiatePlatformUploadLogic) InitiatePlatformUpload(req *types.InitiatePlatformUploadReq) (resp *types.InitiatePlatformUploadResp, err error) {
	store, err := platformlogic.LoadObjectStore(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	objectType := core.StorageObjectType(req.ObjectType)
	objectKey, err := platformlogic.GenerateStorageObjectKey(l.ctx, objectType, req.FileName)
	if err != nil {
		return nil, err
	}
	item, err := l.svcCtx.CoreCli.CreateStorageObject(l.ctx, &core.CreateStorageObjectReq{
		AppId: req.AppId, ObjectType: objectType, ObjectKey: objectKey,
		OriginalName: req.FileName, ContentType: req.ContentType, SizeBytes: req.SizeBytes,
	})
	if err != nil {
		return nil, err
	}
	expires := 15 * time.Minute
	uploadURL, err := store.PresignPut(l.ctx, objectKey, expires)
	if err != nil {
		_, _ = l.svcCtx.CoreCli.FailStorageObject(l.ctx, &core.FailStorageObjectReq{Id: item.Data.Id})
		return nil, err
	}
	expiresAt := time.Now().Add(expires).Unix()
	return &types.InitiatePlatformUploadResp{
		RespBase: platformlogic.PlatformRespBase(item.Base),
		Data:     types.PlatformUploadTicket{ObjectId: item.Data.Id, UploadUrl: uploadURL, ExpiresAt: expiresAt},
	}, nil
}
