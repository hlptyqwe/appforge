<script setup lang="ts">
import ResourcePage from './components/ResourcePage.vue'
import { platformService } from '@/services'
</script>

<template>
  <ResourcePage
    title="APK 签名配置"
    permission-prefix="core:signing-config"
    :list="platformService.listSigningConfigs"
    :create="platformService.createSigningConfig"
    :query-fields="[
      { prop: 'appId', label: '应用 ID', type: 'number' },
      { prop: 'status', label: '状态', type: 'number' },
    ]"
    :columns="[
      { prop: 'appId', label: '应用 ID' },
      { prop: 'name', label: '配置名称' },
      { prop: 'keystoreObjectId', label: 'Keystore 对象 ID' },
      { prop: 'keyAlias', label: 'Key Alias' },
      { prop: 'status', label: '状态' },
    ]"
    :fields="[
      { prop: 'appId', label: '应用 ID', type: 'number', required: true },
      { prop: 'name', label: '配置名称', required: true },
      {
        prop: 'keystoreObjectId',
        label: 'Keystore 文件',
        type: 'file',
        required: true,
        accept: '.jks,.keystore,.p12,.pfx',
        objectType: 2,
        maxBytes: 10485760,
      },
      { prop: 'keyAlias', label: 'Key Alias', required: true },
      { prop: 'keystorePassword', label: 'Keystore 密码', type: 'password', required: true },
      { prop: 'keyPassword', label: 'Key 密码', type: 'password', required: true },
    ]"
  />
</template>
