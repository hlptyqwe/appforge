<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import CursorPagination from '@/components/common/CursorPagination.vue'
import { usePagination } from '@/composables/usePagination'
import { platformService, type PlatformWhiteLabelProduct } from '@/services'
import type {
  PlatformBrandingProfile,
  PlatformSigningConfig,
  PlatformWhiteLabelTemplate,
} from '@/services'

const rows = ref<PlatformWhiteLabelProduct[]>([])
const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const reportVisible = ref(false)
const reportJson = ref('')
const reportCompatible = ref(false)
const editingId = ref(0)
const formStep = ref(0)
const templateOptions = ref<PlatformWhiteLabelTemplate[]>([])
const brandingOptions = ref<PlatformBrandingProfile[]>([])
const signingOptions = ref<PlatformSigningConfig[]>([])
const formRef = ref<FormInstance>()
const query = reactive({ appId: 0, keyword: '', status: 0 })
const form = reactive({
  appId: 0,
  productCode: '',
  productName: '',
  templateId: 0,
  templateRevision: 0,
  brandingProfileId: 0,
  packageName: '',
  signingConfigId: 0,
  parameterValuesJson: '{}',
})
const { pagination, updateFromResponse, resetAndLoad, nextAndLoad, prevAndLoad } =
  usePagination<number>(20)
const dialogTitle = computed(() => (editingId.value ? '编辑白标产品' : '新增白标产品'))
const selectedSigning = computed(() =>
  signingOptions.value.find((item) => item.id === form.signingConfigId),
)
const statusText = (value: number) => ({ 1: '草稿', 2: '启用', 3: '停用' })[value] || '未知'

async function loadData() {
  loading.value = true
  try {
    const response = await platformService.listWhiteLabelProducts({
      ...query,
      cursor: pagination.cursor,
      limit: pagination.limit,
    })
    if (response.code !== 200) throw new Error(response.msg)
    rows.value = response.data || []
    updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载白标产品失败')
  } finally {
    loading.value = false
  }
}

async function loadDependencyOptions() {
  if (form.appId <= 0) return
  try {
    const [templates, branding, signing] = await Promise.all([
      platformService.listWhiteLabelTemplates({ appId: form.appId, limit: 200 }),
      platformService.listBrandingProfiles({ appId: form.appId, limit: 200 }),
      platformService.listSigningConfigs({ appId: form.appId, limit: 200 }),
    ])
    templateOptions.value = (templates.data || []).filter((item) => item.publishedRevision > 0)
    brandingOptions.value = branding.data || []
    signingOptions.value = signing.data || []
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载产品依赖项失败')
  }
}

function selectTemplate(templateId: number) {
  const template = templateOptions.value.find((item) => item.id === templateId)
  if (template) form.templateRevision = template.publishedRevision
}

async function openForm(row?: PlatformWhiteLabelProduct) {
  editingId.value = row?.id || 0
  formStep.value = 0
  Object.assign(form, {
    appId: row?.appId || 0,
    productCode: row?.productCode || '',
    productName: row?.productName || '',
    templateId: row?.templateId || 0,
    templateRevision: row?.templateRevision || 0,
    brandingProfileId: row?.brandingProfileId || 0,
    packageName: row?.packageName || '',
    signingConfigId: row?.signingConfigId || 0,
    parameterValuesJson: row?.parameterValuesJson || '{}',
  })
  dialogVisible.value = true
  if (form.appId > 0) await loadDependencyOptions()
}

async function submit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    JSON.parse(form.parameterValuesJson)
    const response = editingId.value
      ? await platformService.updateWhiteLabelProduct(editingId.value, { ...form })
      : await platformService.createWhiteLabelProduct({ ...form })
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success(editingId.value ? '白标产品已更新' : '白标产品已创建')
    dialogVisible.value = false
    await resetAndLoad(loadData)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存白标产品失败')
  } finally {
    submitting.value = false
  }
}

