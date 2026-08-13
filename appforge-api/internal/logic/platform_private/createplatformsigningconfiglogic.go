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

type CreatePlatformSigningConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformSigningConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformSigningConfigLogic {
	return &CreatePlatformSigningConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformSigningConfigLogic) CreatePlatformSigningConfig(req *types.CreatePlatformSigningConfigReq) (resp *types.PlatformSigningConfigResp, err error) {
	item, err := l.svcCtx.CoreCli.CreateSigningConfig(l.ctx, &corepb.CreateSigningConfigReq{
		AppId: req.AppId, Name: req.Name, KeystoreObjectKey: req.KeystoreObjectKey,
		KeyAlias: req.KeyAlias, KeystorePasswordCiphertext: req.KeystorePasswordCiphertext,
		KeyPasswordCiphertext: req.KeyPasswordCiphertext, SecretRef: req.SecretRef,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformSigningConfigResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformSigningConfig(item.Data)}, nil
}
