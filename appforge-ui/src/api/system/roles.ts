import { get, post, put, del } from '@/utils/request'
import type {
  RespBase,
  SysRole,
  RoleQueryParams,
  CreateRoleRequest,
  UpdateRoleRequest,
  RoleGrantRequest,
} from '@/services'

// ===== types =====

export function apiRoleList(params: RoleQueryParams): Promise<RespBase<SysRole[]>> {
  return get('/admin/system/roles', params)
}

export async function apiRoleCreate(params: CreateRoleRequest): Promise<RespBase> {
  // POST /roles
  return await post('/admin/system/roles', params)
}
export async function apiRoleUpdate(params: UpdateRoleRequest): Promise<RespBase> {
  // PUT /roles
  return await put('/admin/system/roles', params)
}
export async function apiRoleDelete(id: number): Promise<RespBase> {
  // DELETE /roles/:id
  return await del(`/admin/system/roles/${id}`)
}
export async function apiRoleGrant(params: RoleGrantRequest): Promise<RespBase> {
  // POST /roles/grant
  return await post('/admin/system/roles/grant', params)
}
export async function apiRoleGrantDetail(roleId: number): Promise<RespBase<RoleGrantRequest>> {
  return await get(`/admin/system/roles/${roleId}/grant`)
}
