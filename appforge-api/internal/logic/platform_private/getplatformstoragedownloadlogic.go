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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetPlatformStorageDownloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformStorageDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformStorageDownloadLogic {
	return &GetPlatformStorageDownloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformStorageDownloadLogic) GetPlatformStorageDownload(req *types.PlatformStorageObjectIdReq) (resp *types.PlatformStorageDownloadResp, err error) {
	item, err := l.svcCtx.CoreCli.GetStorageObject(l.ctx, &core.StorageObjectIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	if item.Data.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE {
		return nil, status.Error(codes.PermissionDenied, "keystore download is restricted to Builder workers")
	}
	if item.Data.Status != core.StorageObjectStatus_STORAGE_OBJECT_STATUS_READY &&
		item.Data.Status != core.StorageObjectStatus_STORAGE_OBJECT_STATUS_BOUND {
		return nil, status.Error(codes.FailedPrecondition, "storage object is not available")
	}
	store, err := platformlogic.LoadObjectStore(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	expires := 5 * time.Minute
	downloadURL, err := store.PresignGet(l.ctx, item.Data.ObjectKey, expires)
	if err != nil {
		return nil, err
	}
	return &types.PlatformStorageDownloadResp{
		RespBase: platformlogic.PlatformRespBase(item.Base),
		Data: types.PlatformStorageDownload{
			DownloadUrl: downloadURL,
			ExpiresAt:   time.Now().Add(expires).Unix(),
		},
	}, nil
}
