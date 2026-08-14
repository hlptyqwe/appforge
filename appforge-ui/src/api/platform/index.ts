import { get, post } from '@/utils/request'
import axios from 'axios'
import type {
  PlatformApplication,
  PlatformBuildTask,
  PlatformChannel,
  PlatformChannelStats,
  PlatformListReq,
  PlatformSigningConfig,
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
  objectType: 1 | 2,
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
