<script setup lang="ts">
import { computed, ref, onMounted, nextTick } from 'vue'
import { ElMessage, type FormInstance, type TreeInstance } from 'element-plus'
import { useI18n } from 'vue-i18n'
import type { OptionGroup } from '@/services'
import type { RoleQueryParams, SysRole } from '@/services/system/RoleService'
import type { MenuNode, PermItem } from '@/services/system/MenuService'
import { usePagination, useLoading, useConfirm, useForm } from '@/composables'

import { roleService, menuService } from '@/services'
import {
  findFormOptionGroup,
  findOptionGroup,
  getOptionLabel,
  getOptionValueLabel,
} from '@/utils/options'
import { useAuthStore } from '@/stores/auth'
import CrudQueryCard from '@/components/common/CrudQueryCard.vue'
import TenantSelect from '@/components/TenantSelect.vue'
import { IS_AGENT } from '@/config/environment'

const currentAppScope = IS_AGENT ? 2 : 1

type RoleMenuNode = MenuNode & {
  parentId?: number
  menuType?: number
  perms?: string
  sort?: number
  children?: RoleMenuNode[]
}

// ===== i18n =====
const { t } = useI18n()
const auth = useAuthStore()
const optionGroups = ref<OptionGroup[]>([])
const enabledOptions = computed(() => findOptionGroup(optionGroups.value, 'enabled'))
const enabledSelectOptions = computed(() => {
  const options = enabledOptions.value
  return options.length
    ? options
    : [
        { value: 0, code: 'COMMON_STATUS_UNKNOWN' },
        { value: 1, code: 'COMMON_STATUS_ENABLED' },
        { value: 2, code: 'COMMON_STATUS_DISABLED' },
      ]
})
const enabledFormOptions = computed(() => {
  const options = findFormOptionGroup(optionGroups.value, 'enabled')
  return options.length
    ? options
    : [
        { value: 1, code: 'COMMON_STATUS_ENABLED' },
        { value: 2, code: 'COMMON_STATUS_DISABLED' },
      ]
})

// ===== helpers =====
function isSuperRole(r: SysRole | null | undefined) {
  if (!r) return false
  return (
    r.isSuper === true || r.code === 'super_admin' || r.code === 'tenant_super_admin' || r.id === 1
  )
}

function isProtectedRole(r: SysRole | null | undefined) {
  return isSuperRole(r)
}

function isTenantOwnerRole(r: SysRole | null | undefined) {
  return r?.code === 'tenant_owner'
}

function isReadonlyRole(r: SysRole | null | undefined) {
  if (isProtectedRole(r)) return true
  return auth.isTenantUser && isTenantOwnerRole(r)
}

function enabledOptionLabel(value: number | string | undefined, code?: string) {
  if (Number(value) === 1) return t('common.enabled')
  if (Number(value) === 2) return t('common.disabled')
  if (Number(value) === 0) return t('common.all')
  return getOptionLabel(t, code, Number(value))
}

function enabledLabel(value: number | undefined) {
  const label = getOptionValueLabel(optionGroups.value, 'enabled', value, t)
  if (label && label !== value) return label
  return enabledOptionLabel(value)
}

// ===== state =====
const { pagination, updateFromResponse, resetAndLoad, nextAndLoad, prevAndLoad } =
  usePagination<number>(20)
const { loading, withLoading } = useLoading()
const { confirm } = useConfirm()
const { form: queryForm } = useForm({
  initialData: {
    keyword: '',
    enabled: 0 as 0 | 1 | 2,
    appScope: currentAppScope as number | undefined,
  },
})

const tableData = ref<SysRole[]>([])

// ===== list =====
async function fetchList() {
  await withLoading(async () => {
    try {
      const q: RoleQueryParams = {
        keyword: queryForm.keyword,
        enabled: queryForm.enabled,
        appScope: queryForm.appScope,
        cursor: pagination.cursor,
        limit: pagination.limit,
      }
      if (q.enabled === 0) delete q.enabled

      const resp = await roleService.getList(q)
      tableData.value = resp.data || []
      updateFromResponse(resp)
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
    }
  })
}

async function fetchOptions() {
  try {
    const resp = await roleService.getOptions()
    optionGroups.value = resp.data || []
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
  }
}

function toRoleMenuNode(node: MenuNode): RoleMenuNode {
  return {
    ...node,
    children: [],
  }
}

