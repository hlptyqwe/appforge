package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetStorageObjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetStorageObjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStorageObjectLogic {
	return &GetStorageObjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询当前租户的私有存储对象。
func (l *GetStorageObjectLogic) GetStorageObject(in *core.StorageObjectIdReq) (*core.StorageObjectResp, error) {
	tenant, err := tenantID(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "storage object id is required")
	}
	item, err := l.svcCtx.StorageObjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "storage object")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}
	if item.Status == storageStatusDeleted {
		return nil, status.Error(codes.NotFound, "storage object not found")
	}
	return &core.StorageObjectResp{Base: okBase(), Data: mapStorageObject(item)}, nil
}
