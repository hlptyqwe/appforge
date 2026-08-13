import { get } from '@/utils/request'
import type { OptionGroup, RespBase } from '@/services'

type SystemCore = {
  siteName: string
  siteLogo: string
  assetUrl: string
  mustGoogleF2a: number
  options?: OptionGroup[]
}

export function getSystemCore(): Promise<RespBase<SystemCore>> {
  return get<SystemCore>('/admin/system/core')
}

let coreOptionsPromise: Promise<RespBase<OptionGroup[]>> | null = null

export function getCoreOptions(): Promise<RespBase<OptionGroup[]>> {
  if (!coreOptionsPromise) {
    coreOptionsPromise = getSystemCore().then((res) => ({
      ...res,
      data: res.data?.options || [],
    }))
  }

  return coreOptionsPromise
}
