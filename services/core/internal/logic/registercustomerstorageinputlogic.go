package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterCustomerStorageInputLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterCustomerStorageInputLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterCustomerStorageInputLogic {
	return &RegisterCustomerStorageInputLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Agent登记已上传并重新校验的客户存储输入对象。
func (l *RegisterCustomerStorageInputLogic) RegisterCustomerStorageInput(in *core.RegisterCustomerStorageInputReq) (*core.StorageObjectResp, error) {
	return registerCustomerStorageInput(l.ctx, l.svcCtx, in)
}