async function deleteProduct(row: PlatformWhiteLabelProduct) {
  try {
    await ElMessageBox.confirm(
      '仅未启用且没有历史构建的产品可以删除；包名证书历史绑定会保留。',
      '删除确认',
    )
    const response = await platformService.deleteWhiteLabelProduct(row.id)
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('白标产品已删除')
    await resetAndLoad(loadData)
  } catch (error) {
    if (error !== 'cancel' && error !== 'close')
      ElMessage.error(error instanceof Error ? error.message : '删除白标产品失败')
  }
}

async function preflight(row: PlatformWhiteLabelProduct) {
  try {
    const response = await platformService.preflightWhiteLabelProduct(row.id)
    if (response.code !== 200) throw new Error(response.msg)
    reportCompatible.value = Boolean(response.compatible)
    try {
      reportJson.value = JSON.stringify(JSON.parse(response.reportJson || '{}'), null, 2)
    } catch {
      reportJson.value = response.reportJson || ''
    }
    reportVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '白标产品预检失败')
  }
}

async function changeStatus(row: PlatformWhiteLabelProduct) {
  const status = row.status === 2 ? 3 : 2
  try {
    await ElMessageBox.confirm(
      status === 2 ? '启用前将重新执行模板、品牌和证书预检，是否继续？' : '确定停用该白标产品吗？',
      '状态确认',
    )
    const response = await platformService.changeWhiteLabelProductStatus(row.id, status)
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('状态已更新')
    await loadData()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close')
      ElMessage.error(error instanceof Error ? error.message : '状态更新失败')
  }
}

onMounted(loadData)
</script>

