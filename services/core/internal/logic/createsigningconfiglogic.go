package logic

import (
	"context"
	"encoding/hex"
	"strings"

	"appforge/common/secretbox"
	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateSigningConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSigningConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSigningConfigLogic {
	return &CreateSigningConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateSigningConfigLogic) CreateSigningConfig(in *core.CreateSigningConfigReq) (*core.SigningConfigResp, error) {
	tenant, err := tenantID(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.AppId, "app_id"); err != nil {
		return nil, err
	}
	if err := requireText(in.Name, "name", 128); err != nil {
		return nil, err
	}
	if err := requireText(in.KeyAlias, "key_alias", 128); err != nil {
		return nil, err
	}
	mode, err := normalizedSigningMode(in.SigningMode)
	if err != nil {
		return nil, err
	}
	app, err := l.svcCtx.ApplicationModel.FindOne(l.ctx, in.AppId)
	if err != nil {
		return nil, notFoundOrInternal(err, "application")
	}
	if err := ensureTenant(app.TenantId, tenant); err != nil {
		return nil, err
	}
	if existing, findErr := l.svcCtx.SigningConfigModel.FindOneByTenantIdAppIdName(l.ctx, tenant, in.AppId, strings.TrimSpace(in.Name)); findErr == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "signing config name already exists")
	} else if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "check signing config failed: %v", findErr)
	}
	var storageObject *models.TStorageObject
	keystoreObjectKey := ""
	if mode == core.SigningMode_SIGNING_MODE_REMOTE_APK_SIGNER {
		if in.KeystoreObjectId != 0 || strings.TrimSpace(in.KeystorePasswordCiphertext) != "" || strings.TrimSpace(in.KeyPasswordCiphertext) != "" {
			return nil, status.Error(codes.InvalidArgument, "remote signer configuration must not contain Keystore material")
		}
		if err := validateRemoteSignerSecretReference(in.SecretRef); err != nil {
			return nil, err
		}
	} else {
		if err := requirePositive(in.KeystoreObjectId, "keystore_object_id"); err != nil {
			return nil, err
		}
		storageObject, err = l.svcCtx.StorageObjectModel.FindOne(l.ctx, in.KeystoreObjectId)
		if err != nil {
			return nil, notFoundOrInternal(err, "keystore")
		}
		if storageObject.TenantId != tenant ||
			(storageObject.AppId != 0 && storageObject.AppId != in.AppId) ||
			storageObject.ObjectType != int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE) ||
			storageObject.Status != storageStatusReady {
			return nil, status.Error(codes.FailedPrecondition, "keystore is not ready for this application")
		}
		keystoreObjectKey = storageObject.ObjectKey
		if strings.TrimSpace(in.SecretRef) == "" &&
			(strings.TrimSpace(in.KeystorePasswordCiphertext) == "" || strings.TrimSpace(in.KeyPasswordCiphertext) == "") {
			return nil, status.Error(codes.InvalidArgument, "encrypted keystore and key passwords are required")
		}
		if strings.TrimSpace(in.SecretRef) == "" &&
			(!secretbox.IsSealed(in.KeystorePasswordCiphertext) || !secretbox.IsSealed(in.KeyPasswordCiphertext)) {
			return nil, status.Error(codes.InvalidArgument, "keystore and key passwords must use the supported encrypted envelope")
		}
	}
	certificateSHA := strings.ToLower(strings.TrimSpace(in.CertificateSha256))
	decodedCertificateSHA, decodeErr := hex.DecodeString(certificateSHA)
	if decodeErr != nil || len(decodedCertificateSHA) != 32 {
		return nil, status.Error(codes.InvalidArgument, "certificate_sha256 is invalid")
	}
	_, err = l.svcCtx.SigningConfigModel.Insert(l.ctx, &models.TAppSigningConfig{
		TenantId:                   tenant,
		AppId:                      in.AppId,
		Name:                       strings.TrimSpace(in.Name),
		KeystoreObjectId:           in.KeystoreObjectId,
		KeystoreObjectKey:          keystoreObjectKey,
		KeyAlias:                   strings.TrimSpace(in.KeyAlias),
		CertificateSha256:          nullString(certificateSHA),
		KeystorePasswordCiphertext: nullString(in.KeystorePasswordCiphertext),
		KeyPasswordCiphertext:      nullString(in.KeyPasswordCiphertext),
		SecretRef:                  nullString(in.SecretRef),
		Status:                     1,
		CreateBy:                   actorID(l.ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create signing config failed: %v", err)
	}
	if storageObject != nil {
		storageObject.AppId = in.AppId
		storageObject.Status = storageStatusBound
		if err := l.svcCtx.StorageObjectModel.Update(l.ctx, storageObject); err != nil {
			return nil, status.Errorf(codes.Internal, "bind keystore failed: %v", err)
		}
	}
	item, err := l.svcCtx.SigningConfigModel.FindOneByTenantIdAppIdName(l.ctx, tenant, in.AppId, strings.TrimSpace(in.Name))
	if err != nil {
		return nil, notFoundOrInternal(err, "signing config")
	}

	return &core.SigningConfigResp{Base: okBase(), Data: mapSigningConfig(item)}, nil
}
