<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type UploadFile } from 'element-plus'
import CursorPagination from '@/components/common/CursorPagination.vue'
import { IS_AGENT } from '@/config/environment'
import { usePagination } from '@/composables/usePagination'
import {
  platformService,
  type PlatformAirGappedPackage,
  type PlatformApplication,
  type PlatformLocalAgent,
} from '@/services'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const loading = ref(false)
const saving = ref(false)
const agents = ref<PlatformLocalAgent[]>([])
const applications = ref<PlatformApplication[]>([])
const createVisible = ref(false)
const tokenVisible = ref(false)
const registrationToken = ref('')
const registrationExpiresAt = ref(0)
const airGappedVisible = ref(false)
const airGappedBusy = ref(false)
const airGappedAgent = ref<PlatformLocalAgent>()
const airGappedPackage = ref<PlatformAirGappedPackage>()
const airGappedDownloadUrl = ref('')
const airGappedResultFile = ref<File>()
const airGappedUploadProgress = ref(0)
const { pagination, updateFromResponse, resetAndLoad, nextAndLoad, prevAndLoad } =
  usePagination<number>(20)
const query = reactive({ status: 0, tenantId: 0 })
const form = reactive({
  tenantId: 0,
  agentCode: '',
  agentName: '',
  poolCode: 'local',
  artifactMode: 1,
  customerStorageRef: '',
  allowedAppIds: [] as number[],
  maxConcurrency: 1,
  expiresSeconds: 900,
})
const airGappedForm = reactive({
  taskId: 0,
  expiresSeconds: 3600,
  packageCode: '',
})

const canExportAirGapped = computed(() => auth.hasPerm('enterprise:air-gapped:export'))
const canViewAirGapped = computed(() => auth.hasPerm('enterprise:air-gapped:view'))
const canImportAirGapped = computed(
  () =>
    auth.hasPerm('enterprise:air-gapped:import') &&
    auth.hasPerm('enterprise:air-gapped:view') &&
    auth.hasPerm('core:storage:upload'),
)
const canUseAirGapped = computed(
  () => canExportAirGapped.value || canViewAirGapped.value || canImportAirGapped.value,
)

const registerCommand = computed(
  () =>
    "read -rsp '一次性注册码: ' APPFORGE_REGISTRATION_TOKEN && echo && " +
    'printf \'%s\' "$APPFORGE_REGISTRATION_TOKEN" | ' +
    `appforge-local-agent register --control-url ${window.location.origin} ` +
    '--control-ca /path/to/control-ca.crt --gateway-url https://<control-plane-host>:9443 ' +
    '--gateway-ca /path/to/gateway-ca.crt --token-stdin; ' +
    'unset APPFORGE_REGISTRATION_TOKEN',
)

const certificateWarningWindowMs = 24 * 60 * 60 * 1000
const expiringCertificateCount = computed(
  () =>
    agents.value.filter((agent) => {
      const expiresAt = agent.certificateNotAfter || 0
      return expiresAt > 0 && expiresAt <= Date.now() + certificateWarningWindowMs
    }).length,
)

const statusLabels: Record<
  number,
  { label: string; type: 'info' | 'success' | 'warning' | 'danger' }
> = {
  1: { label: '待注册', type: 'info' },
  2: { label: '在线', type: 'success' },
  3: { label: '离线', type: 'warning' },
  4: { label: '已吊销', type: 'danger' },
  5: { label: '需要升级', type: 'danger' },
}
const drainLabels: Record<number, string> = { 1: '接单', 2: '排空中', 3: '已排空' }
const artifactLabels: Record<number, string> = {
  1: '控制面存储',
  2: '客户存储',
  3: '离线任务包',
}
const airGappedStatusLabels: Record<
  number,
  { label: string; type: 'info' | 'success' | 'warning' | 'danger' }
> = {
  1: { label: '准备中', type: 'info' },
  2: { label: '已导出', type: 'warning' },
  3: { label: '已导入', type: 'success' },
  4: { label: '已过期', type: 'danger' },
  5: { label: '已撤销', type: 'danger' },
}

