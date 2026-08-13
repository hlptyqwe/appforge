import type { BaseService, OptionGroup, RespBase } from '@/services'
import { getCoreOptions } from '@/stores/core'
import {
  apiSysTenantDomainCreate,
  apiSysTenantDomainDelete,
  apiSysTenantDomainList,
  apiSysTenantDomainUpdate,
} from '@/api/system/tenant-domains'

export type SysTenantDomainItem = {
  id: number
  tenantId: number
  origin: string
  status: number
  priority: number
  createTimes: number
  updateTimes: number
}

export type SysTenantDomainCreateReq = {
  tenantId: number
  origin: string
  status: number
  priority: number
}

export type SysTenantDomainUpdateReq = Omit<SysTenantDomainCreateReq, 'tenantId'> & { id: number }

export class TenantDomainsService implements BaseService {
  getOptions(): Promise<RespBase<OptionGroup[]>> {
    return getCoreOptions()
  }

  getList(params: { tenantId: number }): Promise<RespBase<SysTenantDomainItem[]>> {
    return apiSysTenantDomainList(params.tenantId)
  }

  create(data: SysTenantDomainCreateReq): Promise<RespBase> {
    return apiSysTenantDomainCreate(data)
  }

  update(id: string | number, data: Partial<SysTenantDomainUpdateReq>): Promise<RespBase> {
    return apiSysTenantDomainUpdate({
      id: Number(id),
      origin: String(data.origin || ''),
      status: Number(data.status),
      priority: Number(data.priority || 0),
    })
  }

  delete(id: string | number): Promise<RespBase> {
    return apiSysTenantDomainDelete(Number(id))
  }
}

export const tenantDomainsService = new TenantDomainsService()
