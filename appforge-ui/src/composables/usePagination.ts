/**
 * 分页 Hook
 */

import { reactive } from 'vue'

export interface PaginationState<TCursor = number> {
  cursor: TCursor | undefined
  nextCursor: TCursor | undefined
  prevCursor: TCursor | undefined
  limit: number
  total: number
  hasNext: boolean
  hasPrev: boolean
}

export interface PaginationResponse<TCursor = number> {
  total?: number
  hasNext?: boolean
  hasPrev?: boolean
  nextCursor?: TCursor
  prevCursor?: TCursor
}

type MaybePromise<T> = T | Promise<T>
type LoadFn = () => MaybePromise<void>

export function usePagination<TCursor = number>(initialLimit = 10) {
  const pagination = reactive({
    cursor: undefined,
    nextCursor: undefined,
    prevCursor: undefined,
    limit: initialLimit,
    total: 0,
    hasNext: false,
    hasPrev: false,
  }) as PaginationState<TCursor>

  const updateFromResponse = (res: PaginationResponse<TCursor>) => {
    pagination.total = res.total || 0
    pagination.hasNext = !!res.hasNext
    pagination.hasPrev = !!res.hasPrev
    pagination.nextCursor = res.nextCursor ?? undefined
    pagination.prevCursor = res.prevCursor ?? undefined
  }

  const reset = () => {
    pagination.cursor = undefined
    pagination.nextCursor = undefined
    pagination.prevCursor = undefined
    pagination.total = 0
    pagination.hasNext = false
    pagination.hasPrev = false
  }

  const nextPage = () => {
    if (pagination.hasNext && pagination.nextCursor !== undefined) {
      pagination.cursor = pagination.nextCursor
      return true
    }
    return false
  }

  const prevPage = () => {
    if (pagination.hasPrev && pagination.prevCursor !== undefined) {
      pagination.cursor = pagination.prevCursor
      return true
    }
    return false
  }

  const resetAndLoad = (load: LoadFn) => {
    reset()
    return load()
  }

  const nextAndLoad = (load: LoadFn) => {
    if (nextPage()) {
      return load()
    }
    return undefined
  }

  const prevAndLoad = (load: LoadFn) => {
    if (prevPage()) {
      return load()
    }
    return undefined
  }

  return {
    pagination,
    updateFromResponse,
    reset,
    nextPage,
    prevPage,
    resetAndLoad,
    nextAndLoad,
    prevAndLoad,
  }
}
