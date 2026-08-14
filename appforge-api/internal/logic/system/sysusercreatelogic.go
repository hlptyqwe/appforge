// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package system

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"appforge/admin-api/internal/logicutil"
	"appforge/common/utils"
	"appforge/proto/system"

	"github.com/zeromicro/go-zero/core/logx"
)

type SysUserCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSysUserCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysUserCreateLogic {
	return &SysUserCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SysUserCreateLogic) SysUserCreate(req *types.SysUserCreateReq) (resp *types.RespBase, err error) {
	tenantID, _ := utils.GetTenantIdFromCtx(l.ctx)
	shouldCountSeat := tenantID > 0 && req.Enabled != 2
	quotaKey := ""
	if shouldCountSeat {
		quotaKey = newSeatQuotaKey(tenantID, "create")
		if err := reserveSeat(l.ctx, l.svcCtx, tenantID, quotaKey); err != nil {
			return nil, err
		}
	}
	resp, err = logicutil.Proxy[types.RespBase](l.ctx, req, l.svcCtx.SystemCli.SysUserCreate)
	if err != nil {
		if shouldCountSeat {
			releaseSeat(l.ctx, l.svcCtx, tenantID, quotaKey)
		}
		return nil, err
	}
	if shouldCountSeat {
		list, listErr := l.svcCtx.SystemCli.SysUserList(l.ctx, &system.SysUserListReq{Keyword: req.Username})
		if listErr != nil {
			return nil, listErr
		}
		var userID int64
		for _, item := range list.Data {
			if item.Username == req.Username && item.TenantId == tenantID {
				userID = item.Id
				break
			}
		}
		if err := confirmSeat(l.ctx, l.svcCtx, tenantID, userID, quotaKey); err != nil {
			return nil, err
		}
	}
	return resp, nil
}
