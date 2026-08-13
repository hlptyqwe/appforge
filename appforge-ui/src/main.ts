import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import '@/styles/admin-crud.css'

import App from '@/App.vue'
import CursorPagination from '@/components/common/CursorPagination.vue'
import { router } from '@/router'
import { i18n, elLocaleMap } from '@/i18n'
import { setupPermDirective } from '@/directives/perm'
import { getSystemCore } from '@/stores/core'
import { buildAssetUrl } from '@/utils/file-url'

const app = createApp(App)

function setFavicon(href: string) {
  let icon = document.querySelector("link[rel~='icon']") as HTMLLinkElement | null
  if (!icon) {
    icon = document.createElement('link')
    icon.rel = 'icon'
    document.head.appendChild(icon)
  }
  icon.href = href
}

;(async () => {
  try {
    const res = await getSystemCore()
    if (res?.code === 200 && res.data) {
      if (res.data.siteName) {
        document.title = res.data.siteName
      }
      if (res.data.siteLogo) {
        setFavicon(buildAssetUrl(res.data.siteLogo))
      }
    }
  } catch (e) {
    console.warn('getSystemCore failed', e)
  }
})()

// Get initial locale from i18n
const initialLocale = i18n.global.locale.value as keyof typeof elLocaleMap
const epLocale = elLocaleMap[initialLocale] || elLocaleMap['zh-CN']

app.use(createPinia())
app.use(i18n)
app.use(router)
app.use(ElementPlus, { locale: epLocale })
app.component('CursorPagination', CursorPagination)

setupPermDirective(app)

app.mount('#app')
