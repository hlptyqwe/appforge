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
	mode := core.SigningMode(req.SigningMode)
	if mode == core.SigningMode_SIGNING_MODE_UNSPECIFIED {
		mode = core.SigningMode_SIGNING_MODE_LOCAL_KEYSTORE
	}
	var keystorePasswordCiphertext, keyPasswordCiphertext string
	var validation *builder.ValidateSigningMaterialResp
	if mode == core.SigningMode_SIGNING_MODE_LOCAL_KEYSTORE {
		if strings.TrimSpace(req.SecretRef) != "" {
			return nil, status.Error(codes.InvalidArgument, "local Keystore external password references are not configured by this API")
		}
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
		storageObject, storageErr := l.svcCtx.CoreCli.GetStorageObject(l.ctx, &core.StorageObjectIdReq{Id: req.KeystoreObjectId})
		if storageErr != nil || storageObject.GetData() == nil {
			return nil, status.Error(codes.InvalidArgument, "keystore object is unavailable")
		}
		validation, err = l.svcCtx.BuilderCli.ValidateSigningMaterial(l.ctx, &builder.ValidateSigningMaterialReq{
			KeystoreObjectKey:          storageObject.Data.ObjectKey,
			KeyAlias:                   req.KeyAlias,
			KeystorePasswordCiphertext: keystorePasswordCiphertext,
			KeyPasswordCiphertext:      keyPasswordCiphertext,
			SigningMode:                mode,
		})
	} else if mode == core.SigningMode_SIGNING_MODE_REMOTE_APK_SIGNER {
		if req.KeystoreObjectId != 0 || strings.TrimSpace(req.KeystorePassword) != "" || strings.TrimSpace(req.KeyPassword) != "" {
			return nil, status.Error(codes.InvalidArgument, "remote signer configuration must not contain Keystore material")
		}
		if strings.TrimSpace(req.SecretRef) == "" || strings.TrimSpace(req.KeyAlias) == "" {
			return nil, status.Error(codes.InvalidArgument, "remote signer secretRef and keyAlias are required")
		}
		validation, err = l.svcCtx.BuilderCli.ValidateSigningMaterial(l.ctx, &builder.ValidateSigningMaterialReq{
			KeyAlias: req.KeyAlias, SigningMode: mode, SecretRef: req.SecretRef,
		})
	} else {
		return nil, status.Error(codes.InvalidArgument, "signingMode is invalid")
	}
	if err != nil {
		return nil, err
	}
	if validation.GetData() == nil || len(validation.Data.CertificateSha256) != 64 {
		return nil, status.Error(codes.FailedPrecondition, "signing certificate fingerprint is unavailable")
	}
	if mode == core.SigningMode_SIGNING_MODE_REMOTE_APK_SIGNER && validation.Data.KeyId != strings.TrimSpace(req.KeyAlias) {
		return nil, status.Error(codes.FailedPrecondition, "remote signer key identity mismatch")
	}
	item, err := l.svcCtx.CoreCli.CreateSigningConfig(l.ctx, &core.CreateSigningConfigReq{
		AppId: req.AppId, Name: req.Name,
		KeyAlias: req.KeyAlias, KeystorePasswordCiphertext: keystorePasswordCiphertext,
		KeyPasswordCiphertext: keyPasswordCiphertext, SecretRef: req.SecretRef,
		KeystoreObjectId:  req.KeystoreObjectId,
		CertificateSha256: validation.Data.CertificateSha256,
		SigningMode:       mode,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformSigningConfigResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformSigningConfig(item.Data)}, nil
}
