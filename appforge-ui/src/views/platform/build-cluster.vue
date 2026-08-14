<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import CursorPagination from '@/components/common/CursorPagination.vue'
import { usePagination } from '@/composables/usePagination'
import {
  platformService,
  type PlatformBuilderNode,
  type PlatformBuildCacheEntry,
  type PlatformBuildClusterMetrics,
  type PlatformBuildConcurrencyPolicy,
  type PlatformBuildSchedulerEvent,
} from '@/services'

type TabName = 'nodes' | 'policies' | 'cache' | 'events'

const activeTab = ref<TabName>('nodes')
const loading = ref(false)
const nodes = ref<PlatformBuilderNode[]>([])
const policies = ref<PlatformBuildConcurrencyPolicy[]>([])
const caches = ref<PlatformBuildCacheEntry[]>([])
const events = ref<PlatformBuildSchedulerEvent[]>([])
const metrics = ref<PlatformBuildClusterMetrics>()
const nodePage = usePagination<number>(20)
const policyPage = usePagination<number>(20)
const cachePage = usePagination<number>(20)
const eventPage = usePagination<number>(20)
const query = reactive({
  poolCode: 'default',
  periodMinutes: 60,
  keyword: '',
  taskId: 0,
  eventType: 0,
})
const policyVisible = ref(false)
const policySaving = ref(false)
const cacheCleaning = ref(false)
const policyForm = reactive({
  id: 0,
  appId: 0,
  poolCode: 'default',
  maxConcurrency: 2,
  fairWeight: 100,
  maxPriority: 100,
  status: 1,
})

function currentPage() {
  if (activeTab.value === 'nodes') return nodePage
  if (activeTab.value === 'policies') return policyPage
  if (activeTab.value === 'cache') return cachePage
  return eventPage
}

async function loadData() {
  loading.value = true
  try {
    const metricsResponse = await platformService.getBuildClusterMetrics({
      poolCode: query.poolCode || undefined,
      periodMinutes: query.periodMinutes,
    })
    if (metricsResponse.code !== 200) throw new Error(metricsResponse.msg)
    metrics.value = metricsResponse.data

    const pager = currentPage()
    const params = {
      cursor: pager.pagination.cursor,
      limit: pager.pagination.limit,
      poolCode: query.poolCode || undefined,
    }
    if (activeTab.value === 'nodes') {
      const response = await platformService.listBuilderNodes({
        ...params,
        keyword: query.keyword || undefined,
      })
      if (response.code !== 200) throw new Error(response.msg)
      nodes.value = response.data || []
      nodePage.updateFromResponse(response)
    } else if (activeTab.value === 'policies') {
      const response = await platformService.listBuildConcurrencyPolicies(params)
      if (response.code !== 200) throw new Error(response.msg)
      policies.value = response.data || []
      policyPage.updateFromResponse(response)
    } else if (activeTab.value === 'cache') {
      const response = await platformService.listBuildCacheEntries({
        ...params,
        keyword: query.keyword || undefined,
      })
      if (response.code !== 200) throw new Error(response.msg)
      caches.value = response.data || []
      cachePage.updateFromResponse(response)
    } else {
      const response = await platformService.listBuildSchedulerEvents({
        ...params,
        taskId: query.taskId || undefined,
        eventType: query.eventType || undefined,
      })
      if (response.code !== 200) throw new Error(response.msg)
      events.value = response.data || []
      eventPage.updateFromResponse(response)
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载构建集群失败')
  } finally {
    loading.value = false
  }
}

function resetAndLoad() {
  currentPage().resetAndLoad(loadData)
}

function onTabChange() {
  resetAndLoad()
}

async function changeDrain(node: PlatformBuilderNode) {
  const target = node.drainStatus === 2 ? 1 : 2
  const label = target === 2 ? '排空' : '恢复接单'
  await ElMessageBox.confirm(`确认让节点 ${node.nodeCode} ${label}？`, '节点状态')
  const response = await platformService.drainBuilderNode(node.id, target)
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success(`${label}成功`)
  await loadData()
}

async function recoverNode(node: PlatformBuilderNode) {
  const result = await ElMessageBox.prompt(
    `确认解除节点 ${node.nodeCode} 的隔离？系统会重新校验心跳、失败次数和磁盘容量。`,
    '恢复节点',
    {
      confirmButtonText: '确认恢复',
      cancelButtonText: '取消',
      inputPlaceholder: '请输入恢复原因',
      inputValidator: (value) => {
        const length = value.trim().length
        return length >= 2 && length <= 200 ? true : '恢复原因需为 2–200 个字符'
      },
    },
  )
  const response = await platformService.recoverBuilderNode(node.id, result.value.trim())
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success('节点已恢复在线')
  await loadData()
}

function openPolicy(item?: PlatformBuildConcurrencyPolicy) {
  Object.assign(
    policyForm,
    item
      ? {
          id: item.id,
          appId: item.appId,
          poolCode: item.poolCode,
          maxConcurrency: item.maxConcurrency,
          fairWeight: item.fairWeight,
          maxPriority: item.maxPriority,
          status: item.status,
        }
      : {
          id: 0,
          appId: 0,
          poolCode: 'default',
          maxConcurrency: 2,
          fairWeight: 100,
          maxPriority: 100,
          status: 1,
        },
  )
  policyVisible.value = true
}

async function savePolicy() {
  policySaving.value = true
  try {
    const response = await platformService.upsertBuildConcurrencyPolicy({ ...policyForm })
    if (response.code !== 200) throw new Error(response.msg)
    policyVisible.value = false
    ElMessage.success('并发策略已保存')
    await policyPage.resetAndLoad(loadData)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存策略失败')
  } finally {
    policySaving.value = false
  }
}

async function invalidateCache(item: PlatformBuildCacheEntry) {
  await ElMessageBox.confirm(`确认失效缓存 ${item.id}？后续任务会重新构建。`, '失效缓存')
  const response = await platformService.invalidateBuildCache(item.id, 'MANUAL_INVALIDATION')
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success('缓存已失效')
  await loadData()
}

async function cleanupExpiredCache() {
  await ElMessageBox.confirm('确认清理已过期的构建缓存？仍被成功任务引用的对象会保留。', '清理缓存')
  cacheCleaning.value = true
  try {
    const response = await platformService.cleanupBuildCache(100, 0)
    if (response.code !== 200) throw new Error(response.msg)
    const result = response.data
    ElMessage.success(
      `已失效 ${result?.invalidatedCount || 0} 条，待回收 ${formatBytes(result?.reclaimableBytes || 0)}`,
    )
    await cachePage.resetAndLoad(loadData)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '清理缓存失败')
  } finally {
    cacheCleaning.value = false
  }
}

