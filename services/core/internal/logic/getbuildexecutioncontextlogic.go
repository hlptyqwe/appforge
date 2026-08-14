package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetBuildExecutionContextLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBuildExecutionContextLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBuildExecutionContextLogic {
	return &GetBuildExecutionContextLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 读取当前Builder已领取任务的内部执行上下文。
func (l *GetBuildExecutionContextLogic) GetBuildExecutionContext(in *core.GetBuildExecutionContextReq) (*core.BuildExecutionContextResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.TaskId, "task_id"); err != nil {
		return nil, err
	}
	if err := validateBuilderRequest(in.BuilderId); err != nil {
		return nil, err
	}
	if err := requirePositive(int64(in.BuilderAttempt), "builder_attempt"); err != nil {
		return nil, err
	}

	var task models.TBuildTask
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &task,
		buildTaskSelect+` WHERE id = ? AND builder_id = ? AND builder_attempt = ? AND status IN (?, ?, ?) AND lease_until > CURRENT_TIMESTAMP`,
		in.TaskId, in.BuilderId, in.BuilderAttempt, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
		return nil, status.Error(codes.NotFound, "build task is not owned by builder or lease has expired")
	}
	app, err := l.svcCtx.ApplicationModel.FindOne(l.ctx, task.AppId)
	if err != nil || app.TenantId != task.TenantId {
		return nil, status.Error(codes.FailedPrecondition, "application snapshot is unavailable")
	}
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, task.ChannelId)
	if err != nil || channel.TenantId != task.TenantId || channel.AppId != task.AppId {
		return nil, status.Error(codes.FailedPrecondition, "channel snapshot is unavailable")
	}
	signing, err := l.svcCtx.SigningConfigModel.FindOne(l.ctx, task.SigningConfigId)
	if err != nil || signing.TenantId != task.TenantId || signing.AppId != task.AppId || signing.Status != 1 {
		return nil, status.Error(codes.FailedPrecondition, "signing configuration is unavailable")
	}
	source, err := l.svcCtx.StorageObjectModel.FindOne(l.ctx, task.SourceApkObjectId)
	if err != nil || source.TenantId != task.TenantId || source.ObjectType != int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK) || source.Status != storageStatusBound {
		return nil, status.Error(codes.FailedPrecondition, "source APK object is unavailable")
	}
	keystore, err := l.svcCtx.StorageObjectModel.FindOne(l.ctx, signing.KeystoreObjectId)
	if err != nil || keystore.TenantId != task.TenantId || keystore.ObjectType != int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE) || keystore.Status != storageStatusBound {
		return nil, status.Error(codes.FailedPrecondition, "keystore object is unavailable")
	}

	return &core.BuildExecutionContextResp{
		Base: okBase(),
		Data: &core.BuildExecutionContext{
			Task:                       mapBuildTask(&task),
			PackageName:                app.PackageName,
			ApiHost:                    stringValue(app.ApiHost),
			ChannelName:                channel.ChannelName,
			LandingUrl:                 stringValue(channel.LandingUrl),
			SourceApk:                  mapStorageObject(source),
			Keystore:                   mapStorageObject(keystore),
			KeyAlias:                   signing.KeyAlias,
			KeystorePasswordCiphertext: stringValue(signing.KeystorePasswordCiphertext),
			KeyPasswordCiphertext:      stringValue(signing.KeyPasswordCiphertext),
			SecretRef:                  stringValue(signing.SecretRef),
		},
	}, nil
}
