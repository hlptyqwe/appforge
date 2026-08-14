<script setup lang="ts">
import ResourcePage from './components/ResourcePage.vue'
import { platformService } from '@/services'
</script>

<template>
  <ResourcePage
    title="应用版本"
    permission-prefix="core:version"
    :list="platformService.listVersions"
    :create="platformService.createVersion"
    :query-fields="[
      { prop: 'appId', label: '应用 ID', type: 'number' },
      { prop: 'status', label: '状态', type: 'number' },
    ]"
    :columns="[
      { prop: 'appId', label: '应用 ID' },
      { prop: 'versionCode', label: 'Version Code' },
      { prop: 'versionName', label: '版本名称' },
      { prop: 'sourceApkObjectId', label: '原始 APK', downloadObject: true },
      { prop: 'sourceApkSha256', label: 'APK SHA-256' },
      { prop: 'status', label: '状态' },
    ]"
    :fields="[
      { prop: 'appId', label: '应用 ID', type: 'number', required: true },
      { prop: 'versionCode', label: 'Version Code', type: 'number', required: true },
      { prop: 'versionName', label: '版本名称', required: true },
      {
        prop: 'sourceApkObjectId',
        label: '原始 APK',
        type: 'file',
        required: true,
        accept: '.apk,application/vnd.android.package-archive',
        objectType: 1,
        maxBytes: 2147483648,
      },
      { prop: 'releaseNotes', label: '发布说明', type: 'textarea' },
      { prop: 'buildConfigJson', label: '构建配置 JSON', type: 'textarea' },
    ]"
  />
</template>
