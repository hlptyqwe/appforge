import type { RespBase } from '@/services/BaseService'
import * as api from '@/api/platform'

export type PlatformListReq = {
  cursor?: number
  limit?: number
  count?: number
  appId?: number
  channelId?: number
  keyword?: string
  status?: number
  poolCode?: string
  drainStatus?: number
  cacheScope?: number
  taskId?: number
  nodeCode?: string
  eventType?: number
}

export type PlatformApplication = {
  id: number
  tenantId: number
  appCode: string
  appName: string
  packageName: string
  description?: string
  iconUrl?: string
  apiHost?: string
  status: number
  createTime: number
  updateTime: number
}

export type PlatformVersion = {
  id: number
  tenantId: number
  appId: number
  versionCode: number
  versionName: string
  sourceApkUrl?: string
  sourceApkSha256?: string
  sourceApkObjectId: number
  releaseNotes?: string
  buildConfigJson?: string
  status: number
  publishedAt?: number
  createTime: number
  updateTime: number
}

export type PlatformChannel = {
  id: number
  tenantId: number
  appId: number
  channelCode: string
  channelName: string
  landingUrl?: string
  downloadUrl?: string
  status: number
  createTime: number
  updateTime: number
}

export type PlatformSigningConfig = {
  id: number
  tenantId: number
  appId: number
  name: string
  keystoreObjectKey: string
  keystoreObjectId: number
  keyAlias: string
  secretRef?: string
  status: number
  lastVerifiedAt?: number
  createTime: number
  updateTime: number
  certificateSha256?: string
}

export type PlatformBuildTask = {
  id: number
  tenantId: number
  appId: number
  versionId: number
  channelId: number
  signingConfigId: number
  channelCode: string
  versionCode: number
  versionName: string
  status: number
  builderId?: string
  builderAttempt: number
  priority: number
  apkUrl?: string
  apkSha256?: string
  apkSize: number
  logUrl?: string
  sourceApkObjectId: number
  apkObjectId: number
  logObjectId: number
  errorMessage?: string
  queuedAt: number
  startTime?: number
  finishTime?: number
  createTime: number
  updateTime: number
  brandingProfileId: number
  brandingRevision: number
  brandingSnapshotJson?: string
  whiteLabelProductId: number
  templateRevision: number
  templateSnapshotJson?: string
  poolCode: string
  cacheKey?: string
  cacheEntryId: number
  cacheHit: boolean
  cancelRequestedAt?: number
  cancelledAt?: number
  cancelReason?: string
  retryOfTaskId: number
}

export type PlatformBuilderNode = {
  id: number
  nodeCode: string
  poolCode: string
  endpoint?: string
  status: number
  drainStatus: number
  maxConcurrency: number
  runningCount: number
  cpuCapacity: number
  memoryCapacity: number
  diskCapacity: number
  diskFree: number
  toolchainVersion: string
  buildProtocolVersion: number
  capabilityJson: string
  lastErrorMessage?: string
  lastHeartbeatAt: number
  createTime: number
  updateTime: number
}

export type PlatformLocalAgentCapability = {
  capabilityKey: string
  capabilityValue: string
}

export type PlatformLocalAgent = {
  id: number
  tenantId: number
  agentCode: string
  agentName: string
  poolCode: string
  status: number
  drainStatus: number
  protocolVersion: number
  agentVersion: string
  artifactMode: number
  customerStorageRef?: string
  allowedAppIds: number[]
  capabilities: PlatformLocalAgentCapability[]
  certificateSerial?: string
  certificateNotAfter?: number
  lastHeartbeatAt?: number
  createTime: number
  updateTime: number
}

export type PlatformLocalAgentRegistration = {
  registrationToken: string
  expiresAt: number
}

export type PlatformDeploymentComponent = {
  code: string
  name: string
  status: 'healthy' | 'degraded' | 'unhealthy' | 'checking'
  message?: string
  checkedAt: number
}

export type PlatformDeploymentMigration = {
  version: string
  description: string
}

