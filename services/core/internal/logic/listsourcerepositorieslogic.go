package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSourceRepositoriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSourceRepositoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSourceRepositoriesLogic {
	return &ListSourceRepositoriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询授权仓库。
func (l *ListSourceRepositoriesLogic) ListSourceRepositories(in *core.SourceRepositoryListReq) (*core.SourceRepositoryListResp, error) {
	return listSourceRepositories(l.ctx, l.svcCtx, in)
}
