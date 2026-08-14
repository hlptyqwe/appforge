// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package auth_public

import (
	"context"

	"appforge/admin-api/internal/logicutil"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/system"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginReq, ip string) (resp *types.LoginResp, err error) {
	return l.LoginWithScope(req, ip, system.ApplicationScope_APPLICATION_SCOPE_ADMIN)
}

// LoginWithScope 使用服务端固定的应用范围登录，客户端不能自行选择管理端或代理端。
func (l *LoginLogic) LoginWithScope(req *types.LoginReq, ip string, appScope system.ApplicationScope) (resp *types.LoginResp, err error) {
	protoReq := &system.LoginReq{
		Username:   req.Username,
		Password:   req.Password,
		GoogleCode: req.GoogleCode,
		Ip:         ip,
		AppScope:   appScope,
	}
	return logicutil.Proxy[types.LoginResp](l.ctx, protoReq, l.svcCtx.SystemCli.Login)
}
