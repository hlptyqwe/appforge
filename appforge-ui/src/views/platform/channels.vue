<script setup lang="ts">
import ResourcePage from './components/ResourcePage.vue'
import { platformService } from '@/services'
</script>

<template>
  <ResourcePage
    title="推广渠道"
    permission-prefix="core:channel"
    :list="platformService.listChannels"
    :create="platformService.createChannel"
    :query-fields="[
      { prop: 'appId', label: '应用 ID', type: 'number' },
      { prop: 'keyword', label: '关键词' },
      { prop: 'status', label: '状态', type: 'number' },
    ]"
    :columns="[
      { prop: 'appId', label: '应用 ID' },
      { prop: 'channelCode', label: '渠道编码' },
      { prop: 'channelName', label: '渠道名称' },
      { prop: 'landingUrl', label: '落地页' },
      { prop: 'downloadUrl', label: '下载地址' },
      { prop: 'status', label: '状态' },
    ]"
    :fields="[
      { prop: 'appId', label: '应用 ID', type: 'number', required: true },
      { prop: 'channelCode', label: '渠道编码' },
      { prop: 'channelName', label: '渠道名称', required: true },
      { prop: 'landingUrl', label: '落地页 URL' },
      { prop: 'downloadUrl', label: '下载 URL' },
    ]"
  />
</template>
