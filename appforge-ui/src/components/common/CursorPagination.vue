<template>
  <div class="pagination-bar">
    <span v-if="showTotal">{{ t('common.totalItems', { count: total }) }}</span>
    <el-button :disabled="disabled || !hasPrev" @click="emit('prev')">
      {{ t('common.prevPage') }}
    </el-button>
    <el-button :disabled="disabled || !hasNext" type="primary" @click="emit('next')">
      {{ t('common.nextPage') }}
    </el-button>
    <el-select
      v-if="showLimit"
      :model-value="limit"
      :teleported="selectTeleported"
      style="width: 100px"
      @change="handleLimitChange"
    >
      <el-option :value="10" :label="t('common.perPage10')" />
      <el-option :value="20" :label="t('common.perPage20')" />
      <el-option :value="50" :label="t('common.perPage50')" />
      <el-option :value="100" :label="t('common.perPage100')" />
    </el-select>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

withDefaults(
  defineProps<{
    total: number
    hasPrev: boolean
    hasNext: boolean
    limit: number
    showLimit?: boolean
    showTotal?: boolean
    disabled?: boolean
    selectTeleported?: boolean
  }>(),
  {
    showLimit: true,
    showTotal: true,
    disabled: false,
    selectTeleported: true,
  },
)

const emit = defineEmits<{
  prev: []
  next: []
  'update:limit': [limit: number]
  limitChange: [limit: number]
}>()

const { t } = useI18n()

function handleLimitChange(value: number) {
  const limit = Number(value)
  emit('update:limit', limit)
  emit('limitChange', limit)
}
</script>
