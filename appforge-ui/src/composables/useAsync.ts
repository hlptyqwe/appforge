/**
 * 异步数据加载 Hook
 */

import { ref, computed } from 'vue'
import { logger } from '@/utils/logger'

export interface UseAsyncOptions<T> {
  initialData?: T
  immediate?: boolean
  onSuccess?: (data: T) => void
  onError?: (error: unknown) => void
}

export function useAsync<T = unknown>(asyncFn: () => Promise<T>, options?: UseAsyncOptions<T>) {
  const { initialData, immediate = true, onSuccess, onError } = options ?? {}

  const data = ref<T | undefined>(initialData)
  const loading = ref(false)
  const error = ref<unknown>(null)

  const isLoading = computed(() => loading.value)
  const isError = computed(() => !!error.value)

  const execute = async () => {
    loading.value = true
    error.value = null

    try {
      const result = await asyncFn()
      data.value = result
      onSuccess?.(result)
      logger.info('Async operation succeeded')
      return result
    } catch (err) {
      error.value = err
      onError?.(err)
      logger.error('Async operation failed', err)
      return null
    } finally {
      loading.value = false
    }
  }

  const retry = () => execute()

  if (immediate) {
    execute()
  }

  return {
    data,
    loading,
    error,
    isLoading,
    isError,
    execute,
    retry,
  }
}
