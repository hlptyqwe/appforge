<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>

<template>
  <el-result icon="warning" title="404" :sub-title="t('error.notFound')">
    <template #extra>
      <el-button type="primary" @click="$router.push('/')">
        {{ t('common.goHome') }}
      </el-button>
    </template>
  </el-result>
</template>
