<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, type FormInstance, type UploadFile, type UploadFiles } from 'element-plus'
import CursorPagination from '@/components/common/CursorPagination.vue'
import { usePagination } from '@/composables/usePagination'
import {
  platformService,
  type PlatformCreateCall,
  type PlatformListCall,
  type PlatformListReq,
} from '@/services'
import { buildApiUrl } from '@/utils/file-url'

export type ResourceField = {
  prop: string
  label: string
  type?: 'text' | 'number' | 'textarea' | 'password' | 'file'
  required?: boolean
  accept?: string
  objectType?: 1 | 2
  maxBytes?: number
  downloadObject?: boolean
  publicDownload?: boolean
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
const uploadFiles = ref<Record<string, File | undefined>>({})
const uploadProgress = ref<Record<string, number>>({})
const { pagination, updateFromResponse, resetAndLoad, nextAndLoad, prevAndLoad } =
  usePagination<number>(20)

function defaultValue(field: ResourceField) {
  return field.type === 'number' ? 0 : ''
}

function selectFile(field: ResourceField, uploadFile: UploadFile, _uploadFiles: UploadFiles) {
  const raw = uploadFile.raw
  if (!raw) return
  if (field.maxBytes && raw.size > field.maxBytes) {
    ElMessage.error(`${field.label}不能超过 ${Math.ceil(field.maxBytes / 1024 / 1024)} MiB`)
    return
  }
  uploadFiles.value[field.prop] = raw
  form[field.prop] = raw.name
}

function fileChangeHandler(field: ResourceField) {
  return (file: UploadFile, files: UploadFiles) => selectFile(field, file, files)
}

function removeFile(field: ResourceField) {
  delete uploadFiles.value[field.prop]
  form[field.prop] = ''
}

async function downloadObject(objectId: unknown) {
	const id = Number(objectId || 0)
	if (!id) return
	try {
		const response = await platformService.getStorageDownload(id)
		if (response.code !== 200 || !response.data?.downloadUrl) {
			throw new Error(response.msg || '生成下载地址失败')
		}
		window.location.assign(response.data.downloadUrl)
	} catch (error) {
		ElMessage.error(error instanceof Error ? error.message : '下载失败')
	}
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
  uploadFiles.value = {}
  uploadProgress.value = {}
  dialogVisible.value = true
}

async function submit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    const payload = { ...form }
    for (const field of props.fields.filter((item) => item.type === 'file')) {
      const file = uploadFiles.value[field.prop]
      if (!file || !field.objectType) throw new Error(`请选择${field.label}`)
      const upload = await platformService.uploadObject(
        file,
        field.objectType,
        Number(payload.appId || 0),
        (percent) => (uploadProgress.value[field.prop] = percent),
      )
      if (upload.code !== 200 || !upload.data) throw new Error(upload.msg || `${field.label}上传失败`)
      payload[field.prop] = upload.data.objectId
    }
    const response = await props.create(payload)
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

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="rows" stripe>
        <el-table-column prop="id" label="ID" width="90" />
        <el-table-column
          v-for="column in columns"
          :key="column.prop"
          :prop="column.prop"
          :label="column.label"
          min-width="130"
          show-overflow-tooltip
        >
          <template #default="scope">
            <el-button
              v-if="column.downloadObject"
              link
              type="primary"
              :disabled="!Number(scope.row[column.prop] || 0)"
              @click="downloadObject(scope.row[column.prop])"
            >
              下载
            </el-button>
            <a
              v-else-if="column.publicDownload && scope.row[column.prop]"
              :href="buildApiUrl(`/d/${encodeURIComponent(scope.row[column.prop])}`)"
              target="_blank"
              rel="noopener noreferrer"
            >
              推广下载
            </a>
            <span v-else>{{ scope.row[column.prop] ?? '' }}</span>
          </template>
        </el-table-column>
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
          <el-upload
            v-else-if="field.type === 'file'"
            :auto-upload="false"
            :limit="1"
            :accept="field.accept"
            :on-change="fileChangeHandler(field)"
            :on-remove="() => removeFile(field)"
          >
            <el-button>选择文件</el-button>
            <template #tip>
              <div class="el-upload__tip">选择后将在创建时上传并校验</div>
              <el-progress
                v-if="uploadProgress[field.prop] > 0"
                :percentage="uploadProgress[field.prop]"
                :status="uploadProgress[field.prop] === 100 ? 'success' : undefined"
              />
            </template>
          </el-upload>
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
