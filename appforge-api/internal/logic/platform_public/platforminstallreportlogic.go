// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_public

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlatformInstallReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformInstallReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformInstallReportLogic {
	return &PlatformInstallReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlatformInstallReportLogic) PlatformInstallReport(req *types.PlatformInstallReportReq) (resp *types.RespBase, err error) {
	item, err := l.svcCtx.CoreCli.ReportInstall(l.ctx, &core.InstallReportReq{AppId: req.AppId, ChannelCode: req.ChannelCode, InstallId: req.InstallId, AppVersion: req.AppVersion, DeviceModel: req.DeviceModel, Ip: req.Ip, FirstOpenTime: req.FirstOpenTime})
	if err != nil {
		return nil, err
	}
	base := platformlogic.PlatformRespBase(item.Base)
	return &types.RespBase{Code: base.Code, Msg: base.Msg, Total: base.Total, HasNext: base.HasNext, HasPrev: base.HasPrev, NextCursor: base.NextCursor, PrevCursor: base.PrevCursor}, nil
}
