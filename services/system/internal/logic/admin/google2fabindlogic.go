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

type Google2FABindLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogle2FABindLogic(ctx context.Context, svcCtx *svc.ServiceContext) *Google2FABindLogic {
	return &Google2FABindLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *Google2FABindLogic) Google2FABind(in *system.Google2FABindReq) (*system.RespBase, error) {
	if in == nil || in.GetUserId() <= 0 || strings.TrimSpace(in.GetSecret()) == "" || strings.TrimSpace(in.GetCode()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id, secret and code are required")
	}
	user, err := l.svcCtx.UserModel.FindOne(l.ctx, in.GetUserId())
	if err != nil {
		return nil, notFound(err, "user")
	}
	if _, err := effectiveTenant(l.ctx, user.TenantId); err != nil {
		return nil, err
	}
	if !totp.Validate(strings.TrimSpace(in.GetCode()), strings.TrimSpace(in.GetSecret())) {
		return nil, status.Error(codes.InvalidArgument, "invalid 2fa code")
	}
	user.GoogleSecret = strings.TrimSpace(in.GetSecret())
	user.GoogleEnabled = 1
	user.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.UserModel.Update(l.ctx, user); err != nil {
		return nil, status.Errorf(codes.Internal, "bind 2fa failed: %v", err)
	}
	_ = l.svcCtx.UserModel.DeleteGoogle2FASecret(l.ctx, user.Id)
	return &system.RespBase{Base: responseBase()}, nil
}
