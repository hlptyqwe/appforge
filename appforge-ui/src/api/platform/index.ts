import { del, get, post, put } from '@/utils/request'
import axios from 'axios'
import type {
  PlatformApplication,
  PlatformBuildTask,
  PlatformBuilderNode,
  PlatformBuildConcurrencyPolicy,
  PlatformBuildCacheEntry,
  PlatformBuildCacheCleanupResult,
  PlatformBuildClusterMetrics,
  PlatformOpenApiCredential,
  PlatformOpenApiCredentialSecret,
  PlatformWebhookEndpoint,
  PlatformWebhookEndpointSecret,
  PlatformWebhookDelivery,
  PlatformSourceIntegration,
  PlatformSourceAvailableRepository,
  PlatformSourceArtifactImportResult,
  PlatformSourceRepository,
  PlatformSourceBuildTrigger,
  PlatformSourceBuildTriggerSecret,
  PlatformSourceWebhookEvent,
  PlatformBuildSchedulerEvent,
  PlatformLocalAgent,
  PlatformLocalAgentRegistration,
  PlatformDeploymentStatus,
  PlatformChannel,
  PlatformChannelStats,
  PlatformListReq,
  PlatformSigningConfig,
  PlatformBrandingProfile,
  PlatformBrandingPreflight,
  PlatformWhiteLabelTemplate,
  PlatformWhiteLabelTemplateRevision,
  PlatformWhiteLabelProduct,
  PlatformWhiteLabelPreflight,
  PlatformVersion,
  PlatformBillingPlan,
  PlatformTenantBilling,
  PlatformUsageMetricSummary,
  PlatformInvoice,
} from '@/services/platform/PlatformService'
import type { RespBase } from '@/services/BaseService'
import { PLATFORM_API_BASE } from '@/config/environment'

const base = PLATFORM_API_BASE

export type PlatformStorageObject = {
  objectId: number
  appId: number
  objectType: number
  originalName: string
  sizeBytes: number
  sha256: string
  status: number
}

type PlatformUploadTicket = {
  objectId: number
  uploadUrl: string
  expiresAt: number
}

export type PlatformStorageDownload = {
  downloadUrl: string
  expiresAt: number
}

export const getStorageDownload = (objectId: number) =>
  get<PlatformStorageDownload>(`${base}/storage/objects/${objectId}/download`)

export async function uploadObject(
  file: File,
  objectType: 1 | 2 | 5 | 6 | 7,
  appId: number,
  onProgress?: (percent: number) => void,
): Promise<RespBase<PlatformStorageObject>> {
  const ticket = await post<PlatformUploadTicket>(`${base}/uploads/initiate`, {
    appId,
    objectType,
    fileName: file.name,
    sizeBytes: file.size,
    contentType: file.type || 'application/octet-stream',
  })
  if (ticket.code !== 200 || !ticket.data) {
    return { code: ticket.code, msg: ticket.msg }
  }

  await axios.put(ticket.data.uploadUrl, file, {
    headers: { 'Content-Type': file.type || 'application/octet-stream' },
    timeout: 0,
    transformRequest: [(value) => value],
    onUploadProgress: (event) => {
      const total = event.total || file.size
      if (total > 0) onProgress?.(Math.min(100, Math.round((event.loaded * 100) / total)))
    },
  })
  return post<PlatformStorageObject>(`${base}/uploads/${ticket.data.objectId}/complete`)
}

export const listApplications = (params: PlatformListReq) =>
  get<PlatformApplication[]>(`${base}/applications`, params)
export const createApplication = (data: Record<string, unknown>) =>
  post<PlatformApplication>(`${base}/applications`, data)

export const listVersions = (params: PlatformListReq) =>
  get<PlatformVersion[]>(`${base}/versions`, params)
export const createVersion = (data: Record<string, unknown>) =>
  post<PlatformVersion>(`${base}/versions`, data)

export const listChannels = (params: PlatformListReq) =>
  get<PlatformChannel[]>(`${base}/channels`, params)
export const createChannel = (data: Record<string, unknown>) =>
  post<PlatformChannel>(`${base}/channels`, data)

export const listSigningConfigs = (params: PlatformListReq) =>
  get<PlatformSigningConfig[]>(`${base}/signing-configs`, params)
export const createSigningConfig = (data: Record<string, unknown>) =>
  post<PlatformSigningConfig>(`${base}/signing-configs`, data)

export const listBrandingProfiles = (params: PlatformListReq) =>
  get<PlatformBrandingProfile[]>(`${base}/branding-profiles`, params)
export const createBrandingProfile = (data: Record<string, unknown>) =>
  post<PlatformBrandingProfile>(`${base}/branding-profiles`, data)
export const updateBrandingProfile = (id: number, data: Record<string, unknown>) =>
  put<PlatformBrandingProfile>(`${base}/branding-profiles/${id}`, data)
