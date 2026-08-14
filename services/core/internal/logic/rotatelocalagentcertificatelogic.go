package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RotateLocalAgentCertificateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRotateLocalAgentCertificateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RotateLocalAgentCertificateLogic {
	return &RotateLocalAgentCertificateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Agent使用当前有效证书和新CSR轮换客户端证书。
func (l *RotateLocalAgentCertificateLogic) RotateLocalAgentCertificate(in *core.RotateLocalAgentCertificateReq) (*core.RegisterLocalAgentResp, error) {
	return rotateLocalAgentCertificate(l.ctx, l.svcCtx, in)
}