function buildMenuTree(flat: MenuNode[]): RoleMenuNode[] {
  const list = (flat || []).filter((x): x is MenuNode => Boolean(x))

  const map = new Map<number, RoleMenuNode>()
  list.forEach((n) => map.set(n.id, toRoleMenuNode(n)))

  const roots: RoleMenuNode[] = []
  map.forEach((node) => {
    const pid = node.parentId
    if (pid && map.has(pid)) {
      map.get(pid)?.children?.push(node)
    } else {
      roots.push(node)
    }
  })

  const sortRec = (arr: RoleMenuNode[]) => {
    arr.sort((a, b) => (a.sort ?? 0) - (b.sort ?? 0))
    arr.forEach((x) => x.children?.length && sortRec(x.children))
  }
  sortRec(roots)

  return roots
}

function loadList() {
  resetAndLoad(fetchList)
}

function resetQuery() {
  queryForm.keyword = ''
  queryForm.enabled = 0
  queryForm.appScope = currentAppScope
  loadList()
}

function nextPage() {
  nextAndLoad(fetchList)
}

function prevPage() {
  prevAndLoad(fetchList)
}

// ===== create/update dialog =====
const editVisible = ref(false)
const editFormRef = ref<FormInstance>()
const { form: editForm } = useForm({
  initialData: {
    id: 0,
    tenantId: undefined as number | undefined,
    name: '',
    code: '',
    remark: '',
    enabled: 1 as 1 | 2,
    appScope: currentAppScope,
  },
})
const editIsUpdate = computed(() => editForm.id > 0)
const { loading: editLoading, withLoading: withEditLoading } = useLoading()

function openCreate() {
  editForm.id = 0
  editForm.tenantId = undefined
  editForm.name = ''
  editForm.code = ''
  editForm.remark = ''
  editForm.enabled = 1
  editForm.appScope = currentAppScope
  editVisible.value = true
}
function openUpdate(row: SysRole) {
  if (isProtectedRole(row)) return
  editForm.id = row.id
  editForm.tenantId = row.tenantId
  editForm.name = row.name
  editForm.code = row.code
  editForm.remark = row.remark || ''
  editForm.enabled = row.enabled === 2 ? 2 : 1
  editForm.appScope = row.appScope || currentAppScope
  editVisible.value = true
}

async function submitEdit() {
  await editFormRef.value?.validate?.()
  await withEditLoading(async () => {
    try {
      const payload = { ...editForm, tenantId: Number(editForm.tenantId || 0) }
      const resp = editIsUpdate.value
        ? await roleService.update(editForm.id, payload)
        : await roleService.create(payload)
      if (resp.code === 200) {
        ElMessage.success(resp.msg || t('common.success'))
        editVisible.value = false
        fetchList()
      } else {
        ElMessage.error(resp.msg || t('common.failed'))
      }
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
    }
  })
}

async function onDelete(row: SysRole) {
  if (isProtectedRole(row)) return
  try {
    await confirm(t('common.confirmDelete'), { type: 'warning' })
    const resp = await roleService.delete(row.id)
    if (resp.code === 200) {
      ElMessage.success(resp.msg || t('common.success'))
      fetchList()
    } else {
      ElMessage.error(resp.msg || t('common.failed'))
    }
  } catch (error: unknown) {
    if ((error instanceof Error ? error.message : '') === 'cancel') return
    ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
  }
}

// ===== grant dialog =====
const grantVisible = ref(false)
const currentRole = ref<SysRole | null>(null)

const { loading: grantLoading, withLoading: withGrantLoading } = useLoading()
const menuTree = ref<RoleMenuNode[]>([])
const menuNodeMap = ref<Map<number, RoleMenuNode>>(new Map())
const permList = ref<PermItem[]>([])

const menuTreeRef = ref<TreeInstance>()
const checkedPermKeys = ref<string[]>([])

const grantReadonly = computed(() => isReadonlyRole(currentRole.value))
const grantDialogTitle = computed(() =>
  t(grantReadonly.value ? 'system.viewGrantTitle' : 'system.grantTitle', {
    role: currentRole.value?.name || '',
  }),
)

function flattenMenuTree(nodes: RoleMenuNode[], result: RoleMenuNode[] = []): RoleMenuNode[] {
  for (const node of nodes || []) {
    result.push(node)
    if (node.children?.length) {
      flattenMenuTree(node.children, result)
    }
  }
  return result
}

function updateMenuNodeMap(nodes: RoleMenuNode[]) {
  const map = new Map<number, RoleMenuNode>()
  flattenMenuTree(nodes).forEach((node) => {
    if (node && node.id != null) {
      map.set(node.id, node)
    }
  })
  menuNodeMap.value = map
}