<template>
  <div class="module-page">
    <el-card shadow="never" class="query-card">
      <el-form inline>
        <el-form-item label="应用 ID">
          <el-input-number
            v-model="query.appId"
            :min="0"
          />
        </el-form-item>
        <el-form-item label="关键词">
          <el-input v-model="query.keyword" clearable />
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            style="width: 130px"
          >
            <el-option label="全部" :value="0" /><el-option label="草稿" :value="1" /><el-option
              label="启用"
              :value="2"
            /><el-option label="停用" :value="3" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="resetAndLoad(loadData)">
            查询
          </el-button><el-button
            v-perm="'core:white-label-product:add'"
            @click="openForm()"
          >
            新增
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="rows" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="appId" label="应用 ID" width="95" />
        <el-table-column prop="productCode" label="产品编码" min-width="140" />
        <el-table-column prop="productName" label="产品名称" min-width="150" />
        <el-table-column prop="packageName" label="Package Name" min-width="220" />
        <el-table-column prop="templateRevision" label="模板修订" width="100" />
        <el-table-column prop="brandingProfileId" label="品牌 ID" width="100" />
        <el-table-column prop="signingConfigId" label="签名 ID" width="100" />
        <el-table-column
          label="状态"
          width="90"
        >
          <template #default="scope">
            {{ statusText(scope.row.status) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="290" fixed="right">
          <template #default="scope">
            <el-button
              v-if="scope.row.status !== 2"
              v-perm="'core:white-label-product:update'"
              link
              type="primary"
              @click="openForm(scope.row)"
            >
              编辑
            </el-button>
            <el-button
              v-perm="'core:white-label-product:preflight'"
              link
              type="success"
              @click="preflight(scope.row)"
            >
              预检
            </el-button>
            <el-button
              v-perm="'core:white-label-product:status'"
              link
              @click="changeStatus(scope.row)"
            >
              {{ scope.row.status === 2 ? '停用' : '启用' }}
            </el-button>
            <el-button
              v-if="scope.row.status !== 2"
              v-perm="'core:white-label-product:delete'"
              link
              type="danger"
              @click="deleteProduct(scope.row)"
            >
              删除
            </el-button>
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

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="780px">
      <el-steps
        :active="formStep"
        finish-status="success"
        simple
        class="product-steps"
      >
        <el-step title="产品信息" />
        <el-step title="模板与品牌" />
        <el-step title="签名与参数" />
      </el-steps>
      <el-form ref="formRef" :model="form" label-width="150px">
        <template v-if="formStep === 0">
          <el-form-item
            label="应用 ID"
            prop="appId"
            :rules="[{ required: true, message: '请输入应用 ID' }]"
          >
            <el-input-number
              v-model="form.appId"
              :min="1"
              :disabled="Boolean(editingId)"
              style="width: 100%"
            />
          </el-form-item>
          <el-form-item>
            <el-button
              link
              type="primary"
              @click="loadDependencyOptions"
            >
              按应用 ID 加载模板、品牌和签名选项
            </el-button>
          </el-form-item>
          <el-form-item
            label="产品编码"
            prop="productCode"
            :rules="[{ required: true, message: '请输入产品编码' }]"
          >
            <el-input
              v-model="form.productCode"
              :disabled="Boolean(editingId)"
              placeholder="lowercase-product-code"
            />
          </el-form-item>
          <el-form-item
            label="产品名称"
            prop="productName"
            :rules="[{ required: true, message: '请输入产品名称' }]"
          >
            <el-input v-model="form.productName" />
          </el-form-item>
        </template>
        <template v-else-if="formStep === 1">
          <el-form-item
            label="模板 ID"
            prop="templateId"
            :rules="[{ required: true, message: '请输入模板 ID' }]"
          >
            <el-select
              v-model="form.templateId"
              filterable
              style="width: 100%"
              @change="selectTemplate"
            >
              <el-option
                v-for="item in templateOptions"
                :key="item.id"
                :label="`${item.templateName} · r${item.publishedRevision}`"
                :value="item.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item
            label="模板修订"
            prop="templateRevision"
            :rules="[{ required: true, message: '请输入模板修订' }]"
          >
            <el-input-number
              v-model="form.templateRevision"
              :min="1"
              style="width: 100%"
            />
          </el-form-item>
          <el-form-item
            label="品牌配置 ID"
            prop="brandingProfileId"
            :rules="[{ required: true, message: '请输入品牌配置 ID' }]"
          >
            <el-select v-model="form.brandingProfileId" filterable style="width: 100%">
              <el-option
                v-for="item in brandingOptions"
                :key="item.id"
                :label="`${item.profileName} · r${item.revision}`"
                :value="item.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item
            label="Package Name"
            prop="packageName"
            :rules="[{ required: true, message: '请输入 Android applicationId' }]"
          >
            <el-input
              v-model="form.packageName"
              placeholder="com.customer.product"
            />
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item
            label="签名配置 ID"
            prop="signingConfigId"
            :rules="[{ required: true, message: '请输入签名配置 ID' }]"
          >
            <el-select v-model="form.signingConfigId" filterable style="width: 100%">
              <el-option
                v-for="item in signingOptions"
                :key="item.id"
                :label="`${item.name} · ${item.certificateSha256?.slice(0, 12) || '未验证'}`"
                :value="item.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="证书 SHA-256">
            <el-input
              :model-value="selectedSigning?.certificateSha256 || '未验证，不能启用产品'"
              readonly
            />
          </el-form-item>
          <el-form-item label="参数值 JSON">
            <el-input
              v-model="form.parameterValuesJson"
              type="textarea"
              :rows="6"
            />
          </el-form-item>
        </template>
      </el-form>
      <el-alert
        title="Package Name 首次绑定证书后不可静默更换；启用前必须通过预检。"
        type="warning"
        :closable="false"
      />
      <template #footer>
        <el-button @click="dialogVisible = false">
          取消
        </el-button><el-button v-if="formStep > 0" @click="formStep--">
          上一步
        </el-button><el-button v-if="formStep < 2" type="primary" @click="formStep++">
          下一步
        </el-button><el-button
          v-else
          type="primary"
          :loading="submitting"
          @click="submit"
        >
          保存
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="reportVisible" title="白标产品预检报告" width="760px">
      <el-alert
        :title="reportCompatible ? '预检通过，可以启用并构建' : '预检未通过'"
        :type="reportCompatible ? 'success' : 'error'"
        :closable="false"
      />
      <pre class="report-json">{{ reportJson }}</pre>
    </el-dialog>
  </div>
</template>

<style scoped>
.report-json {
  max-height: 520px;
  overflow: auto;
  padding: 12px;
  background: var(--el-fill-color-light);
  white-space: pre-wrap;
  font-size: 12px;
}

.product-steps {
  margin-bottom: 22px;
}
</style>
