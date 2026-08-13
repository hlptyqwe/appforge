import type { RespBase, BaseService, OptionGroup } from '@/services'
import { getCoreOptions } from '@/stores/core'

import {
  apiSysTenantList,
  apiSysTenantCreate,
  apiSysTenantUpdate,
  apiSysTenantDelete,
  apiSysTenantDetail,
} from '@/api/system/tenants'

export type SysTenantCreateReq = {
  username: string
  tenantName: string
  tenantPassword: string
  enabled: number // 启用状态：1启用 2禁用
  expireTime: number
  contactName: string
  contactPhone: string
  remark: string
}

export type SysTenantUpdateReq = {
  id: number
  tenantCode?: string
  tenantName?: string
  enabled?: number // 启用状态：1启用 2禁用
  expireTime?: number
  contactName?: string
  contactPhone?: string
  remark?: string
}

export type SysTenantItem = {
  id: number
  tenantCode: string
  tenantName: string
  enabled: number // 启用状态：1启用 2禁用
  expireTime: number
  contactName: string
  contactPhone: string
  remark: string
  createTimes: number
  updateTimes: number
}

export type SysTenantListReq = {
  keyword?: string
  enabled?: number // 启用状态：1启用 2禁用
  tenantCode?: string
  tenantName?: string
  contactName?: string
  contactPhone?: string
  cursor?: number
  limit?: number
}

export type SysTenantDetailReq = {
  tenantId?: number
  tenantCode?: string
}

// ========= 租户服务 =========
export class TenantsService implements BaseService {
  async getOptions(): Promise<RespBase<OptionGroup[]>> {
    return getCoreOptions()
  }

  async getList(params: SysTenantListReq): Promise<RespBase<SysTenantItem[]>> {
    return apiSysTenantList(params)
  }

  async create(params: SysTenantCreateReq): Promise<RespBase> {
    return apiSysTenantCreate(params)
  }

  async update(id: string | number, params: Partial<SysTenantUpdateReq>): Promise<RespBase> {
    return apiSysTenantUpdate({ id: Number(id), ...params })
  }

  async delete(id: string | number): Promise<RespBase> {
    return apiSysTenantDelete(Number(id))
  }

  async detail(params: SysTenantDetailReq): Promise<RespBase<SysTenantItem>> {
    return apiSysTenantDetail(params)
  }
}

export const tenantsService = new TenantsService()