export type PlatformDeploymentLicense = {
  enabled: boolean
  status: 'valid' | 'invalid' | 'not_required'
  licenseId?: string
  customer?: string
  deploymentId?: string
  deploymentModes?: string[]
  features?: string[]
  notBefore?: number
  notAfter?: number
  sequence?: number
  maxTenants?: number
  maxBuilders?: number
  fingerprint?: string
}

export type PlatformDeploymentStatus = {
  deploymentId: string
  deploymentMode: string
  productVersion: string
  targetSchemaVersion: string
  actualSchemaVersion: string
  schemaCompatible: boolean
  maxVersionSkew: number
  agentProtocolCurrent: number
  agentProtocolMinimum: number
  upgradeReady: boolean
  diagnosticsCommand: string
  migrationCount: number
  recentMigrations: PlatformDeploymentMigration[]
  components: PlatformDeploymentComponent[]
  license: PlatformDeploymentLicense
  checkedAt: number
}

export type PlatformBuildConcurrencyPolicy = {
  id: number
  tenantId: number
  appId: number
  poolCode: string
  maxConcurrency: number
  fairWeight: number
  maxPriority: number
  status: number
  createTime: number
  updateTime: number
}

export type PlatformBuildCacheEntry = {
  id: number
  tenantId: number
  cacheScope: number
  cacheKey: string
  toolchainVersion: string
  buildProtocolVersion: number
  inputDigest: string
  artifactObjectId: number
  artifactSha256: string
  sizeBytes: number
  hitCount: number
  status: number
  expiresAt: number
  lastHitAt?: number
  createTime: number
  updateTime: number
}

export type PlatformBuildCacheCleanupResult = {
  invalidatedCount: number
  reclaimableBytes: number
  objectIds: number[]
}

export type PlatformBuildSchedulerEvent = {
  id: number
  tenantId: number
  appId: number
  taskId: number
  nodeCode?: string
  poolCode: string
  eventType: number
  reasonCode?: string
  decisionJson?: string
  createTime: number
}

export type PlatformBuildClusterMetrics = {
  poolCode: string
  periodMinutes: number
  onlineNodes: number
  offlineNodes: number
  drainingNodes: number
  totalSlots: number
  runningSlots: number
  diskCapacity: number
  diskFree: number
  queuedTasks: number
  runningTasks: number
  completedTasks: number
  successTasks: number
  failedTasks: number
  cancelledTasks: number
  averageQueueMs: number
  averageBuildMs: number
  successRate: number
  cacheHitTasks: number
  cacheHitRate: number
  activeCacheEntries: number
  activeCacheBytes: number
  leaseRecoveryCount: number
  cacheValidationFailureCount: number
  oldestQueuedMs: number
  alerts: string[]
}

export type PlatformOpenApiCredential = {
  id: number
  tenantId: number
  credentialName: string
  keyId: string
  scopes: number[]
  appIds: number[]
  ipAllowlist: string[]
  rateLimitPerMinute: number
  status: number
  expiresAt?: number
  graceExpiresAt?: number
  rotatedFromId: number
  lastUsedAt?: number
  createBy: number
  createTime: number
  updateTime: number
}

export type PlatformOpenApiCredentialSecret = {
  credential: PlatformOpenApiCredential
  apiKey: string
}

export type PlatformWebhookEndpoint = {
  id: number
  tenantId: number
  endpointName: string
  endpointUrl: string
  eventTypes: string[]
  secretHint: string
  maxAttempts: number
  status: number
  lastSuccessAt?: number
  lastFailureAt?: number
  createBy: number
  createTime: number
  updateTime: number
}

export type PlatformWebhookEndpointSecret = {
  endpoint: PlatformWebhookEndpoint
  signingSecret: string
}

export type PlatformWebhookDelivery = {
  id: number
  tenantId: number
  endpointId: number
  eventId: string
  eventType: string
  attempt: number
  status: number
  responseStatus: number
  responseBodyExcerpt?: string
  errorMessage?: string
  nextRetryAt: number
  deliveredAt?: number
  createTime: number
  updateTime: number
}

