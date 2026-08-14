package logic

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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

	created := &models.TStorageObject{
		TenantId: tenant, AppId: in.AppId, ObjectType: int64(in.ObjectType),
		ObjectKey: strings.TrimSpace(in.ObjectKey), OriginalName: strings.TrimSpace(in.OriginalName),
		ContentType: strings.TrimSpace(in.ContentType), SizeBytes: in.SizeBytes,
		Status: storageStatusUploading, CreateBy: actorID(l.ctx),
	}
	var item models.TStorageObject
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(txCtx context.Context, session sqlx.Session) error {
		result, err := l.svcCtx.StorageObjectModel.WithSession(session).Insert(txCtx, created)
		if err != nil {
			return status.Errorf(codes.Internal, "create storage object failed: %v", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return status.Errorf(codes.Internal, "read storage object id failed: %v", err)
		}
		if _, err := reserveQuotaInSession(txCtx, session, tenant, "storage.bytes", in.SizeBytes,
			"storage", id, storageQuotaKey(in.ObjectKey), 24*time.Hour); err != nil {
			return err
		}
		if err := session.QueryRowCtx(txCtx, &item, storageObjectSelect+` WHERE id=? AND tenant_id=?`, id, tenant); err != nil {
			return status.Errorf(codes.Internal, "load created storage object failed: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &core.StorageObjectResp{Base: okBase(), Data: mapStorageObject(&item)}, nil
}

func validateStorageObjectInput(in *core.CreateStorageObjectReq, tenant int64) error {
	if in.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK &&
		in.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE &&
		in.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK &&
		in.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG &&
		in.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO &&
		in.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH &&
		in.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_TEMPLATE_FILE &&
		in.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_CACHE {
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
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_CACHE:
		if ext != ".apk" || in.SizeBytes > 2*1024*1024*1024 {
			return status.Error(codes.InvalidArgument, "build cache must be an .apk file no larger than 2 GiB")
		}
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE:
		if ext != ".jks" && ext != ".keystore" && ext != ".p12" && ext != ".pfx" {
			return status.Error(codes.InvalidArgument, "keystore file extension is not supported")
		}
		if in.SizeBytes > 10*1024*1024 {
			return status.Error(codes.InvalidArgument, "keystore must not exceed 10 MiB")
		}
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO,
		core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH:
		if ext != ".png" && ext != ".webp" {
			return status.Error(codes.InvalidArgument, "branding image must be a PNG or WebP file")
		}
		maxSize := int64(10 * 1024 * 1024)
		if in.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO {
			maxSize = 5 * 1024 * 1024
		}
		if in.SizeBytes > maxSize {
			return status.Errorf(codes.InvalidArgument, "branding image must not exceed %d MiB", maxSize/(1024*1024))
		}
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_TEMPLATE_FILE:
		if ext != ".json" && ext != ".xml" && ext != ".txt" && ext != ".png" && ext != ".webp" {
			return status.Error(codes.InvalidArgument, "template file must be JSON, XML, TXT, PNG or WebP")
		}
		if in.SizeBytes > 2*1024*1024 {
			return status.Error(codes.InvalidArgument, "template file must not exceed 2 MiB")
		}
	}
	if strings.TrimSpace(in.ContentType) == "" {
		in.ContentType = "application/octet-stream"
	}
	return nil
}
