package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RecordSourceArtifactLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRecordSourceArtifactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RecordSourceArtifactLogic {
	return &RecordSourceArtifactLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 记录从授权仓库导入并完成校验的APK Artifact来源。
func (l *RecordSourceArtifactLogic) RecordSourceArtifact(in *core.RecordSourceArtifactReq) (*core.SourceArtifactResp, error) {
	return recordSourceArtifact(l.ctx, l.svcCtx, in)
}