function getCheckedButtonPermKeys(): string[] {
  const checkedNodes = (menuTreeRef.value?.getCheckedNodes?.() || []) as RoleMenuNode[]
  return checkedNodes
    .filter((node) => node.menuType === 3 && node.perms)
    .map((node) => node.perms as string)
    .filter(Boolean)
}

function onMenuTreeCheck() {
  checkedPermKeys.value = getCheckedButtonPermKeys()
}

function getInitialCheckedMenuIds(menuIds: number[]): number[] {
  const ids = new Set((menuIds || []).map(Number))
  return Array.from(ids).filter((id) => {
    const node = menuNodeMap.value.get(id)
    return node && !node.children?.length
  })
}

function openGrant(row: SysRole) {
  currentRole.value = row
  grantVisible.value = true
  initGrant(row.id)
}

async function initGrant(roleId: number) {
  await withGrantLoading(async () => {
    try {
      menuTree.value = []
      permList.value = []
      checkedPermKeys.value = []
      await nextTick()
      menuTreeRef.value?.setCheckedKeys?.([])

      const [menusResp, permsResp, detailResp] = await Promise.all([
        menuService.getMenuTree(roleId),
        menuService.getPermissionList(),
        roleService.getRoleGrantDetail(roleId),
      ])

      const menusFlat = menusResp.data || []
      const perms = permsResp.data || []
      const detail = detailResp.data || { menuIds: [], permKeys: [] }

      menuTree.value = buildMenuTree(menusFlat)
      permList.value = perms
      updateMenuNodeMap(menuTree.value)

      const menuIds = detail?.menuIds || []
      const permKeys = detail?.permKeys || []

      await nextTick()
      menuTreeRef.value?.setCheckedKeys(getInitialCheckedMenuIds(menuIds))
      checkedPermKeys.value = getCheckedButtonPermKeys()
      if (!checkedPermKeys.value.length) {
        checkedPermKeys.value = permKeys
      }
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
    }
  })
}

function collectCheckedMenuIds(): number[] {
  const tree = menuTreeRef.value
  if (!tree) return []
  // Element Plus tree: persist both fully checked and half-checked menu ids.
  const full = (tree.getCheckedKeys?.() || []) as number[]
  const half = (tree.getHalfCheckedKeys?.() || []) as number[]
  return Array.from(new Set([...full, ...half])).map(Number)
}

async function submitGrant() {
  if (!currentRole.value) return
  if (grantReadonly.value) {
    ElMessage.warning(t('system.superAdminNoGrant'))
    return
  }

  try {
    const payload = {
      roleId: currentRole.value.id,
      menuIds: collectCheckedMenuIds(),
      permKeys: checkedPermKeys.value,
    }
    const resp = await roleService.grantRole(payload)
    if (resp.code === 200) {
      ElMessage.success(resp.msg || t('common.success'))
      grantVisible.value = false
      fetchList()
    } else {
      ElMessage.error(resp.msg || t('common.failed'))
    }
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
  }
}

function onGrantClosed() {
  currentRole.value = null
  menuTree.value = []
  permList.value = []
  checkedPermKeys.value = []
  menuTreeRef.value?.setCheckedKeys?.([])
}

// ===== init =====
onMounted(async () => {
  await Promise.all([fetchList(), fetchOptions()])
})
</script>

