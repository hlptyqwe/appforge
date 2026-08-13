import type { BaseService, RespBase } from '@/services/BaseService'
import {
  apiSysConfigCreate,
  apiSysConfigDelete,
  apiSysConfigList,
  apiSysConfigUpdate,
} from '@/api/system/config'

export type SysConfigItem = {
  id: number
  tenantId: number
  configKey: string
  configValue: string
  remark?: string
  createTimes: number
  updateTimes: number
}

export type SysConfigListReq = {
  keyword?: string
  tenantId?: number
  cursor?: number
  limit?: number
  count?: number
}

export type SysConfigCreateReq = {
  tenantId?: number
  configKey: string
  configValue: string
  remark?: string
}

export type SysConfigUpdateReq = {
  id: number
  configKey?: string
  configValue?: string
  remark?: string
}

export class ConfigService implements BaseService {
  getList(params: SysConfigListReq): Promise<RespBase<SysConfigItem[]>> {
    return apiSysConfigList(params)
  }

  create(data: SysConfigCreateReq): Promise<RespBase> {
    return apiSysConfigCreate(data)
  }

  update(id: string | number, data: Partial<SysConfigUpdateReq>): Promise<RespBase> {
    return apiSysConfigUpdate({ ...data, id: Number(id) })
  }

  delete(id: string | number): Promise<RespBase> {
    return apiSysConfigDelete(Number(id))
  }
}

export const configService = new ConfigService()
