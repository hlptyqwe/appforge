/**
 * 表单 Hook
 */

import { ref, reactive } from 'vue'
import { logger } from '@/utils/logger'

export interface FormOptions<T> {
  initialData: T
  validate?: (data: T) => boolean | Promise<boolean>
}

export function useForm<T extends object>(options: FormOptions<T>) {
  const { initialData, validate } = options
  const form = reactive({ ...initialData }) as T
  const formRef = ref()
  const errors = reactive<Record<string, string>>({})

  const reset = () => {
    ;(Object.keys(form) as Array<keyof T>).forEach((key) => {
      form[key] = initialData[key]
    })
    // Clear errors
    Object.keys(errors).forEach((key) => {
      delete errors[key]
    })
  }

  const getFormData = (): T => {
    return { ...form } as T
  }

  const setFormData = (data: Partial<T>) => {
    Object.assign(form, data)
  }

  const clear = () => {
    ;(Object.keys(form) as Array<keyof T>).forEach((key) => {
      form[key] = undefined as T[keyof T]
    })
  }

  const submit = async (): Promise<T | null> => {
    try {
      // 使用 Element Plus 的表单验证
      if (formRef.value) {
        await formRef.value.validate()
      }

      // 自定义验证
      if (validate) {
        const isValid = await validate(getFormData())
        if (!isValid) {
          logger.warn('Form validation failed')
          return null
        }
      }

      return getFormData()
    } catch (error) {
      logger.error('Form submit error:', error)
      return null
    }
  }

  return {
    form: form as T,
    formRef,
    errors,
    reset,
    getFormData,
    setFormData,
    clear,
    submit,
  }
}
