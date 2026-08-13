// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	corepb "appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformSigningConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformSigningConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformSigningConfigLogic {
	return &GetPlatformSigningConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformSigningConfigLogic) GetPlatformSigningConfig(req *types.PlatformIdReq) (resp *types.PlatformSigningConfigResp, err error) {
	item, err := l.svcCtx.CoreCli.GetSigningConfig(l.ctx, &corepb.SigningConfigIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformSigningConfigResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformSigningConfig(item.Data)}, nil
}
