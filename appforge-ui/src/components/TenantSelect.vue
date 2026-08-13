<template>
  <el-select
    v-model="selectedValue"
    :disabled="effectiveDisabled"
    :placeholder="placeholder || t('common.pleaseSelect')"
    filterable
    remote
    clearable
    :remote-method="loadTenants"
    :loading="loading"
    style="width: 100%"
    @visible-change="handleVisibleChange"
  >
    <el-option v-if="includeSystem" :label="t('system.systemConfigScope')" :value="0" />
    <el-option
      v-for="item in tenants"
      :key="item.id"
      :label="formatTenantLabel(item)"
      :value="item.id"
    />
  </el-select>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { tenantsService, type SysTenantItem } from '@/services'
import { useAuthStore } from '@/stores/auth'

const props = withDefaults(
  defineProps<{
    modelValue: number | undefined
    disabled?: boolean
    includeSystem?: boolean
    placeholder?: string
  }>(),
  {
    disabled: false,
    includeSystem: false,
    placeholder: '',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: number | undefined]
  change: [value: number | undefined]
  selected: [value: SysTenantItem | null]
}>()

const { t } = useI18n()
const authStore = useAuthStore()
const loading = ref(false)
const tenants = ref<SysTenantItem[]>([])
const forcedTenantId = computed(() =>
  authStore.isTenantUser ? authStore.profileTenantId || undefined : undefined,
)
const effectiveDisabled = computed(() => props.disabled || Boolean(forcedTenantId.value))

const selectedValue = computed({
  get: () => forcedTenantId.value ?? props.modelValue,
  set: (value) => {
    const nextValue =
      forcedTenantId.value ?? (value === undefined || value === null ? undefined : Number(value))
    emit('update:modelValue', nextValue)
    emit('change', nextValue)
    emit(
      'selected',
      nextValue === undefined
        ? null
        : tenants.value.find((tenant) => tenant.id === nextValue) || null,
    )
  },
})

function formatTenantLabel(item: SysTenantItem) {
  const name = item.tenantName || item.tenantCode || String(item.id)
  return `${name} (${item.id})`
}

function mergeTenant(item?: SysTenantItem) {
  if (!item?.id || tenants.value.some((tenant) => tenant.id === item.id)) return
  tenants.value = [item, ...tenants.value]
}

async function loadTenants(keyword = '') {
  if (forcedTenantId.value) {
    await ensureCurrentTenant()
    return
  }
  loading.value = true
  try {
    const res = await tenantsService.getList({
      keyword: keyword || undefined,
      limit: 100,
    })
    tenants.value = res.data || []
    await ensureCurrentTenant()
  } finally {
    loading.value = false
  }
}

async function ensureCurrentTenant() {
  const tenantId = selectedValue.value
  if (!tenantId || tenantId === 0) return
  const existing = tenants.value.find((tenant) => tenant.id === tenantId)
  if (existing) {
    emit('selected', existing)
    return
  }

  const res = await tenantsService.detail({ tenantId })
  mergeTenant(res.data)
  emit('selected', res.data || null)
}

function enforceProfileTenant() {
  const tenantId = forcedTenantId.value
  if (!tenantId || props.modelValue === tenantId) return
  emit('update:modelValue', tenantId)
  emit('change', tenantId)
  ensureCurrentTenant()
}

function handleVisibleChange(visible: boolean) {
  if (visible && tenants.value.length === 0) {
    loadTenants()
  }
}

watch(
  () => props.modelValue,
  () => {
    enforceProfileTenant()
    ensureCurrentTenant()
  },
)

watch(
  forcedTenantId,
  () => {
    enforceProfileTenant()
  },
  { immediate: true, flush: 'sync' },
)

onMounted(() => {
  enforceProfileTenant()
  if (forcedTenantId.value) {
    ensureCurrentTenant()
  } else {
    loadTenants()
  }
})
</script>
