import { get, post } from '@/utils/request'
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

const base = '/admin/core'

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
