// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"
	"strings"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/builder"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreatePlatformSigningConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformSigningConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformSigningConfigLogic {
	return &CreatePlatformSigningConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformSigningConfigLogic) CreatePlatformSigningConfig(req *types.CreatePlatformSigningConfigReq) (resp *types.PlatformSigningConfigResp, err error) {
	var keystorePasswordCiphertext, keyPasswordCiphertext string
	if strings.TrimSpace(req.SecretRef) != "" {
		return nil, status.Error(codes.InvalidArgument, "external secret references are not configured")
	}
	if strings.TrimSpace(req.SecretRef) == "" {
		if strings.TrimSpace(req.KeystorePassword) == "" || strings.TrimSpace(req.KeyPassword) == "" {
			return nil, status.Error(codes.InvalidArgument, "keystorePassword and keyPassword are required")
		}
		keystorePasswordCiphertext, err = l.svcCtx.Secrets.Seal(req.KeystorePassword)
		if err != nil {
			return nil, err
		}
		keyPasswordCiphertext, err = l.svcCtx.Secrets.Seal(req.KeyPassword)
		if err != nil {
			return nil, err
		}
	}
	storageObject, err := l.svcCtx.CoreCli.GetStorageObject(l.ctx, &core.StorageObjectIdReq{Id: req.KeystoreObjectId})
	if err != nil || storageObject.GetData() == nil {
		return nil, status.Error(codes.InvalidArgument, "keystore object is unavailable")
	}
	validation, err := l.svcCtx.BuilderCli.ValidateSigningMaterial(l.ctx, &builder.ValidateSigningMaterialReq{
		KeystoreObjectKey:          storageObject.Data.ObjectKey,
		KeyAlias:                   req.KeyAlias,
		KeystorePasswordCiphertext: keystorePasswordCiphertext,
		KeyPasswordCiphertext:      keyPasswordCiphertext,
	})
	if err != nil {
		return nil, err
	}
	if validation.GetData() == nil || len(validation.Data.CertificateSha256) != 64 {
		return nil, status.Error(codes.FailedPrecondition, "signing certificate fingerprint is unavailable")
	}
	item, err := l.svcCtx.CoreCli.CreateSigningConfig(l.ctx, &core.CreateSigningConfigReq{
		AppId: req.AppId, Name: req.Name,
		KeyAlias: req.KeyAlias, KeystorePasswordCiphertext: keystorePasswordCiphertext,
		KeyPasswordCiphertext: keyPasswordCiphertext, SecretRef: req.SecretRef,
		KeystoreObjectId:  req.KeystoreObjectId,
		CertificateSha256: validation.Data.CertificateSha256,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformSigningConfigResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformSigningConfig(item.Data)}, nil
}
