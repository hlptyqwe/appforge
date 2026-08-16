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

type GetBuildExecutionContextLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBuildExecutionContextLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBuildExecutionContextLogic {
	return &GetBuildExecutionContextLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 读取当前Builder已领取任务的内部执行上下文。
func (l *GetBuildExecutionContextLogic) GetBuildExecutionContext(in *core.GetBuildExecutionContextReq) (*core.BuildExecutionContextResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.TaskId, "task_id"); err != nil {
		return nil, err
	}
	if err := validateBuilderRequest(in.BuilderId); err != nil {
		return nil, err
	}
	if err := requirePositive(int64(in.BuilderAttempt), "builder_attempt"); err != nil {
		return nil, err
	}

	var task models.TBuildTask
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &task,
		buildTaskSelect+` WHERE id = ? AND builder_id = ? AND builder_attempt = ? AND status IN (?, ?, ?) AND lease_until > CURRENT_TIMESTAMP`,
		in.TaskId, in.BuilderId, in.BuilderAttempt, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
		return nil, status.Error(codes.NotFound, "build task is not owned by builder or lease has expired")
	}
	app, err := l.svcCtx.ApplicationModel.FindOne(l.ctx, task.AppId)
	if err != nil || app.TenantId != task.TenantId {
		return nil, status.Error(codes.FailedPrecondition, "application snapshot is unavailable")
	}
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, task.ChannelId)
	if err != nil || channel.TenantId != task.TenantId || channel.AppId != task.AppId {
		return nil, status.Error(codes.FailedPrecondition, "channel snapshot is unavailable")
	}
	signing, err := l.svcCtx.SigningConfigModel.FindOne(l.ctx, task.SigningConfigId)
	if err != nil || signing.TenantId != task.TenantId || signing.AppId != task.AppId || signing.Status != 1 {
		return nil, status.Error(codes.FailedPrecondition, "signing configuration is unavailable")
	}
	source, err := l.svcCtx.StorageObjectModel.FindOne(l.ctx, task.SourceApkObjectId)
	if err != nil || source.TenantId != task.TenantId || source.ObjectType != int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK) || source.Status != storageStatusBound {
		return nil, status.Error(codes.FailedPrecondition, "source APK object is unavailable")
	}
	mode := signingModeOf(signing)
	var keystore *models.TStorageObject
	if mode == core.SigningMode_SIGNING_MODE_LOCAL_KEYSTORE {
		keystore, err = l.svcCtx.StorageObjectModel.FindOne(l.ctx, signing.KeystoreObjectId)
		if err != nil || keystore.TenantId != task.TenantId || keystore.ObjectType != int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE) || keystore.Status != storageStatusBound {
			return nil, status.Error(codes.FailedPrecondition, "keystore object is unavailable")
		}
	} else if signing.KeystoreObjectId != 0 || validateRemoteSignerSecretReference(stringValue(signing.SecretRef)) != nil {
		return nil, status.Error(codes.FailedPrecondition, "remote signing configuration is unavailable")
	}
	branding, err := parseBrandingSnapshot(stringValue(task.BrandingSnapshot))
	if err != nil {
		return nil, err
	}
	whiteLabel, err := parseWhiteLabelBuildSnapshot(stringValue(task.TemplateSnapshot))
	if err != nil {
		return nil, err
	}
	var brandLogo, brandSplash *models.TStorageObject
	templateFiles := make([]*core.StorageObject, 0)
	apiHost := stringValue(app.ApiHost)
	packageName := app.PackageName
	certificateSHA256 := stringValue(signing.CertificateSha256)
	if whiteLabel != nil {
		if task.WhiteLabelProductId != whiteLabel.ProductID || task.TemplateRevision != whiteLabel.TemplateRevision {
			return nil, status.Error(codes.FailedPrecondition, "white-label task snapshot is inconsistent")
		}
		if !strings.EqualFold(whiteLabel.CertificateSHA256, certificateSHA256) {
			return nil, status.Error(codes.FailedPrecondition, "signing certificate no longer matches task snapshot")
		}
		packageName = whiteLabel.TargetPackageName
		objectIDs, bindingErr := templateFileObjectIDs(whiteLabel)
		if bindingErr != nil {
			return nil, bindingErr
		}
		for _, objectID := range objectIDs {
			item, findErr := l.svcCtx.StorageObjectModel.FindOne(l.ctx, objectID)
			if findErr != nil || item.TenantId != task.TenantId || item.AppId != task.AppId ||
				item.ObjectType != int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_TEMPLATE_FILE) || item.Status != storageStatusBound {
				return nil, status.Errorf(codes.FailedPrecondition, "template file object %d is unavailable", objectID)
			}
			templateFiles = append(templateFiles, mapStorageObject(item))
		}
	}
	if branding != nil {
		brandLogo, err = l.svcCtx.StorageObjectModel.FindOne(l.ctx, branding.LogoObjectID)
		if err != nil || brandLogo.TenantId != task.TenantId || brandLogo.AppId != task.AppId ||
			brandLogo.ObjectType != int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO) || brandLogo.Status != storageStatusBound {
			return nil, status.Error(codes.FailedPrecondition, "brand logo object is unavailable")
		}
		brandSplash, err = l.svcCtx.StorageObjectModel.FindOne(l.ctx, branding.SplashObjectID)
		if err != nil || brandSplash.TenantId != task.TenantId || brandSplash.AppId != task.AppId ||
			brandSplash.ObjectType != int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH) || brandSplash.Status != storageStatusBound {
			return nil, status.Error(codes.FailedPrecondition, "brand splash object is unavailable")
		}
		apiHost = branding.APIHost
	}

	return &core.BuildExecutionContextResp{
		Base: okBase(),
		Data: &core.BuildExecutionContext{
			Task:                       mapBuildTask(&task),
			PackageName:                packageName,
			ApiHost:                    apiHost,
			ChannelName:                channel.ChannelName,
			LandingUrl:                 stringValue(channel.LandingUrl),
			SourceApk:                  mapStorageObject(source),
			Keystore:                   mapStorageObject(keystore),
			KeyAlias:                   signing.KeyAlias,
			KeystorePasswordCiphertext: stringValue(signing.KeystorePasswordCiphertext),
			KeyPasswordCiphertext:      stringValue(signing.KeyPasswordCiphertext),
			SecretRef:                  stringValue(signing.SecretRef),
			BrandingSnapshotJson:       stringValue(task.BrandingSnapshot),
			BrandLogo:                  mapStorageObject(brandLogo),
			BrandSplash:                mapStorageObject(brandSplash),
			TemplateSnapshotJson:       stringValue(task.TemplateSnapshot),
			SignerCertificateSha256:    certificateSHA256,
			TemplateFiles:              templateFiles,
			SigningMode:                mode,
		},
	}, nil
}
