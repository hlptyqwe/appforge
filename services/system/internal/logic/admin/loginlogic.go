package adminlogic

import (
	"context"
	"strings"
	"time"

	"appforge/proto/common"
	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LoginLogic) Login(in *system.LoginReq) (*system.LoginResp, error) {
	if in == nil || strings.TrimSpace(in.Username) == "" || in.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "username and password are required")
	}
	appScope := int64(in.GetAppScope())
	if appScope == 0 {
		appScope = int64(system.ApplicationScope_APPLICATION_SCOPE_ADMIN)
	}
	user, err := l.svcCtx.UserModel.FindOneByUsername(l.ctx, strings.TrimSpace(in.Username), appScope)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "username or password is incorrect")
	}
	if user.Enabled != int64(1) || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(in.Password)) != nil {
		return nil, status.Error(codes.Unauthenticated, "username or password is incorrect")
	}
	token, exp, err := loginExpire(l.svcCtx.Config.Jwt.AccessSecret, user, l.svcCtx.Config.Jwt.AccessExpire)
	if err != nil {
		return nil, err
	}
	var roleIDs []int64
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &roleIDs, "SELECT role_id FROM sys_user_role WHERE user_id = ? ORDER BY role_id ASC", user.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "query user roles failed: %v", err)
	}
	user.LastLoginIp = strings.TrimSpace(in.Ip)
	user.LastLoginAt = time.Now().UnixMilli()
	if err := l.svcCtx.UserModel.Update(l.ctx, user); err != nil {
		return nil, status.Errorf(codes.Internal, "update login time failed: %v", err)
	}

	return &system.LoginResp{Base: responseBase(), Data: &system.LoginData{
		Token: token, Exp: exp, UserId: user.Id, Nickname: user.Nickname, RoleIds: roleIDs,
		Google2FaEnabled: common.Enable(user.GoogleEnabled), TenantId: user.TenantId,
		UserType: system.UserType(user.UserType), IsOwner: common.YesNo(user.IsOwner),
		Avatar: user.Avatar, AppScope: system.ApplicationScope(user.AppScope),
	}}, nil
}
