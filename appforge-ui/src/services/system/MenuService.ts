import type { RespBase, BaseService } from '@/services'
import {
  apiMenuTree,
  apiPermList,
  sysMenuList,
  sysMenuCreate,
  sysMenuDelete,
  sysMenuUpdate,
} from '@/api/system/menus'
import type { OptionGroup } from '@/services'
import { getCoreOptions } from '@/stores/core'

// ===== 菜单相关类型定义 =====

export type MenuNode = { id: number; name: string; children?: MenuNode[] }
export type PermItem = { key: string; name: string; group?: string }
export type Menu = SysMenuItem
export type Permission = PermItem
export type MenuQueryParams = SysMenuListReq
export type CreateMenuRequest = SysMenuCreateReq
export type UpdateMenuRequest = SysMenuUpdateReq
export type SysMenuListResp = SysMenuItem[]

export type SysMenuCreateReq = {
  parentId: number
  name: string
  menuType: number
  path: string
  component: string
  icon: string
  sort: number
  visible: number // 显示开关：1显示 2隐藏
  enabled: number // 启用状态：1启用 2禁用
  perms: string
  appScope: number
}

export type SysMenuDeleteReq = {
  id: number
}

export type SysMenuUpdateReq = {
  id: number
  parentId: number
  name: string
  menuType: number
  path: string
  component: string
  icon: string
  sort: number
  visible: number // 显示开关：1显示 2隐藏
  enabled: number // 启用状态：1启用 2禁用
  perms: string
  appScope?: number
}

export type SysMenuListReq = {
  cursor?: number
  limit?: number
  keyword: string
  menuType: number
  enabled: number // 启用状态：1启用 2禁用
  visible: number // 显示开关：1显示 2隐藏
  appScope?: number
}

export type SysMenuItem = {
  id: number
  parentId: number
  name: string
  menuType: number // 1目录 2菜单 3按钮
  path: string
  component: string
  icon: string
  sort: number
  visible: number // 显示开关：1显示 2隐藏
  enabled: number // 启用状态：1启用 2禁用
  perms: string // 按钮必须有，例如 sys:user:add
  appScope: number
}

export type SysMenuTreeItem = SysMenuItem & {
  children?: SysMenuTreeItem[]
}

/**
 * 菜单服务类
 * 实现 BaseService 接口，使用现有的 API 函数
 */
export class MenuService implements BaseService {
  /**
   * 获取菜单树
   */
  async getMenuTree(roleId: number): Promise<RespBase<MenuNode[]>> {
    return apiMenuTree(roleId)
  }

  /**
   * 获取权限列表
   */
  async getPermissionList(): Promise<RespBase<PermItem[]>> {
    return apiPermList()
  }

  /**
   * 获取菜单列表
   */
  async getList(params?: SysMenuListReq): Promise<RespBase<SysMenuItem[]>> {
    return sysMenuList(params || ({} as SysMenuListReq))
  }

  async getOptions(): Promise<RespBase<OptionGroup[]>> {
    return getCoreOptions()
  }

  /**
   * 创建菜单
   */
  async create(data: SysMenuCreateReq): Promise<RespBase> {
    return sysMenuCreate(data)
  }

  /**
   * 更新菜单
   */
  async update(id: string | number, data: SysMenuUpdateReq): Promise<RespBase> {
    return sysMenuUpdate({ ...data, id: Number(id) })
  }

  /**
   * 删除菜单
   */
  async delete(id: string | number): Promise<RespBase> {
    return sysMenuDelete(Number(id))
  }
}

// 导出单例实例
export const menuService = new MenuService()
