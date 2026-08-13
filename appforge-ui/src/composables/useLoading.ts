/**
 * 加载状态 Hook
 */

import { ref } from 'vue'

export function useLoading(initialState = false) {
  const loading = ref(initialState)

  const setLoading = (value: boolean) => {
    loading.value = value
  }

  const startLoading = () => {
    loading.value = true
  }

  const stopLoading = () => {
    loading.value = false
  }

  const withLoading = async <T>(callback: () => Promise<T>): Promise<T> => {
    startLoading()
    try {
      return await callback()
    } finally {
      stopLoading()
    }
  }

  return {
    loading,
    setLoading,
    startLoading,
    stopLoading,
    withLoading,
  }
}
