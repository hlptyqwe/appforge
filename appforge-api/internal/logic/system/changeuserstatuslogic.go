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

type ChangeUserStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChangeUserStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeUserStatusLogic {
	return &ChangeUserStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChangeUserStatusLogic) ChangeUserStatus(req *types.ChangeUserStatusReq) (resp *types.RespBase, err error) {
	detail, err := l.svcCtx.SystemCli.SysUserDetail(l.ctx, &system.SysUserDetailReq{Id: req.Id})
	if err != nil || detail == nil || detail.Data == nil {
		return nil, err
	}
	if int64(detail.Data.Enabled) == req.Enabled {
		return logicutil.Proxy[types.RespBase](l.ctx, req, l.svcCtx.SystemCli.ChangeUserStatus)
	}
	quotaKey := ""
	if detail.Data.TenantId > 0 && req.Enabled == 1 {
		quotaKey = newSeatQuotaKey(detail.Data.TenantId, fmt.Sprintf("enable-%d", req.Id))
		if err := reserveSeat(l.ctx, l.svcCtx, detail.Data.TenantId, quotaKey); err != nil {
			return nil, err
		}
	}
	resp, err = logicutil.Proxy[types.RespBase](l.ctx, req, l.svcCtx.SystemCli.ChangeUserStatus)
	if err != nil {
		if quotaKey != "" {
			releaseSeat(l.ctx, l.svcCtx, detail.Data.TenantId, quotaKey)
		}
		return nil, err
	}
	if quotaKey != "" {
		err = confirmSeat(l.ctx, l.svcCtx, detail.Data.TenantId, req.Id, quotaKey)
	} else if detail.Data.TenantId > 0 && req.Enabled == 2 {
		err = removeSeatUsage(l.ctx, l.svcCtx, detail.Data.TenantId, req.Id,
			fmt.Sprintf("seat-disable:%d:%d", req.Id, detail.Data.UpdateTimes), "user_disabled")
	}
	return resp, err
}
