package adminlogic

import (
	"context"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResetUserPwdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetUserPwdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetUserPwdLogic {
	return &ResetUserPwdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResetUserPwdLogic) ResetUserPwd(in *system.ResetUserPwdReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 || in.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "id and password are required")
	}
	item, err := l.svcCtx.UserModel.FindOne(l.ctx, in.GetId())
	if err != nil {
		return nil, notFound(err, "user")
	}
	if _, err := effectiveTenant(l.ctx, item.TenantId); err != nil {
		return nil, err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(in.GetPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "hash password failed: %v", err)
	}
	item.Password = string(hashed)
	item.PermsVer++
	item.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.UserModel.Update(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "reset password failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
