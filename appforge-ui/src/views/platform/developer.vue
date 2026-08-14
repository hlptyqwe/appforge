<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRoute } from 'vue-router'
import CursorPagination from '@/components/common/CursorPagination.vue'
import { usePagination } from '@/composables/usePagination'
import {
  platformService,
  type PlatformOpenApiCredential,
  type PlatformSourceIntegration,
  type PlatformSourceAvailableRepository,
  type PlatformSourceBuildTrigger,
  type PlatformSourceRepository,
  type PlatformSourceWebhookEvent,
  type PlatformWebhookDelivery,
  type PlatformWebhookEndpoint,
} from '@/services'

const scopeOptions = [
  [1, 'apps:read'],
  [2, 'apps:write'],
  [3, 'versions:read'],
  [4, 'versions:write'],
  [5, 'channels:read'],
  [6, 'channels:write'],
  [7, 'branding:read'],
  [8, 'branding:write'],
  [9, 'builds:read'],
  [10, 'builds:write'],
  [11, 'artifacts:read'],
  [12, 'stats:read'],
] as const
const scopeLabels = Object.fromEntries(scopeOptions) as Record<number, string>
const webhookEventOptions = [
  'build.queued',
  'build.started',
  'build.succeeded',
  'build.failed',
  'build.canceled',
  'artifact.expiring',
  'quota.warning',
  'quota.exceeded',
]

const route = useRoute()
const initialTab =
  route.query.tab === 'source' || route.query.source_connected ? 'source' : 'credentials'
const activeTab = ref(initialTab)
const loading = ref(false)
const saving = ref(false)
const rows = ref<PlatformOpenApiCredential[]>([])
const pager = usePagination<number>(20)
const query = reactive({ keyword: '', status: 0 })
const formVisible = ref(false)
const secretVisible = ref(false)
const oneTimeApiKey = ref('')
const form = reactive({
  credentialName: '',
  scopes: [1, 3, 5, 7, 9, 11] as number[],
  appIds: '',
  ipAllowlist: '',
  rateLimitPerMinute: 60,
  expiresInDays: 90,
})
const webhookLoading = ref(false)
const webhookSaving = ref(false)
const webhookRows = ref<PlatformWebhookEndpoint[]>([])
const webhookPager = usePagination<number>(20)
const webhookQuery = reactive({ keyword: '', status: 0 })
const webhookFormVisible = ref(false)
const webhookSecretVisible = ref(false)
const oneTimeWebhookSecret = ref('')
const webhookForm = reactive({
  id: 0,
  endpointName: '',
  endpointUrl: '',
  eventTypes: ['build.succeeded', 'build.failed'] as string[],
  maxAttempts: 8,
  status: 1,
})
const deliveryLoading = ref(false)
const deliveryRows = ref<PlatformWebhookDelivery[]>([])
const deliveryPager = usePagination<number>(20)
const deliveryQuery = reactive({ endpointId: 0, status: 0, eventType: '' })
const sourceLoading = ref(false)
const sourceRows = ref<PlatformSourceIntegration[]>([])
const sourcePager = usePagination<number>(20)
const sourceQuery = reactive({ platform: 0, status: 0, keyword: '' })
const repositoryLoading = ref(false)
const repositoryRows = ref<PlatformSourceRepository[]>([])
const repositoryPager = usePagination<number>(20)
const repositoryQuery = reactive({ integrationId: 0, status: 0, keyword: '' })
const availableRepositoryVisible = ref(false)
const availableRepositoryLoading = ref(false)
const availableRepositories = ref<PlatformSourceAvailableRepository[]>([])
const selectedSourceIntegration = ref<PlatformSourceIntegration | null>(null)
const artifactImportVisible = ref(false)
const artifactImportSaving = ref(false)
const artifactImportForm = reactive({
  appId: 0,
  repositoryId: 0,
  artifactSource: 2,
  externalArtifactId: '',
  releaseRef: '',
  versionCode: 0,
  versionName: '',
  releaseNotes: '',
})
const sourceTriggerLoading = ref(false)
const sourceTriggerSaving = ref(false)
const sourceTriggerRows = ref<PlatformSourceBuildTrigger[]>([])
const sourceTriggerPager = usePagination<number>(20)
const sourceTriggerQuery = reactive({ repositoryId: 0, appId: 0, status: 0, keyword: '' })
const sourceTriggerFormVisible = ref(false)
const sourceTriggerSecretVisible = ref(false)
const sourceTriggerWebhookUrl = ref('')
const sourceTriggerSigningSecret = ref('')
const sourceTriggerForm = reactive({
  id: 0,
  repositoryId: 0,
  appId: 0,
  triggerName: '',
  eventType: 1,
  refPattern: '*',
  artifactSelector: '',
  channelIds: '',
  signingConfigId: 0,
  brandingProfileId: 0,
  whiteLabelProductId: 0,
  priority: 2,
  poolCode: 'default',
  versionNamePrefix: '',
  status: 1,
})
const sourceEventLoading = ref(false)
const sourceEventRows = ref<PlatformSourceWebhookEvent[]>([])
const sourceEventPager = usePagination<number>(20)
const sourceEventQuery = reactive({ triggerId: 0, status: 0 })

const selectedScopeText = computed(() => form.scopes.map((value) => scopeLabels[value]).join(', '))

async function loadData() {
  loading.value = true
  try {
    const response = await platformService.listOpenApiCredentials({
      cursor: pager.pagination.cursor,
      limit: pager.pagination.limit,
      keyword: query.keyword || undefined,
      status: query.status || undefined,
    })
    if (response.code !== 200 || !response.data) throw new Error(response.msg)
    rows.value = response.data || []
    pager.updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载 API 凭证失败')
  } finally {
    loading.value = false
  }
}

async function loadWebhooks() {
  webhookLoading.value = true
  try {
    const response = await platformService.listWebhookEndpoints({
      cursor: webhookPager.pagination.cursor,
      limit: webhookPager.pagination.limit,
      keyword: webhookQuery.keyword || undefined,
      status: webhookQuery.status || undefined,
    })
    if (response.code !== 200) throw new Error(response.msg)
    webhookRows.value = response.data || []
    webhookPager.updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载 Webhook 失败')
  } finally {
    webhookLoading.value = false
  }
}

async function loadDeliveries() {
  deliveryLoading.value = true
  try {
    const response = await platformService.listWebhookDeliveries({
      cursor: deliveryPager.pagination.cursor,
      limit: deliveryPager.pagination.limit,
      endpointId: deliveryQuery.endpointId || undefined,
      status: deliveryQuery.status || undefined,
      eventType: deliveryQuery.eventType || undefined,
    })
    if (response.code !== 200) throw new Error(response.msg)
    deliveryRows.value = response.data || []
    deliveryPager.updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载 Webhook 投递失败')
  } finally {
    deliveryLoading.value = false
  }
}

