package worker

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"appforge/common/secretbox"
	"appforge/common/secretprovider"
	"appforge/common/storage"
	"appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/config"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Worker continuously claims and executes APK repackaging tasks.
type Worker struct {
	config   config.Config
	builder  builder.BuilderClient
	core     core.CoreClient
	store    storage.ObjectStore
	secrets  *secretbox.Box
	provider *secretprovider.Resolver
	activeMu sync.RWMutex
	active   map[int64]struct{}
	activeWG sync.WaitGroup
}

func New(c config.Config, builderClient builder.BuilderClient, coreClient core.CoreClient) (*Worker, error) {
	if strings.TrimSpace(c.Builder.Id) == "" {
		return nil, errors.New("builder id is required")
	}
	if c.Builder.LeaseSeconds <= 0 {
		c.Builder.LeaseSeconds = 120
	}
	if c.Builder.PollInterval <= 0 {
		c.Builder.PollInterval = 2 * time.Second
	}
	if strings.TrimSpace(c.Builder.PoolCode) == "" {
		c.Builder.PoolCode = "default"
	}
	if c.Builder.MaxConcurrency <= 0 || c.Builder.MaxConcurrency > 64 {
		c.Builder.MaxConcurrency = 1
	}
	if c.Builder.NodeHeartbeat <= 0 {
		c.Builder.NodeHeartbeat = 15 * time.Second
	}
	if strings.TrimSpace(c.Builder.ToolchainVersion) == "" {
		c.Builder.ToolchainVersion = "android-builder-v4"
	}
	if c.Builder.BuildProtocolVersion <= 0 {
		c.Builder.BuildProtocolVersion = 1
	}
	if strings.TrimSpace(c.Builder.CapabilityJson) == "" {
		c.Builder.CapabilityJson = `{"apk":true,"branding":true,"whiteLabel":true,"cache":true}`
	}
	if c.Builder.CacheTtl <= 0 {
		c.Builder.CacheTtl = 7 * 24 * time.Hour
	}
	if strings.TrimSpace(c.Builder.TempDir) == "" {
		c.Builder.TempDir = os.TempDir()
	}
	if c.ObjectCleanup.Interval <= 0 {
		c.ObjectCleanup.Interval = 5 * time.Minute
	}
	if c.ObjectCleanup.StaleAfter < time.Minute {
		c.ObjectCleanup.StaleAfter = 30 * time.Minute
	}
	if c.ObjectCleanup.BatchSize <= 0 || c.ObjectCleanup.BatchSize > 100 {
		c.ObjectCleanup.BatchSize = 100
	}
	store, err := storage.NewObjectStore(c.ObjectStorage.StorageConfig())
	if err != nil {
		return nil, fmt.Errorf("initialize object storage: %w", err)
	}
	secrets, err := secretbox.New(c.SigningSecrets.MasterKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("initialize signing secrets: %w", err)
	}
	providers := make([]secretprovider.Provider, 0, 3)
	if strings.TrimSpace(c.SecretProviders.LocalRoot) != "" {
		provider, providerErr := secretprovider.NewLocalFileProvider(c.SecretProviders.LocalRoot)
		if providerErr != nil {
			return nil, fmt.Errorf("initialize local Secret provider: %w", providerErr)
		}
		providers = append(providers, provider)
	}
	if strings.TrimSpace(c.SecretProviders.KubernetesRoot) != "" {
		provider, providerErr := secretprovider.NewKubernetesFileProvider(c.SecretProviders.KubernetesRoot)
		if providerErr != nil {
			return nil, fmt.Errorf("initialize Kubernetes Secret provider: %w", providerErr)
		}
		providers = append(providers, provider)
	}
	if strings.TrimSpace(c.SecretProviders.Vault.Address) != "" {
		provider, providerErr := secretprovider.NewVaultProvider(c.SecretProviders.Vault.Address,
			c.SecretProviders.Vault.TokenFile, c.SecretProviders.Vault.Namespace, nil, c.SecretProviders.Vault.AllowHTTP)
		if providerErr != nil {
			return nil, fmt.Errorf("initialize Vault Secret provider: %w", providerErr)
		}
		providers = append(providers, provider)
	}
	if strings.TrimSpace(c.SecretProviders.AWS.Region) != "" {
		provider, providerErr := secretprovider.NewAWSSecretsManagerProvider(context.Background(),
			c.SecretProviders.AWS.Region, c.SecretProviders.AWS.Endpoint)
		if providerErr != nil {
			return nil, fmt.Errorf("initialize AWS Secrets Manager provider: %w", providerErr)
		}
		providers = append(providers, provider)
	}
	resolver, err := secretprovider.New(c.SecretProviders.MaxSecretBytes, providers...)
	if err != nil {
		return nil, fmt.Errorf("initialize enterprise Secret resolver: %w", err)
	}
	return &Worker{config: c, builder: builderClient, core: coreClient, store: store, secrets: secrets, provider: resolver, active: make(map[int64]struct{})}, nil
}

