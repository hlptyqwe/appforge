package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyHybridArtifactLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyHybridArtifactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyHybridArtifactLogic {
	return &VerifyHybridArtifactLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 校验并登记三种存储模式下的Artifact引用。
func (l *VerifyHybridArtifactLogic) VerifyHybridArtifact(in *core.VerifyHybridArtifactReq) (*core.RespBase, error) {
	return verifyHybridArtifact(l.ctx, l.svcCtx, in)
}
