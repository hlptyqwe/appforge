package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysConfigDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysConfigDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysConfigDeleteLogic {
	return &SysConfigDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysConfigDeleteLogic) SysConfigDelete(in *system.SysConfigDeleteReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if err := l.svcCtx.ConfigModel.Delete(l.ctx, in.GetId()); err != nil {
		return nil, notFound(err, "system config")
	}
	return &system.RespBase{Base: responseBase()}, nil
}
