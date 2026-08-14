package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FailStorageObjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFailStorageObjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FailStorageObjectLogic {
	return &FailStorageObjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 标记上传或校验失败。
func (l *FailStorageObjectLogic) FailStorageObject(in *core.FailStorageObjectReq) (*core.RespBase, error) {
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
	if item.Status == storageStatusBound || item.Status == storageStatusDeleted {
		return nil, status.Error(codes.FailedPrecondition, "bound or deleted storage object cannot be failed")
	}
	item.Status = storageStatusFailed
	if err := l.svcCtx.StorageObjectModel.Update(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "fail storage object failed: %v", err)
	}
	return &core.RespBase{Base: okBase()}, nil
}