async function loadSourceIntegrations() {
  sourceLoading.value = true
  try {
    const response = await platformService.listSourceIntegrations({
      cursor: sourcePager.pagination.cursor,
      limit: sourcePager.pagination.limit,
      platform: sourceQuery.platform || undefined,
      status: sourceQuery.status || undefined,
      keyword: sourceQuery.keyword || undefined,
    })
    if (response.code !== 200) throw new Error(response.msg)
    sourceRows.value = response.data || []
    sourcePager.updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载代码平台集成失败')
  } finally {
    sourceLoading.value = false
  }
}

async function loadSourceRepositories() {
  repositoryLoading.value = true
  try {
    const response = await platformService.listSourceRepositories({
      cursor: repositoryPager.pagination.cursor,
      limit: repositoryPager.pagination.limit,
      integrationId: repositoryQuery.integrationId || undefined,
      status: repositoryQuery.status || undefined,
      keyword: repositoryQuery.keyword || undefined,
    })
    if (response.code !== 200) throw new Error(response.msg)
    repositoryRows.value = response.data || []
    repositoryPager.updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载授权仓库失败')
  } finally {
    repositoryLoading.value = false
  }
}

async function loadSourceBuildTriggers() {
  sourceTriggerLoading.value = true
  try {
    const response = await platformService.listSourceBuildTriggers({
      cursor: sourceTriggerPager.pagination.cursor,
      limit: sourceTriggerPager.pagination.limit,
      repositoryId: sourceTriggerQuery.repositoryId || undefined,
      appId: sourceTriggerQuery.appId || undefined,
      status: sourceTriggerQuery.status || undefined,
      keyword: sourceTriggerQuery.keyword || undefined,
    })
    if (response.code !== 200) throw new Error(response.msg)
    sourceTriggerRows.value = response.data || []
    sourceTriggerPager.updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载源码构建触发策略失败')
  } finally {
    sourceTriggerLoading.value = false
  }
}

async function loadSourceWebhookEvents() {
  sourceEventLoading.value = true
  try {
    const response = await platformService.listSourceWebhookEvents({
      cursor: sourceEventPager.pagination.cursor,
      limit: sourceEventPager.pagination.limit,
      triggerId: sourceEventQuery.triggerId || undefined,
      status: sourceEventQuery.status || undefined,
    })
    if (response.code !== 200) throw new Error(response.msg)
    sourceEventRows.value = response.data || []
    sourceEventPager.updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载源码 Webhook 事件失败')
  } finally {
    sourceEventLoading.value = false
  }
}

function switchTab(value: string | number) {
  if (String(value) === 'webhooks') {
    void Promise.all([loadWebhooks(), loadDeliveries()])
  } else if (String(value) === 'source') {
    void Promise.all([
      loadSourceIntegrations(),
      loadSourceRepositories(),
      loadSourceBuildTriggers(),
      loadSourceWebhookEvents(),
    ])
  }
}

async function disconnectSource(item: PlatformSourceIntegration) {
  await ElMessageBox.confirm(
    `确认断开 ${item.integrationName}？现有令牌会立即失效。`,
    '断开代码平台',
  )
  const response = await platformService.disconnectSourceIntegration(item.id)
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success('代码平台集成已断开')
  await loadSourceIntegrations()
}

async function connectSource(platform: 1 | 2) {
  try {
    const response = await platformService.createSourceOAuthAuthorization(platform)
    if (response.code !== 200 || !response.data?.authorizationUrl) throw new Error(response.msg)
    window.location.assign(response.data.authorizationUrl)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '代码平台 OAuth 尚未配置')
  }
}

async function revokeRepository(item: PlatformSourceRepository) {
  await ElMessageBox.confirm(`确认撤销仓库 ${item.repositoryFullName}？`, '撤销仓库授权')
  const response = await platformService.revokeSourceRepository(item.id)
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success('仓库授权已撤销')
  await loadSourceRepositories()
}

async function openAvailableRepositories(item: PlatformSourceIntegration) {
  selectedSourceIntegration.value = item
  availableRepositoryVisible.value = true
  availableRepositoryLoading.value = true
  try {
    const response = await platformService.listSourceAvailableRepositories(item.id)
    if (response.code !== 200) throw new Error(response.msg)
    availableRepositories.value = response.data || []
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '读取供应商仓库失败')
  } finally {
    availableRepositoryLoading.value = false
  }
}

async function authorizeRepository(item: PlatformSourceAvailableRepository) {
  if (!selectedSourceIntegration.value) return
  const response = await platformService.authorizeSourceRepository(
    selectedSourceIntegration.value.id,
    item.externalRepositoryId,
  )
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success(`已授权 ${item.repositoryFullName}`)
  await repositoryPager.resetAndLoad(loadSourceRepositories)
}

function openArtifactImport(item?: PlatformSourceRepository) {
  Object.assign(artifactImportForm, {
    appId: 0,
    repositoryId: item?.id || 0,
    artifactSource: 2,
    externalArtifactId: '',
    releaseRef: '',
    versionCode: 0,
    versionName: '',
    releaseNotes: '',
  })
  artifactImportVisible.value = true
}

async function importSourceArtifact() {
  artifactImportSaving.value = true
  try {
    const response = await platformService.importSourceArtifact({ ...artifactImportForm })
    if (response.code !== 200 || !response.data) throw new Error(response.msg)
    artifactImportVisible.value = false
    ElMessage.success(
      `版本 ${response.data.version.id} 已从 commit ${response.data.artifact.commitSha.slice(0, 12)} 导入`,
    )
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '导入 APK Artifact 失败')
  } finally {
    artifactImportSaving.value = false
  }
}

function openSourceTriggerForm(item?: PlatformSourceBuildTrigger, repository?: PlatformSourceRepository) {
  Object.assign(sourceTriggerForm, {
    id: item?.id || 0,
    repositoryId: item?.repositoryId || repository?.id || 0,
    appId: item?.appId || 0,
    triggerName: item?.triggerName || '',
    eventType: item?.eventType || 1,
    refPattern: item?.refPattern || '*',
    artifactSelector: item?.artifactSelector || '',
    channelIds: item?.channelIds?.join(',') || '',
    signingConfigId: item?.signingConfigId || 0,
    brandingProfileId: item?.brandingProfileId || 0,
    whiteLabelProductId: item?.whiteLabelProductId || 0,
    priority: item?.priority ?? 2,
    poolCode: item?.poolCode || 'default',
    versionNamePrefix: item?.versionNamePrefix || '',
    status: item?.status || 1,
  })
  sourceTriggerFormVisible.value = true
}

function sourceTriggerPayload() {
  const channelIds = sourceTriggerForm.channelIds
    .split(',')
    .map((value) => Number(value.trim()))
    .filter((value) => Number.isInteger(value) && value > 0)
  return { ...sourceTriggerForm, channelIds }
}

