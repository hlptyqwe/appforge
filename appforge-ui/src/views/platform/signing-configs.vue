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
      { prop: 'keystoreObjectKey', label: 'Keystore 对象键' },
      { prop: 'keyAlias', label: 'Key Alias' },
      { prop: 'secretRef', label: '密钥引用' },
      { prop: 'status', label: '状态' },
    ]"
    :fields="[
      { prop: 'appId', label: '应用 ID', type: 'number', required: true },
      { prop: 'name', label: '配置名称', required: true },
      { prop: 'keystoreObjectKey', label: 'Keystore 对象键', required: true },
      { prop: 'keyAlias', label: 'Key Alias', required: true },
      { prop: 'keystorePasswordCiphertext', label: 'Keystore 密码密文', type: 'password', required: true },
      { prop: 'keyPasswordCiphertext', label: 'Key 密码密文', type: 'password', required: true },
      { prop: 'secretRef', label: '密钥引用' },
    ]"
  />
</template>
