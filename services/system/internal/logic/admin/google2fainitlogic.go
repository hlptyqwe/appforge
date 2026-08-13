package adminlogic

import (
	"context"
	"encoding/base64"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Google2FAInitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGoogle2FAInitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *Google2FAInitLogic {
	return &Google2FAInitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *Google2FAInitLogic) Google2FAInit(in *system.Google2FAInitReq) (*system.Google2FAInitResp, error) {
	if in == nil || in.GetUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	user, err := l.svcCtx.UserModel.FindOne(l.ctx, in.GetUserId())
	if err != nil {
		return nil, notFound(err, "user")
	}
	if _, err := effectiveTenant(l.ctx, user.TenantId); err != nil {
		return nil, err
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "AppForge", AccountName: user.Username})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate 2fa secret failed: %v", err)
	}
	secret := key.Secret()
	if err := l.svcCtx.UserModel.InsertGoogle2FASecret(l.ctx, user.Id, secret); err != nil {
		return nil, status.Errorf(codes.Internal, "save 2fa secret failed: %v", err)
	}
	qr, err := qrcode.Encode(key.URL(), qrcode.Medium, 256)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate 2fa qr code failed: %v", err)
	}
	return &system.Google2FAInitResp{Base: responseBase(), Data: &system.Google2FAInitData{Secret: secret, OtpauthUrl: key.URL(), QrCode: "data:image/png;base64," + base64.StdEncoding.EncodeToString(qr)}}, nil
}