// Run blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	if err := os.MkdirAll(w.config.Builder.TempDir, 0o700); err != nil {
		return fmt.Errorf("create worker temp directory: %w", err)
	}
	for {
		if err := w.registerNode(ctx); err == nil {
			break
		} else {
			log.Printf("register builder node failed, retrying: %v", err)
		}
		if !waitContext(ctx, 2*time.Second) {
			return nil
		}
	}
	workCtx, cancelWork := context.WithCancel(context.Background())
	defer cancelWork()
	var background sync.WaitGroup
	background.Add(2)
	go func() {
		defer background.Done()
		w.cleanupLoop(ctx)
	}()
	go func() {
		defer background.Done()
		w.nodeHeartbeatLoop(ctx)
	}()
	defer background.Wait()
	for {
		if err := ctx.Err(); err != nil {
			w.gracefulShutdown(cancelWork)
			return nil
		}
		if w.activeCount() >= int(w.config.Builder.MaxConcurrency) {
			if !waitContext(ctx, w.config.Builder.PollInterval) {
				continue
			}
			continue
		}
		handledPreflight := false
		var preflightErr error
		if w.activeCount() == 0 {
			handledPreflight, preflightErr = w.claimAndExecutePreflight(ctx)
		}
		if preflightErr != nil {
			log.Printf("execute branding preflight failed: %v", preflightErr)
		}
		if handledPreflight {
			continue
		}
		claimCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		response, err := w.builder.ClaimScheduledBuildTask(claimCtx, &builder.ClaimScheduledBuildTaskReq{
			NodeCode: w.config.Builder.Id, LeaseSeconds: w.config.Builder.LeaseSeconds,
		})
		cancel()
		if err != nil {
			log.Printf("claim build task failed: %v", err)
			if !waitContext(ctx, w.config.Builder.PollInterval) {
				return nil
			}
			continue
		}
		if response.GetData() == nil || response.GetData().GetId() <= 0 {
			if !waitContext(ctx, w.config.Builder.PollInterval) {
				return nil
			}
			continue
		}
		task := response.Data
		w.addActive(task.Id)
		w.activeWG.Add(1)
		go func() {
			defer w.activeWG.Done()
			defer w.removeActive(task.Id)
			if err := w.execute(workCtx, task); err != nil {
				log.Printf("build task %d finished with error: %v", task.Id, err)
			}
		}()
	}
}

func (w *Worker) activeCount() int {
	w.activeMu.RLock()
	defer w.activeMu.RUnlock()
	return len(w.active)
}

func (w *Worker) activeTaskIDs() []int64 {
	w.activeMu.RLock()
	defer w.activeMu.RUnlock()
	result := make([]int64, 0, len(w.active))
	for id := range w.active {
		result = append(result, id)
	}
	return result
}

func (w *Worker) addActive(id int64) {
	w.activeMu.Lock()
	defer w.activeMu.Unlock()
	w.active[id] = struct{}{}
}

func (w *Worker) removeActive(id int64) {
	w.activeMu.Lock()
	defer w.activeMu.Unlock()
	delete(w.active, id)
}

func (w *Worker) diskCapacity() (int64, int64) {
	var info syscall.Statfs_t
	if err := syscall.Statfs(w.config.Builder.TempDir, &info); err != nil {
		return 0, 0
	}
	return int64(info.Blocks) * int64(info.Bsize), int64(info.Bavail) * int64(info.Bsize)
}

