// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/sourceoauth"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformSourceAvailableRepositoriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformSourceAvailableRepositoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformSourceAvailableRepositoriesLogic {
	return &ListPlatformSourceAvailableRepositoriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformSourceAvailableRepositoriesLogic) ListPlatformSourceAvailableRepositories(req *types.ListPlatformSourceAvailableRepositoriesReq) (resp *types.PlatformSourceAvailableRepositoryListResp, err error) {
	repositories, err := sourceoauth.ListRepositories(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformSourceAvailableRepository, 0, len(repositories))
	for _, repository := range repositories {
		data = append(data, types.PlatformSourceAvailableRepository{ExternalRepositoryId: repository.ExternalRepositoryID,
			RepositoryFullName: repository.RepositoryFullName, DefaultBranch: repository.DefaultBranch})
	}
	return &types.PlatformSourceAvailableRepositoryListResp{RespBase: types.RespBase{Code: 200, Msg: "OK"}, Data: data}, nil
}