export const changeBrandingProfileStatus = (id: number, status: number) =>
  post<PlatformBrandingProfile>(`${base}/branding-profiles/${id}/status`, { status })
export const createBrandingPreflight = (id: number, versionId: number) =>
  post<PlatformBrandingPreflight>(`${base}/branding-profiles/${id}/preflight`, { versionId })
export const listBrandingPreflights = (
  params: PlatformListReq & { brandingProfileId?: number; versionId?: number },
) => get<PlatformBrandingPreflight[]>(`${base}/branding-preflights`, params)

export const listWhiteLabelTemplates = (params: PlatformListReq) =>
  get<PlatformWhiteLabelTemplate[]>(`${base}/white-label/templates`, params)
export const createWhiteLabelTemplate = (data: Record<string, unknown>) =>
  post<PlatformWhiteLabelTemplate>(`${base}/white-label/templates`, data)
export const updateWhiteLabelTemplate = (id: number, data: Record<string, unknown>) =>
  put<PlatformWhiteLabelTemplate>(`${base}/white-label/templates/${id}`, data)
export const copyWhiteLabelTemplate = (id: number, data: Record<string, unknown>) =>
  post<PlatformWhiteLabelTemplate>(`${base}/white-label/templates/${id}/copy`, data)
export const deleteWhiteLabelTemplate = (id: number) => del(`${base}/white-label/templates/${id}`)
export const createWhiteLabelTemplateRevision = (id: number, data: Record<string, unknown>) =>
  post<PlatformWhiteLabelTemplateRevision>(`${base}/white-label/templates/${id}/revisions`, data)
export const getWhiteLabelTemplateRevision = (id: number, revision: number) =>
  get<PlatformWhiteLabelTemplateRevision>(
    `${base}/white-label/templates/${id}/revisions/${revision}`,
  )
export const updateWhiteLabelTemplateRevision = (
  id: number,
  revision: number,
  data: Record<string, unknown>,
) =>
  put<PlatformWhiteLabelTemplateRevision>(
    `${base}/white-label/templates/${id}/revisions/${revision}`,
    data,
  )
export const deleteWhiteLabelTemplateRevision = (id: number, revision: number) =>
  del(`${base}/white-label/templates/${id}/revisions/${revision}`)
export const listWhiteLabelTemplateRevisions = (id: number, params: PlatformListReq = {}) =>
  get<PlatformWhiteLabelTemplateRevision[]>(`${base}/white-label/templates/${id}/revisions`, params)
export const publishWhiteLabelTemplate = (id: number, revision: number) =>
  post<PlatformWhiteLabelTemplate>(`${base}/white-label/templates/${id}/publish`, { revision })
export const changeWhiteLabelTemplateStatus = (id: number, status: number) =>
  post<PlatformWhiteLabelTemplate>(`${base}/white-label/templates/${id}/status`, { status })
export const listWhiteLabelProducts = (params: PlatformListReq) =>
  get<PlatformWhiteLabelProduct[]>(`${base}/white-label/products`, params)
export const createWhiteLabelProduct = (data: Record<string, unknown>) =>
  post<PlatformWhiteLabelProduct>(`${base}/white-label/products`, data)
export const updateWhiteLabelProduct = (id: number, data: Record<string, unknown>) =>
  put<PlatformWhiteLabelProduct>(`${base}/white-label/products/${id}`, data)
export const deleteWhiteLabelProduct = (id: number) => del(`${base}/white-label/products/${id}`)
export const changeWhiteLabelProductStatus = (id: number, status: number) =>
  post<PlatformWhiteLabelProduct>(`${base}/white-label/products/${id}/status`, { status })
export const preflightWhiteLabelProduct = (id: number) =>
  post<never>(`${base}/white-label/products/${id}/preflight`) as Promise<
    RespBase<never> & PlatformWhiteLabelPreflight
  >

export const listBuildTasks = (params: PlatformListReq) =>
  get<PlatformBuildTask[]>(`${base}/build-tasks`, params)
export const createBuildTask = (data: Record<string, unknown>) =>
  post<PlatformBuildTask>(`${base}/build-tasks`, data)
export const cancelBuildTask = (id: number, reason = '') =>
  post<PlatformBuildTask>(`${base}/build-tasks/${id}/cancel`, { reason })
export const retryBuildTask = (id: number, priority = 0) =>
  post<PlatformBuildTask>(`${base}/build-tasks/${id}/retry`, { priority })
export const getBuildClusterMetrics = (params: { poolCode?: string; periodMinutes?: number }) =>
  get<PlatformBuildClusterMetrics>(`${base}/build-cluster/metrics`, params)
export const listOpenApiCredentials = (params: PlatformListReq) =>
  get<PlatformOpenApiCredential[]>(`${base}/developer/credentials`, params)
