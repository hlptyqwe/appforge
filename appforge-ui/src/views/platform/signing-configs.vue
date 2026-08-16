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
      {
        prop: 'signingMode',
        label: '签名模式',
        valueLabels: { '1': '本地 Keystore', '2': '远程签名服务' },
      },
      { prop: 'keystoreObjectId', label: 'Keystore 对象 ID' },
      { prop: 'keyAlias', label: 'Key Alias / Key ID' },
      { prop: 'status', label: '状态' },
    ]"
    :fields="[
      { prop: 'appId', label: '应用 ID', type: 'number', required: true },
      { prop: 'name', label: '配置名称', required: true },
      {
        prop: 'signingMode',
        label: '签名模式',
        type: 'select',
        required: true,
        defaultValue: 1,
        options: [
          { label: '本地 Keystore', value: 1 },
          { label: '远程签名服务', value: 2 },
        ],
      },
      {
        prop: 'keystoreObjectId',
        label: 'Keystore 文件',
        type: 'file',
        required: true,
        visibleWhen: { prop: 'signingMode', equals: 1 },
        accept: '.jks,.keystore,.p12,.pfx',
        objectType: 2,
        maxBytes: 10485760,
      },
      { prop: 'keyAlias', label: 'Key Alias / 远程 Key ID', required: true },
      {
        prop: 'keystorePassword',
        label: 'Keystore 密码',
        type: 'password',
        required: true,
        visibleWhen: { prop: 'signingMode', equals: 1 },
      },
      {
        prop: 'keyPassword',
        label: 'Key 密码',
        type: 'password',
        required: true,
        visibleWhen: { prop: 'signingMode', equals: 1 },
      },
      {
        prop: 'secretRef',
        label: '远程签名 Secret 引用',
        required: true,
        visibleWhen: { prop: 'signingMode', equals: 2 },
      },
    ]"
  />
</template>
