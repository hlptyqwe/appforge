package adminlogic

import (
	"context"
	"strings"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysTenantListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysTenantListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantListLogic {
	return &SysTenantListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysTenantListLogic) SysTenantList(in *system.SysTenantListReq) (*system.SysTenantListResp, error) {
	if in == nil {
		in = &system.SysTenantListReq{}
	}
	cursor, limit := pageValues(in.GetPage())
	items, total, err := l.svcCtx.TenantMode.FindPage(l.ctx, models.TenantPageFilter{
		Keyword: strings.TrimSpace(in.GetKeyword()), Status: int64(in.GetEnabled()),
		TenantCode: strings.TrimSpace(in.GetTenantCode()), TenantName: strings.TrimSpace(in.GetTenantName()),
		ContactName: strings.TrimSpace(in.GetContactName()), ContactPhone: strings.TrimSpace(in.GetContactPhone()), IDs: in.GetIds(),
	}, cursor, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list tenants failed: %v", err)
	}
	data := make([]*system.SysTenantItem, 0, len(items))
	for _, item := range items {
		data = append(data, tenantItem(item))
	}
	var nextCursor int64
	if len(items) > 0 && int64(len(items)) == limit {
		nextCursor = items[len(items)-1].Id
	}
	return &system.SysTenantListResp{Base: responsePage(total, nextCursor > 0, nextCursor, cursor > 0, 0), Data: data}, nil
}
