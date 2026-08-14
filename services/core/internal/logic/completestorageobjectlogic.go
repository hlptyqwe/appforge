package logic

import (
	"context"
	"encoding/hex"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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
	item, err := l.svcCtx.StorageObjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "storage object")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}
	if item.Status != storageStatusUploading {
		return nil, status.Error(codes.FailedPrecondition, "storage object is not uploading")
	}
	if item.SizeBytes != in.SizeBytes {
		return nil, status.Error(codes.FailedPrecondition, "uploaded object size does not match declaration")
	}
	item.Sha256 = nullString(sha256Value)
	item.Status = storageStatusReady
	if err := l.svcCtx.StorageObjectModel.Update(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "complete storage object failed: %v", err)
	}
	item, err = l.svcCtx.StorageObjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "storage object")
	}
	return &core.StorageObjectResp{Base: okBase(), Data: mapStorageObject(item)}, nil
}
