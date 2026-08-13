import { computed, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OptionGroup, OptionItem } from '@/services'
import {
  findFormOptionGroup,
  findOptionGroup,
  getOptionLabel,
  getOptionValueLabel,
} from '@/utils/options'

export type LabeledOptionItem = OptionItem & {
  label: string | number
}

export function useOptions(optionGroups: Ref<OptionGroup[]>) {
  const { t } = useI18n()

  const optionItems = (key: string) =>
    computed<LabeledOptionItem[]>(() =>
      findOptionGroup(optionGroups.value, key).map((item) => ({
        ...item,
        label: getOptionLabel(t, item.code, item.value),
      })),
    )

  const formOptionItems = (key: string) =>
    computed<LabeledOptionItem[]>(() =>
      findFormOptionGroup(optionGroups.value, key).map((item) => ({
        ...item,
        label: getOptionLabel(t, item.code, item.value),
      })),
    )

  const optionLabel = (key: string, value: number | string | undefined) =>
    getOptionValueLabel(optionGroups.value, key, value, t)

  return {
    optionItems,
    formOptionItems,
    optionLabel,
  }
}
