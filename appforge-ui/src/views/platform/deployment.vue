<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { platformService, type PlatformDeploymentStatus } from '@/services'

const loading = ref(false)
const status = ref<PlatformDeploymentStatus>()

const modeLabels: Record<string, string> = {
  saas: 'SaaS',
  dedicated: 'Dedicated',
  private: 'Private',
  hybrid: 'Hybrid',
}

const healthyComponents = computed(
  () => status.value?.components.filter((item) => item.status === 'healthy').length || 0,
)

async function loadStatus() {
  loading.value = true
  try {
    const response = await platformService.getDeploymentStatus()
    if (response.code !== 200 || !response.data) throw new Error(response.msg)
    status.value = response.data
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载企业部署状态失败')
  } finally {
    loading.value = false
  }
}

async function copyDiagnosticsCommand() {
  if (!status.value?.diagnosticsCommand) return
  await navigator.clipboard.writeText(status.value.diagnosticsCommand)
  ElMessage.success('诊断命令已复制')
}

function componentTag(value: string) {
  if (value === 'healthy') return { label: '健康', type: 'success' as const }
  if (value === 'degraded') return { label: '降级', type: 'warning' as const }
  if (value === 'checking') return { label: '检查中', type: 'info' as const }
  return { label: '异常', type: 'danger' as const }
}

function licenseTag(value?: string) {
  if (value === 'valid') return { label: '有效', type: 'success' as const }
  if (value === 'not_required') return { label: '当前模式不需要', type: 'info' as const }
  return { label: '无效', type: 'danger' as const }
}

function formatTime(value?: number) {
  return value ? new Date(value).toLocaleString() : '-'
}

function fingerprint(value?: string) {
  if (!value) return '-'
  return value.length > 24 ? `${value.slice(0, 12)}…${value.slice(-12)}` : value
}

onMounted(loadStatus)
</script>

<template>
  <div class="module-page deployment-page" v-loading="loading">
    <el-card shadow="never" class="summary-card">
      <div class="summary-actions">
        <el-tag :type="status?.upgradeReady ? 'success' : 'warning'" size="large">
          {{ status?.upgradeReady ? '满足升级前置条件' : '升级前置条件未满足' }}
        </el-tag>
        <el-button @click="loadStatus">刷新状态</el-button>
      </div>
      <el-row :gutter="16">
        <el-col :xs="24" :sm="12" :lg="6">
          <el-statistic title="产品版本" :value="status?.productVersion || '-'" />
        </el-col>
        <el-col :xs="24" :sm="12" :lg="6">
          <el-statistic title="部署模式" :value="modeLabels[status?.deploymentMode || ''] || status?.deploymentMode || '-'" />
        </el-col>
        <el-col :xs="24" :sm="12" :lg="6">
          <el-statistic title="健康组件" :value="`${healthyComponents}/${status?.components.length || 0}`" />
        </el-col>
        <el-col :xs="24" :sm="12" :lg="6">
          <el-statistic title="迁移数量" :value="status?.migrationCount || 0" />
        </el-col>
      </el-row>
    </el-card>

    <el-row :gutter="16" class="content-row">
      <el-col :xs="24" :xl="14">
        <el-card shadow="never" class="content-card">
          <template #header>组件与数据库状态</template>
          <el-table :data="status?.components || []" stripe>
            <el-table-column prop="name" label="组件" min-width="160" />
            <el-table-column label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="componentTag(row.status).type">
                  {{ componentTag(row.status).label }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="message" label="说明" min-width="240">
              <template #default="{ row }">{{ row.message || '运行正常' }}</template>
            </el-table-column>
            <el-table-column label="检查时间" width="180">
              <template #default="{ row }">{{ formatTime(row.checkedAt) }}</template>
            </el-table-column>
          </el-table>

          <el-descriptions :column="1" border class="schema-info">
            <el-descriptions-item label="部署 ID">{{ status?.deploymentId || '-' }}</el-descriptions-item>
            <el-descriptions-item label="实际 Schema">{{ status?.actualSchemaVersion || '-' }}</el-descriptions-item>
            <el-descriptions-item label="目标 Schema">
              {{ status?.targetSchemaVersion || '-' }}
              <el-tag :type="status?.schemaCompatible ? 'success' : 'danger'" size="small">
                {{ status?.schemaCompatible ? '一致' : '不一致' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Agent 协议窗口">
              {{ status?.agentProtocolMinimum || '-' }}–{{ status?.agentProtocolCurrent || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="最大升级跨度">{{ status?.maxVersionSkew || '-' }} 个版本窗口</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>

      <el-col :xs="24" :xl="10">
        <el-card shadow="never" class="content-card">
          <template #header>企业许可证</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="状态">
              <el-tag :type="licenseTag(status?.license.status).type">
                {{ licenseTag(status?.license.status).label }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="许可证 ID">{{ status?.license.licenseId || '-' }}</el-descriptions-item>
            <el-descriptions-item label="客户">{{ status?.license.customer || '-' }}</el-descriptions-item>
            <el-descriptions-item label="授权模式">{{ status?.license.deploymentModes?.join('、') || '-' }}</el-descriptions-item>
            <el-descriptions-item label="有效期">{{ formatTime(status?.license.notBefore) }} 至 {{ formatTime(status?.license.notAfter) }}</el-descriptions-item>
            <el-descriptions-item label="序列">{{ status?.license.sequence || '-' }}</el-descriptions-item>
            <el-descriptions-item label="指纹">{{ fingerprint(status?.license.fingerprint) }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="content-row lower-row">
      <el-col :xs="24" :xl="14">
        <el-card shadow="never" class="content-card migration-card">
          <template #header>最近数据库迁移</template>
          <el-table :data="status?.recentMigrations || []" stripe>
            <el-table-column prop="version" label="迁移版本" min-width="250" />
            <el-table-column prop="description" label="说明" min-width="320" />
          </el-table>
        </el-card>
      </el-col>
      <el-col :xs="24" :xl="10">
        <el-card shadow="never" class="content-card">
          <template #header>升级与诊断</template>
          <el-alert
            title="页面只展示状态，不允许从浏览器执行服务器命令"
            type="info"
            :closable="false"
            show-icon
          />
          <ol class="upgrade-list">
            <li>确认组件健康、许可证有效且实际 Schema 与目标版本一致。</li>
            <li>按交付手册完成备份、容量和版本跨度预检。</li>
            <li>先执行 expand 迁移，再滚动升级 API/RPC，最后升级 Worker 与 Agent。</li>
            <li>异常时仅在 Schema 兼容范围内回滚应用镜像。</li>
          </ol>
          <el-input :model-value="status?.diagnosticsCommand || ''" readonly>
            <template #append>
              <el-button @click="copyDiagnosticsCommand">复制</el-button>
            </template>
          </el-input>
          <p class="hint">诊断包默认不包含日志、业务数据、私钥、Keystore 或令牌。</p>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.deployment-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.summary-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-bottom: 16px;
}

.content-row {
  row-gap: 16px;
}

.content-card {
  height: 100%;
}

.schema-info {
  margin-top: 16px;
}

.upgrade-list {
  margin: 16px 0;
  padding-left: 22px;
  line-height: 1.8;
}

.hint {
  margin: 12px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

@media (max-width: 767px) {
  .summary-actions {
    justify-content: space-between;
  }
}
</style>