export type PlatformSourceIntegration = {
  id: number
  tenantId: number
  platform: number
  integrationName: string
  installationRef: string
  tokenExpiresAt?: number
  status: number
  lastSyncAt?: number
  createBy: number
  createTime: number
  updateTime: number
}

export type PlatformSourceRepository = {
  id: number
  tenantId: number
  integrationId: number
  externalRepositoryId: string
  repositoryFullName: string
  defaultBranch?: string
  permissionLevel: string
  status: number
  createTime: number
  updateTime: number
}

export type PlatformSourceAvailableRepository = {
  externalRepositoryId: string
  repositoryFullName: string
  defaultBranch?: string
}

export type PlatformSourceArtifact = {
  id: number
  appId: number
  versionId: number
  integrationId: number
  repositoryId: number
  artifactSource: number
  externalArtifactId: string
  commitSha: string
  pipelineRef?: string
  jobRef?: string
  artifactSha256: string
  storageObjectId: number
  createTime: number
}

export type PlatformSourceArtifactImportResult = {
  version: PlatformVersion
  artifact: PlatformSourceArtifact
}

export type PlatformSourceBuildTrigger = {
  id: number
  tenantId: number
  repositoryId: number
  appId: number
  triggerName: string
  eventType: number
  refPattern: string
  artifactSelector: string
  channelIds: number[]
  signingConfigId: number
  brandingProfileId?: number
  whiteLabelProductId?: number
  priority: number
  poolCode: string
  versionNamePrefix?: string
  status: number
  platform: number
  repositoryFullName: string
  createTime: number
  updateTime: number
}

export type PlatformSourceBuildTriggerSecret = {
  trigger: PlatformSourceBuildTrigger
  webhookUrl: string
  signingSecret: string
}

export type PlatformSourceWebhookEvent = {
  id: number
  triggerId: number
  providerEventId: string
  providerEventType: string
  sourceRef: string
  commitSha: string
  artifactSource: number
  externalArtifactId: string
  releaseRef?: string
  pipelineRef?: string
  jobRef?: string
  payloadSha256: string
  versionCode: number
  versionName: string
  status: number
  attempt: number
  versionId?: number
  buildTaskIds?: number[]
  errorMessage?: string
  createTime: number
  updateTime: number
}

export type PlatformBrandingProfile = {
  id: number
  tenantId: number
  appId: number
  profileName: string
  appName: string
  logoObjectId: number
  splashObjectId: number
  apiHost: string
  rewriteMode: number
  launcherIconTarget?: string
  splashResourceTarget?: string
  runtimeConfigJson?: string
  status: number
  revision: number
  createTime: number
  updateTime: number
}

export type PlatformBrandingPreflight = {
  id: number
  tenantId: number
  appId: number
  brandingProfileId: number
  brandingRevision: number
  versionId: number
  status: number
  reportJson?: string
  sourceApkSha256?: string
  toolchainVersion?: string
  startTime?: number
  finishTime?: number
  createTime: number
  updateTime: number
}

export type PlatformWhiteLabelTemplate = {
  id: number
  tenantId: number
  appId: number
  templateCode: string
  templateName: string
  sourceVersionId: number
  parameterSchemaJson?: string
  compatibilityRulesJson?: string
  status: number
  publishedRevision: number
  createTime: number
  updateTime: number
}

export type PlatformWhiteLabelTemplateRevision = {
  id: number
  tenantId: number
  templateId: number
  revision: number
  packageNameRuleJson: string
  manifestPatchJson?: string
  resourcePatchJson?: string
  extensionFilesJson?: string
  expectedArtifactsJson?: string
  checksum: string
  status: number
  createTime: number
}

export type PlatformWhiteLabelProduct = {
  id: number
  tenantId: number
  appId: number
  productCode: string
  productName: string
  templateId: number
  templateRevision: number
  brandingProfileId: number
  packageName: string
  signingConfigId: number
  parameterValuesJson?: string
  status: number
  createTime: number
  updateTime: number
}

