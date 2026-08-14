package logic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"

	"appforge/common/secretbox"
	"appforge/proto/builder"
	"appforge/proto/common"
	"appforge/services/builder/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ValidateSigningMaterialLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateSigningMaterialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateSigningMaterialLogic {
	return &ValidateSigningMaterialLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 使用实际签名工具校验签名材料，成功后方可创建可用签名配置。
func (l *ValidateSigningMaterialLogic) ValidateSigningMaterial(in *builder.ValidateSigningMaterialReq) (*builder.ValidateSigningMaterialResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	objectKey := strings.TrimSpace(in.KeystoreObjectKey)
	if objectKey == "" || path.Clean(objectKey) != objectKey || !strings.HasPrefix(objectKey, "tenants/") || !strings.Contains(objectKey, "/keystore/") {
		return nil, status.Error(codes.InvalidArgument, "invalid keystore object key")
	}
	if strings.TrimSpace(in.KeyAlias) == "" || len(in.KeyAlias) > 128 {
		return nil, status.Error(codes.InvalidArgument, "key alias is required")
	}
	if !secretbox.IsSealed(in.KeystorePasswordCiphertext) || !secretbox.IsSealed(in.KeyPasswordCiphertext) {
		return nil, status.Error(codes.InvalidArgument, "encrypted passwords are required")
	}
	keystorePassword, err := l.svcCtx.Secrets.Open(in.KeystorePasswordCiphertext)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "keystore password cannot be decrypted")
	}
	keyPassword, err := l.svcCtx.Secrets.Open(in.KeyPasswordCiphertext)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "key password cannot be decrypted")
	}

	tempDir, err := os.MkdirTemp("", "appforge-signing-check-")
	if err != nil {
		return nil, status.Error(codes.Internal, "create signing validation directory failed")
	}
	defer os.RemoveAll(tempDir)
	keystorePath := path.Join(tempDir, "source.keystore")
	reader, err := l.svcCtx.Store.OpenObject(l.ctx, objectKey)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "keystore object is unavailable")
	}
	file, err := os.OpenFile(keystorePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		_ = reader.Close()
		return nil, status.Error(codes.Internal, "create keystore validation file failed")
	}
	written, copyErr := io.Copy(file, io.LimitReader(reader, 10*1024*1024+1))
	readerErr := reader.Close()
	fileErr := file.Close()
	if copyErr != nil || readerErr != nil || fileErr != nil || written <= 0 || written > 10*1024*1024 {
		return nil, status.Error(codes.InvalidArgument, "keystore file is invalid")
	}

	destinationPasswordBytes := make([]byte, 24)
	if _, err := rand.Read(destinationPasswordBytes); err != nil {
		return nil, status.Error(codes.Internal, "prepare signing validation failed")
	}
	destinationPassword := base64.RawURLEncoding.EncodeToString(destinationPasswordBytes)
	destinationPath := path.Join(tempDir, "validated.p12")
	command := exec.CommandContext(l.ctx, "keytool",
		"-importkeystore", "-noprompt",
		"-srckeystore", keystorePath,
		"-srcstorepass:env", "APPFORGE_KEYSTORE_PASSWORD",
		"-srcalias", in.KeyAlias,
		"-srckeypass:env", "APPFORGE_KEY_PASSWORD",
		"-destkeystore", destinationPath,
		"-deststoretype", "PKCS12",
		"-deststorepass:env", "APPFORGE_DESTINATION_PASSWORD",
		"-destkeypass:env", "APPFORGE_DESTINATION_PASSWORD",
	)
	command.Env = append(os.Environ(),
		"APPFORGE_KEYSTORE_PASSWORD="+keystorePassword,
		"APPFORGE_KEY_PASSWORD="+keyPassword,
		"APPFORGE_DESTINATION_PASSWORD="+destinationPassword,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		l.Errorf("keystore validation failed: %v (%s)", err, safeToolOutput(output))
		return nil, status.Error(codes.InvalidArgument, "keystore password, key password or alias is invalid")
	}
	if info, err := os.Stat(destinationPath); err != nil || info.Size() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "keystore alias does not contain a private key")
	}
	certificateCommand := exec.CommandContext(l.ctx, "keytool", "-exportcert", "-keystore", keystorePath,
		"-storepass:env", "APPFORGE_KEYSTORE_PASSWORD", "-alias", in.KeyAlias)
	certificateCommand.Env = append(os.Environ(), "APPFORGE_KEYSTORE_PASSWORD="+keystorePassword)
	certificate, err := certificateCommand.Output()
	keystorePassword = ""
	keyPassword = ""
	destinationPassword = ""
	if err != nil || len(certificate) == 0 {
		return nil, status.Error(codes.InvalidArgument, "signing certificate cannot be exported")
	}
	fingerprint := sha256.Sum256(certificate)

	return &builder.ValidateSigningMaterialResp{
		Base: &common.RespBase{Code: 200, Msg: "OK"},
		Data: &builder.SigningMaterialValidation{CertificateSha256: hex.EncodeToString(fingerprint[:])},
	}, nil
}

func safeToolOutput(output []byte) string {
	value := strings.TrimSpace(string(output))
	if len(value) > 500 {
		value = value[:500]
	}
	return fmt.Sprintf("keytool output length=%d", len(value))
}
