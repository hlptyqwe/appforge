package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSourceRepositoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSourceRepositoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSourceRepositoryLogic {
	return &GetSourceRepositoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询单个授权仓库。
func (l *GetSourceRepositoryLogic) GetSourceRepository(in *core.SourceRepositoryIdReq) (*core.SourceRepositoryResp, error) {
	return getSourceRepository(l.ctx, l.svcCtx, in)
}
