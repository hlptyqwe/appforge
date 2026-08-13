import { get } from '@/utils/request'
import type { RespBase, LoginLogItem, LoginLogListReq, OpLogItem, OpLogListReq } from '@/services'

// ===== 登录日志 =====
export function apiLoginLogList(params: LoginLogListReq): Promise<RespBase<LoginLogItem[]>> {
  return get<LoginLogItem[]>('/admin/system/logs/login', params)
}

// ===== 操作日志 =====
export function apiOpLogList(params: OpLogListReq): Promise<RespBase<OpLogItem[]>> {
  return get<OpLogItem[]>('/admin/system/logs/op', params)
}