async function loadAgents() {
  loading.value = true
  try {
    const response = await platformService.listLocalAgents({
      cursor: pagination.cursor,
      limit: pagination.limit,
      status: query.status || undefined,
      tenantId: !IS_AGENT && query.tenantId > 0 ? query.tenantId : undefined,
    })
    if (response.code !== 200) throw new Error(response.msg)
    agents.value = response.data || []
    updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载本地构建节点失败')
  } finally {
    loading.value = false
  }
}

async function loadApplications() {
  const response = await platformService.listApplications({ limit: 100 })
  if (response.code === 200) applications.value = response.data || []
}

function openCreate() {
  Object.assign(form, {
    tenantId: query.tenantId || 0,
    agentCode: '',
    agentName: '',
    poolCode: 'local',
    artifactMode: 1,
    customerStorageRef: '',
    allowedAppIds: [],
    maxConcurrency: 1,
    expiresSeconds: 900,
  })
  createVisible.value = true
}

async function createRegistration() {
  if (!form.agentCode.trim() || !form.agentName.trim() || !form.poolCode.trim()) {
    ElMessage.warning('请填写节点编码、名称和构建池')
    return
  }
  if (!form.allowedAppIds.length) {
    ElMessage.warning('至少授权一个应用')
    return
  }
  if (form.artifactMode === 2 && !form.customerStorageRef.trim()) {
    ElMessage.warning('客户存储模式必须填写 Secret 引用')
    return
  }
  saving.value = true
  try {
    const response = await platformService.createLocalAgentRegistration({
      tenantId: !IS_AGENT ? form.tenantId : 0,
      agentCode: form.agentCode.trim(),
      agentName: form.agentName.trim(),
      poolCode: form.poolCode.trim(),
      artifactMode: form.artifactMode,
      customerStorageRef: form.artifactMode === 2 ? form.customerStorageRef.trim() : '',
      allowedAppIds: form.allowedAppIds,
      capabilities: [
        { capabilityKey: 'apk', capabilityValue: 'true' },
        { capabilityKey: 'max_concurrency', capabilityValue: String(form.maxConcurrency) },
      ],
      expiresSeconds: form.expiresSeconds,
    })
    if (response.code !== 200 || !response.registrationToken) throw new Error(response.msg)
    registrationToken.value = response.registrationToken
    registrationExpiresAt.value = response.expiresAt
    createVisible.value = false
    tokenVisible.value = true
    await resetAndLoad(loadAgents)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '创建注册令牌失败')
  } finally {
    saving.value = false
  }
}

async function changeDrain(agent: PlatformLocalAgent) {
  const target = agent.drainStatus === 1 ? 2 : 1
  const action = target === 2 ? '排空' : '恢复接单'
  await ElMessageBox.confirm(`确认让节点 ${agent.agentName} ${action}？`, '本地节点状态')
  const response = await platformService.drainLocalAgent(agent.id, target, agent.tenantId)
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success(`${action}成功`)
  await loadAgents()
}

async function revoke(agent: PlatformLocalAgent) {
  const result = await ElMessageBox.prompt(
    `吊销后节点证书立即失效，节点 ${agent.agentName} 将不能再领取任务。`,
    '吊销本地节点',
    {
      confirmButtonText: '确认吊销',
      inputPlaceholder: '请输入吊销原因',
      inputPattern: /\S+/,
      inputErrorMessage: '必须填写原因',
      type: 'warning',
    },
  )
  const response = await platformService.revokeLocalAgent(agent.id, result.value, agent.tenantId)
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success('节点及其有效证书已吊销')
  await loadAgents()
}

function resetAirGappedResult() {
  airGappedPackage.value = undefined
  airGappedDownloadUrl.value = ''
  airGappedResultFile.value = undefined
  airGappedUploadProgress.value = 0
}

function openAirGapped(agent: PlatformLocalAgent) {
  airGappedAgent.value = agent
  Object.assign(airGappedForm, { taskId: 0, expiresSeconds: 3600, packageCode: '' })
  resetAirGappedResult()
  airGappedVisible.value = true
}

