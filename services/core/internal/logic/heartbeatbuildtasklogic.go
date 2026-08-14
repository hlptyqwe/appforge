package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HeartbeatBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHeartbeatBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HeartbeatBuildTaskLogic {
	return &HeartbeatBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HeartbeatBuildTaskLogic) HeartbeatBuildTask(in *core.HeartbeatBuildTaskReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateBuilderRequest(in.BuilderId); err != nil {
		return nil, err
	}
	if in.TaskId <= 0 || in.BuilderAttempt <= 0 {
		return nil, status.Error(codes.InvalidArgument, "task_id and builder_attempt must be greater than zero")
	}
	seconds := leaseSeconds(in.LeaseSeconds)
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(txCtx context.Context, session sqlx.Session) error {
		result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET lease_until = DATE_ADD(CURRENT_TIMESTAMP(3), INTERVAL ? SECOND),
update_time = CURRENT_TIMESTAMP(3) WHERE status IN (?, ?, ?) AND id = ? AND builder_id = ? AND builder_attempt = ?
AND lease_until > CURRENT_TIMESTAMP(3)`, seconds, buildStatusBuilding, buildStatusSigning, buildStatusUploading,
			in.TaskId, in.BuilderId, in.BuilderAttempt)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return status.Error(codes.NotFound, "build task is not owned by builder or is already finished")
		}
		slotResult, err := session.ExecCtx(txCtx, `UPDATE t_build_slot_lease SET lease_until = DATE_ADD(CURRENT_TIMESTAMP(3), INTERVAL ? SECOND),
update_time = CURRENT_TIMESTAMP(3) WHERE task_id = ? AND node_code = ? AND builder_attempt = ? AND status = ?
AND lease_until > CURRENT_TIMESTAMP(3)`, seconds, in.TaskId, in.BuilderId, in.BuilderAttempt, buildSlotActive)
		if err != nil {
			return err
		}
		slotAffected, err := slotResult.RowsAffected()
		if err != nil || slotAffected != 1 {
			return status.Error(codes.FailedPrecondition, "build slot lease is missing or expired")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return workerBase(), nil
}
