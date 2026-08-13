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

type Google2FAResetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogle2FAResetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *Google2FAResetLogic {
	return &Google2FAResetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *Google2FAResetLogic) Google2FAReset(in *system.Google2FAResetReq) (*system.RespBase, error) {
	if in == nil || in.GetUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	user, err := l.svcCtx.UserModel.FindOne(l.ctx, in.GetUserId())
	if err != nil {
		return nil, notFound(err, "user")
	}
	if _, err := effectiveTenant(l.ctx, user.TenantId); err != nil {
		return nil, err
	}
	user.GoogleSecret = ""
	user.GoogleEnabled = 2
	user.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.UserModel.Update(l.ctx, user); err != nil {
		return nil, status.Errorf(codes.Internal, "reset 2fa failed: %v", err)
	}
	_ = l.svcCtx.UserModel.DeleteGoogle2FASecret(l.ctx, user.Id)
	return &system.RespBase{Base: responseBase()}, nil
}
