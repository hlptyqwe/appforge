package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelBuildTaskLogic {
	return &CancelBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 取消待处理或执行中的V4构建任务并立即推进fencing代次。
func (l *CancelBuildTaskLogic) CancelBuildTask(in *core.CancelBuildTaskReq) (*core.BuildTaskResp, error) {
	return cancelBuildTask(l.ctx, l.svcCtx, in)
}
