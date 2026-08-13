import { createI18n } from 'vue-i18n'
import elZhCN from 'element-plus/es/locale/lang/zh-cn'
import elEnUS from 'element-plus/es/locale/lang/en'
import zhCN from './locales/zh-CN'
import enUS from './locales/en-US'

export type Locale = 'zh-CN' | 'en-US'

const saved = (localStorage.getItem('locale') as Locale) || 'zh-CN'

export const i18n = createI18n({
  legacy: false,
  locale: saved,
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS,
  },
})

// Store Element Plus locale for dynamic switching
export const elLocaleMap = {
  'zh-CN': elZhCN,
  'en-US': elEnUS,
}

export function setLocale(locale: Locale) {
  i18n.global.locale.value = locale
  localStorage.setItem('locale', locale)
}
