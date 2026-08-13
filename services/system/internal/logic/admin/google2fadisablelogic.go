package adminlogic

import (
	"context"
	"strings"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/pquerna/otp/totp"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Google2FADisableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogle2FADisableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *Google2FADisableLogic {
	return &Google2FADisableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *Google2FADisableLogic) Google2FADisable(in *system.Google2FADisableReq) (*system.RespBase, error) {
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
	if code := strings.TrimSpace(in.GetCode()); code != "" && user.GoogleSecret != "" && !totp.Validate(code, user.GoogleSecret) {
		return nil, status.Error(codes.InvalidArgument, "invalid 2fa code")
	}
	user.GoogleEnabled = 2
	user.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.UserModel.Update(l.ctx, user); err != nil {
		return nil, status.Errorf(codes.Internal, "disable 2fa failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
