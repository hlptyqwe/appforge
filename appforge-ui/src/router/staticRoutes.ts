import type { RouteRecordRaw } from 'vue-router'

export const staticRoutes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/login/index.vue'),
    meta: { public: true, titleKey: 'route.login' },
  },
  {
    path: '/google2fa-bind',
    name: 'Google2faBind',
    component: () => import('@/views/auth/Google2faBind.vue'),
    meta: { titleKey: 'route.google2faBind', skipGoogle2fa: true },
  },
  {
    path: '/',
    name: 'Layout',
    component: () => import('@/layout/index.vue'),
    children: [
      {
        path: '/home',
        name: 'Home',
        component: () => import('@/views/home/index.vue'),
        meta: { titleKey: 'route.home', affix: true },
      },
    ],
  },
]
