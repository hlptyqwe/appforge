package adminlogic

import (
	"context"
	"strings"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProfileLogic {
	return &UpdateProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateProfileLogic) UpdateProfile(in *system.UpdateProfileReq) (*system.RespBase, error) {
	userID := actorID(l.ctx)
	if userID <= 0 {
		return nil, status.Error(codes.Unauthenticated, "login user is required")
	}
	item, err := l.svcCtx.UserModel.FindOne(l.ctx, userID)
	if err != nil {
		return nil, notFound(err, "user")
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if in.Nickname != nil {
		item.Nickname = strings.TrimSpace(in.GetNickname())
	}
	if in.Avatar != nil {
		item.Avatar = strings.TrimSpace(in.GetAvatar())
	}
	if in.Password != nil {
		if len(in.GetPassword()) < 6 {
			return nil, status.Error(codes.InvalidArgument, "password must be at least 6 characters")
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(in.GetPassword()), bcrypt.DefaultCost)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "hash password failed: %v", err)
		}
		item.Password = string(hashed)
	}
	item.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.UserModel.Update(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update profile failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