func (w *Worker) registerNode(ctx context.Context) error {
	capacity, free := w.diskCapacity()
	endpoint := strings.TrimSpace(w.config.Builder.Endpoint)
	if endpoint == "" {
		endpoint, _ = os.Hostname()
	}
	registerCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := w.builder.RegisterBuilderNode(registerCtx, &builder.RegisterBuilderNodeReq{
		NodeCode: w.config.Builder.Id, PoolCode: w.config.Builder.PoolCode, Endpoint: endpoint,
		MaxConcurrency: w.config.Builder.MaxConcurrency, CpuCapacity: int32(runtime.NumCPU() * 1000),
		DiskCapacity: capacity, DiskFree: free, ToolchainVersion: w.config.Builder.ToolchainVersion,
		BuildProtocolVersion: w.config.Builder.BuildProtocolVersion, CapabilityJson: w.config.Builder.CapabilityJson,
	})
	return err
}

func (w *Worker) nodeHeartbeatLoop(ctx context.Context) {
	w.sendNodeHeartbeat(ctx)
	ticker := time.NewTicker(w.config.Builder.NodeHeartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sendNodeHeartbeat(ctx)
		}
	}
}

func (w *Worker) sendNodeHeartbeat(ctx context.Context) {
	_, free := w.diskCapacity()
	heartbeatCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := w.builder.BuilderNodeHeartbeat(heartbeatCtx, &builder.BuilderNodeHeartbeatReq{
		NodeCode: w.config.Builder.Id, RunningCount: int32(w.activeCount()), DiskFree: free,
		RunningTaskIds: w.activeTaskIDs(),
	})
	if err != nil && ctx.Err() == nil {
		log.Printf("builder node heartbeat failed: %v", err)
	}
}

func (w *Worker) gracefulShutdown(cancelWork context.CancelFunc) {
	drainCtx, cancelDrain := context.WithTimeout(context.Background(), 15*time.Second)
	_, err := w.builder.DrainBuilderNode(drainCtx, &builder.DrainBuilderNodeReq{
		NodeCode: w.config.Builder.Id, DrainStatus: builder.BuilderDrainStatus_BUILDER_DRAIN_STATUS_DRAINING,
	})
	cancelDrain()
	if err != nil {
		log.Printf("drain builder node failed: %v", err)
	}
	done := make(chan struct{})
	go func() {
		w.activeWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Minute):
		log.Printf("builder shutdown grace period expired; cancelling %d active tasks", w.activeCount())
		cancelWork()
		<-done
	}
}

func (w *Worker) cleanupLoop(ctx context.Context) {
	w.cleanupBuildCaches(ctx)
	w.cleanupExpiredObjects(ctx)
	ticker := time.NewTicker(w.config.ObjectCleanup.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.cleanupBuildCaches(ctx)
			w.cleanupExpiredObjects(ctx)
		}
	}
}

func (w *Worker) cleanupBuildCaches(ctx context.Context) {
	capacity, free := w.diskCapacity()
	targetFreeBytes := int64(0)
	minimumFree := int64(1024 * 1024 * 1024)
	if capacity > 0 && capacity/10 > minimumFree {
		minimumFree = capacity / 10
	}
	if capacity > 0 && free < minimumFree {
		targetFreeBytes = minimumFree - free
	}
	cleanupCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	response, err := w.builder.CleanupBuildCache(cleanupCtx, &builder.CleanupBuildCacheReq{
		Limit: w.config.ObjectCleanup.BatchSize, TargetFreeBytes: targetFreeBytes,
	})
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("cleanup build cache failed: %v", err)
		}
		return
	}
	if data := response.GetData(); data.GetInvalidatedCount() > 0 {
		log.Printf("build cache cleanup completed: invalidated=%d reclaimable_bytes=%d released_objects=%d",
			data.GetInvalidatedCount(), data.GetReclaimableBytes(), len(data.GetObjectIds()))
	}
}

func (w *Worker) cleanupExpiredObjects(ctx context.Context) {
	cleanupCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	response, err := w.core.ClaimExpiredStorageObjects(cleanupCtx, &core.ClaimExpiredStorageObjectsReq{
		StaleSeconds: int64(w.config.ObjectCleanup.StaleAfter / time.Second),
		Limit:        w.config.ObjectCleanup.BatchSize,
	})
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("claim expired storage objects failed: %v", err)
		}
		return
	}
	deleted := 0
	for _, object := range response.GetData() {
		if object.GetId() <= 0 || strings.TrimSpace(object.GetObjectKey()) == "" {
			log.Printf("skip invalid expired storage object metadata: id=%d", object.GetId())
			continue
		}
		if err := w.store.DeleteObject(cleanupCtx, object.GetObjectKey()); err != nil {
			log.Printf("delete expired storage object %d failed: %v", object.GetId(), err)
			continue
		}
		if _, err := w.core.MarkStorageObjectDeleted(cleanupCtx, &core.MarkStorageObjectDeletedReq{
			Id: object.GetId(), ObjectKey: object.GetObjectKey(),
		}); err != nil {
			log.Printf("mark expired storage object %d deleted failed: %v", object.GetId(), err)
			continue
		}
		deleted++
	}
	if deleted > 0 {
		log.Printf("expired storage object cleanup completed: deleted=%d", deleted)
	}
}

func waitContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (w *Worker) execute(parent context.Context, task *builder.BuildTask) error {
	ctx, cancel := context.WithCancel(parent)
	var heartbeat sync.WaitGroup
	heartbeat.Add(1)
	go func() {
		defer heartbeat.Done()
		w.heartbeat(ctx, cancel, task.Id, task.BuilderAttempt)
	}()
	defer func() {
		cancel()
		heartbeat.Wait()
	}()

	workDir, err := os.MkdirTemp(w.config.Builder.TempDir, fmt.Sprintf("task-%d-", task.Id))
	if err != nil {
		return w.fail(ctx, task, "", 0, fmt.Errorf("create task directory: %w", err))
	}
	defer os.RemoveAll(workDir)
	logPath := filepath.Join(workDir, "build.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return w.fail(ctx, task, "", 0, fmt.Errorf("create build log: %w", err))
	}
	logClosed := false
	closeLog := func() {
		if !logClosed {
			_ = logFile.Close()
			logClosed = true
		}
	}
	defer closeLog()
	w.writeLog(logFile, "task %d claimed by %s", task.Id, w.config.Builder.Id)

	execution, err := w.core.GetBuildExecutionContext(ctx, &core.GetBuildExecutionContextReq{
		TaskId: task.Id, BuilderId: w.config.Builder.Id, BuilderAttempt: task.BuilderAttempt,
	})
	if err != nil || execution.GetData() == nil {
		if err == nil {
			err = errors.New("empty build execution context")
		}
		closeLog()
		return w.fail(ctx, task, logPath, 0, fmt.Errorf("load execution context: %w", err))
	}
	data := execution.Data
	sourcePath := filepath.Join(workDir, "source.apk")
	keystorePath := filepath.Join(workDir, "signing.keystore")
	if err := w.downloadVerified(ctx, data.SourceApk, sourcePath); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, fmt.Errorf("download source APK: %w", err))
	}
	if err := w.downloadVerified(ctx, data.Keystore, keystorePath); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, fmt.Errorf("download keystore: %w", err))
	}
	branding, err := decodeBuildBrandingSnapshot(data.BrandingSnapshotJson)
	if err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, err)
	}
	whiteLabel, err := decodeWhiteLabelBuildSnapshot(data.TemplateSnapshotJson)
	if err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, err)
	}
	if whiteLabel != nil {
		if err := whiteLabel.decryptSensitiveParameters(w.secrets.Open); err != nil {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, fmt.Errorf("decrypt white-label parameters: %w", err))
		}
	}
	if whiteLabel != nil && branding == nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, errors.New("white-label build requires a branding snapshot"))
	}
	templateFiles := make(map[int64]string, len(data.TemplateFiles))
	for _, object := range data.TemplateFiles {
		if object.GetId() <= 0 || object.GetObjectType() != core.StorageObjectType_STORAGE_OBJECT_TYPE_TEMPLATE_FILE {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, errors.New("template file metadata is invalid"))
		}
		if _, exists := templateFiles[object.Id]; exists {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, errors.New("duplicate template file metadata"))
		}
		objectPath := filepath.Join(workDir, "template-files", fmt.Sprint(object.Id))
		if err := os.MkdirAll(filepath.Dir(objectPath), 0o700); err != nil {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, err)
		}
		if err := w.downloadVerified(ctx, object, objectPath); err != nil {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, fmt.Errorf("download template file %d: %w", object.Id, err))
		}
		templateFiles[object.Id] = objectPath
	}
	logoPath := filepath.Join(workDir, "brand-logo")
	splashPath := filepath.Join(workDir, "brand-splash")
	if branding != nil {
		if err := w.downloadVerified(ctx, data.BrandLogo, logoPath); err != nil {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, fmt.Errorf("download brand logo: %w", err))
		}
		if err := w.downloadVerified(ctx, data.BrandSplash, splashPath); err != nil {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, fmt.Errorf("download brand splash: %w", err))
		}
	}
	if err := runAndLog(ctx, logFile, nil, "aapt", "dump", "badging", sourcePath); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, fmt.Errorf("validate source APK: %w", err))
	}

	payload := channelPayload{
		SchemaVersion: 1, TenantID: data.Task.TenantId, AppID: data.Task.AppId,
		VersionID: data.Task.VersionId, BuildTaskID: data.Task.Id,
		ChannelCode: data.Task.ChannelCode, ChannelName: data.ChannelName,
		APIHost: data.ApiHost, LandingURL: data.LandingUrl,
		VersionCode: data.Task.VersionCode, VersionName: data.Task.VersionName,
		BuildTime: time.Now().UTC().Format(time.RFC3339),
	}
	intermediatePath := sourcePath
	cacheHit := false
	if branding != nil && strings.TrimSpace(task.CacheKey) != "" {
		cacheResp, cacheErr := w.builder.ResolveBuildCache(ctx, &builder.ResolveBuildCacheReq{
			TaskId: task.Id, NodeCode: w.config.Builder.Id, BuilderAttempt: task.BuilderAttempt,
			ToolchainVersion:     w.config.Builder.ToolchainVersion,
			BuildProtocolVersion: w.config.Builder.BuildProtocolVersion,
		})
		if cacheErr != nil {
			w.writeLog(logFile, "cache resolve skipped: %v", cacheErr)
		} else if cacheResp.GetData().GetHit() && cacheResp.GetData().GetArtifact() != nil {
			entry := cacheResp.GetData().GetEntry()
			artifact := cacheResp.GetData().GetArtifact()
			candidate := filepath.Join(workDir, "cached-intermediate.apk")
			downloadErr := w.downloadVerified(ctx, &core.StorageObject{
				Id: artifact.GetId(), TenantId: data.Task.TenantId,
				ObjectType: core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_CACHE,
				ObjectKey:  artifact.GetObjectKey(), OriginalName: artifact.GetOriginalName(),
				ContentType: artifact.GetContentType(), SizeBytes: artifact.GetSizeBytes(), Sha256: artifact.GetSha256(),
			}, candidate)
			if downloadErr == nil {
				confirmResp, confirmErr := w.builder.ResolveBuildCache(ctx, &builder.ResolveBuildCacheReq{
					TaskId: task.Id, NodeCode: w.config.Builder.Id, BuilderAttempt: task.BuilderAttempt,
					ToolchainVersion:     w.config.Builder.ToolchainVersion,
					BuildProtocolVersion: w.config.Builder.BuildProtocolVersion,
					ConfirmHit:           true,
				})
				if confirmErr == nil && confirmResp.GetData().GetHit() {
					intermediatePath = candidate
					cacheHit = true
					w.writeLog(logFile, "validated build cache hit: entry=%d", entry.GetId())
				} else {
					w.writeLog(logFile, "cache hit confirmation skipped: entry=%d error=%v", entry.GetId(), confirmErr)
				}
			} else {
				w.writeLog(logFile, "build cache artifact invalid: entry=%d error=%v", entry.GetId(), downloadErr)
				_, _ = w.builder.InvalidateBuildCache(ctx, &builder.InvalidateBuildCacheReq{
					Id: entry.GetId(), TaskId: task.Id, NodeCode: w.config.Builder.Id, Reason: "CACHE_DOWNLOAD_VALIDATION_FAILED",
				})
			}
		}
	}
	if branding != nil && !cacheHit {
		intermediatePath, err = buildBrandedAPK(ctx, logFile, sourcePath, logoPath, splashPath, workDir,
			branding, whiteLabel, templateFiles, data.BrandLogo, data.BrandSplash)
		if err != nil {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, err)
		}
		cacheSize, cacheSHA, digestErr := fileDigest(intermediatePath)
		if digestErr == nil {
			cacheKey, keyErr := storage.GenerateTenantObjectKey(data.Task.TenantId, "build-cache", "branded-intermediate.apk")
			if keyErr == nil {
				if uploadErr := w.uploadFile(ctx, intermediatePath, cacheKey, cacheSize, "application/vnd.android.package-archive"); uploadErr == nil {
					_, publishErr := w.builder.PublishBuildCache(ctx, &builder.PublishBuildCacheReq{
						TaskId: task.Id, NodeCode: w.config.Builder.Id, BuilderAttempt: task.BuilderAttempt,
						ToolchainVersion:     w.config.Builder.ToolchainVersion,
						BuildProtocolVersion: w.config.Builder.BuildProtocolVersion,
						ArtifactObjectKey:    cacheKey, ArtifactSha256: cacheSHA, SizeBytes: cacheSize,
						TtlSeconds: int64(w.config.Builder.CacheTtl / time.Second),
					})
					if publishErr != nil {
						if isDefinitiveOwnershipError(publishErr) {
							_ = w.store.DeleteObject(context.WithoutCancel(ctx), cacheKey)
						}
						w.writeLog(logFile, "cache publish skipped: %v", publishErr)
					} else {
						w.writeLog(logFile, "build cache published: sha256=%s", cacheSHA)
					}
				} else {
					w.writeLog(logFile, "cache upload skipped: %v", uploadErr)
				}
			}
		}
	}
	unsignedPath := filepath.Join(workDir, "unsigned.apk")
	if err := injectChannelAsset(intermediatePath, unsignedPath, payload); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, err)
	}
	w.writeLog(logFile, "channel and branding assets injected: channel=%s branding_revision=%d cache_hit=%t", data.Task.ChannelCode, data.Task.BrandingRevision, cacheHit)
	if err := w.report(ctx, task.Id, task.BuilderAttempt, builder.BuildTaskStatus_BUILD_TASK_STATUS_SIGNING, "signing APK", 55); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, err)
	}

	alignedPath := filepath.Join(workDir, "aligned.apk")
	signedPath := filepath.Join(workDir, "channel.apk")
	if err := runAndLog(ctx, logFile, nil, "zipalign", "-f", "4", unsignedPath, alignedPath); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, fmt.Errorf("zipalign APK: %w", err))
	}
	var keystorePassword, keyPassword string
	if strings.TrimSpace(data.SecretRef) != "" {
		resolved, resolveErr := w.provider.ResolveSigningSecret(ctx, data.SecretRef)
		if resolveErr != nil {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, errors.New("resolve external signing secret failed"))
		}
		keystorePassword, keyPassword = resolved.KeystorePassword, resolved.KeyPassword
		defer resolved.Erase()
	} else {
		keystorePassword, err = w.secrets.Open(data.KeystorePasswordCiphertext)
		if err != nil {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, errors.New("decrypt keystore password failed"))
		}
		keyPassword, err = w.secrets.Open(data.KeyPasswordCiphertext)
		if err != nil {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, errors.New("decrypt key password failed"))
		}
	}
	secretEnv := []string{
		"APPFORGE_KEYSTORE_PASSWORD=" + keystorePassword,
		"APPFORGE_KEY_PASSWORD=" + keyPassword,
	}
	if err := runAndLog(ctx, logFile, secretEnv, "keytool", "-list", "-keystore", keystorePath,
		"-storepass:env", "APPFORGE_KEYSTORE_PASSWORD", "-alias", data.KeyAlias); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, fmt.Errorf("validate keystore alias or password: %w", err))
	}
	if err := verifyKeystoreCertificate(ctx, keystorePath, data.KeyAlias, keystorePassword, data.SignerCertificateSha256); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, err)
	}
	if err := runAndLog(ctx, logFile, secretEnv, "apksigner", "sign", "--ks", keystorePath,
		"--ks-key-alias", data.KeyAlias, "--ks-pass", "env:APPFORGE_KEYSTORE_PASSWORD",
		"--key-pass", "env:APPFORGE_KEY_PASSWORD", "--out", signedPath, alignedPath); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, fmt.Errorf("sign APK: %w", err))
	}
	keystorePassword = ""
	keyPassword = ""
	secretEnv = nil
	if err := runAndLog(ctx, logFile, nil, "apksigner", "verify", "--verbose", "--print-certs", signedPath); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, fmt.Errorf("verify APK signature: %w", err))
	}
	if err := verifyPackageName(ctx, logFile, signedPath, data.PackageName); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, err)
	}
	if branding != nil {
		if err := verifyApplicationLabel(ctx, logFile, signedPath, branding.AppName); err != nil {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, err)
		}
		if err := verifyAPKAsset(signedPath, brandingAssetPath, `"revision":`+fmt.Sprint(branding.Revision)); err != nil {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, err)
		}
	}
	if whiteLabel != nil {
		if err := verifyAPKAsset(signedPath, whiteLabelAssetPath, `"templateChecksum":"`+whiteLabel.TemplateChecksum+`"`); err != nil {
			closeLog()
			return w.fail(ctx, task, logPath, data.Task.TenantId, err)
		}
	}
	if err := w.report(ctx, task.Id, task.BuilderAttempt, builder.BuildTaskStatus_BUILD_TASK_STATUS_UPLOADING, "uploading build artifacts", 85); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, err)
	}

	apkSize, apkSHA, err := fileDigest(signedPath)
	if err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, err)
	}
	apkKey, err := storage.GenerateTenantObjectKey(data.Task.TenantId, "build-apk", "channel.apk")
	if err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, err)
	}
	if err := w.uploadFile(ctx, signedPath, apkKey, apkSize, "application/vnd.android.package-archive"); err != nil {
		closeLog()
		return w.fail(ctx, task, logPath, data.Task.TenantId, fmt.Errorf("upload APK: %w", err))
	}
	w.writeLog(logFile, "APK uploaded and verified: size=%d sha256=%s", apkSize, apkSHA)
	closeLog()
	logKey, logSize, logSHA, logErr := w.uploadLog(ctx, data.Task.TenantId, logPath)
	if logErr != nil {
		_ = w.store.DeleteObject(ctx, apkKey)
		return w.fail(ctx, task, "", data.Task.TenantId, fmt.Errorf("upload build log: %w", logErr))
	}
	_, err = w.builder.CompleteBuildTask(ctx, &builder.CompleteBuildTaskReq{
		TaskId: task.Id, BuilderId: w.config.Builder.Id,
		ApkUrl: apkKey, ApkObjectKey: apkKey, ApkSha256: apkSHA, ApkSize: apkSize,
		LogUrl: logKey, LogObjectKey: logKey, LogSha256: logSHA, LogSize: logSize, BuilderAttempt: task.BuilderAttempt,
	})
	if err != nil {
		if isDefinitiveOwnershipError(err) {
			cleanupCtx := context.WithoutCancel(ctx)
			_ = w.store.DeleteObject(cleanupCtx, apkKey)
			_ = w.store.DeleteObject(cleanupCtx, logKey)
		}
		return fmt.Errorf("complete build task: %w", err)
	}
	return nil
}

