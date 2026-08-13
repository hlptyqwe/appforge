<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, type FormInstance } from 'element-plus'
import CursorPagination from '@/components/common/CursorPagination.vue'
import { usePagination } from '@/composables/usePagination'
import type { PlatformCreateCall, PlatformListCall, PlatformListReq } from '@/services'

export type ResourceField = {
  prop: string
  label: string
  type?: 'text' | 'number' | 'textarea' | 'password'
  required?: boolean
}

const props = defineProps<{
  title: string
  permissionPrefix: string
  columns: ResourceField[]
  fields: ResourceField[]
  queryFields?: ResourceField[]
  list: PlatformListCall
  create: PlatformCreateCall
}>()

const rows = ref<Record<string, any>[]>([])
const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const formRef = ref<FormInstance>()
const query = reactive<Record<string, any>>({})
const form = reactive<Record<string, any>>({})
const { pagination, updateFromResponse, resetAndLoad, nextAndLoad, prevAndLoad } =
  usePagination<number>(20)

function defaultValue(field: ResourceField) {
  return field.type === 'number' ? 0 : ''
}

function resetValues(target: Record<string, any>, fields: ResourceField[]) {
  Object.keys(target).forEach((key) => delete target[key])
  fields.forEach((field) => (target[field.prop] = defaultValue(field)))
}

async function loadData() {
  loading.value = true
  try {
    const response = await props.list({
      ...(query as PlatformListReq),
      cursor: pagination.cursor,
      limit: pagination.limit,
    })
    if (response.code !== 200) throw new Error(response.msg)
    rows.value = response.data || []
    updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : `加载${props.title}失败`)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  resetValues(form, props.fields)
  dialogVisible.value = true
}

async function submit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    const response = await props.create({ ...form })
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('创建成功')
    dialogVisible.value = false
    await resetAndLoad(loadData)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '创建失败')
  } finally {
    submitting.value = false
  }
}

resetValues(query, props.queryFields || [])
onMounted(loadData)
</script>

<template>
  <div class="module-page">
    <el-card shadow="never" class="query-card">
      <el-form inline>
        <el-form-item v-for="field in queryFields || []" :key="field.prop" :label="field.label">
          <el-input-number
            v-if="field.type === 'number'"
            v-model="query[field.prop]"
            :min="0"
            controls-position="right"
          />
          <el-input v-else v-model="query[field.prop]" clearable />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="resetAndLoad(loadData)">查询</el-button>
          <el-button v-perm="`${permissionPrefix}:add`" @click="openCreate">新增</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never">
      <template #header>{{ title }}</template>
      <el-table v-loading="loading" :data="rows" stripe>
        <el-table-column prop="id" label="ID" width="90" />
        <el-table-column
          v-for="column in columns"
          :key="column.prop"
          :prop="column.prop"
          :label="column.label"
          min-width="130"
          show-overflow-tooltip
        />
      </el-table>
      <CursorPagination
        v-model:limit="pagination.limit"
        :total="pagination.total"
        :has-prev="pagination.hasPrev"
        :has-next="pagination.hasNext"
        @prev="prevAndLoad(loadData)"
        @next="nextAndLoad(loadData)"
        @limit-change="resetAndLoad(loadData)"
      />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="`新增${title}`" width="620px">
      <el-form ref="formRef" :model="form" label-width="140px">
        <el-form-item
          v-for="field in fields"
          :key="field.prop"
          :label="field.label"
          :prop="field.prop"
          :rules="field.required ? [{ required: true, message: `请输入${field.label}` }] : []"
        >
          <el-input-number
            v-if="field.type === 'number'"
            v-model="form[field.prop]"
            :min="0"
            controls-position="right"
            style="width: 100%"
          />
          <el-input
            v-else
            v-model="form[field.prop]"
            :type="field.type === 'textarea' ? 'textarea' : field.type === 'password' ? 'password' : 'text'"
            :rows="field.type === 'textarea' ? 5 : undefined"
            show-password
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>
