package adminlogic

import (
	"context"
	"strings"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateOpLogLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOpLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOpLogLogic {
	return &CreateOpLogLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateOpLogLogic) CreateOpLog(in *system.CreateOpLogReq) (*system.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	item := &models.SysOpLog{
		TenantId: in.GetTenantId(), UserId: in.GetUserId(), Username: strings.TrimSpace(in.GetUsername()),
		Module: strings.TrimSpace(in.GetModule()), Action: strings.TrimSpace(in.GetAction()),
		Method: methodValue(in.GetMethod()), Path: strings.TrimSpace(in.GetPath()),
		Req: nullText(in.GetReq()), Resp: nullText(in.GetResp()), Ip: strings.TrimSpace(in.GetIp()),
		CostMs: in.GetCostMs(), CreateTimes: time.Now().UnixMilli(), UpdateTimes: time.Now().UnixMilli(),
	}
	if _, err := l.svcCtx.OpLogModel.Insert(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "create operation log failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
