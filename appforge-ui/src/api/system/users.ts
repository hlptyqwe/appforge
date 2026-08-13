import { get, post, put, del } from '@/utils/request'
import type { RespBase, SysUserItem, Google2FABindInitResp } from '@/services'

export function apiUserList(params: {
  keyword?: string
  enabled?: number
  cursor?: number
  limit?: number
  appScope?: number
}): Promise<RespBase<SysUserItem[]>> {
  return get<SysUserItem[]>('/admin/system/users', params)
}

export function apiUserDetail(id: number): Promise<RespBase<SysUserItem>> {
  return get<SysUserItem>(`/admin/system/users/${id}`)
}

export function apiUserCreate(data: {
  username: string
  password: string
  nickname?: string
  enabled?: number
  roleIds?: number[]
  appScope: number
}): Promise<RespBase> {
  return post<RespBase>('/admin/system/users', data)
}

export function apiUserUpdate(data: {
  id: number
  nickname?: string
  enabled?: number
  roleIds?: number[]
  appScope?: number
}): Promise<RespBase> {
  return put<RespBase>('/admin/system/users', data)
}

export function apiUserDelete(id: number): Promise<RespBase> {
  return del<RespBase>(`/admin/system/users/${id}`)
}

export function apiChangeUserEnabled(data: { id: number; enabled: number }): Promise<RespBase> {
  return post<RespBase>('/admin/system/users/status', data)
}

export function apiResetUserPwd(data: { id: number; password: string }): Promise<RespBase> {
  return post<RespBase>('/admin/system/users/resetPwd', data)
}

export function apiAssignUserRoles(data: { userId: number; roleIds: number[] }): Promise<RespBase> {
  return post<RespBase>('/admin/system/users/assignRoles', data)
}

// ---- Google 2FA ----
export function apiGoogle2faInit(data: {
  userId: number
}): Promise<RespBase<Google2FABindInitResp>> {
  return post<Google2FABindInitResp>('/admin/system/users/google2fa/init', data)
}

export function apiGoogle2faBind(data: {
  userId: number
  secret: string
  code: string
}): Promise<RespBase> {
  return post<RespBase>('/admin/system/users/google2fa/bind', data)
}

export function apiGoogle2faEnable(data: { userId: number; code: string }): Promise<RespBase> {
  return post<RespBase>('/admin/system/users/google2fa/enable', data)
}

export function apiGoogle2faDisable(data: { userId: number; code?: string }): Promise<RespBase> {
  return post<RespBase>('/admin/system/users/google2fa/disable', data)
}

export function apiGoogle2faReset(data: { userId: number }): Promise<RespBase> {
  return post<RespBase>('/admin/system/users/google2fa/reset', data)
}