export const createOpenApiCredential = (data: Record<string, unknown>) =>
  post<PlatformOpenApiCredentialSecret>(`${base}/developer/credentials`, data)
export const rotateOpenApiCredential = (id: number, graceSeconds: number) =>
  post<PlatformOpenApiCredentialSecret>(`${base}/developer/credentials/${id}/rotate`, {
    graceSeconds,
  })
export const revokeOpenApiCredential = (id: number) =>
  post<PlatformOpenApiCredential>(`${base}/developer/credentials/${id}/revoke`)
export const listWebhookEndpoints = (params: PlatformListReq) =>
  get<PlatformWebhookEndpoint[]>(`${base}/developer/webhooks`, params)
export const createWebhookEndpoint = (data: Record<string, unknown>) =>
  post<PlatformWebhookEndpointSecret>(`${base}/developer/webhooks`, data)
export const updateWebhookEndpoint = (id: number, data: Record<string, unknown>) =>
  put<PlatformWebhookEndpoint>(`${base}/developer/webhooks/${id}`, data)
export const rotateWebhookEndpointSecret = (id: number) =>
  post<PlatformWebhookEndpointSecret>(`${base}/developer/webhooks/${id}/rotate-secret`)
export const testWebhookEndpoint = (id: number) =>
  post<never>(`${base}/developer/webhooks/${id}/test`)
export const listWebhookDeliveries = (
  params: Omit<PlatformListReq, 'eventType'> & { endpointId?: number; eventType?: string },
) => get<PlatformWebhookDelivery[]>(`${base}/developer/webhook-deliveries`, params)
export const replayWebhookDelivery = (id: number) =>
  post<PlatformWebhookDelivery>(`${base}/developer/webhook-deliveries/${id}/replay`)
export const listSourceIntegrations = (params: PlatformListReq & { platform?: number }) =>
  get<PlatformSourceIntegration[]>(`${base}/developer/source-integrations`, params)
export const createSourceOAuthAuthorization = (platform: 1 | 2) =>
  post<{ authorizationUrl: string }>(`${base}/developer/source-integrations/${platform}/authorize`)
export const getSourceIntegration = (id: number) =>
  get<PlatformSourceIntegration>(`${base}/developer/source-integrations/${id}`)
export const disconnectSourceIntegration = (id: number) =>
  post<PlatformSourceIntegration>(`${base}/developer/source-integrations/${id}/disconnect`)
export const listSourceRepositories = (params: PlatformListReq & { integrationId?: number }) =>
  get<PlatformSourceRepository[]>(`${base}/developer/source-repositories`, params)
export const revokeSourceRepository = (id: number) =>
  post<PlatformSourceRepository>(`${base}/developer/source-repositories/${id}/revoke`)
export const listSourceAvailableRepositories = (integrationId: number) =>
  get<PlatformSourceAvailableRepository[]>(
    `${base}/developer/source-integrations/${integrationId}/available-repositories`,
  )
export const authorizeSourceRepository = (integrationId: number, externalRepositoryId: string) =>
  post<PlatformSourceRepository>(
    `${base}/developer/source-integrations/${integrationId}/repositories/${encodeURIComponent(externalRepositoryId)}/authorize`,
  )
export const importSourceArtifact = (data: Record<string, unknown>) =>
  post<PlatformSourceArtifactImportResult>(`${base}/developer/source-artifacts/import`, data)
export const listSourceBuildTriggers = (
  params: PlatformListReq & { repositoryId?: number; appId?: number },
) => get<PlatformSourceBuildTrigger[]>(`${base}/developer/source-build-triggers`, params)
export const createSourceBuildTrigger = (data: Record<string, unknown>) =>
  post<PlatformSourceBuildTriggerSecret>(`${base}/developer/source-build-triggers`, data)
export const getSourceBuildTrigger = (id: number) =>
  get<PlatformSourceBuildTrigger>(`${base}/developer/source-build-triggers/${id}`)
export const updateSourceBuildTrigger = (id: number, data: Record<string, unknown>) =>
  put<PlatformSourceBuildTrigger>(`${base}/developer/source-build-triggers/${id}`, data)
export const rotateSourceBuildTriggerSecret = (id: number) =>
  post<PlatformSourceBuildTriggerSecret>(
    `${base}/developer/source-build-triggers/${id}/rotate-secret`,
  )
export const listSourceWebhookEvents = (params: PlatformListReq & { triggerId?: number }) =>
  get<PlatformSourceWebhookEvent[]>(`${base}/developer/source-webhook-events`, params)
export const listBuilderNodes = (params: PlatformListReq) =>
  get<PlatformBuilderNode[]>(`${base}/build-cluster/nodes`, params)
export const drainBuilderNode = (id: number, drainStatus: number) =>
  post<PlatformBuilderNode>(`${base}/build-cluster/nodes/${id}/drain`, { drainStatus })
