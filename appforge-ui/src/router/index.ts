import { createMemoryHistory, createRouter, type RouteRecordRaw } from 'vue-router'
import { staticRoutes } from './staticRoutes'
import { useAuthStore } from '@/stores/auth'
import { getSystemCore } from '@/stores/core'
import { buildRoutesFromMenus } from './dynamic'

const browserRootPath = '/'
const memoryRouteStorageKey = 'admin-ui:memory-route'

if (typeof window !== 'undefined') {
  const currentUrl = `${window.location.pathname}${window.location.search}${window.location.hash}`
  if (currentUrl !== browserRootPath) {
    window.history.replaceState(window.history.state, '', browserRootPath)
  }
}

export const router = createRouter({
  history: createMemoryHistory(),
  routes: staticRoutes,
})

function getSavedMemoryRoute() {
  if (typeof window === 'undefined') return ''

  const path = window.sessionStorage.getItem(memoryRouteStorageKey) || ''
  if (!path || path === '/' || path.startsWith('/login')) return ''
  return path
}

function saveMemoryRoute(path: string) {
  if (typeof window === 'undefined') return
  if (!path || path === '/' || path.startsWith('/login')) return

  window.sessionStorage.setItem(memoryRouteStorageKey, path)
}

const dynamicRouteNames = new Set<string>()
let dynamicAdded = false
let mustGoogleF2a: number | null = null

function ensureNotFoundRoute() {
  if (router.hasRoute('NotFound')) return

  router.addRoute({
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/error/404.vue'),
    meta: { titleKey: 'route.notFound' },
  })
}

function addDynamicRoutes(routes: RouteRecordRaw[]) {
  routes.forEach((route) => {
    router.addRoute('Layout', route)
    if (route.name) {
      dynamicRouteNames.add(String(route.name))
    }
  })
  ensureNotFoundRoute()
  dynamicAdded = true
}

function isAvailableAuthorizedPath(path: string) {
  if (!path.startsWith('/') || path.startsWith('//') || path.startsWith('/login')) {
    return false
  }

  const resolved = router.resolve(path)
  return (
    resolved.matched.length > 0 && !resolved.matched.some((record) => record.name === 'NotFound')
  )
}

export function resolvePostLoginPath(
  requestedPath: string,
  menus: Parameters<typeof buildRoutesFromMenus>[0],
) {
  if (!dynamicAdded) {
    addDynamicRoutes(buildRoutesFromMenus(menus))
  }

  const requested = requestedPath.trim()
  const candidates = requested && requested !== '/' ? [requested] : [getSavedMemoryRoute()]
  candidates.push('/home')

  for (const candidate of candidates) {
    if (candidate && isAvailableAuthorizedPath(candidate)) {
      return candidate
    }
  }

  return '/home'
}

export function resetDynamicRoutes() {
  dynamicRouteNames.forEach((name) => {
    if (router.hasRoute(name)) {
      router.removeRoute(name)
    }
  })
  dynamicRouteNames.clear()

  if (router.hasRoute('NotFound')) {
    router.removeRoute('NotFound')
  }

  dynamicAdded = false
}

async function isGoogle2faRequired() {
  if (mustGoogleF2a === null) {
    try {
      const res = await getSystemCore()
      mustGoogleF2a = Number(res.data?.mustGoogleF2a || 0)
    } catch {
      mustGoogleF2a = 0
    }
  }

  return mustGoogleF2a === 1
}

function isGoogle2faEnabled(value: unknown) {
  return value === 1 || value === '1' || value === 'ENABLE_ENABLED'
}

router.beforeEach(async (to) => {
  const auth = useAuthStore()

  if ((to.meta as any)?.public) return true

  if (!auth.token) return { path: '/login', query: { redirect: to.fullPath } }

  if (!auth.isProfileLoaded) {
    await auth.fetchProfile()
  }

  const shouldBindGoogle2fa =
    (await isGoogle2faRequired()) && !isGoogle2faEnabled(auth.user?.google2FaEnabled)
  const isGoogle2faBindPage = to.name === 'Google2faBind'

  if (shouldBindGoogle2fa && !isGoogle2faBindPage) {
    return {
      path: '/google2fa-bind',
      replace: true,
    }
  }

  if (!shouldBindGoogle2fa && isGoogle2faBindPage) {
    return { path: '/home', replace: true }
  }

  if (isGoogle2faBindPage) return true

  if (to.path === '/') {
    return {
      path: resolvePostLoginPath(getSavedMemoryRoute(), auth.menus),
      replace: true,
    }
  }

  if (!dynamicAdded) {
    addDynamicRoutes(buildRoutesFromMenus(auth.menus))
    return { ...to, replace: true }
  }

  return true
})

router.afterEach((to) => {
  if (to.name === 'NotFound') return
  saveMemoryRoute(to.fullPath)
})