func (w *Worker) heartbeat(ctx context.Context, cancel context.CancelFunc, taskID int64, builderAttempt int32) {
	interval := time.Duration(w.config.Builder.LeaseSeconds) * time.Second / 3
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			heartbeatCtx, heartbeatCancel := context.WithTimeout(ctx, 10*time.Second)
			_, err := w.builder.HeartbeatBuildTask(heartbeatCtx, &builder.HeartbeatBuildTaskReq{
				TaskId: taskID, BuilderId: w.config.Builder.Id, LeaseSeconds: w.config.Builder.LeaseSeconds, BuilderAttempt: builderAttempt,
			})
			heartbeatCancel()
			if err != nil {
				log.Printf("build task %d heartbeat failed: %v", taskID, err)
				cancel()
				return
			}
		}
	}
}

func (w *Worker) report(ctx context.Context, taskID int64, builderAttempt int32, state builder.BuildTaskStatus, message string, progress int32) error {
	_, err := w.builder.ReportBuildProgress(ctx, &builder.ReportBuildProgressReq{
		TaskId: taskID, BuilderId: w.config.Builder.Id, Status: state, Message: message, Progress: progress, BuilderAttempt: builderAttempt,
	})
	return err
}

func (w *Worker) downloadVerified(ctx context.Context, object *core.StorageObject, destination string) error {
	if object == nil || object.ObjectKey == "" || object.SizeBytes <= 0 || len(object.Sha256) != 64 {
		return errors.New("invalid storage object metadata")
	}
	reader, err := w.store.OpenObject(ctx, object.ObjectKey)
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(reader, object.SizeBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written != object.SizeBytes || hex.EncodeToString(hash.Sum(nil)) != strings.ToLower(object.Sha256) {
		return errors.New("object size or SHA-256 mismatch")
	}
	return nil
}

func (w *Worker) uploadFile(ctx context.Context, filePath, key string, size int64, contentType string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	return w.store.PutObject(ctx, key, file, size, contentType)
}

func (w *Worker) uploadLog(ctx context.Context, tenantID int64, logPath string) (string, int64, string, error) {
	size, sha, err := fileDigest(logPath)
	if err != nil {
		return "", 0, "", err
	}
	key, err := storage.GenerateTenantObjectKey(tenantID, "build-log", "build.log")
	if err != nil {
		return "", 0, "", err
	}
	if err := w.uploadFile(ctx, logPath, key, size, "text/plain; charset=utf-8"); err != nil {
		return "", 0, "", err
	}
	return key, size, sha, nil
}

func (w *Worker) fail(ctx context.Context, task *builder.BuildTask, logPath string, tenantID int64, cause error) error {
	var logKey, logSHA string
	var logSize int64
	if logPath != "" && tenantID > 0 {
		logKey, logSize, logSHA, _ = w.uploadLog(context.WithoutCancel(ctx), tenantID, logPath)
	}
	message := cause.Error()
	if len(message) > 2000 {
		message = message[:2000]
	}
	_, reportErr := w.builder.FailBuildTask(context.WithoutCancel(ctx), &builder.FailBuildTaskReq{
		TaskId: task.Id, BuilderId: w.config.Builder.Id, ErrorMessage: message,
		LogUrl: logKey, LogObjectKey: logKey, LogSha256: logSHA, LogSize: logSize, BuilderAttempt: task.BuilderAttempt,
	})
	if reportErr != nil {
		if logKey != "" && isDefinitiveOwnershipError(reportErr) {
			_ = w.store.DeleteObject(context.WithoutCancel(ctx), logKey)
		}
		return fmt.Errorf("%v; report failure: %w", cause, reportErr)
	}
	return cause
}

func isDefinitiveOwnershipError(err error) bool {
	switch status.Code(err) {
	case codes.NotFound, codes.FailedPrecondition, codes.PermissionDenied:
		return true
	default:
		return false
	}
}

func (w *Worker) writeLog(file io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(file, "%s "+format+"\n", append([]any{time.Now().UTC().Format(time.RFC3339)}, args...)...)
}

func runAndLog(ctx context.Context, output io.Writer, extraEnv []string, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout = output
	command.Stderr = output
	if len(extraEnv) > 0 {
		command.Env = append(os.Environ(), extraEnv...)
	}
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func verifyPackageName(ctx context.Context, output io.Writer, apkPath, expected string) error {
	command := exec.CommandContext(ctx, "aapt", "dump", "badging", apkPath)
	result, err := command.CombinedOutput()
	_, _ = output.Write(result)
	if err != nil {
		return fmt.Errorf("read APK package name: %w", err)
	}
	if !strings.Contains(string(result), "package: name='"+expected+"'") {
		return fmt.Errorf("APK package name does not match application package %q", expected)
	}
	return nil
}

func verifyApplicationLabel(ctx context.Context, output io.Writer, apkPath, expected string) error {
	command := exec.CommandContext(ctx, "aapt", "dump", "badging", apkPath)
	result, err := command.CombinedOutput()
	_, _ = output.Write(result)
	if err != nil {
		return fmt.Errorf("read APK application label: %w", err)
	}
	if !strings.Contains(string(result), "application-label:'"+expected+"'") &&
		!strings.Contains(string(result), ":'"+expected+"'") {
		return fmt.Errorf("APK application label does not match branding name %q", expected)
	}
	return nil
}

func verifyAPKAsset(apkPath, assetPath, expected string) error {
	archive, err := zip.OpenReader(apkPath)
	if err != nil {
		return fmt.Errorf("open built APK for asset verification: %w", err)
	}
	defer archive.Close()
	for _, item := range archive.File {
		if path.Clean(item.Name) != assetPath {
			continue
		}
		reader, err := item.Open()
		if err != nil {
			return err
		}
		content, readErr := io.ReadAll(io.LimitReader(reader, 1024*1024))
		_ = reader.Close()
		if readErr != nil {
			return readErr
		}
		if !strings.Contains(string(content), expected) {
			return fmt.Errorf("APK asset %s does not contain expected snapshot", assetPath)
		}
		return nil
	}
	return fmt.Errorf("APK asset %s is missing", assetPath)
}

func fileDigest(filePath string) (int64, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}
