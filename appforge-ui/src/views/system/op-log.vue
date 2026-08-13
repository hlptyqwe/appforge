<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { usePagination } from '@/composables/usePagination'
import { useLoading } from '@/composables/useLoading'
import { useForm } from '@/composables/useForm'
import { formatDate } from '@/utils'
import { logService, type OptionGroup } from '@/services'
import type { OpLogItem } from '@/services/system/LogService'
import { findOptionGroup, getOptionLabel } from '@/utils/options'
import CrudQueryCard from '@/components/common/CrudQueryCard.vue'

const { t } = useI18n()
const optionGroups = ref<OptionGroup[]>([])
const methodOptions = computed(() => findOptionGroup(optionGroups.value, 'method'))

// Pagination and list
const { pagination, updateFromResponse, resetAndLoad, prevAndLoad, nextAndLoad } =
  usePagination<number>(20)
const { loading, withLoading } = useLoading()

// Query form
const { form: queryForm } = useForm({
  initialData: {
    username: '',
    method: '',
    path: '',
  },
})

const list_ref = ref<OpLogItem[]>([])

async function fetchList() {
  await withLoading(async () => {
    try {
      const res = await logService.getOperationLogs({
        username: queryForm.username || undefined,
        method: queryForm.method || undefined,
        path: queryForm.path || undefined,
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

async function fetchOptions() {
  try {
    const res = await logService.getOptions()
    if (res.code !== 200) throw new Error(res.msg)
    optionGroups.value = res.data || []
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : t('common.loadFailed'))
  }
}

function loadList() {
  resetAndLoad(fetchList)
}

function resetQuery() {
  queryForm.username = ''
  queryForm.method = ''
  queryForm.path = ''
  resetAndLoad(fetchList)
}

function nextPage() {
  nextAndLoad(fetchList)
}

function prevPage() {
  prevAndLoad(fetchList)
}

onMounted(() => {
  fetchOptions()
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

      <el-form-item :label="t('common.method')">
        <el-select v-model="queryForm.method" :placeholder="t('common.pleaseSelect')" clearable>
          <el-option
            v-for="item in methodOptions"
            :key="item.value"
            :label="getOptionLabel(t, item.code, item.value)"
            :value="item.code"
          />
        </el-select>
      </el-form-item>

      <el-form-item :label="t('common.path')">
        <el-input v-model="queryForm.path" :placeholder="t('common.pleaseInputPath')" clearable />
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
        <el-table-column prop="tenantId" :label="t('common.tenantId')" width="100" />
        <el-table-column prop="username" :label="t('common.username')" min-width="120" />
        <el-table-column
          prop="module"
          :label="t('common.module')"
          min-width="130"
          show-overflow-tooltip
        />
        <el-table-column prop="action" :label="t('common.action')" min-width="100" />
        <el-table-column prop="method" :label="t('common.method')" width="80">
          <template #default="{ row }">
            <el-tag>{{ row.method }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column
          prop="path"
          :label="t('common.path')"
          min-width="150"
          show-overflow-tooltip
        />
        <el-table-column prop="ip" :label="t('common.ipAddress')" min-width="130" />
        <el-table-column
          prop="resp"
          :label="t('common.response')"
          min-width="150"
          show-overflow-tooltip
        />
        <el-table-column prop="costMs" :label="t('common.costMs')" width="110">
          <template #default="{ row }">
            <span style="color: #666">{{ row.costMs }}ms</span>
          </template>
        </el-table-column>
        <el-table-column prop="createTimes" :label="t('common.createTimes')" min-width="170">
          <template #default="{ row }">
            <span style="color: #666">{{ formatDate(row.createTimes) }}</span>
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
