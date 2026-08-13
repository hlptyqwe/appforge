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
	protoReq := &system.LoginReq{
		Username:   req.Username,
		Password:   req.Password,
		GoogleCode: req.GoogleCode,
		Ip:         ip,
		AppScope:   system.ApplicationScope_APPLICATION_SCOPE_ADMIN,
	}
	return logicutil.Proxy[types.LoginResp](l.ctx, protoReq, l.svcCtx.SystemCli.Login)
}