async function exportAirGappedPackage() {
  const agent = airGappedAgent.value
  if (!agent || agent.artifactMode !== 3) {
    ElMessage.error('只能为隔离网离线节点导出任务包')
    return
  }
  if (!Number.isSafeInteger(airGappedForm.taskId) || airGappedForm.taskId <= 0) {
    ElMessage.warning('请输入有效的待构建任务 ID')
    return
  }
  airGappedBusy.value = true
  try {
    const response = await platformService.prepareAirGappedExport({
      agentId: agent.id,
      taskId: airGappedForm.taskId,
      expiresSeconds: airGappedForm.expiresSeconds,
    })
    if (response.code !== 200 || !response.data?.package || !response.data.downloadUrl) {
      throw new Error(response.msg || '导出离线任务包失败')
    }
    airGappedPackage.value = response.data.package
    airGappedDownloadUrl.value = response.data.downloadUrl
    airGappedForm.packageCode = response.data.package.packageCode
    ElMessage.success('任务已锁定，请立即下载并安全转移任务包')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '导出离线任务包失败')
  } finally {
    airGappedBusy.value = false
  }
}

async function queryAirGappedPackage() {
  const packageCode = airGappedForm.packageCode.trim()
  if (!packageCode) {
    ElMessage.warning('请输入离线包编码')
    return
  }
  airGappedBusy.value = true
  try {
    const response = await platformService.getAirGappedPackage(packageCode)
    if (response.code !== 200 || !response.data) {
      throw new Error(response.msg || '查询离线包失败')
    }
    airGappedPackage.value = response.data
    airGappedDownloadUrl.value = ''
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '查询离线包失败')
  } finally {
    airGappedBusy.value = false
  }
}

function selectAirGappedResult(uploadFile: UploadFile) {
  const file = uploadFile.raw
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.zip')) {
    ElMessage.error('结果包必须是 .zip 文件')
    airGappedResultFile.value = undefined
    return
  }
  if (file.size <= 0 || file.size > 3 * 1024 * 1024 * 1024) {
    ElMessage.error('结果包必须大于 0 且不超过 3 GiB')
    airGappedResultFile.value = undefined
    return
  }
  airGappedResultFile.value = file
  airGappedUploadProgress.value = 0
}

function removeAirGappedResult() {
  airGappedResultFile.value = undefined
  airGappedUploadProgress.value = 0
}

async function importAirGappedResult() {
  const packageState = airGappedPackage.value
  const file = airGappedResultFile.value
  if (!packageState || packageState.packageCode !== airGappedForm.packageCode.trim()) {
    ElMessage.warning('请先查询并确认离线包状态')
    return
  }
  if (packageState.status !== 2) {
    ElMessage.warning('只有“已导出”状态的离线包可以导入结果')
    return
  }
  if (!file) {
    ElMessage.warning('请选择 Local Agent 生成的签名结果 ZIP')
    return
  }
  await ElMessageBox.confirm(
    `将结果包导入任务 ${packageState.taskId}（attempt ${packageState.builderAttempt}），确认文件来自绑定的离线 Agent？`,
    '导入 AIR_GAPPED 结果',
    { type: 'warning' },
  )
  airGappedBusy.value = true
  try {
    const upload = await platformService.uploadObject(file, 10, packageState.appId, (percent) => {
      airGappedUploadProgress.value = percent
    })
    if (upload.code !== 200 || !upload.data?.objectId) {
      throw new Error(upload.msg || '上传离线结果包失败')
    }
    const response = await platformService.importAirGappedResult(
      packageState.packageCode,
      upload.data.objectId,
    )
    if (response.code !== 200 || !response.data) {
      throw new Error(response.msg || '导入离线结果包失败')
    }
    airGappedPackage.value = response.data
    airGappedResultFile.value = undefined
    ElMessage.success('结果签名与完整性校验通过，构建结果已导入')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '导入离线结果包失败')
  } finally {
    airGappedBusy.value = false
  }
}

function downloadAirGappedPackage() {
  if (!airGappedDownloadUrl.value) return
  window.location.assign(airGappedDownloadUrl.value)
}

