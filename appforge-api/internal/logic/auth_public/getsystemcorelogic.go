// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package auth_public

import (
	"context"
	"encoding/json"

	"appforge/admin-api/internal/logicutil"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/system"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/mr"
)

type GetSystemCoreLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSystemCoreLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSystemCoreLogic {
	return &GetSystemCoreLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSystemCoreLogic) GetSystemCore() (resp *types.GetSystemCoreResp, err error) {
	tenantId := int64(0)
	coreKey := system.SysConfigType_SYSTEM_CORE
	storageKey := system.SysConfigType_OBJECT_STORAGE
	var coreConfig, storageConfig *system.SysConfigDetailResp
	err = mr.Finish(
		func() error {
			var callErr error
			coreConfig, callErr = l.svcCtx.SystemCli.SysConfigDetail(l.ctx, &system.SysConfigDetailReq{
				TenantId: &tenantId, ConfigKey: &coreKey,
			})
			return callErr
		},
		func() error {
			var callErr error
			storageConfig, callErr = l.svcCtx.SystemCli.SysConfigDetail(l.ctx, &system.SysConfigDetailReq{
				TenantId: &tenantId, ConfigKey: &storageKey,
			})
			return callErr
		},
	)
	if err != nil {
		return logicutil.SystemErrorResp[types.GetSystemCoreResp](l.ctx, err)
	}
	if coreConfig.GetBase().GetCode() != 200 || coreConfig.GetData() == nil {
		return &types.GetSystemCoreResp{
			RespBase: types.RespBase{
				Code: coreConfig.GetBase().GetCode(),
				Msg:  coreConfig.GetBase().GetMsg(),
			},
		}, nil
	}

	var core system.SystemCore
	err = json.Unmarshal([]byte(coreConfig.GetData().GetConfigValue()), &core)
	if err != nil {
		return logicutil.SystemErrorResp[types.GetSystemCoreResp](l.ctx, err)
	}

	if storageConfig.GetBase().GetCode() != 200 || storageConfig.GetData() == nil {
		return &types.GetSystemCoreResp{
			RespBase: types.RespBase{
				Code: storageConfig.GetBase().GetCode(),
				Msg:  storageConfig.GetBase().GetMsg(),
			},
		}, nil
	}
	var storage system.ObjectStorageConfig
	err = json.Unmarshal([]byte(storageConfig.GetData().GetConfigValue()), &storage)
	if err != nil {
		return logicutil.SystemErrorResp[types.GetSystemCoreResp](l.ctx, err)
	}
	assetUrl := ""
	switch storage.OssType {
	case 1:
		assetUrl = storage.AliyunOss.BucketUrl
	case 2:
		assetUrl = storage.TencentCos.BucketUrl
	case 3:
		assetUrl = storage.Minio.BucketUrl
	}
	return &types.GetSystemCoreResp{
		RespBase: types.RespBase{
			Code: coreConfig.GetBase().GetCode(),
			Msg:  coreConfig.GetBase().GetMsg(),
		},
		Data: types.GetSystemCore{
			SiteName:      core.SiteName,
			SiteLogo:      core.SiteLogo,
			AssetUrl:      assetUrl,
			MustGoogleF2a: int64(core.AdminMustGoogleF2A),
			Options:       logicutil.CoreOptions(),
		},
	}, nil
}