export const recoverBuilderNode = (id: number, reason: string) =>
  post<PlatformBuilderNode>(`${base}/build-cluster/nodes/${id}/recover`, { reason })
export const listBuildConcurrencyPolicies = (params: PlatformListReq) =>
  get<PlatformBuildConcurrencyPolicy[]>(`${base}/build-cluster/policies`, params)
export const upsertBuildConcurrencyPolicy = (data: Record<string, unknown>) =>
  post<PlatformBuildConcurrencyPolicy>(`${base}/build-cluster/policies`, data)
export const listBuildCacheEntries = (params: PlatformListReq) =>
  get<PlatformBuildCacheEntry[]>(`${base}/build-cluster/cache`, params)
export const invalidateBuildCache = (id: number, reason = '') =>
  post<PlatformBuildCacheEntry>(`${base}/build-cluster/cache/${id}/invalidate`, { reason })
export const cleanupBuildCache = (limit = 100, targetFreeBytes = 0) =>
  post<PlatformBuildCacheCleanupResult>(`${base}/build-cluster/cache/cleanup`, {
    limit,
    targetFreeBytes,
  })
export const listBuildSchedulerEvents = (params: PlatformListReq) =>
  get<PlatformBuildSchedulerEvent[]>(`${base}/build-cluster/events`, params)
export const listLocalAgents = (params: PlatformListReq & { tenantId?: number }) =>
  get<PlatformLocalAgent[]>(`${base}/enterprise/local-agents`, params)
export const getLocalAgent = (id: number, tenantId = 0) =>
  get<PlatformLocalAgent>(`${base}/enterprise/local-agents/${id}`, tenantId ? { tenantId } : {})
export const createLocalAgentRegistration = (data: Record<string, unknown>) =>
  post<PlatformLocalAgent>(`${base}/enterprise/local-agents`, data) as Promise<
    RespBase<PlatformLocalAgent> & PlatformLocalAgentRegistration
  >
export const drainLocalAgent = (id: number, drainStatus: number, tenantId = 0) =>
  post<PlatformLocalAgent>(`${base}/enterprise/local-agents/${id}/drain`, {
    drainStatus,
    tenantId,
  })
export const revokeLocalAgent = (id: number, reason: string, tenantId = 0) =>
  post<PlatformLocalAgent>(`${base}/enterprise/local-agents/${id}/revoke`, { reason, tenantId })
export const getDeploymentStatus = () =>
  get<PlatformDeploymentStatus>(`${base}/enterprise/deployment`)

export const listBillingPlans = (
  params: { page?: number; pageSize?: number; status?: number } = {},
) => get<PlatformBillingPlan[]>(`${base}/billing/plans`, params)
export const createBillingPlan = (data: Record<string, unknown>) =>
  post<PlatformBillingPlan>(`${base}/billing/plans`, data)
export const retireBillingPlan = (id: number) =>
  post<PlatformBillingPlan>(`${base}/billing/plans/${id}/retire`)
export const getTenantBilling = (tenantId = 0) =>
  get<never>(`${base}/billing/subscription`, tenantId ? { tenantId } : {}) as Promise<
    RespBase<never> & PlatformTenantBilling
  >
export const upsertManualSubscription = (data: Record<string, unknown>) =>
  post<never>(`${base}/billing/contracts`, data) as Promise<RespBase<never> & PlatformTenantBilling>
export const changeSubscription = (planId: number, mode = 1, tenantId = 0) =>
  post<never>(`${base}/billing/subscription/change`, { tenantId, planId, mode }) as Promise<
    RespBase<never> & PlatformTenantBilling
  >
export const cancelSubscription = (immediately = false, tenantId = 0) =>
  post<never>(`${base}/billing/subscription/cancel`, { tenantId, immediately }) as Promise<
    RespBase<never> & PlatformTenantBilling
  >
export const getBillingUsage = (params: { tenantId?: number; periodKey?: string } = {}) =>
  get<PlatformUsageMetricSummary[]>(`${base}/billing/usage`, params) as Promise<
    RespBase<PlatformUsageMetricSummary[]> & { periodKey?: string }
  >
export const listInvoices = (
  params: { tenantId?: number; status?: number; page?: number; pageSize?: number } = {},
) => get<PlatformInvoice[]>(`${base}/billing/invoices`, params)
export const createBillingCheckout = (planId: number) =>
  post<never>(`${base}/billing/checkout`, { planId }) as Promise<
    RespBase<never> & { checkoutUrl: string; sessionId: string }
  >

export function getChannelStats(params: {
  appId: number
  channelId: number
  startTime?: number
  endTime?: number
}): Promise<RespBase<PlatformChannelStats>> {
  return get<PlatformChannelStats>(`${base}/channel-stats`, params)
}
