package logic

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateStorageObjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateStorageObjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateStorageObjectLogic {
	return &CreateStorageObjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建待上传的私有存储对象元数据。
func (l *CreateStorageObjectLogic) CreateStorageObject(in *core.CreateStorageObjectReq) (*core.StorageObjectResp, error) {
	tenant, err := tenantID(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateStorageObjectInput(in, tenant); err != nil {
		return nil, err
	}
	if in.AppId > 0 {
		app, findErr := l.svcCtx.ApplicationModel.FindOne(l.ctx, in.AppId)
		if findErr != nil {
			return nil, notFoundOrInternal(findErr, "application")
		}
		if err := ensureTenant(app.TenantId, tenant); err != nil {
			return nil, err
		}
	}

	result, err := l.svcCtx.StorageObjectModel.Insert(l.ctx, &models.TStorageObject{
		TenantId: tenant, AppId: in.AppId, ObjectType: int64(in.ObjectType),
		ObjectKey: strings.TrimSpace(in.ObjectKey), OriginalName: strings.TrimSpace(in.OriginalName),
		ContentType: strings.TrimSpace(in.ContentType), SizeBytes: in.SizeBytes,
		Status: storageStatusUploading, CreateBy: actorID(l.ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create storage object failed: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read storage object id failed: %v", err)
	}
	item, err := l.svcCtx.StorageObjectModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, notFoundOrInternal(err, "storage object")
	}
	return &core.StorageObjectResp{Base: okBase(), Data: mapStorageObject(item)}, nil
}

func validateStorageObjectInput(in *core.CreateStorageObjectReq, tenant int64) error {
	if in.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK &&
		in.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE &&
		in.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK &&
		in.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG {
		return status.Error(codes.InvalidArgument, "invalid storage object type")
	}
	if err := requireText(in.ObjectKey, "object_key", 500); err != nil {
		return err
	}
	prefix := fmt.Sprintf("tenants/%d/", tenant)
	if !strings.HasPrefix(strings.TrimSpace(in.ObjectKey), prefix) || strings.Contains(in.ObjectKey, "..") {
		return status.Error(codes.InvalidArgument, "object_key is outside tenant namespace")
	}
	if err := requireText(in.OriginalName, "original_name", 255); err != nil {
		return err
	}
	if in.SizeBytes <= 0 {
		return status.Error(codes.InvalidArgument, "size_bytes must be greater than zero")
	}
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(in.OriginalName)))
	switch in.ObjectType {
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK:
		if ext != ".apk" || in.SizeBytes > 2*1024*1024*1024 {
			return status.Error(codes.InvalidArgument, "source APK must be an .apk file no larger than 2 GiB")
		}
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE:
		if ext != ".jks" && ext != ".keystore" && ext != ".p12" && ext != ".pfx" {
			return status.Error(codes.InvalidArgument, "keystore file extension is not supported")
		}
		if in.SizeBytes > 10*1024*1024 {
			return status.Error(codes.InvalidArgument, "keystore must not exceed 10 MiB")
		}
	}
	if strings.TrimSpace(in.ContentType) == "" {
		in.ContentType = "application/octet-stream"
	}
	return nil
}
