import type { OptionGroup, RespBase } from '@/services'
import { apiLoginLogList, apiOpLogList } from '@/api/system/logs'
import { getCoreOptions } from '@/stores/core'

// ===== 日志相关类型定义 =====

// 登录日志项
export type LoginLogItem = {
  id: number
  userId: number
  username: string
  ip: string
  ua: string
  success: number
  msg?: string
  loginAt: number
}

// 登录日志列表请求
export interface LoginLogListReq {
  cursor?: number
  limit?: number
  username?: string
  success?: number
}

// 登录日志列表响应
export interface LoginLogListResp {
  code: number
  msg: string
  data: LoginLogItem[]
  total?: number
}

// 操作日志项
export type OpLogItem = {
  id: number
  tenantId: number
  userId: number
  username: string
  module: string
  action: string
  method: string
  path: string
  req?: string
  resp?: string
  ip: string
  costMs: number
  createTimes: number
  updateTimes: number
}

// 操作日志列表请求
export interface OpLogListReq {
  cursor?: number
  limit?: number
  username?: string
  path?: string
  method?: string
}

// 操作日志列表响应
export interface OpLogListResp {
  code: number
  msg: string
  data: OpLogItem[]
  total?: number
}

// 日志接口定义（复用现有的类型）
export interface LoginLog extends LoginLogItem {}
export interface OperationLog extends OpLogItem {}

export interface LoginLogQueryParams extends LoginLogListReq {}
export interface OperationLogQueryParams extends OpLogListReq {}

/**
 * 日志服务类
 * 使用现有的 API 函数
 */
export class LogService {
  protected baseURL: string

  constructor() {
    this.baseURL = '/admin/logs'
  }

  /**
   * 获取登录日志列表
   */
  async getLoginLogs(params?: LoginLogQueryParams): Promise<RespBase<LoginLog[]>> {
    return apiLoginLogList(params || {})
  }

  /**
   * 获取操作日志列表
   */
  async getOperationLogs(params?: OperationLogQueryParams): Promise<RespBase<OperationLog[]>> {
    return apiOpLogList(params || {})
  }

  async getOptions(): Promise<RespBase<OptionGroup[]>> {
    return getCoreOptions()
  }
}

// 导出单例实例
export const logService = new LogService()
