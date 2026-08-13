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

	_, err = l.svcCtx.VersionModel.Insert(l.ctx, &models.TAppVersion{
		TenantId:        tenant,
		AppId:           in.AppId,
		VersionCode:     in.VersionCode,
		VersionName:     strings.TrimSpace(in.VersionName),
		SourceApkUrl:    nullString(in.SourceApkUrl),
		SourceApkSha256: nullString(in.SourceApkSha256),
		ReleaseNotes:    nullString(in.ReleaseNotes),
		BuildConfig:     nullString(in.BuildConfigJson),
		Status:          int64(core.VersionStatus_VERSION_STATUS_DRAFT),
		CreateBy:        actorID(l.ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create version failed: %v", err)
	}
	item, err := l.svcCtx.VersionModel.FindOneByAppIdVersionCode(l.ctx, in.AppId, in.VersionCode)
	if err != nil {
		return nil, notFoundOrInternal(err, "version")
	}

	return &core.VersionResp{Base: okBase(), Data: mapVersion(item)}, nil
}
