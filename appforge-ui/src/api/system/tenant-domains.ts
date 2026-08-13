import { del, get, post, put } from '@/utils/request'
import type { RespBase } from '@/services'
import type {
  SysTenantDomainCreateReq,
  SysTenantDomainItem,
  SysTenantDomainUpdateReq,
} from '@/services/system/TenantDomainsService'

export function apiSysTenantDomainList(tenantId: number): Promise<RespBase<SysTenantDomainItem[]>> {
  return get<SysTenantDomainItem[]>('/admin/system/tenant-domains', { tenantId })
}

export function apiSysTenantDomainCreate(data: SysTenantDomainCreateReq): Promise<RespBase> {
  return post('/admin/system/tenant-domains', data)
}

export function apiSysTenantDomainUpdate(data: SysTenantDomainUpdateReq): Promise<RespBase> {
  return put('/admin/system/tenant-domains', data)
}

export function apiSysTenantDomainDelete(id: number): Promise<RespBase> {
  return del(`/admin/system/tenant-domains/${id}`)
}
