package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(txCtx context.Context, session sqlx.Session) error {
		var item models.TStorageObject
		if err := session.QueryRowCtx(txCtx, &item, storageObjectSelect+` WHERE id=? AND tenant_id=? FOR UPDATE`, in.Id, tenant); err != nil {
			return notFoundOrInternal(err, "storage object")
		}
		if item.Status == storageStatusBound || item.Status == storageStatusDeleted {
			return status.Error(codes.FailedPrecondition, "bound or deleted storage object cannot be failed")
		}
		if item.Status == storageStatusUploading {
			if err := releaseQuotaInSession(txCtx, session, tenant, "storage.bytes", storageQuotaKey(item.ObjectKey)); err != nil {
				return err
			}
		}
		item.Status = storageStatusFailed
		if err := l.svcCtx.StorageObjectModel.WithSession(session).Update(txCtx, &item); err != nil {
			return status.Errorf(codes.Internal, "fail storage object failed: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &core.RespBase{Base: okBase()}, nil
}