export type PlatformWhiteLabelPreflight = {
  compatible: boolean
  reportJson: string
}

export type PlatformBillingPlan = {
  id: number
  planCode: string
  planName: string
  billingCycle: number
  priceAmount: number
  currency: string
  featureJson: string
  buildsPerCycle: number
  maxBuildConcurrency: number
  storageBytes: number
  maxUploadBytes: number
  teamSeats: number
  apiRateLimit: number
  chargeFailedBuild: boolean
  chargeCacheHit: boolean
  chargeRetryBuild: boolean
  status: number
  version: number
}

export type PlatformTenantSubscription = {
  id: number
  tenantId: number
  planId: number
  planVersion: number
  status: number
  source: number
  currentPeriodStart: number
  currentPeriodEnd: number
  cancelAtPeriodEnd: boolean
  graceUntil?: number
  pendingPlanId: number
}

export type PlatformTenantEntitlement = {
  buildsPerCycle: number
  maxBuildConcurrency: number
  storageBytes: number
  maxUploadBytes: number
  teamSeats: number
  apiRateLimit: number
  validUntil: number
  status: number
}

export type PlatformTenantBilling = {
  subscription: PlatformTenantSubscription
  entitlement: PlatformTenantEntitlement
  plan: PlatformBillingPlan
}

export type PlatformUsageMetricSummary = {
  metric: number
  usedQuantity: number
  reservedQuantity: number
  limitQuantity: number
  usagePercent: number
}

export type PlatformBillingUsage = {
  periodKey: string
  data: PlatformUsageMetricSummary[]
}

export type PlatformInvoice = {
  id: number
  invoiceNo: string
  externalInvoiceId?: string
  status: number
  currency: string
  totalAmount: number
  paidAmount: number
  refundedAmount: number
  periodStart: number
  periodEnd: number
  paidAt?: number
  createTime: number
}

export type PlatformChannelStats = {
  channelId: number
  channelCode: string
  clicks: number
  downloads: number
  installs: number
  registrations: number
  firstPays: number
  pays: number
}

