// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package system

import (
	"context"

	"appforge/admin-api/internal/logicutil"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/common"
	systempb "appforge/proto/system"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpLogListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpLogListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpLogListLogic {
	return &OpLogListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpLogListLogic) OpLogList(req *types.OpLogListReq) (resp *types.OpLogListResp, err error) {
	result, err := l.svcCtx.SystemCli.OpLogList(l.ctx, &systempb.OpLogListReq{
		Page: &common.PageReq{
			Cursor: req.Cursor,
			Limit:  req.Limit,
			Count:  req.Count,
		},
		Username: req.Username,
		Method:   toRequestMethod(req.Method),
		Path:     req.Path,
	})
	if err != nil {
		return logicutil.SystemErrorResp[types.OpLogListResp](l.ctx, err)
	}

	resp = &types.OpLogListResp{Data: make([]types.OpLogItem, 0, len(result.Data))}
	if result.Base != nil {
		resp.Code = result.Base.Code
		resp.Msg = result.Base.Msg
		resp.Total = result.Base.Total
		resp.HasNext = result.Base.HasNext
		resp.HasPrev = result.Base.HasPrev
		resp.NextCursor = result.Base.NextCursor
		resp.PrevCursor = result.Base.PrevCursor
	}
	for _, item := range result.Data {
		if item == nil {
			continue
		}
		resp.Data = append(resp.Data, mapOpLogItem(item))
	}

	return resp, nil
}

func mapOpLogItem(item *systempb.OpLogItem) types.OpLogItem {
	return types.OpLogItem{
		Id:          item.Id,
		TenantId:    item.TenantId,
		UserId:      item.UserId,
		Username:    item.Username,
		Module:      item.Module,
		Action:      item.Action,
		Method:      fromRequestMethod(item.Method),
		Path:        item.Path,
		Req:         item.Req,
		Resp:        item.Resp,
		Ip:          item.Ip,
		CostMs:      item.CostMs,
		CreateTimes: item.CreateTimes,
		UpdateTimes: item.UpdateTimes,
	}
}
