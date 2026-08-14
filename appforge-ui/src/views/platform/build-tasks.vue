<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import ResourcePage from './components/ResourcePage.vue'
import { platformService, type PlatformBuildTask } from '@/services'

const page = ref<InstanceType<typeof ResourcePage>>()

async function cancelTask(row: PlatformBuildTask) {
  const result = await ElMessageBox.prompt('请输入取消原因', '取消构建', { inputValue: '用户取消' })
  const response = await platformService.cancelBuildTask(row.id, result.value)
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success('构建任务已取消')
  await page.value?.loadData()
}

async function retryTask(row: PlatformBuildTask) {
  await ElMessageBox.confirm(`确认重试构建任务 ${row.id}？`, '重试构建')
  const response = await platformService.retryBuildTask(row.id)
  if (response.code !== 200) throw new Error(response.msg)
  ElMessage.success(`已创建重试任务 ${response.data?.id}`)
  await page.value?.loadData()
}
</script>

<template>
  <ResourcePage
    ref="page"
    title="APK 构建任务"
    permission-prefix="core:build-task"
    :list="platformService.listBuildTasks"
    :create="platformService.createBuildTask"
    :query-fields="[
      { prop: 'appId', label: '应用 ID', type: 'number' },
      { prop: 'channelId', label: '渠道 ID', type: 'number' },
      { prop: 'status', label: '状态', type: 'number' },
    ]"
    :columns="[
      { prop: 'appId', label: '应用 ID' },
      { prop: 'versionName', label: '版本' },
      { prop: 'channelCode', label: '渠道' },
      { prop: 'brandingRevision', label: '品牌修订' },
      { prop: 'whiteLabelProductId', label: '白标产品 ID' },
      { prop: 'templateRevision', label: '模板修订' },
      { prop: 'status', label: '状态' },
      { prop: 'builderId', label: '构建器' },
      { prop: 'poolCode', label: '构建池' },
      { prop: 'cacheHit', label: '缓存命中' },
      { prop: 'apkObjectId', label: 'APK 下载', downloadObject: true },
      { prop: 'logObjectId', label: '构建日志', downloadObject: true },
      { prop: 'errorMessage', label: '错误信息' },
    ]"
    :fields="[
      { prop: 'appId', label: '应用 ID', type: 'number', required: true },
      { prop: 'versionId', label: '版本 ID', type: 'number', required: true },
      { prop: 'channelId', label: '渠道 ID', type: 'number', required: true },
      { prop: 'signingConfigId', label: '签名配置 ID', type: 'number' },
      { prop: 'brandingProfileId', label: '品牌配置 ID', type: 'number' },
      { prop: 'whiteLabelProductId', label: '白标产品 ID', type: 'number' },
      { prop: 'priority', label: '优先级', type: 'number' },
      { prop: 'poolCode', label: '构建池' },
    ]"
  >
    <template #actions="{ row }">
      <el-button
        v-if="[1, 2, 3, 4].includes(row.status)"
        link
        type="danger"
        @click="cancelTask(row)"
        >取消</el-button
      >
      <el-button v-if="[6, 7].includes(row.status)" link type="primary" @click="retryTask(row)"
        >重试</el-button
      >
    </template>
  </ResourcePage>
</template>
