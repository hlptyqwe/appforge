// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_public

import (
	"context"
	"time"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlatformChannelDownloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformChannelDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformChannelDownloadLogic {
	return &PlatformChannelDownloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlatformChannelDownloadLogic) PlatformChannelDownload(req *types.PlatformChannelDownloadReq) error {
	_, err := l.Resolve(req.ChannelCode, req.ChannelCode, "", "")
	return err
}

// Resolve 解析公开渠道产物并生成短时私有下载地址。
func (l *PlatformChannelDownloadLogic) Resolve(channelCode, eventKey, ip, userAgent string) (string, error) {
	artifact, err := l.svcCtx.CoreCli.ResolveChannelDownload(l.ctx, &core.ResolveChannelDownloadReq{
		ChannelCode: channelCode, EventKey: eventKey, Ip: ip, UserAgent: userAgent,
	})
	if err != nil {
		return "", err
	}
	store, err := platformlogic.LoadObjectStore(l.ctx, l.svcCtx)
	if err != nil {
		return "", err
	}
	return store.PresignGet(l.ctx, artifact.GetData().GetObjectKey(), 5*time.Minute)
}
