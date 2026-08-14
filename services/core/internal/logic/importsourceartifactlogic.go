package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ImportSourceArtifactLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewImportSourceArtifactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ImportSourceArtifactLogic {
	return &ImportSourceArtifactLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 在同一数据库事务中创建版本、绑定存储对象并记录供应商Artifact来源。
func (l *ImportSourceArtifactLogic) ImportSourceArtifact(in *core.ImportSourceArtifactReq) (*core.SourceArtifactImportResp, error) {
	return importSourceArtifact(l.ctx, l.svcCtx, in)
}
