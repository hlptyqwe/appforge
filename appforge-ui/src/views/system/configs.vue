<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { configService, type SysConfigCreateReq, type SysConfigItem } from '@/services'
import TenantSelect from '@/components/TenantSelect.vue'
import CursorPagination from '@/components/common/CursorPagination.vue'
import { usePagination } from '@/composables/usePagination'
import { formatDate } from '@/utils'

const loading = ref(false)
const submitting = ref(false)
const rows = ref<SysConfigItem[]>([])
const query = reactive({ tenantId: 0, keyword: '' })
const dialogVisible = ref(false)
const editing = ref(false)
const formRef = ref<FormInstance>()
const form = reactive<SysConfigCreateReq & { id: number }>({
  id: 0,
  tenantId: 0,
  configKey: '',
  configValue: '{}',
  remark: '',
})
const { pagination, updateFromResponse, resetAndLoad, nextAndLoad, prevAndLoad } =
  usePagination<number>(20)

async function loadList() {
  loading.value = true
  try {
    const response = await configService.getList({
      tenantId: query.tenantId,
      keyword: query.keyword || undefined,
      cursor: pagination.cursor,
      limit: pagination.limit,
    })
    if (response.code !== 200) throw new Error(response.msg)
    rows.value = response.data || []
    updateFromResponse(response)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载配置失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = false
  Object.assign(form, { id: 0, tenantId: query.tenantId, configKey: '', configValue: '{}', remark: '' })
  dialogVisible.value = true
}

function openEdit(row: SysConfigItem) {
  editing.value = true
  Object.assign(form, row)
  dialogVisible.value = true
}

async function submit() {
  await formRef.value?.validate()
  try {
    JSON.parse(form.configValue)
  } catch {
    ElMessage.error('配置值必须是合法 JSON')
    return
  }
  submitting.value = true
  try {
    const response = editing.value
      ? await configService.update(form.id, {
          configKey: form.configKey,
          configValue: form.configValue,
          remark: form.remark,
        })
      : await configService.create({
          tenantId: form.tenantId,
          configKey: form.configKey,
          configValue: form.configValue,
          remark: form.remark,
        })
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success(editing.value ? '配置已更新' : '配置已创建')
    dialogVisible.value = false
    await loadList()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存失败')
  } finally {
    submitting.value = false
  }
}

async function remove(row: SysConfigItem) {
  try {
    await ElMessageBox.confirm(`确定删除配置“${row.configKey}”吗？`, '删除配置', { type: 'warning' })
    const response = await configService.delete(row.id)
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('配置已删除')
    await loadList()
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(error instanceof Error ? error.message : '删除失败')
  }
}

onMounted(loadList)
</script>

<template>
  <div class="module-page">
    <el-card shadow="never" class="query-card">
      <el-form inline>
        <el-form-item label="租户">
          <TenantSelect v-model="query.tenantId" include-system />
        </el-form-item>
        <el-form-item label="配置键">
          <el-input v-model="query.keyword" clearable placeholder="按配置键搜索" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="resetAndLoad(loadList)">查询</el-button>
          <el-button v-perm="'sys:config:add'" @click="openCreate">
            <el-icon><Plus /></el-icon>新增
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="rows" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="tenantId" label="租户 ID" width="100" />
        <el-table-column prop="configKey" label="配置键" min-width="180" />
        <el-table-column prop="configValue" label="JSON 配置值" min-width="320" show-overflow-tooltip />
        <el-table-column prop="remark" label="备注" min-width="140" />
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">{{ formatDate(row.updateTimes) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button v-perm="'sys:config:update'" link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button v-perm="'sys:config:delete'" link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <CursorPagination
        v-model:limit="pagination.limit"
        :total="pagination.total"
        :has-prev="pagination.hasPrev"
        :has-next="pagination.hasNext"
        @prev="prevAndLoad(loadList)"
        @next="nextAndLoad(loadList)"
        @limit-change="resetAndLoad(loadList)"
      />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑配置' : '新增配置'" width="680px">
      <el-form ref="formRef" :model="form" label-width="90px">
        <el-form-item label="租户">
          <TenantSelect v-model="form.tenantId" include-system :disabled="editing" />
        </el-form-item>
        <el-form-item label="配置键" prop="configKey" :rules="[{ required: true, message: '请输入配置键' }]">
          <el-input v-model="form.configKey" />
        </el-form-item>
        <el-form-item label="JSON 值" prop="configValue" :rules="[{ required: true, message: '请输入配置值' }]">
          <el-input v-model="form.configValue" type="textarea" :rows="12" />
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
