// ===== 通用类型定义 =====

export type RespBase<T = any> = {
  code: number
  msg: string
  data?: T
  total?: number
  nextCursor?: number
  prevCursor?: number
  hasNext?: boolean
  hasPrev?: boolean
}

export type PageReq = {
  cursor?: number
  limit?: number
  count?: number
}

export interface BaseServiceOptions {
  baseURL?: string
  timeout?: number
}

/**
 * 基础服务接口
 * 定义通用的服务方法签名
 */
export interface BaseService {
  /**
   * 获取列表（支持分页）
   */
  getList?(params?: Record<string, any>): Promise<RespBase<any>>

  /**
   * 获取详情
   */
  getDetail?(id: string | number): Promise<RespBase<any>>

  /**
   * 创建
   */
  create?(data: Record<string, any>): Promise<RespBase<any>>

  /**
   * 更新
   */
  update?(id: string | number, data: Record<string, any>): Promise<RespBase<any>>

  /**
   * 删除
   */
  delete?(id: string | number): Promise<RespBase<any>>

  /**
   * 批量删除
   */
  batchDelete?(ids: (string | number)[]): Promise<RespBase<any>>

  /**
   * 部分更新
   */
  patch?(id: string | number, data: Record<string, any>): Promise<RespBase<any>>
}

/**
 * 基础服务实现类
 * 提供一些通用功能
 */
export abstract class BaseServiceImpl {
  protected baseURL: string
  protected timeout: number

  constructor(baseURL: string, options: BaseServiceOptions = {}) {
    this.baseURL = baseURL
    this.timeout = options.timeout || 10000
  }
}

export type OptionItem = {
  value: number
  code: string
}

export type OptionGroup = {
  key: string
  label: string
  options: OptionItem[]
}
