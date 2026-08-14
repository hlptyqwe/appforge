package logic

import (
	"context"
	"path"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MarkStorageObjectDeletedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkStorageObjectDeletedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkStorageObjectDeletedLogic {
	return &MarkStorageObjectDeletedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 确认物理对象已删除并把元数据置为已删除。
func (l *MarkStorageObjectDeletedLogic) MarkStorageObjectDeleted(in *core.MarkStorageObjectDeletedReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.Id, "id"); err != nil {
		return nil, err
	}
	objectKey := strings.TrimSpace(in.ObjectKey)
	if objectKey == "" || len(objectKey) > 500 || path.Clean(objectKey) != objectKey || !strings.HasPrefix(objectKey, "tenants/") {
		return nil, status.Error(codes.InvalidArgument, "object_key is invalid")
	}
	result, err := l.svcCtx.DB.ExecCtx(l.ctx, `UPDATE t_storage_object SET status = ?, update_time = CURRENT_TIMESTAMP
WHERE id = ? AND object_key = ? AND status = ?`, storageStatusDeleted, in.Id, objectKey, storageStatusFailed)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mark storage object deleted failed: %v", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read storage object delete result failed: %v", err)
	}
	if affected != 1 {
		var existing struct {
			Status int64 `db:"status"`
		}
		if queryErr := l.svcCtx.DB.QueryRowCtx(l.ctx, &existing,
			`SELECT status FROM t_storage_object WHERE id = ? AND object_key = ? LIMIT 1`, in.Id, objectKey); queryErr == nil && existing.Status == storageStatusDeleted {
			return workerBase(), nil
		}
		return nil, status.Error(codes.NotFound, "failed storage object was not found")
	}

	return workerBase(), nil
}
