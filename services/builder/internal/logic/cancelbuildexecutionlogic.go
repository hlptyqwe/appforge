package logic

import (
	"context"

	"appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelBuildExecutionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelBuildExecutionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelBuildExecutionLogic {
	return &CancelBuildExecutionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 取消V4构建执行并推进fencing代次。
func (l *CancelBuildExecutionLogic) CancelBuildExecution(in *builder.CancelBuildExecutionReq) (*builder.BuildTaskResp, error) {
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.CancelBuildTask(toCoreContext(l.ctx), &core.CancelBuildTaskReq{Id: in.GetId(), Reason: in.GetReason()})
	if err != nil {
		return nil, err
	}
	return &builder.BuildTaskResp{Base: resp.Base, Data: mapBuildTask(resp.Data)}, nil
}
