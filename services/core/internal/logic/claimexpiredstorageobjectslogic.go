package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ClaimExpiredStorageObjectsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClaimExpiredStorageObjectsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimExpiredStorageObjectsLogic {
	return &ClaimExpiredStorageObjectsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 领取超时未完成或失败的上传对象，供后台清理物理文件。
func (l *ClaimExpiredStorageObjectsLogic) ClaimExpiredStorageObjects(in *core.ClaimExpiredStorageObjectsReq) (*core.ClaimExpiredStorageObjectsResp, error) {
	if in == nil {
		in = &core.ClaimExpiredStorageObjectsReq{}
	}
	staleSeconds := in.StaleSeconds
	if staleSeconds == 0 {
		staleSeconds = 30 * 60
	}
	if staleSeconds < 60 || staleSeconds > 7*24*60*60 {
		return nil, status.Error(codes.InvalidArgument, "stale_seconds must be between 60 and 604800")
	}
	limit := in.Limit
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 100 {
		return nil, status.Error(codes.InvalidArgument, "limit must be between 1 and 100")
	}

	// Atomically make stale UPLOADING rows ineligible for completion before the
	// physical object is removed. FAILED rows are deliberately retryable: an
	// object-store outage must not make an orphan permanent.
	if _, err := l.svcCtx.DB.ExecCtx(l.ctx, `UPDATE t_storage_object
SET status = ?, update_time = CURRENT_TIMESTAMP
WHERE status = ? AND create_time < DATE_SUB(CURRENT_TIMESTAMP, INTERVAL ? SECOND)
ORDER BY id LIMIT ?`, storageStatusFailed, storageStatusUploading, staleSeconds, limit); err != nil {
		return nil, status.Errorf(codes.Internal, "claim stale storage objects failed: %v", err)
	}

	var rows []struct {
		ID        int64  `db:"id"`
		ObjectKey string `db:"object_key"`
	}
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &rows, `SELECT id, object_key FROM t_storage_object
WHERE status = ? AND create_time < DATE_SUB(CURRENT_TIMESTAMP, INTERVAL ? SECOND)
ORDER BY id LIMIT ?`, storageStatusFailed, staleSeconds, limit); err != nil {
		return nil, status.Errorf(codes.Internal, "list stale storage objects failed: %v", err)
	}
	data := make([]*core.ExpiredStorageObject, 0, len(rows))
	for _, row := range rows {
		data = append(data, &core.ExpiredStorageObject{Id: row.ID, ObjectKey: row.ObjectKey})
	}

	return &core.ClaimExpiredStorageObjectsResp{Base: okBase(), Data: data}, nil
}
