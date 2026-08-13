import { get, post, put, del } from '@/utils/request'
import type {
  RespBase,
  SysTenantListReq,
  SysTenantItem,
  SysTenantCreateReq,
  SysTenantUpdateReq,
  SysTenantDetailReq,
} from '@/services'

// ===== API 函数 =====

export function apiSysTenantList(params: SysTenantListReq): Promise<RespBase<SysTenantItem[]>> {
  return get<SysTenantItem[]>('/admin/system/tenants', params)
}

export function apiSysTenantCreate(data: SysTenantCreateReq): Promise<RespBase> {
  return post('/admin/system/tenants', data)
}

export function apiSysTenantUpdate(data: SysTenantUpdateReq): Promise<RespBase> {
  return put('/admin/system/tenants', data)
}

export function apiSysTenantDelete(id: number): Promise<RespBase> {
  return del(`/admin/system/tenants/${id}`)
}

export function apiSysTenantDetail(params: SysTenantDetailReq): Promise<RespBase<SysTenantItem>> {
  return get<SysTenantItem>('/admin/system/tenant/detail', params)
}
