package adminlogic

import (
	"context"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ChangeUserStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangeUserStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeUserStatusLogic {
	return &ChangeUserStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChangeUserStatusLogic) ChangeUserStatus(in *system.ChangeUserStatusReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 || in.GetEnabled() == 0 {
		return nil, status.Error(codes.InvalidArgument, "id and enabled are required")
	}
	item, err := l.svcCtx.UserModel.FindOne(l.ctx, in.GetId())
	if err != nil {
		return nil, notFound(err, "user")
	}
	if _, err := effectiveTenant(l.ctx, item.TenantId); err != nil {
		return nil, err
	}
	item.Enabled = int64(in.GetEnabled())
	item.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.UserModel.Update(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "change user status failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