function formatBytes(value: number) {
  if (!value) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let size = value
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index++
  }
  return `${size.toFixed(index ? 1 : 0)} ${units[index]}`
}

function formatTime(value?: number) {
  return value ? new Date(value).toLocaleString() : '-'
}

function formatDuration(value: number) {
  if (!value) return '0 秒'
  if (value < 1000) return `${value} ms`
  if (value < 60_000) return `${(value / 1000).toFixed(1)} 秒`
  return `${(value / 60_000).toFixed(1)} 分钟`
}

function formatRate(value: number) {
  return `${(value * 100).toFixed(1)}%`
}

const alertLabels: Record<string, string> = {
  BUILDER_OFFLINE: '存在离线或隔离的 Builder 节点',
  BUILDER_LOW_DISK: '存在磁盘可用空间低于 10% 的 Builder 节点',
  QUEUE_BACKLOG: '最老排队任务已等待超过 5 分钟',
  RECENT_BUILD_FAILURE: '统计窗口内存在构建失败任务',
  LEASE_RECOVERY: '统计窗口内发生过租约恢复',
  CACHE_VALIDATION_FAILURE: '统计窗口内发生过缓存校验失败',
}

onMounted(loadData)
</script>

<template>
  <div class="module-page cluster-page">
    <el-card shadow="never" class="query-card">
      <el-form inline>
        <el-form-item label="构建池">
          <el-input v-model="query.poolCode" clearable />
        </el-form-item>
        <el-form-item label="统计窗口">
          <el-select
            v-model="query.periodMinutes"
            style="width: 120px"
          >
            <el-option :value="60" label="最近 1 小时" /><el-option
              :value="360"
              label="最近 6 小时"
            /><el-option :value="1440" label="最近 24 小时" />
          </el-select>
        </el-form-item>
        <el-form-item
          v-if="activeTab === 'nodes' || activeTab === 'cache'"
          label="关键词"
        >
          <el-input
            v-model="query.keyword"
            clearable
          />
        </el-form-item>
        <el-form-item
          v-if="activeTab === 'events'"
          label="任务 ID"
        >
          <el-input-number
            v-model="query.taskId"
            :min="0"
          />
        </el-form-item>
        <el-form-item
          v-if="activeTab === 'events'"
          label="事件类型"
        >
          <el-input-number
            v-model="query.eventType"
            :min="0"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="resetAndLoad">
            查询
          </el-button>
          <el-button v-if="activeTab === 'policies'" @click="openPolicy()">
            新增策略
          </el-button>
          <el-button
            v-if="activeTab === 'cache'"
            :loading="cacheCleaning"
            @click="cleanupExpiredCache"
          >
            清理过期缓存
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="table-card">
      <div v-if="metrics" class="metrics-grid">
        <div class="metric-item">
          <span>在线节点</span><strong>{{ metrics.onlineNodes }}</strong>
        </div>
        <div class="metric-item">
          <span>运行槽位</span><strong>{{ metrics.runningSlots }}/{{ metrics.totalSlots }}</strong>
        </div>
        <div class="metric-item">
          <span>排队 / 执行</span><strong>{{ metrics.queuedTasks }}/{{ metrics.runningTasks }}</strong>
        </div>
        <div class="metric-item">
          <span>成功率</span><strong>{{ formatRate(metrics.successRate) }}</strong>
        </div>
        <div class="metric-item">
          <span>平均排队 / 构建</span><strong>{{ formatDuration(metrics.averageQueueMs) }} /
            {{ formatDuration(metrics.averageBuildMs) }}</strong>
        </div>
        <div class="metric-item">
          <span>缓存命中 / 占用</span><strong>{{ formatRate(metrics.cacheHitRate) }} /
            {{ formatBytes(metrics.activeCacheBytes) }}</strong>
        </div>
      </div>
      <div v-if="metrics?.alerts?.length" class="cluster-alerts">
        <el-alert
          v-for="code in metrics.alerts"
          :key="code"
          :title="alertLabels[code] || code"
          type="warning"
          :closable="false"
          show-icon
        />
      </div>

      <el-tabs v-model="activeTab" @tab-change="onTabChange">
        <el-tab-pane label="Builder 节点" name="nodes" />
        <el-tab-pane label="并发策略" name="policies" />
        <el-tab-pane label="构建缓存" name="cache" />
        <el-tab-pane label="调度事件" name="events" />
      </el-tabs>

      <el-table
        v-if="activeTab === 'nodes'"
        v-loading="loading"
        :data="nodes"
        stripe
      >
        <el-table-column prop="nodeCode" label="节点" min-width="150" />
        <el-table-column prop="poolCode" label="构建池" width="110" />
        <el-table-column
          label="健康"
          width="90"
        >
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : row.status === 3 ? 'danger' : 'info'">
              {{
                row.status === 1 ? '在线' : row.status === 3 ? '隔离' : '离线'
              }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="槽位"
          width="110"
        >
          <template #default="{ row }">
            {{ row.runningCount }}/{{ row.maxConcurrency }}
          </template>
        </el-table-column>
        <el-table-column prop="toolchainVersion" label="工具链" min-width="150" />
        <el-table-column
          label="磁盘可用"
          width="120"
        >
          <template #default="{ row }">
            {{ formatBytes(row.diskFree) }}
          </template>
        </el-table-column>
        <el-table-column
          label="最近心跳"
          min-width="180"
        >
          <template #default="{ row }">
            {{
              formatTime(row.lastHeartbeatAt)
            }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="180"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              v-if="row.status === 3"
              link
              type="danger"
              @click="recoverNode(row)"
            >
              恢复节点
            </el-button><el-button link type="primary" @click="changeDrain(row)">
              {{
                row.drainStatus === 2 ? '恢复接单' : '排空'
              }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-table
        v-else-if="activeTab === 'policies'"
        v-loading="loading"
        :data="policies"
        stripe
      >
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="appId" label="应用 ID（0=租户）" min-width="150" />
        <el-table-column prop="poolCode" label="构建池" />
        <el-table-column prop="maxConcurrency" label="最大并发" />
        <el-table-column prop="fairWeight" label="公平权重" />
        <el-table-column prop="maxPriority" label="最高优先级" />
        <el-table-column prop="status" label="状态" />
        <el-table-column
          label="操作"
          width="90"
        >
          <template #default="{ row }">
            <el-button
              link
              type="primary"
              :disabled="row.tenantId === 0"
              @click="openPolicy(row)"
            >
              编辑
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-table
        v-else-if="activeTab === 'cache'"
        v-loading="loading"
        :data="caches"
        stripe
      >
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column
          prop="cacheKey"
          label="缓存键"
          min-width="220"
          show-overflow-tooltip
        />
        <el-table-column prop="toolchainVersion" label="工具链" min-width="150" />
        <el-table-column prop="hitCount" label="命中" width="80" />
        <el-table-column
          label="大小"
          width="110"
        >
          <template #default="{ row }">
            {{ formatBytes(row.sizeBytes) }}
          </template>
        </el-table-column>
        <el-table-column
          label="到期时间"
          min-width="180"
        >
          <template #default="{ row }">
            {{ formatTime(row.expiresAt) }}
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="80" />
        <el-table-column
          label="操作"
          width="90"
        >
          <template #default="{ row }">
            <el-button
              link
              type="danger"
              :disabled="row.status !== 1"
              @click="invalidateCache(row)"
            >
              失效
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-table
        v-else
        v-loading="loading"
        :data="events"
        stripe
      >
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="taskId" label="任务 ID" width="100" />
        <el-table-column prop="nodeCode" label="节点" min-width="140" />
        <el-table-column prop="eventType" label="类型" width="80" />
        <el-table-column prop="reasonCode" label="原因" min-width="180" />
        <el-table-column
          prop="decisionJson"
          label="调度决策"
          min-width="260"
          show-overflow-tooltip
        />
        <el-table-column
          label="时间"
          min-width="180"
        >
          <template #default="{ row }">
            {{ formatTime(row.createTime) }}
          </template>
        </el-table-column>
      </el-table>

      <CursorPagination
        v-model:limit="currentPage().pagination.limit"
        :total="currentPage().pagination.total"
        :has-prev="currentPage().pagination.hasPrev"
        :has-next="currentPage().pagination.hasNext"
        @prev="currentPage().prevAndLoad(loadData)"
        @next="currentPage().nextAndLoad(loadData)"
        @limit-change="currentPage().resetAndLoad(loadData)"
      />
    </el-card>

    <el-dialog v-model="policyVisible" title="构建并发策略" width="520px">
      <el-form :model="policyForm" label-width="130px">
        <el-form-item label="应用 ID">
          <el-input-number
            v-model="policyForm.appId"
            :min="0"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="构建池">
          <el-input v-model="policyForm.poolCode" />
        </el-form-item>
        <el-form-item label="最大并发">
          <el-input-number
            v-model="policyForm.maxConcurrency"
            :min="1"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="公平权重">
          <el-input-number
            v-model="policyForm.fairWeight"
            :min="1"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="最高优先级">
          <el-input-number
            v-model="policyForm.maxPriority"
            :min="0"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="policyForm.status"
            style="width: 100%"
          >
            <el-option :value="1" label="启用" /><el-option :value="2" label="停用" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="policyVisible = false">
          取消
        </el-button><el-button
          type="primary"
          :loading="policySaving"
          @click="savePolicy"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.metrics-grid {
  display: grid;
  flex: 0 0 auto;
  grid-template-columns: repeat(6, minmax(120px, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}
.metric-item {
  padding: 10px 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  background: var(--el-fill-color-light);
}
.metric-item span,
.metric-item strong {
  display: block;
}
.metric-item span {
  margin-bottom: 6px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.metric-item strong {
  color: var(--el-text-color-primary);
  font-size: 16px;
}
.cluster-alerts {
  display: grid;
  flex: 0 0 auto;
  grid-template-columns: repeat(2, minmax(240px, 1fr));
  gap: 8px;
  margin-bottom: 12px;
}
.cluster-page :deep(.el-tabs__header) {
  flex: 0 0 auto;
  margin-bottom: 12px;
}
.cluster-page :deep(.el-table) {
  flex: 1;
  min-height: 0;
}
@media (max-width: 1400px) {
  .metrics-grid {
    grid-template-columns: repeat(3, minmax(140px, 1fr));
  }
}
</style>
