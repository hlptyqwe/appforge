<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type UploadFile } from 'element-plus'
import CursorPagination from '@/components/common/CursorPagination.vue'
import { usePagination } from '@/composables/usePagination'
import { platformService, type PlatformBrandingPreflight, type PlatformBrandingProfile } from '@/services'

const rows = ref<PlatformBrandingProfile[]>([])
const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const editingId = ref(0)
const formRef = ref<FormInstance>()
const logoFile = ref<File>()
const splashFile = ref<File>()
const logoPreview = ref('')
const splashPreview = ref('')
const preflightVisible = ref(false)
const preflightLoading = ref(false)
const preflights = ref<PlatformBrandingPreflight[]>([])
const preflightProfileId = ref(0)
const query = reactive({ appId: 0, keyword: '', status: 0 })
const form = reactive({
  appId: 0,
  profileName: '',
  appName: '',
  apiHost: '',
  rewriteMode: 1,
  launcherIconTarget: '',
  splashResourceTarget: '',
  runtimeConfigJson: '',
  logoObjectId: 0,
  splashObjectId: 0,
})
const { pagination, updateFromResponse, resetAndLoad, nextAndLoad, prevAndLoad } = usePagination<number>(20)

const dialogTitle = computed(() => (editingId.value ? '编辑品牌配置' : '新增品牌配置'))
const statusText = (status: number) => ({ 1: '草稿', 2: '启用', 3: '停用' })[status] || '未知'
const preflightStatusText = (status: number) => ({ 1: '处理中', 2: '兼容', 3: '不兼容', 4: '执行失败' })[status] || '未知'

async function loadData() {
  loading.value = true
  try {
    const response = await platformService.listBrandingProfiles({
      ...query,
      cursor: pagination.cursor,
      limit: pagination.limit,
    })
    if (response.code !== 200) throw new Error(response.msg)
    rows.value = response.data || []
    updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载品牌配置失败')
  } finally {
    loading.value = false
  }
}

function clearPreviews() {
  if (logoPreview.value) URL.revokeObjectURL(logoPreview.value)
  if (splashPreview.value) URL.revokeObjectURL(splashPreview.value)
  logoPreview.value = ''
  splashPreview.value = ''
}

function openForm(row?: PlatformBrandingProfile) {
  clearPreviews()
  editingId.value = row?.id || 0
  Object.assign(form, {
    appId: row?.appId || 0,
    profileName: row?.profileName || '',
    appName: row?.appName || '',
    apiHost: row?.apiHost || '',
    rewriteMode: row?.rewriteMode || 1,
    launcherIconTarget: row?.launcherIconTarget || '',
    splashResourceTarget: row?.splashResourceTarget || '',
    runtimeConfigJson: row?.runtimeConfigJson || '',
    logoObjectId: row?.logoObjectId || 0,
    splashObjectId: row?.splashObjectId || 0,
  })
  logoFile.value = undefined
  splashFile.value = undefined
  dialogVisible.value = true
}

function selectImage(kind: 'logo' | 'splash', upload: UploadFile) {
  const file = upload.raw
  if (!file) return
  const max = kind === 'logo' ? 5 * 1024 * 1024 : 10 * 1024 * 1024
  if (file.size > max) {
    ElMessage.error(`${kind === 'logo' ? 'Logo' : '启动图'}文件过大`)
    return
  }
  if (kind === 'logo') {
    if (logoPreview.value) URL.revokeObjectURL(logoPreview.value)
    logoFile.value = file
    logoPreview.value = URL.createObjectURL(file)
  } else {
    if (splashPreview.value) URL.revokeObjectURL(splashPreview.value)
    splashFile.value = file
    splashPreview.value = URL.createObjectURL(file)
  }
}

