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

type CreateVersionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateVersionLogic {
	return &CreateVersionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateVersionLogic) CreateVersion(in *core.CreateVersionReq) (*core.VersionResp, error) {
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
	if err := requirePositive(in.VersionCode, "version_code"); err != nil {
		return nil, err
	}
	if err := requireText(in.VersionName, "version_name", 64); err != nil {
		return nil, err
	}
	if err := requirePositive(in.SourceApkObjectId, "source_apk_object_id"); err != nil {
		return nil, err
	}
	app, err := l.svcCtx.ApplicationModel.FindOne(l.ctx, in.AppId)
	if err != nil {
		return nil, notFoundOrInternal(err, "application")
	}
	if err := ensureTenant(app.TenantId, tenant); err != nil {
		return nil, err
	}
	if existing, findErr := l.svcCtx.VersionModel.FindOneByAppIdVersionCode(l.ctx, in.AppId, in.VersionCode); findErr == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "version_code already exists")
	} else if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Error(codes.Internal, "check version_code failed")
	}
	storageObject, err := l.svcCtx.StorageObjectModel.FindOne(l.ctx, in.SourceApkObjectId)
	if err != nil {
		return nil, notFoundOrInternal(err, "source APK")
	}
	if storageObject.TenantId != tenant ||
		(storageObject.AppId != 0 && storageObject.AppId != in.AppId) ||
		storageObject.ObjectType != int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK) ||
		storageObject.Status != storageStatusReady {
		return nil, status.Error(codes.FailedPrecondition, "source APK is not ready for this application")
	}

	_, err = l.svcCtx.VersionModel.Insert(l.ctx, &models.TAppVersion{
		TenantId:          tenant,
		AppId:             in.AppId,
		VersionCode:       in.VersionCode,
		VersionName:       strings.TrimSpace(in.VersionName),
		SourceApkObjectId: in.SourceApkObjectId,
		SourceApkUrl:      nullString(storageObject.ObjectKey),
		SourceApkSha256:   storageObject.Sha256,
		ReleaseNotes:      nullString(in.ReleaseNotes),
		BuildConfig:       nullString(in.BuildConfigJson),
		Status:            int64(core.VersionStatus_VERSION_STATUS_DRAFT),
		CreateBy:          actorID(l.ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create version failed: %v", err)
	}
	storageObject.AppId = in.AppId
	storageObject.Status = storageStatusBound
	if err := l.svcCtx.StorageObjectModel.Update(l.ctx, storageObject); err != nil {
		return nil, status.Errorf(codes.Internal, "bind source APK failed: %v", err)
	}
	item, err := l.svcCtx.VersionModel.FindOneByAppIdVersionCode(l.ctx, in.AppId, in.VersionCode)
	if err != nil {
		return nil, notFoundOrInternal(err, "version")
	}

	return &core.VersionResp{Base: okBase(), Data: mapVersion(item)}, nil
}
