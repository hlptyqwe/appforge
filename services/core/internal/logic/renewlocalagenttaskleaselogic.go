package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RenewLocalAgentTaskLeaseLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRenewLocalAgentTaskLeaseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RenewLocalAgentTaskLeaseLogic {
	return &RenewLocalAgentTaskLeaseLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Agent按任务attempt续租，旧进程不能回写。
func (l *RenewLocalAgentTaskLeaseLogic) RenewLocalAgentTaskLease(in *core.RenewLocalAgentTaskLeaseReq) (*core.RespBase, error) {
	return renewLocalAgentTaskLease(l.ctx, l.svcCtx, in)
}