async function saveSourceTrigger() {
  sourceTriggerSaving.value = true
  try {
    const payload = sourceTriggerPayload()
    if (!payload.channelIds.length) throw new Error('至少填写一个渠道 ID')
    if (sourceTriggerForm.id) {
      const response = await platformService.updateSourceBuildTrigger(
        sourceTriggerForm.id,
        payload,
      )
      if (response.code !== 200) throw new Error(response.msg)
      ElMessage.success('源码构建触发策略已更新')
    } else {
      const response = await platformService.createSourceBuildTrigger(payload)
      if (response.code !== 200 || !response.data) throw new Error(response.msg)
      sourceTriggerWebhookUrl.value = response.data.webhookUrl
      sourceTriggerSigningSecret.value = response.data.signingSecret
      sourceTriggerSecretVisible.value = true
      ElMessage.success('源码构建触发策略已创建')
    }
    sourceTriggerFormVisible.value = false
    await sourceTriggerPager.resetAndLoad(loadSourceBuildTriggers)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存源码构建触发策略失败')
  } finally {
    sourceTriggerSaving.value = false
  }
}

async function rotateSourceTriggerSecret(item: PlatformSourceBuildTrigger) {
  await ElMessageBox.confirm('轮换后旧回调地址和签名 Secret 会立即失效，是否继续？', '轮换密钥')
  const response = await platformService.rotateSourceBuildTriggerSecret(item.id)
  if (response.code !== 200 || !response.data) throw new Error(response.msg)
  sourceTriggerWebhookUrl.value = response.data.webhookUrl
  sourceTriggerSigningSecret.value = response.data.signingSecret
  sourceTriggerSecretVisible.value = true
}

async function copySourceTriggerSecret() {
  await navigator.clipboard.writeText(
    `${sourceTriggerWebhookUrl.value}\n${sourceTriggerSigningSecret.value}`,
  )
  ElMessage.success('回调地址和签名 Secret 已复制')
}

function sourceTriggerEventLabel(value: number) {
  return value === 1 ? 'Release 发布' : value === 2 ? 'CI 成功' : '未知'
}

function sourceWebhookStatusLabel(value: number) {
  return (
    { 1: '待处理', 2: '处理中', 3: '成功', 4: '已忽略', 5: '失败' }[value] || '未知'
  )
}

function sourcePlatformLabel(platform: number) {
  return platform === 1 ? 'GitHub' : platform === 2 ? 'GitLab' : '未知'
}

function sourceStatusLabel(status: number) {
  return status === 1 ? '已连接' : status === 2 ? '已断开' : status === 3 ? '异常' : '未知'
}

function resetAndLoad() {
  pager.resetAndLoad(loadData)
}

function openCreate() {
  Object.assign(form, {
    credentialName: '',
    scopes: [1, 3, 5, 7, 9, 11],
    appIds: '',
    ipAllowlist: '',
    rateLimitPerMinute: 60,
    expiresInDays: 90,
  })
  formVisible.value = true
}

function openWebhookForm(item?: PlatformWebhookEndpoint) {
  Object.assign(webhookForm, {
    id: item?.id || 0,
    endpointName: item?.endpointName || '',
    endpointUrl: item?.endpointUrl || '',
    eventTypes: item?.eventTypes?.length
      ? [...item.eventTypes]
      : ['build.succeeded', 'build.failed'],
    maxAttempts: item?.maxAttempts || 8,
    status: item?.status || 1,
  })
  webhookFormVisible.value = true
}

async function saveWebhook() {
  webhookSaving.value = true
  try {
    const payload = {
      endpointName: webhookForm.endpointName,
      endpointUrl: webhookForm.endpointUrl,
      eventTypes: webhookForm.eventTypes,
      maxAttempts: webhookForm.maxAttempts,
      status: webhookForm.status,
    }
    if (webhookForm.id) {
      const response = await platformService.updateWebhookEndpoint(webhookForm.id, payload)
      if (response.code !== 200) throw new Error(response.msg)
      ElMessage.success('Webhook 已更新')
    } else {
      const response = await platformService.createWebhookEndpoint(payload)
      if (response.code !== 200 || !response.data?.signingSecret) throw new Error(response.msg)
      oneTimeWebhookSecret.value = response.data.signingSecret
      webhookSecretVisible.value = true
    }
    webhookFormVisible.value = false
    await webhookPager.resetAndLoad(loadWebhooks)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存 Webhook 失败')
  } finally {
    webhookSaving.value = false
  }
}

async function rotateWebhookSecret(item: PlatformWebhookEndpoint) {
  await ElMessageBox.confirm('旧签名 Secret 会立即失效，确认轮换？', '轮换 Webhook Secret')
  const response = await platformService.rotateWebhookEndpointSecret(item.id)
  if (response.code !== 200 || !response.data?.signingSecret) throw new Error(response.msg)
  oneTimeWebhookSecret.value = response.data.signingSecret
  webhookSecretVisible.value = true
  await loadWebhooks()
}

async function testWebhook(item: PlatformWebhookEndpoint) {
  const response = await platformService.testWebhookEndpoint(item.id)
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success('测试事件已进入投递队列')
  window.setTimeout(() => void deliveryPager.resetAndLoad(loadDeliveries), 1200)
}

async function replayDelivery(item: PlatformWebhookDelivery) {
  const response = await platformService.replayWebhookDelivery(item.id)
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success('投递已重新进入队列')
  await loadDeliveries()
}

async function copyWebhookSecret() {
  await navigator.clipboard.writeText(oneTimeWebhookSecret.value)
  ElMessage.success('已复制；请保存到接收端的安全密钥存储')
}

function parseIds(value: string) {
  return value
    .split(',')
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isInteger(item) && item > 0)
}

function parseLines(value: string) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

async function createCredential() {
  saving.value = true
  try {
    const response = await platformService.createOpenApiCredential({
      credentialName: form.credentialName,
      scopes: form.scopes,
      appIds: parseIds(form.appIds),
      ipAllowlist: parseLines(form.ipAllowlist),
      rateLimitPerMinute: form.rateLimitPerMinute,
      expiresAt: form.expiresInDays ? Date.now() + form.expiresInDays * 24 * 60 * 60 * 1000 : 0,
    })
    if (response.code !== 200 || !response.data?.apiKey) throw new Error(response.msg)
    formVisible.value = false
    oneTimeApiKey.value = response.data.apiKey
    secretVisible.value = true
    await resetAndLoad()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '创建 API 凭证失败')
  } finally {
    saving.value = false
  }
}

async function rotateCredential(item: PlatformOpenApiCredential) {
  await ElMessageBox.confirm(
    '新 Key 会立即生效，旧 Key 保留 24 小时过渡期。继续轮换？',
    '轮换 API Key',
  )
  const response = await platformService.rotateOpenApiCredential(item.id, 24 * 60 * 60)
  if (response.code !== 200 || !response.data?.apiKey) throw new Error(response.msg)
  oneTimeApiKey.value = response.data.apiKey
  secretVisible.value = true
  await loadData()
}

