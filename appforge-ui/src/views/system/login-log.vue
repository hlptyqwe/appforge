<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { usePagination } from '@/composables/usePagination'
import { useLoading } from '@/composables/useLoading'
import { useForm } from '@/composables/useForm'
import { formatDate } from '@/utils'
import { logService } from '@/services'
import type { LoginLogItem } from '@/services/system/LogService'
import CrudQueryCard from '@/components/common/CrudQueryCard.vue'

const { t } = useI18n()

// Pagination and list
const { pagination, updateFromResponse, resetAndLoad, prevAndLoad, nextAndLoad } =
  usePagination<number>(20)
const { loading, withLoading } = useLoading()

// Query form
const { form: queryForm } = useForm({
  initialData: {
    username: '',
    success: undefined as number | undefined,
  },
})

const list_ref = ref<LoginLogItem[]>([])

async function fetchList() {
  await withLoading(async () => {
    try {
      const res = await logService.getLoginLogs({
        username: queryForm.username || undefined,
        success: queryForm.success,
        cursor: pagination.cursor,
        limit: pagination.limit,
      })
      if (res.code !== 200) throw new Error(res.msg)
      list_ref.value = res.data || []
      updateFromResponse(res)
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : t('common.loadFailed'))
    }
  })
}

function loadList() {
  resetAndLoad(fetchList)
}

function resetQuery() {
  queryForm.username = ''
  queryForm.success = undefined
  resetAndLoad(fetchList)
}

function nextPage() {
  nextAndLoad(fetchList)
}

function prevPage() {
  prevAndLoad(fetchList)
}

onMounted(() => {
  fetchList()
})
</script>

<template>
  <div class="module-page">
    <CrudQueryCard :model="queryForm" @search="loadList" @reset="resetQuery">
      <el-form-item :label="t('common.username')">
        <el-input
          v-model="queryForm.username"
          :placeholder="t('common.pleaseInputUsername')"
          clearable
        />
      </el-form-item>

      <el-form-item :label="t('common.result')">
        <el-select
          v-model="queryForm.success"
          :placeholder="t('common.pleaseSelectResult')"
          clearable
        >
          <el-option :label="t('common.success')" :value="1" />
          <el-option :label="t('common.failed')" :value="0" />
        </el-select>
      </el-form-item>
    </CrudQueryCard>

    <el-card class="table-card">
      <!-- Table -->
      <el-table
        v-loading="loading"
        :data="list_ref"
        row-key="id"
        style="margin-bottom: 16px"
      >
        <el-table-column prop="id" :label="t('common.id')" width="70" />
        <el-table-column prop="username" :label="t('common.username')" min-width="120" />
        <el-table-column prop="ip" :label="t('common.loginIP')" min-width="130" />
        <el-table-column
          prop="ua"
          :label="t('common.userAgent')"
          min-width="180"
          show-overflow-tooltip
        />
        <el-table-column prop="success" :label="t('common.result')" width="100">
          <template #default="{ row }">
            <el-tag :type="row.success === 1 ? 'success' : 'danger'">
              {{ row.success === 1 ? t('common.loginSuccess') : t('common.loginFailed') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          prop="msg"
          :label="t('common.failureReason')"
          min-width="180"
          show-overflow-tooltip
        >
          <template #default="{ row }">
            <span v-if="row.success !== 1">{{ row.msg }} </span>
            <span v-else style="color: #999">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="loginAt" :label="t('common.loginTime')" min-width="170">
          <template #default="{ row }">
            <span style="color: #666">{{ formatDate(row.loginAt) }}</span>
          </template>
        </el-table-column>
      </el-table>

      <!-- Pagination -->
      <CursorPagination
        v-model:limit="pagination.limit"
        :total="pagination.total"
        :has-prev="pagination.hasPrev"
        :has-next="pagination.hasNext"
        @prev="prevPage"
        @next="nextPage"
        @limit-change="
          () => {
            resetAndLoad(fetchList)
          }
        "
      />
    </el-card>
  </div>
</template>
