// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package system

import (
	"context"
	"fmt"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"appforge/admin-api/internal/logicutil"
	"appforge/proto/system"

	"github.com/zeromicro/go-zero/core/logx"
)

type SysUserDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSysUserDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysUserDeleteLogic {
	return &SysUserDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SysUserDeleteLogic) SysUserDelete(req *types.SysUserDeleteReq) (resp *types.RespBase, err error) {
	detail, err := l.svcCtx.SystemCli.SysUserDetail(l.ctx, &system.SysUserDetailReq{Id: req.Id})
	if err != nil || detail == nil || detail.Data == nil {
		return nil, err
	}
	resp, err = logicutil.Proxy[types.RespBase](l.ctx, req, l.svcCtx.SystemCli.SysUserDelete)
	if err == nil && detail.Data.TenantId > 0 && int64(detail.Data.Enabled) == 1 {
		err = removeSeatUsage(l.ctx, l.svcCtx, detail.Data.TenantId, req.Id,
			fmt.Sprintf("seat-delete:%d", req.Id), "user_deleted")
	}
	return resp, err
}