async function submit() {
  await formRef.value?.validate()
  if (!editingId.value && (!logoFile.value || !splashFile.value)) {
    ElMessage.error('请选择 Logo 和启动图')
    return
  }
  submitting.value = true
  try {
    const payload: Record<string, unknown> = { ...form }
    if (logoFile.value) {
      const upload = await platformService.uploadObject(logoFile.value, 5, form.appId)
      if (upload.code !== 200 || !upload.data) throw new Error(upload.msg || 'Logo 上传失败')
      payload.logoObjectId = upload.data.objectId
    }
    if (splashFile.value) {
      const upload = await platformService.uploadObject(splashFile.value, 6, form.appId)
      if (upload.code !== 200 || !upload.data) throw new Error(upload.msg || '启动图上传失败')
      payload.splashObjectId = upload.data.objectId
    }
    const response = editingId.value
      ? await platformService.updateBrandingProfile(editingId.value, payload)
      : await platformService.createBrandingProfile(payload)
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success(editingId.value ? '更新成功，品牌修订号已递增' : '创建成功')
    dialogVisible.value = false
    await resetAndLoad(loadData)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存品牌配置失败')
  } finally {
    submitting.value = false
  }
}

async function changeStatus(row: PlatformBrandingProfile) {
  const status = row.status === 2 ? 3 : 2
  try {
    await ElMessageBox.confirm(`确定${status === 2 ? '启用' : '停用'}“${row.profileName}”吗？`, '状态确认')
    const response = await platformService.changeBrandingProfileStatus(row.id, status)
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('状态已更新')
    await loadData()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error instanceof Error ? error.message : '状态更新失败')
  }
}

async function startPreflight(row: PlatformBrandingProfile) {
  try {
    const result = await ElMessageBox.prompt('请输入需要预检的应用版本 ID', '创建兼容性预检', {
      inputPattern: /^[1-9]\d*$/,
      inputErrorMessage: '请输入有效的版本 ID',
    })
    const response = await platformService.createBrandingPreflight(row.id, Number(result.value))
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('预检已提交，Builder 将异步处理')
    await showPreflights(row)
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error instanceof Error ? error.message : '创建预检失败')
  }
}

async function loadPreflights() {
  preflightLoading.value = true
  try {
    const response = await platformService.listBrandingPreflights({ brandingProfileId: preflightProfileId.value, limit: 100 })
    if (response.code !== 200) throw new Error(response.msg)
    preflights.value = response.data || []
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载预检记录失败')
  } finally {
    preflightLoading.value = false
  }
}

async function showPreflights(row: PlatformBrandingProfile) {
  preflightProfileId.value = row.id
  preflightVisible.value = true
  await loadPreflights()
}