async function revokeCredential(item: PlatformOpenApiCredential) {
  await ElMessageBox.confirm(
    `确认立即吊销 ${item.credentialName}？调用会立刻失效。`,
    '吊销 API Key',
  )
  const response = await platformService.revokeOpenApiCredential(item.id)
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success('API Key 已吊销')
  await loadData()
}

async function copySecret() {
  await navigator.clipboard.writeText(oneTimeApiKey.value)
  ElMessage.success('已复制；请保存到安全的密钥存储')
}

function statusLabel(status: number) {
  return status === 1 ? '启用' : status === 2 ? '轮换过渡' : status === 3 ? '已吊销' : '已过期'
}

function webhookStatusLabel(status: number) {
  return status === 1 ? '启用' : status === 2 ? '暂停' : '已吊销'
}

function deliveryStatusLabel(status: number) {
  return ['未知', '待投递', '投递中', '成功', '等待重试', '死信'][status] || '未知'
}

function formatTime(value?: number) {
  return value ? new Date(value).toLocaleString() : '-'
}

onMounted(() => {
  if (activeTab.value === 'source') {
    void Promise.all([
      loadSourceIntegrations(),
      loadSourceRepositories(),
      loadSourceBuildTriggers(),
      loadSourceWebhookEvents(),
    ])
    if (route.query.source_connected === '1') ElMessage.success('代码平台连接成功')
    if (route.query.source_error) ElMessage.error('代码平台连接失败，请重试或检查 OAuth 配置')
    return
  }
  void loadData()
})
</script>

