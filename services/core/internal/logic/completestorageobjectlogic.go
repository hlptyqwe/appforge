package logic

import (
	"context"
	"encoding/hex"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CompleteStorageObjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCompleteStorageObjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteStorageObjectLogic {
	return &CompleteStorageObjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 完成上传校验并把对象置为可用。
func (l *CompleteStorageObjectLogic) CompleteStorageObject(in *core.CompleteStorageObjectReq) (*core.StorageObjectResp, error) {
	tenant, err := tenantID(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "storage object id is required")
	}
	if in.SizeBytes <= 0 {
		return nil, status.Error(codes.InvalidArgument, "size_bytes must be greater than zero")
	}
	sha256Value := strings.ToLower(strings.TrimSpace(in.Sha256))
	decoded, decodeErr := hex.DecodeString(sha256Value)
	if decodeErr != nil || len(decoded) != 32 {
		return nil, status.Error(codes.InvalidArgument, "sha256 must be 64 hexadecimal characters")
	}
	var item models.TStorageObject
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := session.QueryRowCtx(txCtx, &item, storageObjectSelect+` WHERE id=? AND tenant_id=? FOR UPDATE`, in.Id, tenant); err != nil {
			return notFoundOrInternal(err, "storage object")
		}
		if item.Status != storageStatusUploading {
			return status.Error(codes.FailedPrecondition, "storage object is not uploading")
		}
		if item.SizeBytes != in.SizeBytes {
			return status.Error(codes.FailedPrecondition, "uploaded object size does not match declaration")
		}
		usageMetric, _ := mapUsageMetric(storageUsageMetric(item.ObjectType))
		if err := confirmQuotaInSession(txCtx, session, tenant, "storage.bytes", storageQuotaKey(item.ObjectKey),
			usageMetric, item.Id, billingUsageMetadata(map[string]any{"objectType": item.ObjectType})); err != nil {
			return err
		}
		item.Sha256 = nullString(sha256Value)
		item.Status = storageStatusReady
		if err := l.svcCtx.StorageObjectModel.WithSession(session).Update(txCtx, &item); err != nil {
			return status.Errorf(codes.Internal, "complete storage object failed: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &core.StorageObjectResp{Base: okBase(), Data: mapStorageObject(&item)}, nil
}
