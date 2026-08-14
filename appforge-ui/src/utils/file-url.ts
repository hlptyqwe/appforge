import { ENV } from '@/config/environment'

function joinUrl(base: string, path: string) {
  if (base.endsWith('/') && path.startsWith('/')) {
    return `${base.slice(0, -1)}${path}`
  }
  if (!base.endsWith('/') && !path.startsWith('/')) {
    return `${base}/${path}`
  }
  return `${base}${path}`
}

export function buildApiUrl(path: string) {
  if (!path) return ENV.API_BASE_URL
  return /^https?:\/\//i.test(path) ? path : joinUrl(ENV.API_BASE_URL, path)
}

export function buildAssetUrl(
  url?: string,
  options: {
    withTimestamp?: boolean
  } = {},
) {
  if (!url) return ''

  const fullUrl = buildApiUrl(url)

  if (options.withTimestamp === false) {
    return fullUrl
  }

  return `${fullUrl}${fullUrl.includes('?') ? '&' : '?'}t=${Date.now()}`
}
