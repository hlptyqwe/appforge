package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthorizeSourceRepositoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAuthorizeSourceRepositoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthorizeSourceRepositoryLogic {
	return &AuthorizeSourceRepositoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 同步OAuth/App明确授权的只读仓库。
func (l *AuthorizeSourceRepositoryLogic) AuthorizeSourceRepository(in *core.AuthorizeSourceRepositoryReq) (*core.SourceRepositoryResp, error) {
	return authorizeSourceRepository(l.ctx, l.svcCtx, in)
}
