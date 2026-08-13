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

export function buildAssetUrl(
  url?: string,
  options: {
    withTimestamp?: boolean
  } = {},
) {
  if (!url) return ''

  const fullUrl = /^https?:\/\//i.test(url) ? url : joinUrl(ENV.API_BASE_URL, url)

  if (options.withTimestamp === false) {
    return fullUrl
  }

  return `${fullUrl}${fullUrl.includes('?') ? '&' : '?'}t=${Date.now()}`
}
