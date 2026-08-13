<script setup lang="ts">
import ResourcePage from './components/ResourcePage.vue'
import { platformService } from '@/services'
</script>

<template>
  <ResourcePage
    title="应用"
    permission-prefix="core:application"
    :list="platformService.listApplications"
    :create="platformService.createApplication"
    :query-fields="[
      { prop: 'keyword', label: '关键词' },
      { prop: 'status', label: '状态', type: 'number' },
    ]"
    :columns="[
      { prop: 'appCode', label: '应用编码' },
      { prop: 'appName', label: '应用名称' },
      { prop: 'packageName', label: '包名' },
      { prop: 'apiHost', label: 'API Host' },
      { prop: 'status', label: '状态' },
    ]"
    :fields="[
      { prop: 'appCode', label: '应用编码', required: true },
      { prop: 'appName', label: '应用名称', required: true },
      { prop: 'packageName', label: 'Android 包名', required: true },
      { prop: 'description', label: '描述', type: 'textarea' },
      { prop: 'iconUrl', label: '图标 URL' },
      { prop: 'apiHost', label: 'API Host' },
    ]"
  />
</template>
