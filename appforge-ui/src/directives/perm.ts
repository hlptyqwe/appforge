import type { App, DirectiveBinding } from 'vue'
import { useAuthStore } from '@/stores/auth'

type PermValue = string | string[] | undefined

function hasPermission(value: PermValue) {
  const auth = useAuthStore()
  const need = Array.isArray(value) ? value.filter(Boolean) : value ? [value] : []

  if (!need.length) return true
  return need.some((perm) => auth.hasPerm(perm))
}

function updateElement(el: HTMLElement, value: PermValue) {
  if (hasPermission(value)) return
  el.remove()
}

export function setupPermDirective(app: App) {
  app.directive('perm', {
    mounted(el: HTMLElement, binding: DirectiveBinding<PermValue>) {
      updateElement(el, binding.value)
    },
    updated(el: HTMLElement, binding: DirectiveBinding<PermValue>) {
      updateElement(el, binding.value)
    },
  })
}
