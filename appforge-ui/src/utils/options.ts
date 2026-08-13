import type { ComposerTranslation } from 'vue-i18n'
import type { OptionGroup, OptionItem } from '@/services'

export function findOptionGroup(groups: OptionGroup[] | undefined, key: string): OptionItem[] {
  return groups?.find((item) => item.key === key)?.options || []
}

export function withoutUnknownOptions(options: OptionItem[]): OptionItem[] {
  return options.filter((item) => !String(item.code || '').endsWith('_UNKNOWN'))
}

export function findFormOptionGroup(groups: OptionGroup[] | undefined, key: string): OptionItem[] {
  return withoutUnknownOptions(findOptionGroup(groups, key))
}

export function getOptionLabel(
  t: ComposerTranslation,
  code?: string,
  value?: number,
): string | number {
  if (!code) return value || ''
  const key = `options.${code}`
  const translated = t(key)
  return translated === key ? value || code : translated
}

export function getOptionValueLabel(
  groups: OptionGroup[] | undefined,
  key: string,
  value: number | string | undefined,
  t: ComposerTranslation,
): string | number {
  const option = findOptionGroup(groups, key).find((item) => String(item.value) === String(value))
  return getOptionLabel(t, option?.code, option?.value || 0)
}
