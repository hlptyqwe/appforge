package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SignAirGappedManifestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSignAirGappedManifestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SignAirGappedManifestLogic {
	return &SignAirGappedManifestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 使用控制面Agent CA签署严格规范化任务Manifest。
func (l *SignAirGappedManifestLogic) SignAirGappedManifest(in *core.SignAirGappedManifestReq) (*core.SignAirGappedManifestResp, error) {
	return signAirGappedManifest(l.ctx, l.svcCtx, in)
}
