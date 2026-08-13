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
  keyAlias: string
  secretRef?: string
  status: number
  lastVerifiedAt?: number
  createTime: number
  updateTime: number
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
  errorMessage?: string
  queuedAt: number
  startTime?: number
  finishTime?: number
  createTime: number
  updateTime: number
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
  listChannels = api.listChannels
  createChannel = api.createChannel
  listSigningConfigs = api.listSigningConfigs
  createSigningConfig = api.createSigningConfig
  listBuildTasks = api.listBuildTasks
  createBuildTask = api.createBuildTask
  getChannelStats = api.getChannelStats
}

export type PlatformListCall = (params: PlatformListReq) => Promise<RespBase<any[]>>
export type PlatformCreateCall = (
  data: Record<string, unknown>,
) => Promise<RespBase<any>>

export const platformService = new PlatformService()
