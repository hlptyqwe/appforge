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

type CreateApplicationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateApplicationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateApplicationLogic {
	return &CreateApplicationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateApplicationLogic) CreateApplication(in *core.CreateApplicationReq) (*core.ApplicationResp, error) {
	tenant, err := tenantID(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	for _, item := range []struct {
		value string
		field string
		max   int
	}{{in.AppCode, "app_code", 64}, {in.AppName, "app_name", 128}, {in.PackageName, "package_name", 255}} {
		if err := requireText(item.value, item.field, item.max); err != nil {
			return nil, err
		}
	}
	if len(strings.TrimSpace(in.Description)) > 500 {
		return nil, status.Error(codes.InvalidArgument, "description is too long")
	}
	if existing, findErr := l.svcCtx.ApplicationModel.FindOneByTenantIdAppCode(l.ctx, tenant, strings.TrimSpace(in.AppCode)); findErr == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "app_code already exists")
	} else if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Error(codes.Internal, "check app_code failed")
	}
	if existing, findErr := l.svcCtx.ApplicationModel.FindOneByTenantIdPackageName(l.ctx, tenant, strings.TrimSpace(in.PackageName)); findErr == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "package_name already exists")
	} else if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Error(codes.Internal, "check package_name failed")
	}

	_, err = l.svcCtx.ApplicationModel.Insert(l.ctx, &models.TAppApplication{
		TenantId:    tenant,
		AppCode:     strings.TrimSpace(in.AppCode),
		AppName:     strings.TrimSpace(in.AppName),
		PackageName: strings.TrimSpace(in.PackageName),
		Description: nullString(in.Description),
		IconUrl:     nullString(in.IconUrl),
		ApiHost:     nullString(in.ApiHost),
		Status:      int64(core.ApplicationStatus_APPLICATION_STATUS_ENABLED),
		CreateBy:    actorID(l.ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create application failed: %v", err)
	}
	item, err := l.svcCtx.ApplicationModel.FindOneByTenantIdAppCode(l.ctx, tenant, strings.TrimSpace(in.AppCode))
	if err != nil {
		return nil, notFoundOrInternal(err, "application")
	}

	return &core.ApplicationResp{Base: okBase(), Data: mapApplication(item)}, nil
}
