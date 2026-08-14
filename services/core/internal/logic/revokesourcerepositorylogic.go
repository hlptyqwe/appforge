package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RevokeSourceRepositoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRevokeSourceRepositoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevokeSourceRepositoryLogic {
	return &RevokeSourceRepositoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 撤销仓库授权。
func (l *RevokeSourceRepositoryLogic) RevokeSourceRepository(in *core.SourceRepositoryIdReq) (*core.SourceRepositoryResp, error) {
	return revokeSourceRepository(l.ctx, l.svcCtx, in)
}
