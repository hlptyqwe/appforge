<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import CursorPagination from '@/components/common/CursorPagination.vue'
import { IS_AGENT } from '@/config/environment'
import { usePagination } from '@/composables/usePagination'
import { platformService, type PlatformApplication, type PlatformLocalAgent } from '@/services'

const loading = ref(false)
const saving = ref(false)
const agents = ref<PlatformLocalAgent[]>([])
const applications = ref<PlatformApplication[]>([])
const createVisible = ref(false)
const tokenVisible = ref(false)
const registrationToken = ref('')
const registrationExpiresAt = ref(0)
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

const registerCommand = computed(
  () =>
    `appforge-local-agent register --control-url ${window.location.origin} ` +
    '--gateway-url https://<control-plane-host>:9443 --gateway-ca /path/to/gateway-ca.crt ' +
    `--token ${registrationToken.value}`,
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

async function copyText(value: string) {
  await navigator.clipboard.writeText(value)
  ElMessage.success('已复制')
}

function formatTime(value?: number) {
  return value ? new Date(value).toLocaleString() : '-'
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
          <el-button type="primary" @click="resetAndLoad(loadAgents)">
            查询
          </el-button>
          <el-button @click="openCreate">
            新增节点
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="agents" stripe>
        <el-table-column prop="agentCode" label="节点编码" min-width="150" />
        <el-table-column prop="agentName" label="节点名称" min-width="150" />
        <el-table-column prop="poolCode" label="构建池" width="110" />
        <el-table-column label="状态" width="105">
          <template #default="{ row }">
            <el-tag :type="statusLabels[row.status]?.type || 'info'">
              {{
                statusLabels[row.status]?.label || row.status
              }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="接单" width="90">
          <template #default="{ row }">
            {{
              drainLabels[row.drainStatus] || row.drainStatus
            }}
          </template>
        </el-table-column>
        <el-table-column label="产物模式" min-width="130">
          <template #default="{ row }">
            {{
              artifactLabels[row.artifactMode] || row.artifactMode
            }}
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
            {{ formatTime(row.certificateNotAfter) }}
          </template>
        </el-table-column>
        <el-table-column label="最近心跳" min-width="180">
          <template #default="{ row }">
            {{ formatTime(row.lastHeartbeatAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.status !== 4"
              link
              type="primary"
              @click="changeDrain(row)"
            >
              {{ row.drainStatus === 1 ? '排空' : '恢复' }}
            </el-button>
            <el-button
              v-if="row.status !== 4"
              link
              type="danger"
              @click="revoke(row)"
            >
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
        <el-form-item
          v-if="!IS_AGENT"
          label="租户 ID"
        >
          <el-input-number
            v-model="form.tenantId"
            :min="0"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="节点编码">
          <el-input
            v-model="form.agentCode"
            maxlength="64"
            placeholder="例如 shenzhen-builder-01"
          />
        </el-form-item>
        <el-form-item label="节点名称">
          <el-input
            v-model="form.agentName"
            maxlength="128"
          />
        </el-form-item>
        <el-form-item label="构建池">
          <el-input
            v-model="form.poolCode"
            maxlength="64"
          />
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
          <el-select
            v-model="form.allowedAppIds"
            multiple
            filterable
            style="width: 100%"
          >
            <el-option
              v-for="app in applications"
              :key="app.id"
              :value="app.id"
              :label="`${app.appName} (${app.id})`"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="最大并发">
          <el-input-number
            v-model="form.maxConcurrency"
            :min="1"
            :max="64"
            style="width: 100%"
          />
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
        <el-button @click="createVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="saving"
          @click="createRegistration"
        >
          生成注册令牌
        </el-button>
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
          <el-button @click="copyText(registrationToken)">
            复制令牌
          </el-button>
        </template>
      </el-input>
      <p>注册命令（请替换网关地址和网关 CA 路径）：</p>
      <el-input
        :model-value="registerCommand"
        type="textarea"
        :rows="4"
        readonly
      />
      <template #footer>
        <el-button type="primary" @click="copyText(registerCommand)">
          复制命令
        </el-button><el-button @click="tokenVisible = false">
          我已保存
        </el-button>
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
</style>
