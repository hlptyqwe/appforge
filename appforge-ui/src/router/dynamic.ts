import type { RouteRecordRaw } from 'vue-router'
import type { MenuNode } from '@/stores/auth'

const viewModules = import.meta.glob('@/views/**/*.vue')

function resolveView(component?: string) {
  if (!component) return null
  const key = `/src/views/${component}.vue`
  return viewModules[key] ? viewModules[key] : null
}

function sortChildren(nodes: MenuNode[]) {
  return [...(nodes || [])].sort((a, b) => (a.sort ?? 0) - (b.sort ?? 0))
}

export function buildRoutesFromMenus(menus: MenuNode[]): RouteRecordRaw[] {
  const routes: RouteRecordRaw[] = []

  const dfs = (nodes: MenuNode[]) => {
    for (const n of sortChildren(nodes)) {
      if (n.enabled !== 1 || n.visible !== 1) continue
      if (n.menuType === 2 && n.path) {
        const comp = resolveView(n.component)
        routes.push({
          path: n.path,
          name: `Menu_${n.id}`,
          component: (comp as unknown) || (() => import('@/views/error/missing-view.vue')),
          meta: {
            title: n.name,
            titleKey: `menu.${n.id}`,
            icon: n.icon,
            menuId: n.id,
            missing: comp ? '' : n.component,
          },
        })
      }
      if (n.children?.length) dfs(n.children)
    }
  }

  dfs(menus || [])
  return routes
}
