import { ref } from 'vue'
import { getSystemCore } from '@/stores/core'
import { logger } from '@/utils/logger'
import { buildAssetUrl } from '@/utils/file-url'

export type SystemCoreState = {
  siteName: string
  siteLogo: string
  assetUrl: string
}

const DEFAULT_SYSTEM_CORE: SystemCoreState = {
  siteName: 'Admin UI',
  siteLogo: '',
  assetUrl: '',
}

function normalizeSystemCore(data: Record<string, unknown>): SystemCoreState {
  return {
    siteName: String(data.siteName || data.site_name || DEFAULT_SYSTEM_CORE.siteName),
    siteLogo: String(data.siteLogo || data.site_logo || DEFAULT_SYSTEM_CORE.siteLogo),
    assetUrl: String(data.assetUrl || data.asset_url || DEFAULT_SYSTEM_CORE.assetUrl),
  }
}

function joinAssetUrl(base: string, path: string) {
  path = path.replace(/^\.\//, '/')

  if (base.endsWith('/') && path.startsWith('/')) {
    return `${base.slice(0, -1)}${path}`
  }
  if (!base.endsWith('/') && !path.startsWith('/')) {
    return `${base}/${path}`
  }
  return `${base}${path}`
}

export function buildSystemAssetUrl(assetUrl: string, url?: string) {
  console.info('buildSystemAssetUrl', { assetUrl, url })
  if (!url) return ''
  if (/^https?:\/\//i.test(url)) return url
  if (!assetUrl) return buildAssetUrl(url)
  return joinAssetUrl(assetUrl, url)
}

function ensureFavicon(href: string) {
  let icon = document.querySelector("link[rel~='icon']") as HTMLLinkElement | null
  if (!icon) {
    icon = document.createElement('link')
    icon.rel = 'icon'
    document.head.appendChild(icon)
  }
  icon.href = href
}

export function applySystemBranding(core: SystemCoreState) {
  if (core.siteName) {
    document.title = core.siteName
  }
  if (core.siteLogo) {
    ensureFavicon(buildAssetUrl(core.siteLogo))
  }
}

export function useSystemCore(initial?: Partial<SystemCoreState>) {
  const systemCore = ref<SystemCoreState>({
    ...DEFAULT_SYSTEM_CORE,
    ...initial,
  })

  async function loadSystemCore() {
    try {
      const res = await getSystemCore()
      if (res?.code === 200 && res.data) {
        systemCore.value = normalizeSystemCore(res.data as Record<string, unknown>)
      }
      return systemCore.value
    } catch (error) {
      logger.warn('Failed to load system core config', error)
      return systemCore.value
    }
  }

  return {
    systemCore,
    loadSystemCore,
  }
}
