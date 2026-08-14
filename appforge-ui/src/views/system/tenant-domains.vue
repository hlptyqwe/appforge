<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import {
  tenantDomainsService,
  type SysTenantDomainCreateReq,
  type SysTenantDomainItem,
} from '@/services'
import TenantSelect from '@/components/TenantSelect.vue'
import { formatDate } from '@/utils'

const tenantId = ref(0)
const loading = ref(false)
const submitting = ref(false)
const rows = ref<SysTenantDomainItem[]>([])
const dialogVisible = ref(false)
const editing = ref(false)
const formRef = ref<FormInstance>()
const form = reactive<SysTenantDomainCreateReq & { id: number }>({
  id: 0,
  tenantId: 0,
  origin: '',
  status: 1,
  priority: 0,
})

async function loadList() {
  if (!tenantId.value) {
    rows.value = []
    return
  }
  loading.value = true
  try {
    const response = await tenantDomainsService.getList({ tenantId: tenantId.value })
    if (response.code !== 200) throw new Error(response.msg)
    rows.value = response.data || []
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载域名失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  if (!tenantId.value) {
    ElMessage.warning('请先选择租户')
    return
  }
  editing.value = false
  Object.assign(form, { id: 0, tenantId: tenantId.value, origin: '', status: 1, priority: 0 })
  dialogVisible.value = true
}

function openEdit(row: SysTenantDomainItem) {
  editing.value = true
  Object.assign(form, row)
  dialogVisible.value = true
}

async function submit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    const response = editing.value
      ? await tenantDomainsService.update(form.id, form)
      : await tenantDomainsService.create(form)
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await loadList()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存失败')
  } finally {
    submitting.value = false
  }
}

async function remove(row: SysTenantDomainItem) {
  try {
    await ElMessageBox.confirm(`确定删除域名“${row.origin}”吗？`, '删除域名', { type: 'warning' })
    const response = await tenantDomainsService.delete(row.id)
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success('删除成功')
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
          <TenantSelect v-model="tenantId" @change="loadList" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadList">查询</el-button>
          <el-button v-perm="'sys:tenant-domain:add'" @click="openCreate">新增域名</el-button>
        </el-form-item>
      </el-form>
    </el-card>
    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="rows" stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="origin" label="Origin" min-width="260" />
        <el-table-column prop="status" label="状态" width="100" />
        <el-table-column prop="priority" label="优先级" width="100" />
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">{{ formatDate(row.updateTimes) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button
              v-perm="'sys:tenant-domain:update'"
              link
              type="primary"
              @click="openEdit(row)"
              >编辑</el-button
            >
            <el-button v-perm="'sys:tenant-domain:delete'" link type="danger" @click="remove(row)"
              >删除</el-button
            >
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="editing ? '编辑租户域名' : '新增租户域名'"
      width="560px"
    >
      <el-form ref="formRef" :model="form" label-width="100px">
        <el-form-item
          label="Origin"
          prop="origin"
          :rules="[{ required: true, message: '请输入域名 Origin' }]"
        >
          <el-input v-model="form.origin" placeholder="https://example.com" />
        </el-form-item>
        <el-form-item label="状态"
          ><el-input-number v-model="form.status" :min="1" :max="3"
        /></el-form-item>
        <el-form-item label="优先级"
          ><el-input-number v-model="form.priority" :min="0"
        /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
