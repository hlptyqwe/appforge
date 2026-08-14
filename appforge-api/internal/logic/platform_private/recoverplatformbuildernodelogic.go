// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type RecoverPlatformBuilderNodeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRecoverPlatformBuilderNodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RecoverPlatformBuilderNodeLogic {
	return &RecoverPlatformBuilderNodeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RecoverPlatformBuilderNodeLogic) RecoverPlatformBuilderNode(req *types.RecoverPlatformBuilderNodeReq) (resp *types.PlatformBuilderNodeResp, err error) {
	item, err := l.svcCtx.CoreCli.RecoverBuilderNode(l.ctx, &core.RecoverBuilderNodeReq{Id: req.Id, Reason: req.Reason})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBuilderNodeResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBuilderNode(item.Data)}, nil
}
