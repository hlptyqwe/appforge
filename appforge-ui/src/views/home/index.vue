<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'
const auth = useAuthStore()
const { t } = useI18n()
</script>

<template>
  <el-card>
    <template #header>
      {{ t('route.home') }}
    </template>
    <div>{{ t('home.welcome', { name: auth.user?.nickname || auth.user?.username || '-' }) }}</div>
    <div style="margin-top: 8px; color: #666">
      {{ t('home.multiLangDesc') }}
    </div>
  </el-card>
</template>
