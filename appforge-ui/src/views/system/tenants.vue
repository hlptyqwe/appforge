<template>
  <div class="sys-tenants module-page">
    <CrudQueryCard :model="queryForm" @search="loadList" @reset="resetQuery">
      <el-form-item :label="t('system.tenantCode')">
        <el-input
          v-model="queryForm.tenantCode"
          :placeholder="t('system.tenantCodePlaceholder')"
          clearable
          @keyup.enter="loadList"
        />
      </el-form-item>
      <el-form-item :label="t('system.tenantName')">
        <el-input
          v-model="queryForm.tenantName"
          :placeholder="t('system.tenantNamePlaceholder')"
          clearable
          @keyup.enter="loadList"
        />
      </el-form-item>
      <el-form-item :label="t('system.contactName')">
        <el-input
          v-model="queryForm.contactName"
          :placeholder="t('system.contactNamePlaceholder')"
          clearable
          @keyup.enter="loadList"
        />
      </el-form-item>
      <el-form-item :label="t('system.enabled')">
        <el-select
          v-model="queryForm.enabled"
          :placeholder="t('system.pleaseSelectStatus')"
          clearable
          @change="loadList"
        >
          <el-option
            v-for="item in enabledSelectOptions"
            :key="item.value"
            :label="enabledOptionLabel(item.value, item.code)"
            :value="item.value"
          />
        </el-select>
      </el-form-item>
      <template #actions>
        <el-button v-perm="'sys:tenant:add'" type="primary" @click="handleCreate">
          {{ t('common.add') }}
        </el-button>
      </template>
    </CrudQueryCard>

    <el-card class="table-card" shadow="never">
      <el-table
        v-loading="loading"
        :data="list"
        :empty-text="t('common.noData')"
        stripe
      >
        <el-table-column
          prop="id"
          :label="t('common.id')"
          width="80"
          align="center"
        />
        <el-table-column prop="tenantCode" :label="t('system.tenantCode')" min-width="150" />
        <el-table-column prop="tenantName" :label="t('system.tenantName')" min-width="150" />
        <el-table-column prop="contactName" :label="t('system.contactName')" min-width="120" />
        <el-table-column prop="contactPhone" :label="t('system.contactPhone')" min-width="130" />
        <el-table-column :label="t('system.enabled')" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.enabled === 1 ? 'success' : 'info'">
              {{ enabledLabel(row.enabled) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          prop="expireTime"
          :label="t('system.expireTime')"
          min-width="160"
          align="center"
        >
          <template #default="{ row }">
            {{ formatDate(row.expireTime) }}
          </template>
        </el-table-column>
        <el-table-column prop="remark" :label="t('common.remark')" min-width="150" />
        <el-table-column
          prop="createTimes"
          :label="t('common.createTimes')"
          width="160"
          align="center"
        >
          <template #default="{ row }">
            {{ formatDate(row.createTimes) }}
          </template>
        </el-table-column>
        <el-table-column
          :label="t('common.actions')"
          width="140"
          align="center"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              v-perm="'sys:tenant:update'"
              type="primary"
              size="small"
              @click="handleEdit(row)"
            >
              {{ t('common.edit') }}
            </el-button>
            <el-button
              v-perm="'sys:tenant:delete'"
              type="danger"
              size="small"
              @click="handleDelete(row)"
            >
              {{ t('common.delete') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <CursorPagination
        v-model:limit="pagination.limit"
        :total="pagination.total"
        :has-prev="pagination.hasPrev"
        :has-next="pagination.hasNext"
        @prev="prevPage"
        @next="nextPage"
        @limit-change="handleSizeChange"
      />
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? t('system.editTenant') : t('system.addTenant')"
      width="700px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="formRef"
        :model="formData"
        :rules="formRules"
        label-width="120px"
      >
        <el-row :gutter="20">
          <el-col v-if="isEdit" :span="12">
            <el-form-item :label="t('system.tenantCode')" prop="tenantCode">
              <el-input
                v-model="formData.tenantCode"
                :placeholder="t('system.pleaseInputTenantCode')"
                maxlength="50"
                show-word-limit
                disabled
              />
            </el-form-item>
          </el-col>
          <el-col v-else :span="12">
            <el-form-item :label="t('common.username')" prop="username">
              <el-input
                v-model="formData.username"
                :placeholder="t('common.pleaseInputUsername')"
                maxlength="50"
                show-word-limit
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('system.tenantName')" prop="tenantName">
              <el-input
                v-model="formData.tenantName"
                :placeholder="t('system.pleaseInputTenantName')"
                maxlength="100"
                show-word-limit
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row v-if="!isEdit" :gutter="20">
          <el-col :span="12">
            <el-form-item :label="t('common.password')" prop="tenantPassword">
              <el-input
                v-model="formData.tenantPassword"
                type="password"
                :placeholder="t('common.pleaseInputNewPassword')"
                maxlength="100"
                show-password
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item :label="t('system.contactName')" prop="contactName">
              <el-input
                v-model="formData.contactName"
                :placeholder="t('system.pleaseInputContactName')"
                maxlength="50"
                show-word-limit
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('system.contactPhone')" prop="contactPhone">
              <el-input
                v-model="formData.contactPhone"
                :placeholder="t('system.pleaseInputContactPhone')"
                maxlength="20"
                show-word-limit
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item :label="t('system.enabled')" prop="enabled">
              <el-select v-model="formData.enabled" style="width: 100%">
                <el-option
                  v-for="item in enabledFormOptions"
                  :key="item.value"
                  :label="enabledOptionLabel(item.value, item.code)"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('system.expireTime')" prop="expireTime">
              <el-date-picker
                v-model="formData.expireTime"
                type="datetime"
                :placeholder="t('common.pleaseSelect')"
                format="YYYY-MM-DD HH:mm:ss"
                value-format="x"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item :label="t('common.remark')">
          <el-input
            v-model="formData.remark"
            type="textarea"
            :rows="4"
            :placeholder="t('common.remark')"
            maxlength="200"
            show-word-limit
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">
          {{ t('common.cancel') }}
        </el-button>
        <el-button
          v-perm="isEdit ? 'sys:tenant:update' : 'sys:tenant:add'"
          type="primary"
          :loading="submitLoading"
          @click="handleSubmit"
        >
          {{ t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { tenantsService } from '@/services/system/TenantsService'
import type { OptionGroup } from '@/services'
import type { SysTenantItem, SysTenantCreateReq } from '@/services/system/TenantsService'
import { usePagination } from '@/composables/usePagination'
import { useLoading } from '@/composables/useLoading'
import { useForm } from '@/composables/useForm'
import { formatDate } from '@/utils'
import {
  findFormOptionGroup,
  findOptionGroup,
  getOptionLabel,
  getOptionValueLabel,
} from '@/utils/options'
import CrudQueryCard from '@/components/common/CrudQueryCard.vue'

const { t } = useI18n()
const optionGroups = ref<OptionGroup[]>([])
const enabledOptions = computed(() => findOptionGroup(optionGroups.value, 'enabled'))
const enabledSelectOptions = computed(() => {
  const options = enabledOptions.value
  return options.length
    ? options
    : [
        { value: 0, code: 'COMMON_STATUS_UNKNOWN' },
        { value: 1, code: 'COMMON_STATUS_ENABLED' },
        { value: 2, code: 'COMMON_STATUS_DISABLED' },
      ]
})
const enabledFormOptions = computed(() => {
  const options = findFormOptionGroup(optionGroups.value, 'enabled')
  return options.length
    ? options
    : [
        { value: 1, code: 'COMMON_STATUS_ENABLED' },
        { value: 2, code: 'COMMON_STATUS_DISABLED' },
      ]
})

// Pagination and main list
const { pagination, updateFromResponse, resetAndLoad, prevAndLoad, nextAndLoad } =
  usePagination<number>(20)
const list = ref<SysTenantItem[]>([])
const { loading, withLoading } = useLoading()

// Query form
const { form: queryForm } = useForm({
  initialData: {
    tenantCode: '',
    tenantName: '',
    contactName: '',
    enabled: 0,
  },
})

// Dialog and form
const dialogVisible = ref(false)
const isEdit = ref(false)
const submitLoading = ref(false)
const formRef = ref()

const { form: formData, reset: resetForm } = useForm({
  initialData: {
    id: 0,
    tenantCode: '',
    username: '',
    tenantName: '',
    tenantPassword: '',
    enabled: 1,
    expireTime: Date.now() + 365 * 24 * 60 * 60 * 1000,
    contactName: '',
    contactPhone: '',
    remark: '',
  },
})

// Form validation rules
const formRules = {
  tenantCode: [{ required: true, message: t('system.pleaseInputTenantCode'), trigger: 'blur' }],
  username: [{ required: true, message: t('common.pleaseInputUsername'), trigger: 'blur' }],
  tenantName: [{ required: true, message: t('system.pleaseInputTenantName'), trigger: 'blur' }],
  tenantPassword: [
    { required: true, message: t('common.pleaseInputNewPassword'), trigger: 'blur' },
  ],
  enabled: [{ required: true, message: t('system.pleaseSelectStatus'), trigger: 'change' }],
  expireTime: [{ required: true, message: t('validation.required'), trigger: 'change' }],
  contactName: [{ required: true, message: t('system.pleaseInputContactName'), trigger: 'blur' }],
  contactPhone: [{ required: true, message: t('system.pleaseInputContactPhone'), trigger: 'blur' }],
}

function enabledOptionLabel(value: number | string | undefined, code?: string) {
  if (Number(value) === 1) return t('common.enabled')
  if (Number(value) === 2) return t('common.disabled')
  if (Number(value) === 0) return t('common.all')
  return getOptionLabel(t, code, Number(value))
}

function enabledLabel(value: number | undefined) {
  const label = getOptionValueLabel(optionGroups.value, 'enabled', value, t)
  if (label && label !== value) return label
  return enabledOptionLabel(value)
}

// Fetch list
async function loadList() {
  await withLoading(async () => {
    try {
      const params = {
        tenantCode: queryForm.tenantCode || undefined,
        tenantName: queryForm.tenantName || undefined,
        contactName: queryForm.contactName || undefined,
        enabled: queryForm.enabled === 0 ? undefined : queryForm.enabled,
        cursor: pagination.cursor,
        limit: pagination.limit,
      }
      const res = await tenantsService.getList(params)
      if (res.code !== 200) throw new Error(res.msg || 'list failed')
      list.value = res.data || []
      updateFromResponse(res)
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : t('common.loadFailed'))
    }
  })
}

async function fetchOptions() {
  try {
    const res = await tenantsService.getOptions()
    if (res.code !== 200) throw new Error(res.msg || 'options failed')
    optionGroups.value = res.data || []
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : t('common.loadFailed'))
  }
}

// Handle pagination
function handleSizeChange(size: number) {
  pagination.limit = size
  resetAndLoad(loadList)
}

// Handle reset
function resetQuery() {
  queryForm.tenantCode = ''
  queryForm.tenantName = ''
  queryForm.contactName = ''
  queryForm.enabled = 0
  resetAndLoad(loadList)
}

function nextPage() {
  nextAndLoad(loadList)
}

function prevPage() {
  prevAndLoad(loadList)
}

// Handle create
function handleCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

// Handle edit
function handleEdit(row: SysTenantItem) {
  isEdit.value = true
  resetForm()
  Object.assign(formData, {
    id: row.id,
    tenantCode: row.tenantCode,
    tenantName: row.tenantName,
    enabled: row.enabled,
    expireTime: row.expireTime,
    contactName: row.contactName,
    contactPhone: row.contactPhone,
    remark: row.remark || '',
  })
  dialogVisible.value = true
}

// Handle delete
async function handleDelete(row: SysTenantItem) {
  try {
    await ElMessageBox.confirm(t('common.confirmDelete'), t('common.warning'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })

    const res = await tenantsService.delete(row.id)
    if (res.code !== 200) throw new Error(res.msg || 'delete failed')

    ElMessage.success(t('common.deleteSuccess'))
    loadList()
  } catch (error: unknown) {
    if ((error instanceof Error ? error.message : '') !== 'cancel') {
      ElMessage.error(error instanceof Error ? error.message : t('common.deleteFailed'))
    }
  }
}

// Handle submit
async function handleSubmit() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()

    submitLoading.value = true

    if (isEdit.value) {
      const res = await tenantsService.update(formData.id, {
        tenantName: formData.tenantName,
        enabled: formData.enabled,
        expireTime: formData.expireTime,
        contactName: formData.contactName,
        contactPhone: formData.contactPhone,
        remark: formData.remark || '',
      })
      if (res.code !== 200) throw new Error(res.msg || t('common.updateFailed'))
      ElMessage.success(t('common.updateSuccess'))
    } else {
      const data: SysTenantCreateReq = {
        username: formData.username,
        tenantName: formData.tenantName,
        tenantPassword: formData.tenantPassword,
        enabled: formData.enabled,
        expireTime: formData.expireTime,
        contactName: formData.contactName,
        contactPhone: formData.contactPhone,
        remark: formData.remark || '',
      }
      const res = await tenantsService.create(data)
      if (res.code !== 200) throw new Error(res.msg || t('common.createFailed'))
      ElMessage.success(t('common.createSuccess'))
    }

    dialogVisible.value = false
    loadList()
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : t('common.operationFailed'))
  } finally {
    submitLoading.value = false
  }
}

// Initialize
onMounted(() => {
  fetchOptions()
  loadList()
})
</script>
