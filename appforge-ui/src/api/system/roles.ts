import { get, post, put, del } from '@/utils/request'
import type {
  RespBase,
  SysRole,
  RoleQueryParams,
  CreateRoleRequest,
  UpdateRoleRequest,
  RoleGrantRequest,
} from '@/services'
import { TEAM_API_BASE } from '@/config/environment'

// ===== types =====

export function apiRoleList(params: RoleQueryParams): Promise<RespBase<SysRole[]>> {
	return get(`${TEAM_API_BASE}/roles`, params)
}

export async function apiRoleCreate(params: CreateRoleRequest): Promise<RespBase> {
  // POST /roles
	return await post(`${TEAM_API_BASE}/roles`, params)
}
export async function apiRoleUpdate(params: UpdateRoleRequest): Promise<RespBase> {
  // PUT /roles
	return await put(`${TEAM_API_BASE}/roles`, params)
}
export async function apiRoleDelete(id: number): Promise<RespBase> {
  // DELETE /roles/:id
	return await del(`${TEAM_API_BASE}/roles/${id}`)
}
export async function apiRoleGrant(params: RoleGrantRequest): Promise<RespBase> {
  // POST /roles/grant
	return await post(`${TEAM_API_BASE}/roles/grant`, params)
}
export async function apiRoleGrantDetail(roleId: number): Promise<RespBase<RoleGrantRequest>> {
	return await get(`${TEAM_API_BASE}/roles/${roleId}/grant`)
}