<template>
  <div class="module-page">
    <CrudQueryCard :model="queryForm" @search="loadList" @reset="resetQuery">
      <el-form-item :label="t('common.keyword')">
        <el-input v-model="queryForm.keyword" :placeholder="t('common.keyword')" clearable />
      </el-form-item>

      <el-form-item :label="t('common.enabled')">
        <el-select
          v-model="queryForm.enabled"
          :placeholder="t('common.enabled')"
          @change="loadList"
        >
          <el-option
            v-for="item in enabledSelectOptions"
            :key="item.value"
            :label="enabledOptionLabel(item.value, item.code)"
            :value="item.value"
          />
        </el-select>
      </el-form-item>

      <template #actions>
        <el-button v-perm="'sys:role:add'" type="primary" @click="openCreate">
          {{ t('common.add') }}
        </el-button>
      </template>
    </CrudQueryCard>

    <el-card class="table-card">
      <el-table v-loading="loading" :data="tableData" style="width: 100%">
        <el-table-column prop="id" :label="t('common.id')" width="90" />
        <el-table-column prop="name" :label="t('system.roleName')" min-width="160" />
        <el-table-column prop="code" :label="t('system.roleCode')" min-width="160" />
        <el-table-column :label="t('common.enabled')" width="110">
          <template #default="{ row }">
            <el-tag :type="row.enabled === 1 ? 'success' : 'info'">
              {{ enabledLabel(row.enabled) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="remark" :label="t('common.remark')" min-width="200" />

        <el-table-column :label="t('common.actions')" align="center" width="320" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="!isReadonlyRole(row)"
              v-perm="'sys:role:update'"
              size="small"
              @click="openUpdate(row)"
            >
              {{ t('common.edit') }}
            </el-button>

            <el-button
              v-if="isReadonlyRole(row)"
              v-perm="'sys:role:grant:detail'"
              size="small"
              @click="openGrant(row)"
            >
              {{ t('system.viewGrant') }}
            </el-button>

            <el-button v-else v-perm="'sys:role:grant'" size="small" @click="openGrant(row)">
              {{ t('system.grant') }}
            </el-button>

            <el-button
              v-if="!isReadonlyRole(row)"
              v-perm="'sys:role:delete'"
              size="small"
              type="danger"
              @click="onDelete(row)"
            >
              {{ t('common.delete') }}
            </el-button>

            <el-tag v-if="isSuperRole(row)" type="warning" style="margin-left: 8px">
              {{ t('system.superAdmin') }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>

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

    <el-dialog
      v-model="editVisible"
      :title="editIsUpdate ? t('system.roleEdit') : t('system.roleAdd')"
      width="520px"
    >
      <el-form ref="editFormRef" :model="editForm" label-width="110px">
        <el-form-item
          :label="t('common.tenantId')"
          prop="tenantId"
          :rules="[{ required: true, message: t('common.required') }]"
        >
          <TenantSelect v-model="editForm.tenantId" include-system :disabled="editIsUpdate" />
        </el-form-item>

        <el-form-item
          :label="t('system.roleName')"
          prop="name"
          :rules="[{ required: true, message: t('common.required') }]"
        >
          <el-input v-model="editForm.name" />
        </el-form-item>

        <el-form-item
          :label="t('system.roleCode')"
          prop="code"
          :rules="[{ required: true, message: t('common.required') }]"
        >
          <el-input v-model="editForm.code" :disabled="editIsUpdate" />
        </el-form-item>

        <el-form-item
          :label="t('common.enabled')"
          prop="enabled"
          :rules="[{ required: true, message: t('common.required') }]"
        >
          <el-select v-model="editForm.enabled" style="width: 100%">
            <el-option
              v-for="item in enabledFormOptions"
              :key="item.value"
              :label="enabledOptionLabel(item.value, item.code)"
              :value="item.value"
            />
          </el-select>
        </el-form-item>

        <el-form-item :label="t('common.remark')" prop="remark">
          <el-input v-model="editForm.remark" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="editVisible = false">
          {{ t('common.cancel') }}
        </el-button>
        <el-button
          v-perm="editIsUpdate ? 'sys:role:update' : 'sys:role:add'"
          type="primary"
          :loading="editLoading"
          @click="submitEdit"
        >
          {{ t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="grantVisible"
      :title="grantDialogTitle"
      width="400px"
      :style="{ maxWidth: '460px' }"
      @closed="onGrantClosed"
    >
      <el-alert
        v-if="grantReadonly"
        type="warning"
        :closable="false"
        :title="t('system.superAdminAllPerms')"
        style="margin-bottom: 12px"
      />

      <div v-loading="grantLoading">
        <div
          v-if="!menuTree || menuTree.length === 0"
          style="padding: 24px; text-align: center; color: #999"
        >
          {{ t('common.noData') }}
        </div>

        <el-tree
          v-else
          ref="menuTreeRef"
          :data="menuTree"
          node-key="id"
          show-checkbox
          :props="{ label: 'name', children: 'children' }"
          :check-strictly="false"
          :disabled="grantReadonly"
          :default-expand-all="false"
          style="max-height: 420px; overflow: auto"
          @check="onMenuTreeCheck"
        />
      </div>

      <template #footer>
        <el-button @click="grantVisible = false">
          {{ t('common.cancel') }}
        </el-button>
        <el-button
          v-if="!grantReadonly"
          v-perm="'sys:role:grant'"
          type="primary"
          @click="submitGrant"
        >
          {{ t('common.save') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>
