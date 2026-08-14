<script setup lang="ts">
import ResourcePage from './components/ResourcePage.vue'
import { platformService } from '@/services'
</script>

<template>
  <ResourcePage
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
      { prop: 'status', label: '状态' },
      { prop: 'builderId', label: '构建器' },
      { prop: 'apkObjectId', label: 'APK 下载', downloadObject: true },
      { prop: 'logObjectId', label: '构建日志', downloadObject: true },
      { prop: 'errorMessage', label: '错误信息' },
    ]"
    :fields="[
      { prop: 'appId', label: '应用 ID', type: 'number', required: true },
      { prop: 'versionId', label: '版本 ID', type: 'number', required: true },
      { prop: 'channelId', label: '渠道 ID', type: 'number', required: true },
      { prop: 'signingConfigId', label: '签名配置 ID', type: 'number', required: true },
      { prop: 'priority', label: '优先级', type: 'number' },
    ]"
  />
</template>