export class PlatformService {
  listApplications = api.listApplications
  createApplication = api.createApplication
  listVersions = api.listVersions
  createVersion = api.createVersion
  uploadObject = api.uploadObject
  listChannels = api.listChannels
  createChannel = api.createChannel
  listSigningConfigs = api.listSigningConfigs
  createSigningConfig = api.createSigningConfig
  listBrandingProfiles = api.listBrandingProfiles
  createBrandingProfile = api.createBrandingProfile
  updateBrandingProfile = api.updateBrandingProfile
  changeBrandingProfileStatus = api.changeBrandingProfileStatus
  createBrandingPreflight = api.createBrandingPreflight
  listBrandingPreflights = api.listBrandingPreflights
  listWhiteLabelTemplates = api.listWhiteLabelTemplates
  createWhiteLabelTemplate = api.createWhiteLabelTemplate
  updateWhiteLabelTemplate = api.updateWhiteLabelTemplate
  copyWhiteLabelTemplate = api.copyWhiteLabelTemplate
  deleteWhiteLabelTemplate = api.deleteWhiteLabelTemplate
  createWhiteLabelTemplateRevision = api.createWhiteLabelTemplateRevision
  getWhiteLabelTemplateRevision = api.getWhiteLabelTemplateRevision
  updateWhiteLabelTemplateRevision = api.updateWhiteLabelTemplateRevision
  deleteWhiteLabelTemplateRevision = api.deleteWhiteLabelTemplateRevision
  listWhiteLabelTemplateRevisions = api.listWhiteLabelTemplateRevisions
  publishWhiteLabelTemplate = api.publishWhiteLabelTemplate
  changeWhiteLabelTemplateStatus = api.changeWhiteLabelTemplateStatus
  listWhiteLabelProducts = api.listWhiteLabelProducts
  createWhiteLabelProduct = api.createWhiteLabelProduct
  updateWhiteLabelProduct = api.updateWhiteLabelProduct
  deleteWhiteLabelProduct = api.deleteWhiteLabelProduct
  changeWhiteLabelProductStatus = api.changeWhiteLabelProductStatus
  preflightWhiteLabelProduct = api.preflightWhiteLabelProduct
  listBuildTasks = api.listBuildTasks
  createBuildTask = api.createBuildTask
  cancelBuildTask = api.cancelBuildTask
  retryBuildTask = api.retryBuildTask
  getBuildClusterMetrics = api.getBuildClusterMetrics
  listOpenApiCredentials = api.listOpenApiCredentials
  createOpenApiCredential = api.createOpenApiCredential
  rotateOpenApiCredential = api.rotateOpenApiCredential
  revokeOpenApiCredential = api.revokeOpenApiCredential
  listWebhookEndpoints = api.listWebhookEndpoints
  createWebhookEndpoint = api.createWebhookEndpoint
  updateWebhookEndpoint = api.updateWebhookEndpoint
  rotateWebhookEndpointSecret = api.rotateWebhookEndpointSecret
  testWebhookEndpoint = api.testWebhookEndpoint
  listWebhookDeliveries = api.listWebhookDeliveries
  replayWebhookDelivery = api.replayWebhookDelivery
  listSourceIntegrations = api.listSourceIntegrations
  createSourceOAuthAuthorization = api.createSourceOAuthAuthorization
  getSourceIntegration = api.getSourceIntegration
  disconnectSourceIntegration = api.disconnectSourceIntegration
  listSourceRepositories = api.listSourceRepositories
  revokeSourceRepository = api.revokeSourceRepository
  listSourceAvailableRepositories = api.listSourceAvailableRepositories
  authorizeSourceRepository = api.authorizeSourceRepository
  importSourceArtifact = api.importSourceArtifact
  listSourceBuildTriggers = api.listSourceBuildTriggers
  createSourceBuildTrigger = api.createSourceBuildTrigger
  getSourceBuildTrigger = api.getSourceBuildTrigger
  updateSourceBuildTrigger = api.updateSourceBuildTrigger
  rotateSourceBuildTriggerSecret = api.rotateSourceBuildTriggerSecret
  listSourceWebhookEvents = api.listSourceWebhookEvents
  listBuilderNodes = api.listBuilderNodes
  drainBuilderNode = api.drainBuilderNode
  recoverBuilderNode = api.recoverBuilderNode
  listBuildConcurrencyPolicies = api.listBuildConcurrencyPolicies
  upsertBuildConcurrencyPolicy = api.upsertBuildConcurrencyPolicy
  listBuildCacheEntries = api.listBuildCacheEntries
  invalidateBuildCache = api.invalidateBuildCache
  cleanupBuildCache = api.cleanupBuildCache
  listBuildSchedulerEvents = api.listBuildSchedulerEvents
  listLocalAgents = api.listLocalAgents
  getLocalAgent = api.getLocalAgent
  createLocalAgentRegistration = api.createLocalAgentRegistration
  drainLocalAgent = api.drainLocalAgent
  revokeLocalAgent = api.revokeLocalAgent
  getDeploymentStatus = api.getDeploymentStatus
  getChannelStats = api.getChannelStats
  getStorageDownload = api.getStorageDownload
  listBillingPlans = api.listBillingPlans
  createBillingPlan = api.createBillingPlan
  retireBillingPlan = api.retireBillingPlan
  getTenantBilling = api.getTenantBilling
  upsertManualSubscription = api.upsertManualSubscription
  changeSubscription = api.changeSubscription
  cancelSubscription = api.cancelSubscription
  getBillingUsage = api.getBillingUsage
  listInvoices = api.listInvoices
  createBillingCheckout = api.createBillingCheckout
}

export type PlatformListCall = (params: PlatformListReq) => Promise<RespBase<any[]>>
export type PlatformCreateCall = (data: Record<string, unknown>) => Promise<RespBase<any>>

export const platformService = new PlatformService()
