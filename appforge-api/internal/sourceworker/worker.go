package sourceworker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"appforge/admin-api/internal/sourceoauth"
	"appforge/admin-api/internal/svc"
	"appforge/common/utils"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Worker reliably imports verified provider Artifacts and creates only the builds
// fixed in the tenant's predefined source trigger policy.
type Worker struct {
	svcCtx       *svc.ServiceContext
	workerID     string
	pollInterval time.Duration
	leaseSeconds int32
}

func New(svcCtx *svc.ServiceContext) *Worker {
	pollInterval := svcCtx.Config.SourceTriggerWorker.PollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	workerID := strings.TrimSpace(svcCtx.Config.SourceTriggerWorker.WorkerId)
	if workerID == "" {
		workerID = "source-trigger-worker"
	}
	leaseSeconds := svcCtx.Config.SourceTriggerWorker.LeaseSeconds
	if leaseSeconds < 30 || leaseSeconds > 1800 {
		leaseSeconds = 600
	}
	return &Worker{svcCtx: svcCtx, workerID: workerID, pollInterval: pollInterval, leaseSeconds: leaseSeconds}
}

func (w *Worker) Start(ctx context.Context) {
	if w == nil || w.svcCtx == nil || !w.svcCtx.Config.SourceTriggerWorker.Enabled {
		return
	}
	go func() {
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()
		w.runCycle(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.runCycle(ctx)
			}
		}
	}()
}

func (w *Worker) runCycle(ctx context.Context) {
	for index := 0; index < 10; index++ {
		claimed, err := w.svcCtx.CoreCli.ClaimSourceWebhookEvent(ctx, &core.ClaimSourceWebhookEventReq{
			WorkerId: w.workerID, LeaseSeconds: w.leaseSeconds,
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("claim source webhook event failed: %v", err)
			return
		}
		if claimed.GetData().GetEvent() == nil || claimed.Data.Trigger == nil {
			return
		}
		w.process(ctx, claimed.Data.Event, claimed.Data.Trigger)
	}
}

func (w *Worker) process(ctx context.Context, event *core.SourceWebhookEvent, trigger *core.SourceBuildTrigger) {
	rpcCtx := context.WithValue(ctx, utils.CtxKeyTenantId, event.TenantId)
	imported, err := sourceoauth.ImportArtifact(rpcCtx, w.svcCtx, trigger.AppId, trigger.RepositoryId,
		event.ArtifactSource, event.ExternalArtifactId, event.ReleaseRef, event.VersionCode, event.VersionName,
		fmt.Sprintf("Imported by source webhook %s from %s at %s", event.ProviderEventId, trigger.RepositoryFullName, event.SourceRef))
	if err != nil {
		w.fail(rpcCtx, event.Id, err)
		return
	}
	if imported.GetData().GetVersion() == nil || imported.Data.Artifact == nil {
		w.fail(rpcCtx, event.Id, status.Error(codes.Internal, "source artifact import response is incomplete"))
		return
	}
	buildTaskIDs := make([]int64, 0, len(trigger.ChannelIds))
	for _, channelID := range trigger.ChannelIds {
		created, createErr := w.svcCtx.CoreCli.CreateBuildTask(rpcCtx, &core.CreateBuildTaskReq{AppId: trigger.AppId,
			VersionId: imported.Data.Version.Id, ChannelId: channelID, SigningConfigId: trigger.SigningConfigId,
			Priority: trigger.Priority, BrandingProfileId: trigger.BrandingProfileId,
			WhiteLabelProductId: trigger.WhiteLabelProductId, PoolCode: trigger.PoolCode, SourceWebhookEventId: event.Id})
		if createErr != nil {
			w.fail(rpcCtx, event.Id, createErr)
			return
		}
		buildTaskIDs = append(buildTaskIDs, created.Data.Id)
	}
	if _, err := w.svcCtx.CoreCli.CompleteSourceWebhookEvent(rpcCtx, &core.CompleteSourceWebhookEventReq{Id: event.Id,
		VersionId: imported.Data.Version.Id, BuildTaskIds: buildTaskIDs, CommitSha: imported.Data.Artifact.CommitSha}); err != nil {
		logx.WithContext(rpcCtx).Errorf("complete source webhook event %d failed: %v", event.Id, err)
	}
}

func (w *Worker) fail(ctx context.Context, eventID int64, err error) {
	code := status.Code(err)
	retryable := code == codes.Unavailable || code == codes.DeadlineExceeded || code == codes.ResourceExhausted || code == codes.Internal || code == codes.Aborted
	message := status.Convert(err).Message()
	if message == "" {
		message = "source webhook processing failed"
	}
	if _, failErr := w.svcCtx.CoreCli.FailSourceWebhookEvent(ctx, &core.FailSourceWebhookEventReq{Id: eventID,
		ErrorMessage: message, Retryable: retryable}); failErr != nil {
		logx.WithContext(ctx).Errorf("record source webhook event %d failure failed: %v", eventID, failErr)
	}
}
