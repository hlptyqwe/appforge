import { del, get, post, put } from '@/utils/request'
import axios from 'axios'
import type {
  PlatformApplication,
  PlatformBuildTask,
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

export function getChannelStats(params: {
  appId: number
  channelId: number
  startTime?: number
  endTime?: number
}): Promise<RespBase<PlatformChannelStats>> {
  return get<PlatformChannelStats>(`${base}/channel-stats`, params)
}