function formattedReport(value?: string) {
  if (!value) return ''
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

onMounted(loadData)
</script>

<template>
  <div class="module-page">
    <el-card shadow="never" class="query-card">
      <el-form inline>
        <el-form-item label="应用 ID"><el-input-number v-model="query.appId" :min="0" /></el-form-item>
        <el-form-item label="关键词"><el-input v-model="query.keyword" clearable /></el-form-item>
        <el-form-item label="状态"><el-select v-model="query.status" style="width: 120px"><el-option label="全部" :value="0" /><el-option label="草稿" :value="1" /><el-option label="启用" :value="2" /><el-option label="停用" :value="3" /></el-select></el-form-item>
        <el-form-item><el-button type="primary" @click="resetAndLoad(loadData)">查询</el-button><el-button v-perm="'core:branding:add'" @click="openForm()">新增</el-button></el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="rows" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="appId" label="应用 ID" width="100" />
        <el-table-column prop="profileName" label="配置名称" min-width="140" />
        <el-table-column prop="appName" label="AppName" min-width="140" />
        <el-table-column prop="apiHost" label="API Host" min-width="220" show-overflow-tooltip />
        <el-table-column prop="revision" label="修订" width="80" />
        <el-table-column label="模式" width="110"><template #default="scope">{{ scope.row.rewriteMode === 2 ? '运行时契约' : '资源重建' }}</template></el-table-column>
        <el-table-column label="状态" width="90"><template #default="scope">{{ statusText(scope.row.status) }}</template></el-table-column>
        <el-table-column label="操作" width="300" fixed="right">
          <template #default="scope">
            <el-button v-perm="'core:branding:update'" link type="primary" @click="openForm(scope.row)">编辑</el-button>
            <el-button v-perm="'core:branding:status'" link @click="changeStatus(scope.row)">{{ scope.row.status === 2 ? '停用' : '启用' }}</el-button>
            <el-button v-perm="'core:branding:preflight'" link type="success" @click="startPreflight(scope.row)">预检</el-button>
            <el-button v-perm="'core:branding:view'" link @click="showPreflights(scope.row)">记录</el-button>
          </template>
        </el-table-column>
      </el-table>
      <CursorPagination v-model:limit="pagination.limit" :total="pagination.total" :has-prev="pagination.hasPrev" :has-next="pagination.hasNext" @prev="prevAndLoad(loadData)" @next="nextAndLoad(loadData)" @limit-change="resetAndLoad(loadData)" />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="700px" @closed="clearPreviews">
      <el-form ref="formRef" :model="form" label-width="150px">
        <el-form-item label="应用 ID" prop="appId" :rules="[{ required: true, message: '请输入应用 ID' }]"><el-input-number v-model="form.appId" :min="1" :disabled="Boolean(editingId)" style="width: 100%" /></el-form-item>
        <el-form-item label="配置名称" prop="profileName" :rules="[{ required: true, message: '请输入配置名称' }]"><el-input v-model="form.profileName" /></el-form-item>
        <el-form-item label="AppName" prop="appName" :rules="[{ required: true, message: '请输入 AppName' }]"><el-input v-model="form.appName" /></el-form-item>
        <el-form-item label="API Host" prop="apiHost" :rules="[{ required: true, message: '请输入 HTTPS API Host' }]"><el-input v-model="form.apiHost" placeholder="https://api.example.com" /></el-form-item>
        <el-form-item label="改写模式"><el-radio-group v-model="form.rewriteMode"><el-radio :value="1">资源重建</el-radio><el-radio :value="2">运行时契约</el-radio></el-radio-group></el-form-item>
        <el-form-item label="Launcher 资源"><el-input v-model="form.launcherIconTarget" placeholder="mipmap/ic_launcher" /></el-form-item>
        <el-form-item v-if="form.rewriteMode === 1" label="启动图资源"><el-input v-model="form.splashResourceTarget" placeholder="drawable/splash_logo" /></el-form-item>
        <el-form-item label="运行时扩展 JSON"><el-input v-model="form.runtimeConfigJson" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="Logo"><el-upload :auto-upload="false" :limit="1" accept=".png,.webp" :on-change="(file: UploadFile) => selectImage('logo', file)"><el-button>选择 Logo</el-button><template #tip><div class="el-upload__tip">正方形 512–2048 px，最大 5 MiB；编辑时不选则保留原对象 {{ form.logoObjectId || '' }}</div></template></el-upload><img v-if="logoPreview" :src="logoPreview" class="image-preview" alt="Logo 预览" /></el-form-item>
        <el-form-item label="启动图"><el-upload :auto-upload="false" :limit="1" accept=".png,.webp" :on-change="(file: UploadFile) => selectImage('splash', file)"><el-button>选择启动图</el-button><template #tip><div class="el-upload__tip">最小短边 720 px，最大 10 MiB；编辑时不选则保留原对象 {{ form.splashObjectId || '' }}</div></template></el-upload><img v-if="splashPreview" :src="splashPreview" class="image-preview splash" alt="启动图预览" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="submitting" @click="submit">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="preflightVisible" title="兼容性预检记录" width="820px">
      <div class="preflight-toolbar"><el-button :loading="preflightLoading" @click="loadPreflights">刷新</el-button></div>
      <el-table v-loading="preflightLoading" :data="preflights" max-height="520">
        <el-table-column prop="versionId" label="版本 ID" width="100" />
        <el-table-column prop="brandingRevision" label="品牌修订" width="100" />
        <el-table-column label="状态" width="100"><template #default="scope">{{ preflightStatusText(scope.row.status) }}</template></el-table-column>
        <el-table-column prop="toolchainVersion" label="工具链" width="150" />
        <el-table-column label="报告" min-width="340"><template #default="scope"><pre class="report-json">{{ formattedReport(scope.row.reportJson) }}</pre></template></el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<style scoped>
.image-preview { width: 80px; height: 80px; margin-left: 16px; object-fit: contain; border: 1px solid var(--el-border-color); border-radius: 8px; }
.image-preview.splash { width: 120px; }
.preflight-toolbar { display: flex; justify-content: flex-end; margin-bottom: 12px; }
.report-json { margin: 0; max-height: 180px; overflow: auto; white-space: pre-wrap; font-size: 12px; }
</style>