async function copyText(value: string) {
  await navigator.clipboard.writeText(value)
  ElMessage.success('已复制')
}

function formatTime(value?: number) {
  return value ? new Date(value).toLocaleString() : '-'
}

function certificateAlert(value?: number) {
  if (!value) return null
  const remaining = value - Date.now()
  if (remaining <= 0) return { label: '已过期', type: 'danger' as const }
  if (remaining <= certificateWarningWindowMs) {
    return { label: '24小时内到期', type: 'warning' as const }
  }
  return null
}

onMounted(async () => {
  await Promise.all([loadAgents(), loadApplications()])
})
</script>

<template>
  <div class="module-page local-agent-page">
    <el-card shadow="never" class="query-card">
      <el-form inline>
        <el-form-item v-if="!IS_AGENT" label="租户 ID">
          <el-input-number v-model="query.tenantId" :min="0" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" style="width: 140px">
            <el-option :value="0" label="全部" />
            <el-option
              v-for="(item, value) in statusLabels"
              :key="value"
              :value="Number(value)"
              :label="item.label"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="resetAndLoad(loadAgents)"> 查询 </el-button>
          <el-button @click="openCreate"> 新增节点 </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="table-card">
      <el-alert
        v-if="expiringCertificateCount > 0"
        :title="`${expiringCertificateCount} 个节点证书已过期或将在 24 小时内到期，请确认自动轮换或重新注册`"
        type="warning"
        show-icon
        :closable="false"
        style="margin-bottom: 16px"
      />
      <el-table v-loading="loading" :data="agents" stripe>
        <el-table-column prop="agentCode" label="节点编码" min-width="150" />
        <el-table-column prop="agentName" label="节点名称" min-width="150" />
        <el-table-column prop="poolCode" label="构建池" width="110" />
        <el-table-column label="状态" width="105">
          <template #default="{ row }">
            <el-tag :type="statusLabels[row.status]?.type || 'info'">
              {{ statusLabels[row.status]?.label || row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="接单" width="90">
          <template #default="{ row }">
            {{ drainLabels[row.drainStatus] || row.drainStatus }}
          </template>
        </el-table-column>
        <el-table-column label="产物模式" min-width="130">
          <template #default="{ row }">
            {{ artifactLabels[row.artifactMode] || row.artifactMode }}
          </template>
        </el-table-column>
        <el-table-column label="授权应用" min-width="150">
          <template #default="{ row }">
            {{ row.allowedAppIds.join(', ') || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="版本 / 协议" min-width="135">
          <template #default="{ row }">
            {{ row.agentVersion || '-' }} / {{ row.protocolVersion }}
          </template>
        </el-table-column>
        <el-table-column label="证书到期" min-width="180">
          <template #default="{ row }">
            <div>{{ formatTime(row.certificateNotAfter) }}</div>
            <el-tag
              v-if="certificateAlert(row.certificateNotAfter)"
              size="small"
              :type="certificateAlert(row.certificateNotAfter)?.type"
            >
              {{ certificateAlert(row.certificateNotAfter)?.label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="最近心跳" min-width="180">
          <template #default="{ row }">
            {{ formatTime(row.lastHeartbeatAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="235" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.artifactMode === 3 && canUseAirGapped"
              link
              type="primary"
              @click="openAirGapped(row)"
            >
              离线包
            </el-button>
            <el-button v-if="row.status !== 4" link type="primary" @click="changeDrain(row)">
              {{ row.drainStatus === 1 ? '排空' : '恢复' }}
            </el-button>
            <el-button v-if="row.status !== 4" link type="danger" @click="revoke(row)">
              吊销
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <CursorPagination
        v-model:limit="pagination.limit"
        :total="pagination.total"
        :has-prev="pagination.hasPrev"
        :has-next="pagination.hasNext"
        @prev="prevAndLoad(loadAgents)"
        @next="nextAndLoad(loadAgents)"
        @limit-change="resetAndLoad(loadAgents)"
      />
    </el-card>

    <el-dialog v-model="createVisible" title="新增本地构建节点" width="620px">
      <el-form :model="form" label-width="130px">
        <el-form-item v-if="!IS_AGENT" label="租户 ID">
          <el-input-number v-model="form.tenantId" :min="0" style="width: 100%" />
        </el-form-item>
        <el-form-item label="节点编码">
          <el-input
            v-model="form.agentCode"
            maxlength="64"
            placeholder="例如 shenzhen-builder-01"
          />
        </el-form-item>
        <el-form-item label="节点名称">
          <el-input v-model="form.agentName" maxlength="128" />
        </el-form-item>
        <el-form-item label="构建池">
          <el-input v-model="form.poolCode" maxlength="64" />
        </el-form-item>
        <el-form-item label="产物模式">
          <el-select v-model="form.artifactMode" style="width: 100%">
            <el-option :value="1" label="控制面私有对象存储" />
            <el-option :value="2" label="客户对象存储" />
            <el-option :value="3" label="隔离网离线任务包" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.artifactMode === 2" label="存储 Secret 引用">
          <el-input
            v-model="form.customerStorageRef"
            maxlength="500"
            placeholder="vault://、aws-sm:// 或 local-file:// 引用"
          />
        </el-form-item>
        <el-form-item label="授权应用">
          <el-select v-model="form.allowedAppIds" multiple filterable style="width: 100%">
            <el-option
              v-for="app in applications"
              :key="app.id"
              :value="app.id"
              :label="`${app.appName} (${app.id})`"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="最大并发">
          <el-input-number v-model="form.maxConcurrency" :min="1" :max="64" style="width: 100%" />
        </el-form-item>
        <el-form-item label="令牌有效期">
          <el-input-number
            v-model="form.expiresSeconds"
            :min="60"
            :max="86400"
            style="width: 100%"
          /><span class="field-hint">秒，仅可使用一次</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false"> 取消 </el-button>
        <el-button type="primary" :loading="saving" @click="createRegistration">
          生成注册令牌
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="airGappedVisible"
      title="AIR_GAPPED 离线任务包"
      width="760px"
      :close-on-click-modal="false"
    >
      <el-alert
        title="任务包和结果包必须通过受控介质转移。浏览器只负责控制面私有上传/下载，不会连接离线 Agent。"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 18px"
      />

      <el-descriptions :column="2" border style="margin-bottom: 18px">
        <el-descriptions-item label="节点">
          {{ airGappedAgent?.agentName }}（{{ airGappedAgent?.id }}）
        </el-descriptions-item>
        <el-descriptions-item label="构建池">
          {{ airGappedAgent?.poolCode || '-' }}
        </el-descriptions-item>
      </el-descriptions>

      <section v-if="canExportAirGapped" class="air-gapped-section">
        <h4>1. 导出签名任务包</h4>
        <el-form inline>
          <el-form-item label="待构建任务 ID">
            <el-input-number
              v-model="airGappedForm.taskId"
              :min="1"
              :step="1"
              controls-position="right"
            />
          </el-form-item>
          <el-form-item label="有效期">
            <el-input-number
              v-model="airGappedForm.expiresSeconds"
              :min="300"
              :max="86400"
              :step="300"
              controls-position="right"
            />
            <span class="field-hint">秒</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="airGappedBusy" @click="exportAirGappedPackage">
              锁定任务并导出
            </el-button>
          </el-form-item>
        </el-form>
      </section>

      <section v-if="canViewAirGapped" class="air-gapped-section">
        <h4>{{ canExportAirGapped ? '2' : '1' }}. 查询离线包</h4>
        <el-input
          v-model="airGappedForm.packageCode"
          maxlength="128"
          placeholder="输入导出时生成的 package code"
        >
          <template #append>
            <el-button :loading="airGappedBusy" @click="queryAirGappedPackage"> 查询 </el-button>
          </template>
        </el-input>
      </section>

      <el-descriptions v-if="airGappedPackage" :column="2" border class="air-gapped-package">
        <el-descriptions-item label="包编码" :span="2">
          <span class="monospace">{{ airGappedPackage.packageCode }}</span>
          <el-button link type="primary" @click="copyText(airGappedPackage.packageCode)">
            复制
          </el-button>
        </el-descriptions-item>
        <el-descriptions-item label="任务 / Attempt">
          {{ airGappedPackage.taskId }} / {{ airGappedPackage.builderAttempt }}
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="airGappedStatusLabels[airGappedPackage.status]?.type || 'info'">
            {{ airGappedStatusLabels[airGappedPackage.status]?.label || airGappedPackage.status }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="应用 ID">
          {{ airGappedPackage.appId }}
        </el-descriptions-item>
        <el-descriptions-item label="到期时间">
          {{ formatTime(airGappedPackage.expiresAt) }}
        </el-descriptions-item>
        <el-descriptions-item label="任务包 SHA-256" :span="2">
          <span class="monospace">{{ airGappedPackage.exportSha256 || '-' }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="结果包 SHA-256" :span="2">
          <span class="monospace">{{ airGappedPackage.resultSha256 || '-' }}</span>
        </el-descriptions-item>
      </el-descriptions>

      <div v-if="airGappedDownloadUrl" class="air-gapped-download">
        <el-button type="success" @click="downloadAirGappedPackage"> 下载任务 ZIP </el-button>
        <span>短时地址只显示在本次导出响应中；请校验上方 SHA-256。</span>
      </div>

      <section v-if="canImportAirGapped" class="air-gapped-section">
        <h4>{{ canExportAirGapped ? '3' : '2' }}. 上传并导入 Agent 签名结果包</h4>
        <el-upload
          :auto-upload="false"
          :limit="1"
          accept=".zip,application/zip"
          :on-change="selectAirGappedResult"
          :on-remove="removeAirGappedResult"
        >
          <el-button>选择结果 ZIP</el-button>
          <template #tip>
            <div class="el-upload__tip">
              必须先查询包状态；文件会上传到控制面私有对象存储并接受完整签名、身份、防重放和 SHA
              校验。
            </div>
          </template>
        </el-upload>
        <el-progress
          v-if="airGappedUploadProgress > 0"
          :percentage="airGappedUploadProgress"
          style="margin-top: 12px"
        />
        <el-button
          type="primary"
          :loading="airGappedBusy"
          :disabled="!airGappedResultFile || airGappedPackage?.status !== 2"
          style="margin-top: 14px"
          @click="importAirGappedResult"
        >
          校验并导入结果
        </el-button>
      </section>

      <el-alert
        v-if="!canImportAirGapped && canViewAirGapped"
        title="当前角色仅可查询。导入结果还需要 enterprise:air-gapped:import 和 core:storage:upload 权限。"
        type="info"
        :closable="false"
        style="margin-top: 18px"
      />
      <template #footer>
        <el-button @click="airGappedVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="tokenVisible"
      title="请立即保存注册令牌"
      width="760px"
      :close-on-click-modal="false"
    >
      <el-alert
        title="注册令牌只显示这一次；使用后立即失效。Agent 私钥只在客户机器本地生成。"
        type="warning"
        :closable="false"
        show-icon
      />
      <p>有效期至：{{ formatTime(registrationExpiresAt) }}</p>
      <el-input :model-value="registrationToken" readonly>
        <template #append>
          <el-button @click="copyText(registrationToken)"> 复制令牌 </el-button>
        </template>
      </el-input>
      <p>注册命令（请替换网关地址和网关 CA 路径）：</p>
      <el-input :model-value="registerCommand" type="textarea" :rows="4" readonly />
      <template #footer>
        <el-button type="primary" @click="copyText(registerCommand)"> 复制命令 </el-button
        ><el-button @click="tokenVisible = false"> 我已保存 </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.local-agent-page :deep(.el-table) {
  flex: 1;
  min-height: 0;
}
.field-hint {
  margin-left: 10px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.air-gapped-section + .air-gapped-section {
  margin-top: 20px;
}
.air-gapped-section h4 {
  margin: 0 0 12px;
}
.air-gapped-package {
  margin-top: 18px;
}
.air-gapped-download {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 14px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.monospace {
  overflow-wrap: anywhere;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}
</style>