<template>
  <div class="module-page">
    <el-tabs v-model="activeTab" class="developer-tabs" @tab-change="switchTab">
      <el-tab-pane label="API Key" name="credentials" />
      <el-tab-pane label="Webhook" name="webhooks" />
      <el-tab-pane label="GitHub / GitLab" name="source" />
      <el-tab-pane label="文档与 CI/CD" name="docs" />
    </el-tabs>

    <div v-if="activeTab === 'credentials'">
      <el-card shadow="never" class="query-card">
        <el-form inline>
          <el-form-item label="关键词"><el-input v-model="query.keyword" clearable /></el-form-item>
          <el-form-item label="状态">
            <el-select v-model="query.status" style="width: 130px">
              <el-option :value="0" label="全部" />
              <el-option :value="1" label="启用" />
              <el-option :value="2" label="轮换过渡" />
              <el-option :value="3" label="已吊销" />
              <el-option :value="4" label="已过期" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="resetAndLoad">查询</el-button>
            <el-button type="primary" plain @click="openCreate">创建 API Key</el-button>
          </el-form-item>
        </el-form>
      </el-card>

      <el-card shadow="never" class="table-card">
        <el-table v-loading="loading" :data="rows" stripe>
          <el-table-column prop="credentialName" label="名称" min-width="150" />
          <el-table-column prop="keyId" label="Key ID" min-width="145" />
          <el-table-column label="Scopes" min-width="260" show-overflow-tooltip>
            <template #default="{ row }">{{
              row.scopes.map((item: number) => scopeLabels[item]).join(', ')
            }}</template>
          </el-table-column>
          <el-table-column label="应用范围" min-width="130">
            <template #default="{ row }">{{
              row.appIds.length ? row.appIds.join(', ') : '全部应用'
            }}</template>
          </el-table-column>
          <el-table-column label="状态" width="110">
            <template #default="{ row }"
              ><el-tag>{{ statusLabel(row.status) }}</el-tag></template
            >
          </el-table-column>
          <el-table-column label="最近使用" min-width="175">
            <template #default="{ row }">{{ formatTime(row.lastUsedAt) }}</template>
          </el-table-column>
          <el-table-column label="到期时间" min-width="175">
            <template #default="{ row }">{{ formatTime(row.expiresAt) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="150" fixed="right">
            <template #default="{ row }">
              <el-button
                link
                type="primary"
                :disabled="row.status !== 1"
                @click="rotateCredential(row)"
                >轮换</el-button
              >
              <el-button
                link
                type="danger"
                :disabled="row.status === 3"
                @click="revokeCredential(row)"
                >吊销</el-button
              >
            </template>
          </el-table-column>
        </el-table>
        <CursorPagination
          v-model:limit="pager.pagination.limit"
          :total="pager.pagination.total"
          :has-prev="pager.pagination.hasPrev"
          :has-next="pager.pagination.hasNext"
          @prev="pager.prevAndLoad(loadData)"
          @next="pager.nextAndLoad(loadData)"
          @limit-change="pager.resetAndLoad(loadData)"
        />
      </el-card>
    </div>

    <div v-else-if="activeTab === 'webhooks'">
      <el-card shadow="never" class="query-card">
        <el-form inline>
          <el-form-item label="关键词">
            <el-input v-model="webhookQuery.keyword" clearable />
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="webhookQuery.status" style="width: 130px">
              <el-option :value="0" label="全部" />
              <el-option :value="1" label="启用" />
              <el-option :value="2" label="暂停" />
              <el-option :value="3" label="已吊销" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="webhookPager.resetAndLoad(loadWebhooks)"
              >查询</el-button
            >
            <el-button type="primary" plain @click="openWebhookForm()">新增 Webhook</el-button>
          </el-form-item>
        </el-form>
      </el-card>

      <el-card shadow="never" class="table-card webhook-card">
        <el-table v-loading="webhookLoading" :data="webhookRows" stripe>
          <el-table-column prop="endpointName" label="名称" min-width="150" />
          <el-table-column
            prop="endpointUrl"
            label="HTTPS URL"
            min-width="300"
            show-overflow-tooltip
          />
          <el-table-column label="事件" min-width="260" show-overflow-tooltip>
            <template #default="{ row }">{{ row.eventTypes.join(', ') }}</template>
          </el-table-column>
          <el-table-column label="密钥提示" width="110">
            <template #default="{ row }">***{{ row.secretHint }}</template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }"
              ><el-tag>{{ webhookStatusLabel(row.status) }}</el-tag></template
            >
          </el-table-column>
          <el-table-column label="最近成功" min-width="170">
            <template #default="{ row }">{{ formatTime(row.lastSuccessAt) }}</template>
          </el-table-column>
          <el-table-column label="最近失败" min-width="170">
            <template #default="{ row }">{{ formatTime(row.lastFailureAt) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="230" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="openWebhookForm(row)">编辑</el-button>
              <el-button link type="primary" :disabled="row.status !== 1" @click="testWebhook(row)"
                >测试</el-button
              >
              <el-button
                link
                type="warning"
                :disabled="row.status === 3"
                @click="rotateWebhookSecret(row)"
                >轮换密钥</el-button
              >
            </template>
          </el-table-column>
        </el-table>
        <CursorPagination
          v-model:limit="webhookPager.pagination.limit"
          :total="webhookPager.pagination.total"
          :has-prev="webhookPager.pagination.hasPrev"
          :has-next="webhookPager.pagination.hasNext"
          @prev="webhookPager.prevAndLoad(loadWebhooks)"
          @next="webhookPager.nextAndLoad(loadWebhooks)"
          @limit-change="webhookPager.resetAndLoad(loadWebhooks)"
        />
      </el-card>

      <el-card shadow="never" class="query-card delivery-query">
        <el-form inline>
          <el-form-item label="端点 ID">
            <el-input-number v-model="deliveryQuery.endpointId" :min="0" :controls="false" />
          </el-form-item>
          <el-form-item label="事件">
            <el-select v-model="deliveryQuery.eventType" clearable style="width: 180px">
              <el-option
                v-for="event in webhookEventOptions"
                :key="event"
                :label="event"
                :value="event"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="deliveryQuery.status" style="width: 130px">
              <el-option :value="0" label="全部" />
              <el-option :value="1" label="待投递" />
              <el-option :value="2" label="投递中" />
              <el-option :value="3" label="成功" />
              <el-option :value="4" label="等待重试" />
              <el-option :value="5" label="死信" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="deliveryPager.resetAndLoad(loadDeliveries)"
              >查询投递</el-button
            >
          </el-form-item>
        </el-form>
      </el-card>

      <el-card shadow="never" class="table-card">
        <el-table v-loading="deliveryLoading" :data="deliveryRows" stripe>
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column prop="endpointId" label="端点 ID" width="95" />
          <el-table-column prop="eventId" label="Event ID" min-width="250" show-overflow-tooltip />
          <el-table-column prop="eventType" label="事件" min-width="150" />
          <el-table-column prop="attempt" label="尝试" width="75" />
          <el-table-column label="状态" width="105">
            <template #default="{ row }"
              ><el-tag>{{ deliveryStatusLabel(row.status) }}</el-tag></template
            >
          </el-table-column>
          <el-table-column prop="responseStatus" label="HTTP" width="80" />
          <el-table-column label="结果" min-width="240" show-overflow-tooltip>
            <template #default="{ row }">{{
              row.errorMessage || row.responseBodyExcerpt || '-'
            }}</template>
          </el-table-column>
          <el-table-column label="更新时间" min-width="175">
            <template #default="{ row }">{{ formatTime(row.updateTime) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="90" fixed="right">
            <template #default="{ row }">
              <el-button
                link
                type="primary"
                :disabled="![3, 4, 5].includes(row.status)"
                @click="replayDelivery(row)"
                >重放</el-button
              >
            </template>
          </el-table-column>
        </el-table>
        <CursorPagination
          v-model:limit="deliveryPager.pagination.limit"
          :total="deliveryPager.pagination.total"
          :has-prev="deliveryPager.pagination.hasPrev"
          :has-next="deliveryPager.pagination.hasNext"
          @prev="deliveryPager.prevAndLoad(loadDeliveries)"
          @next="deliveryPager.nextAndLoad(loadDeliveries)"
          @limit-change="deliveryPager.resetAndLoad(loadDeliveries)"
        />
      </el-card>
    </div>

    <div v-else-if="activeTab === 'source'">
      <el-alert
        title="连接必须通过受信任 OAuth/App 回调完成；页面和 API 均不会返回已保存的访问令牌。"
        type="info"
        :closable="false"
        show-icon
        class="source-alert"
      />
      <el-card shadow="never" class="query-card">
        <el-form inline>
          <el-form-item label="平台">
            <el-select v-model="sourceQuery.platform" style="width: 130px">
              <el-option :value="0" label="全部" />
              <el-option :value="1" label="GitHub" />
              <el-option :value="2" label="GitLab" />
            </el-select>
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="sourceQuery.status" style="width: 130px">
              <el-option :value="0" label="全部" />
              <el-option :value="1" label="已连接" />
              <el-option :value="2" label="已断开" />
              <el-option :value="3" label="异常" />
            </el-select>
          </el-form-item>
          <el-form-item label="关键词"
            ><el-input v-model="sourceQuery.keyword" clearable
          /></el-form-item>
          <el-form-item>
            <el-button type="primary" @click="sourcePager.resetAndLoad(loadSourceIntegrations)"
              >查询</el-button
            >
            <el-button type="primary" plain @click="connectSource(1)">连接 GitHub</el-button>
            <el-button type="primary" plain @click="connectSource(2)">连接 GitLab</el-button>
          </el-form-item>
        </el-form>
      </el-card>
      <el-card shadow="never" class="table-card">
        <el-table v-loading="sourceLoading" :data="sourceRows" stripe>
          <el-table-column label="平台" width="100"
            ><template #default="{ row }">{{
              sourcePlatformLabel(row.platform)
            }}</template></el-table-column
          >
          <el-table-column prop="integrationName" label="名称" min-width="160" />
          <el-table-column
            prop="installationRef"
            label="安装标识"
            min-width="220"
            show-overflow-tooltip
          />
          <el-table-column label="状态" width="110"
            ><template #default="{ row }"
              ><el-tag>{{ sourceStatusLabel(row.status) }}</el-tag></template
            ></el-table-column
          >
          <el-table-column label="令牌到期" min-width="175"
            ><template #default="{ row }">{{
              formatTime(row.tokenExpiresAt)
            }}</template></el-table-column
          >
          <el-table-column label="最近同步" min-width="175"
            ><template #default="{ row }">{{
              formatTime(row.lastSyncAt)
            }}</template></el-table-column
          >
          <el-table-column label="操作" width="190" fixed="right"
            ><template #default="{ row }"
              ><el-button
                link
                type="primary"
                :disabled="row.status !== 1"
                @click="openAvailableRepositories(row)"
                >授权仓库</el-button
              >
              ><el-button
                link
                type="danger"
                :disabled="row.status !== 1"
                @click="disconnectSource(row)"
                >断开</el-button
              ></template
            ></el-table-column
          >
        </el-table>
        <CursorPagination
          v-model:limit="sourcePager.pagination.limit"
          :total="sourcePager.pagination.total"
          :has-prev="sourcePager.pagination.hasPrev"
          :has-next="sourcePager.pagination.hasNext"
          @prev="sourcePager.prevAndLoad(loadSourceIntegrations)"
          @next="sourcePager.nextAndLoad(loadSourceIntegrations)"
          @limit-change="sourcePager.resetAndLoad(loadSourceIntegrations)"
        />
      </el-card>

      <el-card shadow="never" class="query-card delivery-query">
        <el-form inline>
          <el-form-item label="集成 ID"
            ><el-input-number v-model="repositoryQuery.integrationId" :min="0" :controls="false"
          /></el-form-item>
          <el-form-item label="状态"
            ><el-select v-model="repositoryQuery.status" style="width: 130px"
              ><el-option :value="0" label="全部" /><el-option
                :value="1"
                label="已授权" /><el-option :value="2" label="已撤销" /></el-select
          ></el-form-item>
          <el-form-item label="仓库"
            ><el-input v-model="repositoryQuery.keyword" clearable
          /></el-form-item>
          <el-form-item
            ><el-button type="primary" @click="repositoryPager.resetAndLoad(loadSourceRepositories)"
              >查询仓库</el-button
            ><el-button type="primary" plain @click="openArtifactImport()"
              >导入 APK Artifact</el-button
            ></el-form-item
          >
        </el-form>
      </el-card>
      <el-card shadow="never" class="table-card">
        <el-table v-loading="repositoryLoading" :data="repositoryRows" stripe>
          <el-table-column prop="integrationId" label="集成 ID" width="100" />
          <el-table-column prop="repositoryFullName" label="授权仓库" min-width="260" />
          <el-table-column prop="defaultBranch" label="默认分支" min-width="130" />
          <el-table-column prop="permissionLevel" label="权限" width="90" />
          <el-table-column label="状态" width="100"
            ><template #default="{ row }"
              ><el-tag>{{ row.status === 1 ? '已授权' : '已撤销' }}</el-tag></template
            ></el-table-column
          >
          <el-table-column label="操作" width="230" fixed="right"
            ><template #default="{ row }"
              ><el-button
                link
                type="primary"
                :disabled="row.status !== 1"
                @click="openArtifactImport(row)"
                >导入</el-button
              >
              <el-button
                link
                type="primary"
                :disabled="row.status !== 1"
                @click="openSourceTriggerForm(undefined, row)"
                >触发策略</el-button
              >
              ><el-button
                link
                type="danger"
                :disabled="row.status !== 1"
                @click="revokeRepository(row)"
                >撤销</el-button
              ></template
            ></el-table-column
          >
        </el-table>
        <CursorPagination
          v-model:limit="repositoryPager.pagination.limit"
          :total="repositoryPager.pagination.total"
          :has-prev="repositoryPager.pagination.hasPrev"
          :has-next="repositoryPager.pagination.hasNext"
          @prev="repositoryPager.prevAndLoad(loadSourceRepositories)"
          @next="repositoryPager.nextAndLoad(loadSourceRepositories)"
          @limit-change="repositoryPager.resetAndLoad(loadSourceRepositories)"
        />
      </el-card>

      <el-card shadow="never" class="query-card delivery-query">
        <el-form inline>
          <el-form-item label="仓库 ID"
            ><el-input-number v-model="sourceTriggerQuery.repositoryId" :min="0" :controls="false"
          /></el-form-item>
          <el-form-item label="应用 ID"
            ><el-input-number v-model="sourceTriggerQuery.appId" :min="0" :controls="false"
          /></el-form-item>
          <el-form-item label="状态"
            ><el-select v-model="sourceTriggerQuery.status" style="width: 120px"
              ><el-option :value="0" label="全部" /><el-option :value="1" label="启用" /><el-option
                :value="2"
                label="停用" /></el-select
          ></el-form-item>
          <el-form-item label="策略"
            ><el-input v-model="sourceTriggerQuery.keyword" clearable
          /></el-form-item>
          <el-form-item
            ><el-button
              type="primary"
              @click="sourceTriggerPager.resetAndLoad(loadSourceBuildTriggers)"
              >查询策略</el-button
            ><el-button type="primary" plain @click="openSourceTriggerForm()"
              >新增触发策略</el-button
            ></el-form-item
          >
        </el-form>
      </el-card>
      <el-card shadow="never" class="table-card">
        <template #header>预定义源码构建触发策略</template>
        <el-table v-loading="sourceTriggerLoading" :data="sourceTriggerRows" stripe>
          <el-table-column prop="triggerName" label="策略" min-width="150" />
          <el-table-column prop="repositoryFullName" label="授权仓库" min-width="220" />
          <el-table-column label="事件" width="120"
            ><template #default="{ row }">{{ sourceTriggerEventLabel(row.eventType) }}</template></el-table-column
          >
          <el-table-column prop="refPattern" label="Tag / 分支" min-width="130" />
          <el-table-column prop="artifactSelector" label="Artifact / Job" min-width="170" />
          <el-table-column label="渠道" min-width="130"
            ><template #default="{ row }">{{ row.channelIds.join(', ') }}</template></el-table-column
          >
          <el-table-column label="状态" width="90"
            ><template #default="{ row }"
              ><el-tag :type="row.status === 1 ? 'success' : 'info'">{{
                row.status === 1 ? '启用' : '停用'
              }}</el-tag></template
            ></el-table-column
          >
          <el-table-column label="操作" width="150" fixed="right"
            ><template #default="{ row }"
              ><el-button link type="primary" @click="openSourceTriggerForm(row)">编辑</el-button
              ><el-button link type="warning" @click="rotateSourceTriggerSecret(row)"
                >轮换密钥</el-button
              ></template
            ></el-table-column
          >
        </el-table>
        <CursorPagination
          v-model:limit="sourceTriggerPager.pagination.limit"
          :total="sourceTriggerPager.pagination.total"
          :has-prev="sourceTriggerPager.pagination.hasPrev"
          :has-next="sourceTriggerPager.pagination.hasNext"
          @prev="sourceTriggerPager.prevAndLoad(loadSourceBuildTriggers)"
          @next="sourceTriggerPager.nextAndLoad(loadSourceBuildTriggers)"
          @limit-change="sourceTriggerPager.resetAndLoad(loadSourceBuildTriggers)"
        />
      </el-card>

      <el-card shadow="never" class="query-card delivery-query">
        <el-form inline>
          <el-form-item label="策略 ID"
            ><el-input-number v-model="sourceEventQuery.triggerId" :min="0" :controls="false"
          /></el-form-item>
          <el-form-item label="处理状态"
            ><el-select v-model="sourceEventQuery.status" style="width: 130px"
              ><el-option :value="0" label="全部" /><el-option :value="1" label="待处理" /><el-option
                :value="2"
                label="处理中" /><el-option :value="3" label="成功" /><el-option
                :value="5"
                label="失败" /></el-select
          ></el-form-item>
          <el-form-item
            ><el-button type="primary" @click="sourceEventPager.resetAndLoad(loadSourceWebhookEvents)"
              >查询事件</el-button
            ></el-form-item
          >
        </el-form>
      </el-card>
      <el-card shadow="never" class="table-card">
        <template #header>源码平台入站 Webhook 事件</template>
        <el-table v-loading="sourceEventLoading" :data="sourceEventRows" stripe>
          <el-table-column prop="id" label="事件 ID" width="100" />
          <el-table-column prop="triggerId" label="策略 ID" width="100" />
          <el-table-column prop="providerEventType" label="供应商事件" min-width="155" />
          <el-table-column prop="sourceRef" label="Tag / 分支" min-width="130" />
          <el-table-column prop="versionName" label="版本" min-width="130" />
          <el-table-column label="Commit" min-width="130"
            ><template #default="{ row }">{{ row.commitSha?.slice(0, 12) || '-' }}</template></el-table-column
          >
          <el-table-column label="状态" width="100"
            ><template #default="{ row }"
              ><el-tag :type="row.status === 3 ? 'success' : row.status === 5 ? 'danger' : 'info'">{{
                sourceWebhookStatusLabel(row.status)
              }}</el-tag></template
            ></el-table-column
          >
          <el-table-column prop="attempt" label="尝试" width="80" />
          <el-table-column prop="errorMessage" label="错误" min-width="220" show-overflow-tooltip />
          <el-table-column label="创建时间" min-width="175"
            ><template #default="{ row }">{{ formatTime(row.createTime) }}</template></el-table-column
          >
        </el-table>
        <CursorPagination
          v-model:limit="sourceEventPager.pagination.limit"
          :total="sourceEventPager.pagination.total"
          :has-prev="sourceEventPager.pagination.hasPrev"
          :has-next="sourceEventPager.pagination.hasNext"
          @prev="sourceEventPager.prevAndLoad(loadSourceWebhookEvents)"
          @next="sourceEventPager.nextAndLoad(loadSourceWebhookEvents)"
          @limit-change="sourceEventPager.resetAndLoad(loadSourceWebhookEvents)"
        />
      </el-card>
    </div>

    <div v-else class="docs-grid">
      <el-card shadow="never"
        ><template #header>Open API v1</template>
        <p>统一入口：<code>/open/v1</code></p>
        <p>契约文件：<code>docs/openapi-v1.yaml</code></p>
        <p>写操作必须携带 <code>Idempotency-Key</code>。</p></el-card
      >
      <el-card shadow="never"
        ><template #header>appforgectl</template>
        <p><code>appforgectl auth configure</code></p>
        <p><code>appforgectl version upload</code></p>
        <p><code>appforgectl build create</code> / <code>build wait</code></p></el-card
      >
      <el-card shadow="never"
        ><template #header>CI/CD 模板</template>
        <p>GitHub：<code>.github/actions/appforge-build</code></p>
        <p>GitLab：<code>docs/ci/gitlab-appforge-build.yml</code></p>
        <p>Shell：<code>docs/ci/appforge-build.sh</code></p></el-card
      >
    </div>

    <el-dialog
      v-model="availableRepositoryVisible"
      :title="`选择 ${selectedSourceIntegration?.integrationName || ''} 可访问的仓库`"
      width="760px"
    >
      <el-alert
        title="只有在这里明确授权的仓库才能用于 Artifact 导入；平台只保存 read 权限。"
        type="warning"
        :closable="false"
        show-icon
      />
      <el-table
        v-loading="availableRepositoryLoading"
        :data="availableRepositories"
        stripe
        class="available-repository-table"
      >
        <el-table-column prop="repositoryFullName" label="仓库" min-width="360" />
        <el-table-column prop="defaultBranch" label="默认分支" min-width="140" />
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button link type="primary" @click="authorizeRepository(row)">授权</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <el-dialog
      v-model="sourceTriggerFormVisible"
      :title="sourceTriggerForm.id ? '编辑源码构建触发策略' : '新增源码构建触发策略'"
      width="760px"
    >
      <el-form :model="sourceTriggerForm" label-width="150px">
        <el-form-item label="授权仓库 ID"
          ><el-input-number
            v-model="sourceTriggerForm.repositoryId"
            :min="1"
            :controls="false"
            :disabled="sourceTriggerForm.id > 0"
        /></el-form-item>
        <el-form-item label="应用 ID"
          ><el-input-number
            v-model="sourceTriggerForm.appId"
            :min="1"
            :controls="false"
            :disabled="sourceTriggerForm.id > 0"
        /></el-form-item>
        <el-form-item label="策略名称"
          ><el-input v-model="sourceTriggerForm.triggerName" maxlength="128"
        /></el-form-item>
        <el-form-item label="供应商事件"
          ><el-radio-group v-model="sourceTriggerForm.eventType"
            ><el-radio-button :value="1">Release 发布</el-radio-button
            ><el-radio-button :value="2">CI 成功</el-radio-button></el-radio-group
          ></el-form-item
        >
        <el-form-item label="Tag / 分支规则"
          ><el-input v-model="sourceTriggerForm.refPattern" placeholder="glob，例如 v* 或 release/*"
        /></el-form-item>
        <el-form-item label="Artifact / Job"
          ><el-input
            v-model="sourceTriggerForm.artifactSelector"
            :placeholder="sourceTriggerForm.eventType === 1 ? 'Release 附件文件名' : 'Artifact 或 Job 名称'"
        /></el-form-item>
        <el-form-item label="目标渠道 ID"
          ><el-input v-model="sourceTriggerForm.channelIds" placeholder="多个渠道用逗号分隔"
        /></el-form-item>
        <el-form-item label="签名配置 ID"
          ><el-input-number v-model="sourceTriggerForm.signingConfigId" :min="0" :controls="false"
        /></el-form-item>
        <el-form-item label="品牌配置 ID"
          ><el-input-number v-model="sourceTriggerForm.brandingProfileId" :min="0" :controls="false"
        /></el-form-item>
        <el-form-item label="白标产品 ID"
          ><el-input-number v-model="sourceTriggerForm.whiteLabelProductId" :min="0" :controls="false"
        /></el-form-item>
        <el-form-item label="构建池 / 优先级">
          <el-input v-model="sourceTriggerForm.poolCode" style="width: 220px" />
          <el-input-number
            v-model="sourceTriggerForm.priority"
            :min="0"
            :max="100"
            style="margin-left: 12px"
          />
        </el-form-item>
        <el-form-item label="版本名称前缀"
          ><el-input v-model="sourceTriggerForm.versionNamePrefix" maxlength="32"
        /></el-form-item>
        <el-form-item v-if="sourceTriggerForm.id" label="状态"
          ><el-radio-group v-model="sourceTriggerForm.status"
            ><el-radio-button :value="1">启用</el-radio-button
            ><el-radio-button :value="2">停用</el-radio-button></el-radio-group
          ></el-form-item
        >
      </el-form>
      <el-alert
        title="策略只能访问已授权仓库；回调仅接受签名正确、仓库匹配、Tag/分支匹配且 Artifact 选择器唯一命中的事件。"
        type="info"
        :closable="false"
        show-icon
      />
      <template #footer
        ><el-button @click="sourceTriggerFormVisible = false">取消</el-button
        ><el-button type="primary" :loading="sourceTriggerSaving" @click="saveSourceTrigger"
          >保存</el-button
        ></template
      >
    </el-dialog>

    <el-dialog v-model="sourceTriggerSecretVisible" title="源码 Webhook 一次性配置" width="720px">
      <el-alert
        title="回调地址和签名 Secret 只在创建或轮换时显示，请立即保存到代码平台 Webhook 配置。"
        type="warning"
        :closable="false"
        show-icon
      />
      <el-form label-width="120px" class="secret-form">
        <el-form-item label="Payload URL"
          ><el-input :model-value="sourceTriggerWebhookUrl" readonly
        /></el-form-item>
        <el-form-item label="Signing Secret"
          ><el-input :model-value="sourceTriggerSigningSecret" readonly type="password" show-password
        /></el-form-item>
      </el-form>
      <template #footer
        ><el-button
          type="primary"
          @click="copySourceTriggerSecret"
          >复制配置</el-button
        ><el-button @click="sourceTriggerSecretVisible = false">我已保存</el-button></template
      >
    </el-dialog>

    <el-dialog v-model="artifactImportVisible" title="从授权仓库导入 APK Artifact" width="680px">
      <el-form :model="artifactImportForm" label-width="140px">
        <el-form-item label="应用 ID"
          ><el-input-number v-model="artifactImportForm.appId" :min="1" :controls="false"
        /></el-form-item>
        <el-form-item label="授权仓库 ID"
          ><el-input-number v-model="artifactImportForm.repositoryId" :min="1" :controls="false"
        /></el-form-item>
        <el-form-item label="Artifact 来源"
          ><el-radio-group v-model="artifactImportForm.artifactSource"
            ><el-radio-button :value="1">Release</el-radio-button
            ><el-radio-button :value="2">CI Job</el-radio-button></el-radio-group
          ></el-form-item
        >
        <el-form-item label="供应商 Artifact ID"
          ><el-input v-model="artifactImportForm.externalArtifactId"
        /></el-form-item>
        <el-form-item v-if="artifactImportForm.artifactSource === 1" label="Release Tag"
          ><el-input v-model="artifactImportForm.releaseRef" placeholder="例如 v1.2.0"
        /></el-form-item>
        <el-form-item label="versionCode"
          ><el-input-number v-model="artifactImportForm.versionCode" :min="1" :controls="false"
        /></el-form-item>
        <el-form-item label="versionName"
          ><el-input v-model="artifactImportForm.versionName"
        /></el-form-item>
        <el-form-item label="发布说明"
          ><el-input v-model="artifactImportForm.releaseNotes" type="textarea" :rows="3"
        /></el-form-item>
      </el-form>
      <el-alert
        title="仅拉取供应商 Release/CI Artifact，不执行仓库脚本；ZIP 必须且只能包含一个 APK。"
        type="info"
        :closable="false"
        show-icon
      />
      <template #footer
        ><el-button @click="artifactImportVisible = false">取消</el-button
        ><el-button type="primary" :loading="artifactImportSaving" @click="importSourceArtifact"
          >导入并创建版本</el-button
        ></template
      >
    </el-dialog>

    <el-dialog v-model="formVisible" title="创建 API Key" width="620px">
      <el-form :model="form" label-width="130px">
        <el-form-item label="名称"
          ><el-input v-model="form.credentialName" maxlength="128"
        /></el-form-item>
        <el-form-item label="Scopes">
          <el-select v-model="form.scopes" multiple style="width: 100%">
            <el-option
              v-for="item in scopeOptions"
              :key="item[0]"
              :value="item[0]"
              :label="item[1]"
            />
          </el-select>
          <div class="form-hint">{{ selectedScopeText }}</div>
        </el-form-item>
        <el-form-item label="应用 ID"
          ><el-input v-model="form.appIds" placeholder="留空表示全部；多个用逗号分隔"
        /></el-form-item>
        <el-form-item label="IP 白名单">
          <el-input
            v-model="form.ipAllowlist"
            type="textarea"
            :rows="3"
            placeholder="留空不限制；每行一个 IP 或 CIDR"
          />
        </el-form-item>
        <el-form-item label="每分钟上限"
          ><el-input-number v-model="form.rateLimitPerMinute" :min="1" :max="10000"
        /></el-form-item>
        <el-form-item label="有效天数"
          ><el-input-number v-model="form.expiresInDays" :min="0" :max="3650" /><span
            class="form-hint"
            >0 表示长期有效</span
          ></el-form-item
        >
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="createCredential">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="webhookFormVisible"
      :title="webhookForm.id ? '编辑 Webhook' : '新增 Webhook'"
      width="720px"
    >
      <el-form :model="webhookForm" label-width="120px">
        <el-form-item label="名称">
          <el-input v-model="webhookForm.endpointName" maxlength="128" />
        </el-form-item>
        <el-form-item label="HTTPS URL">
          <el-input
            v-model="webhookForm.endpointUrl"
            placeholder="https://example.com/appforge/webhook"
          />
          <div class="form-hint">禁止内网、环回、云元数据地址和重定向。</div>
        </el-form-item>
        <el-form-item label="订阅事件">
          <el-select v-model="webhookForm.eventTypes" multiple style="width: 100%">
            <el-option
              v-for="event in webhookEventOptions"
              :key="event"
              :label="event"
              :value="event"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="最大尝试次数">
          <el-input-number v-model="webhookForm.maxAttempts" :min="1" :max="20" />
        </el-form-item>
        <el-form-item v-if="webhookForm.id" label="状态">
          <el-radio-group v-model="webhookForm.status">
            <el-radio-button :value="1">启用</el-radio-button>
            <el-radio-button :value="2">暂停</el-radio-button>
            <el-radio-button :value="3">吊销</el-radio-button>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="webhookFormVisible = false">取消</el-button>
        <el-button type="primary" :loading="webhookSaving" @click="saveWebhook">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="webhookSecretVisible"
      title="请立即保存 Webhook Signing Secret"
      width="680px"
      :close-on-click-modal="false"
    >
      <el-alert
        title="Signing Secret 只显示这一次，关闭后只能重新轮换。"
        type="warning"
        :closable="false"
        show-icon
      />
      <el-input v-model="oneTimeWebhookSecret" readonly class="secret-value">
        <template #append><el-button @click="copyWebhookSecret">复制</el-button></template>
      </el-input>
      <template #footer>
        <el-button type="primary" @click="webhookSecretVisible = false">我已安全保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="secretVisible"
      title="请立即保存 API Key"
      width="680px"
      :close-on-click-modal="false"
    >
      <el-alert
        title="API Key 只显示这一次，关闭后无法再次查看。"
        type="warning"
        :closable="false"
        show-icon
      />
      <el-input v-model="oneTimeApiKey" readonly class="secret-value">
        <template #append><el-button @click="copySecret">复制</el-button></template>
      </el-input>
      <template #footer
        ><el-button type="primary" @click="secretVisible = false">我已安全保存</el-button></template
      >
    </el-dialog>
  </div>
</template>

<style scoped>
.developer-tabs {
  margin-bottom: 12px;
}
.delivery-query {
  margin-top: 16px;
}
.form-hint {
  margin-left: 8px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.secret-value {
  margin-top: 16px;
}
.source-alert {
  margin-bottom: 16px;
}
.docs-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 16px;
}
.available-repository-table {
  margin-top: 16px;
}
</style>
