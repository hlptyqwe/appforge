import { del, get, post, put } from '@/utils/request'
import type {
  RespBase,
  MenuNode,
  PermItem,
  SysMenuCreateReq,
  SysMenuUpdateReq,
  SysMenuListReq,
  SysMenuListResp,
} from '@/services'

export async function apiMenuTree(roleId: number): Promise<RespBase<MenuNode[]>> {
  return await get(`/admin/system/menus/tree/${roleId}`)
}
export async function apiPermList(): Promise<RespBase<PermItem[]>> {
  return await get('/admin/system/perms')
}

/** 新增菜单 */
export async function sysMenuCreate(data: SysMenuCreateReq): Promise<RespBase> {
  return await post('/admin/system/menus', data)
}

/** 更新菜单 */
export async function sysMenuUpdate(data: SysMenuUpdateReq): Promise<RespBase> {
  return await put('/admin/system/menus', data)
}

/** 删除菜单 */
export async function sysMenuDelete(id: number): Promise<RespBase> {
  return await del(`/admin/system/menus/${id}`)
}

/** 菜单列表 */
export async function sysMenuList(data: SysMenuListReq): Promise<RespBase<SysMenuListResp>> {
  return await get('/admin/system/menus', data)
}
