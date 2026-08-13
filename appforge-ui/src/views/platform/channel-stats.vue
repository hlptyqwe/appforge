<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { platformService, type PlatformChannelStats } from '@/services'

const loading = ref(false)
const query = reactive({ appId: 0, channelId: 0, startTime: 0, endTime: 0 })
const stats = ref<PlatformChannelStats | null>(null)

async function loadStats() {
  if (!query.appId || !query.channelId) {
    ElMessage.warning('请输入应用 ID 和渠道 ID')
    return
  }
  loading.value = true
  try {
    const response = await platformService.getChannelStats({
      appId: query.appId,
      channelId: query.channelId,
      startTime: query.startTime || undefined,
      endTime: query.endTime || undefined,
    })
    if (response.code !== 200) throw new Error(response.msg)
    stats.value = response.data || null
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载统计失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="module-page">
    <el-card shadow="never" class="query-card">
      <el-form inline>
        <el-form-item label="应用 ID"><el-input-number v-model="query.appId" :min="1" /></el-form-item>
        <el-form-item label="渠道 ID"><el-input-number v-model="query.channelId" :min="1" /></el-form-item>
        <el-form-item label="开始时间戳"><el-input-number v-model="query.startTime" :min="0" /></el-form-item>
        <el-form-item label="结束时间戳"><el-input-number v-model="query.endTime" :min="0" /></el-form-item>
        <el-form-item><el-button type="primary" :loading="loading" @click="loadStats">查询</el-button></el-form-item>
      </el-form>
    </el-card>
    <el-card v-if="stats" shadow="never">
      <template #header>渠道转化统计：{{ stats.channelCode }}</template>
      <el-row :gutter="16">
        <el-col v-for="item in [
          ['点击', stats.clicks],
          ['下载', stats.downloads],
          ['安装', stats.installs],
          ['注册', stats.registrations],
          ['首充', stats.firstPays],
          ['付费', stats.pays],
        ]" :key="String(item[0])" :span="4">
          <el-statistic :title="String(item[0])" :value="Number(item[1])" />
        </el-col>
      </el-row>
    </el-card>
  </div>
</template>
