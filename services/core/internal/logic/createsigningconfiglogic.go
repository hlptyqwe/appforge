package logic

import (
	"context"
	"strings"

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
	if err := requireText(in.KeystoreObjectKey, "keystore_object_key", 500); err != nil {
		return nil, err
	}
	if err := requireText(in.KeyAlias, "key_alias", 128); err != nil {
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
	_, err = l.svcCtx.SigningConfigModel.Insert(l.ctx, &models.TAppSigningConfig{
		TenantId:                   tenant,
		AppId:                      in.AppId,
		Name:                       strings.TrimSpace(in.Name),
		KeystoreObjectKey:          strings.TrimSpace(in.KeystoreObjectKey),
		KeyAlias:                   strings.TrimSpace(in.KeyAlias),
		KeystorePasswordCiphertext: nullString(in.KeystorePasswordCiphertext),
		KeyPasswordCiphertext:      nullString(in.KeyPasswordCiphertext),
		SecretRef:                  nullString(in.SecretRef),
		Status:                     1,
		CreateBy:                   actorID(l.ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create signing config failed: %v", err)
	}
	item, err := l.svcCtx.SigningConfigModel.FindOneByTenantIdAppIdName(l.ctx, tenant, in.AppId, strings.TrimSpace(in.Name))
	if err != nil {
		return nil, notFoundOrInternal(err, "signing config")
	}

	return &core.SigningConfigResp{Base: okBase(), Data: mapSigningConfig(item)}, nil
}
