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

type Google2FAEnableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogle2FAEnableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *Google2FAEnableLogic {
	return &Google2FAEnableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *Google2FAEnableLogic) Google2FAEnable(in *system.Google2FAEnableReq) (*system.RespBase, error) {
	if in == nil || in.GetUserId() <= 0 || strings.TrimSpace(in.GetCode()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and code are required")
	}
	user, err := l.svcCtx.UserModel.FindOne(l.ctx, in.GetUserId())
	if err != nil {
		return nil, notFound(err, "user")
	}
	if _, err := effectiveTenant(l.ctx, user.TenantId); err != nil {
		return nil, err
	}
	secret := user.GoogleSecret
	if secret == "" {
		secret, err = l.svcCtx.UserModel.GetGoogle2FASecret(l.ctx, user.Id)
		if err != nil {
			return nil, status.Error(codes.FailedPrecondition, "2fa is not initialized")
		}
	}
	if !totp.Validate(strings.TrimSpace(in.GetCode()), secret) {
		return nil, status.Error(codes.InvalidArgument, "invalid 2fa code")
	}
	user.GoogleSecret = secret
	user.GoogleEnabled = 1
	user.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.UserModel.Update(l.ctx, user); err != nil {
		return nil, status.Errorf(codes.Internal, "enable 2fa failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
