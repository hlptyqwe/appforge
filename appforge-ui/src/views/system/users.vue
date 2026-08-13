<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { userService, roleService, type OptionGroup } from '@/services'
import { ArrowDown } from '@element-plus/icons-vue'
import type { SysUserItem } from '@/services'
import { SysRole } from '@/services/system/RoleService'
import { usePagination } from '@/composables/usePagination'
import { useLoading } from '@/composables/useLoading'
import { useForm } from '@/composables/useForm'
import { useConfirm } from '@/composables/useConfirm'
import { formatDate } from '@/utils'
import {
  findFormOptionGroup,
  findOptionGroup,
  getOptionLabel,
  getOptionValueLabel,
} from '@/utils/options'
import CrudQueryCard from '@/components/common/CrudQueryCard.vue'

const { t } = useI18n()
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

// Pagination and main list
const { pagination, updateFromResponse, resetAndLoad, prevAndLoad, nextAndLoad } =
  usePagination<number>(20)
const list = ref<SysUserItem[]>([])
const { loading, withLoading: withMainLoading } = useLoading()

// Query form
const { form: queryForm } = useForm({
  initialData: {
    keyword: '',
    enabled: undefined as number | undefined,
  },
})

async function fetchOptions() {
  try {
    const res = await userService.getOptions()
    if (res.code !== 200) throw new Error(res.msg || 'options failed')
    optionGroups.value = res.data || []
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : t('common.loadFailed'))
  }
}

async function fetchList() {
  await withMainLoading(async () => {
    try {
      const res = await userService.getList({
        keyword: queryForm.keyword || undefined,
        enabled: queryForm.enabled === 0 ? undefined : queryForm.enabled,
        cursor: pagination.cursor,
        limit: pagination.limit,
      })
      if (res.code !== 200) throw new Error(res.msg || 'list failed')
      list.value = res.data || []
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
  queryForm.keyword = ''
  queryForm.enabled = undefined
  resetAndLoad(fetchList)
}

function nextPage() {
  nextAndLoad(fetchList)
}

function prevPage() {
  prevAndLoad(fetchList)
}

const { loading: roleLoading, withLoading: withRoleLoading } = useLoading()
const roles = ref<SysRole[]>([])
async function fetchRoles() {
  await withRoleLoading(async () => {
    try {
      const res = await roleService.getList({ cursor: undefined, limit: 9999, enabled: 1 })
      if (res.code !== 200) throw new Error(res.msg || 'role list failed')
      roles.value = res.data || []
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : t('common.loadFailed'))
    }
  })
}

const editVisible = ref(false)
const editMode = ref<'create' | 'update'>('create')
const { form: editForm } = useForm({
  initialData: {
    id: 0,
    username: '',
    password: '',
    nickname: '',
    enabled: 1,
    roleIds: [] as number[],
    appScope: 1,
  },
})
const selectableRoles = computed(() =>
	roles.value.filter((role) => role.appScope === 1),
)
const { loading: editFormLoading, withLoading: withEditLoading } = useLoading()

function openCreate() {
  editMode.value = 'create'
  editForm.id = 0
  editForm.username = ''
  editForm.password = ''
  editForm.nickname = ''
  editForm.enabled = 1
  editForm.roleIds = []
  editForm.appScope = 1
  editVisible.value = true
}

function openEdit(row: SysUserItem) {
  if (isProtectedUser(row)) return
  editMode.value = 'update'
  editForm.id = row.id
  editForm.username = row.username
  editForm.password = ''
  editForm.nickname = row.nickname || ''
  editForm.enabled = row.enabled
  editForm.roleIds = (row.roleIds || []).slice()
  editForm.appScope = row.appScope || 1
  editVisible.value = true
}

async function submitEdit() {
  await withEditLoading(async () => {
    try {
      if (editMode.value === 'create') {
        if (!editForm.username || !editForm.password) {
          ElMessage.warning(t('common.pleaseInputAccountAndPassword'))
          return
        }
        const res = await userService.create({
          username: editForm.username,
          password: editForm.password,
          nickname: editForm.nickname || undefined,
          enabled: editForm.enabled,
          roleIds: editForm.roleIds,
          appScope: editForm.appScope,
        })
        if (res.code !== 200) throw new Error(res.msg || 'create failed')
        ElMessage.success(t('common.success'))
      } else {
        const res = await userService.update(editForm.id, {
          nickname: editForm.nickname || undefined,
          enabled: editForm.enabled,
          roleIds: editForm.roleIds,
          appScope: editForm.appScope,
        })
        if (res.code !== 200) throw new Error(res.msg || 'update failed')
        ElMessage.success(t('common.success'))
      }
      editVisible.value = false
      fetchList()
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
    }
  })
}

const { confirm } = useConfirm()

async function onDelete(row: SysUserItem) {
  if (isProtectedUser(row)) return
  try {
    await confirm(t('common.confirmDeleteUser', { username: row.username }), { type: 'warning' })
    const res = await userService.delete(row.id)
    if (res.code !== 200) throw new Error(res.msg || 'delete failed')
    ElMessage.success(t('common.success'))
    fetchList()
  } catch (error: unknown) {
    if ((error instanceof Error ? error.message : '') === 'cancel') return
    ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
  }
}

async function onToggleEnabled(row: SysUserItem) {
  if (isProtectedUser(row)) return
  try {
    const next = row.enabled === 1 ? 2 : 1
    const res = await userService.updateUserEnabled(row.id, next)
    if (res.code !== 200) throw new Error(res.msg || 'enabled failed')
    ElMessage.success(t('common.success'))
    fetchList()
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
  }
}

const pwdVisible = ref(false)
const { form: pwdForm } = useForm({
  initialData: { id: 0, username: '', password: '' },
})
const { loading: pwdSubmitLoading, withLoading: withPwdLoading } = useLoading()

function openResetPwd(row: SysUserItem) {
  if (isProtectedUser(row)) return
  pwdForm.id = row.id
  pwdForm.username = row.username
  pwdForm.password = ''
  pwdVisible.value = true
}

async function submitResetPwd() {
  await withPwdLoading(async () => {
    try {
      if (!pwdForm.password) {
        ElMessage.warning(t('common.pleaseInputNewPassword'))
        return
      }
      const res = await userService.resetPassword(pwdForm.id, pwdForm.password)
      if (res.code !== 200) throw new Error(res.msg || 'reset pwd failed')
      ElMessage.success(t('common.success'))
      pwdVisible.value = false
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
    }
  })
}

const roleVisible = ref(false)
const { form: roleForm } = useForm({
  initialData: { userId: 0, username: '', roleIds: [] as number[] },
})
const { loading: roleAssignLoading, withLoading: withRoleAssignLoading } = useLoading()

function openAssignRoles(row: SysUserItem) {
  if (isProtectedUser(row)) return
  roleForm.userId = row.id
  roleForm.username = row.username
  roleForm.roleIds = (row.roleIds || []).slice()
  roleVisible.value = true
}

async function submitAssignRoles() {
  await withRoleAssignLoading(async () => {
    try {
      const res = await userService.assignUserRoles(roleForm.userId, roleForm.roleIds)
      if (res.code !== 200) throw new Error(res.msg || 'assign roles failed')
      ElMessage.success(t('common.success'))
      roleVisible.value = false
      fetchList()
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
    }
  })
}

// ---------- Google 2FA ----------
const g2Visible = ref(false)
const { form: g2User } = useForm({
  initialData: { userId: 0, username: '' },
})
const { form: g2Init } = useForm({
  initialData: { secret: '', otpauthUrl: '', qrCode: '' },
})
const { form: g2Form } = useForm({
  initialData: { code: '' },
})
const { loading: g2InitLoading, withLoading: withG2InitLoading } = useLoading()
const { loading: g2EnableLoading, withLoading: withG2EnableLoading } = useLoading()
const { loading: g2DisableLoading, withLoading: withG2DisableLoading } = useLoading()

function openGoogle2fa(row: SysUserItem) {
  g2User.userId = row.id
  g2User.username = row.username
  g2Init.secret = ''
  g2Init.otpauthUrl = ''
  g2Init.qrCode = ''
  g2Form.code = ''
  g2Visible.value = true
}

async function doG2Init() {
  await withG2InitLoading(async () => {
    try {
      const res = await userService.initGoogle2FA(g2User.userId)
      if (res.code !== 200) throw new Error(res.msg || 'init failed')
      g2Init.secret = res.data?.secret || ''
      g2Init.otpauthUrl = res.data?.otpauthUrl || ''
      g2Init.qrCode = res.data?.qrCode || ''
      ElMessage.success(t('common.success'))
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
    }
  })
}

async function doG2Bind() {
  try {
    if (!g2Form.code) {
      ElMessage.warning(t('common.pleaseInputCode'))
      return
    }
    const res = await userService.bindGoogle2FA(g2User.userId, g2Init.secret, g2Form.code)
    if (res.code !== 200) throw new Error(res.msg || 'bind failed')
    ElMessage.success(t('common.success'))
    fetchList()
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
  }
}

async function copySecret() {
  if (!g2Init.secret) {
    ElMessage.warning(t('common.noData'))
    return
  }
  try {
    await navigator.clipboard.writeText(g2Init.secret)
    ElMessage.success(t('common.copied'))
  } catch (_) {
    ElMessage.error(t('common.copyFailed'))
  }
}

async function copyOtpauthUrl() {
  if (!g2Init.otpauthUrl) {
    ElMessage.warning(t('common.noData'))
    return
  }
  try {
    await navigator.clipboard.writeText(g2Init.otpauthUrl)
    ElMessage.success(t('common.copied'))
  } catch (_) {
    ElMessage.error(t('common.copyFailed'))
  }
}

async function doG2Enable() {
  await withG2EnableLoading(async () => {
    try {
      if (!g2Form.code) {
        ElMessage.warning(t('common.pleaseInputCode'))
        return
      }
      const res = await userService.enableGoogle2FA(g2User.userId, g2Form.code)
      if (res.code !== 200) throw new Error(res.msg || 'enable failed')
      ElMessage.success(t('common.success'))
      fetchList()
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
    }
  })
}

async function doG2Disable() {
  await withG2DisableLoading(async () => {
    try {
      const res = await userService.disableGoogle2FA(g2User.userId, g2Form.code || undefined)
      if (res.code !== 200) throw new Error(res.msg || 'disable failed')
      ElMessage.success(t('common.success'))
      fetchList()
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
    }
  })
}

async function doG2Reset() {
  try {
    await confirm(t('common.confirmReset2fa'), { type: 'warning' })
    const res = await userService.resetGoogle2FA(g2User.userId)
    if (res.code !== 200) throw new Error(res.msg || 'reset failed')
    ElMessage.success(t('common.success'))
    fetchList()
  } catch (error: unknown) {
    if ((error instanceof Error ? error.message : '') === 'cancel') return
    ElMessage.error(error instanceof Error ? error.message : t('common.failed'))
  }
}

const roleNameMap = computed(() => {
  const m = new Map<number, string>()
  roles.value.forEach((r) => m.set(r.id, r.name))
  return m
})

const roleMap = computed(() => {
  const m = new Map<number, SysRole>()
  roles.value.forEach((r) => m.set(r.id, r))
  return m
})

function isProtectedUser(row: SysUserItem) {
  if (row.id === 1) return true
  return (row.roleIds || []).some((roleId) => {
    const code = roleMap.value.get(roleId)?.code
    return code === 'super_admin' || code === 'tenant_super_admin' || row.userType === 2
  })
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

onMounted(async () => {
  await Promise.all([fetchRoles(), fetchList(), fetchOptions()])
})
</script>

<template>
  <div class="module-page">
    <CrudQueryCard :model="queryForm" @search="loadList" @reset="resetQuery">
      <el-form-item :label="t('common.keyword')">
        <el-input
          v-model="queryForm.keyword"
          clearable
          :placeholder="t('common.accountNicknameKeyword')"
          @keyup.enter="loadList"
        />
      </el-form-item>

      <el-form-item :label="t('common.enabled')">
        <el-select v-model="queryForm.enabled" clearable :placeholder="t('common.enabled')">
          <el-option
            v-for="o in enabledSelectOptions"
            :key="o.value"
            :label="enabledOptionLabel(o.value, o.code)"
            :value="o.value"
          />
        </el-select>
      </el-form-item>

      <template #actions>
        <el-button v-perm="'sys:user:add'" type="primary" @click="openCreate">
          {{ t('common.add') }}
        </el-button>
      </template>
    </CrudQueryCard>

    <el-card class="table-card">
      <el-table v-loading="loading" :data="list" row-key="id">
        <el-table-column prop="id" :label="t('common.id')" width="80" />
        <el-table-column prop="username" :label="t('common.username')" min-width="140" />
        <el-table-column prop="nickname" :label="t('common.nickname')" min-width="160" />
        <el-table-column :label="t('common.role')" min-width="180">
          <template #default="{ row }">
            <el-tag v-for="rid in row.roleIds || []" :key="rid" style="margin-right: 6px">
              {{ roleNameMap.get(rid) || '#' + rid }}
            </el-tag>
            <span v-if="!(row.roleIds && row.roleIds.length)" style="color: #999">-</span>
          </template>
        </el-table-column>

        <el-table-column :label="t('common.google2fa')" width="110">
          <template #default="{ row }">
            <el-tag :type="row.google2FaEnabled === 1 ? 'success' : 'info'">
              {{ row.google2FaEnabled === 1 ? t('common.enabled') : t('common.disabled') }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="t('common.enabled')" width="110">
          <template #default="{ row }">
            <el-tag :type="row.enabled === 1 ? 'success' : 'danger'">
              {{ enabledLabel(row.enabled) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="t('common.createTimes')" min-width="170">
          <template #default="{ row }">
            <span style="color: #666">{{ formatDate(row.createTimes) }}</span>
          </template>
        </el-table-column>

        <el-table-column
          :label="t('common.actions')"
          align="center"
          width="90"
          fixed="right"
        >
          <template #default="{ row }">
            <el-dropdown trigger="click">
              <el-button size="small">
                {{ t('common.actions') }}
                <el-icon class="el-icon--right">
                  <ArrowDown />
                </el-icon>
              </el-button>

              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item
                    v-perm="'sys:user:update'"
                    :disabled="isProtectedUser(row)"
                    @click="openEdit(row)"
                  >
                    {{ t('perms.sys:user:update') }}
                  </el-dropdown-item>

                  <el-dropdown-item
                    v-perm="'sys:user:resetpwd'"
                    :disabled="isProtectedUser(row)"
                    @click="openResetPwd(row)"
                  >
                    {{ t('perms.sys:user:resetpwd') }}
                  </el-dropdown-item>

                  <el-dropdown-item
                    v-perm="'sys:user:assignrole'"
                    :disabled="isProtectedUser(row)"
                    @click="openAssignRoles(row)"
                  >
                    {{ t('perms.sys:user:assignrole') }}
                  </el-dropdown-item>

                  <el-dropdown-item v-perm="'sys:user:google2fa'" @click="openGoogle2fa(row)">
                    {{ t('perms.sys:user:google2fa') }}
                  </el-dropdown-item>

                  <el-dropdown-item
                    v-perm="'sys:user:status'"
                    divided
                    :disabled="isProtectedUser(row)"
                    @click="onToggleEnabled(row)"
                  >
                    {{ row.enabled === 1 ? t('common.disable') : t('common.enable') }}
                  </el-dropdown-item>

                  <el-dropdown-item
                    v-perm="'sys:user:delete'"
                    :disabled="isProtectedUser(row)"
                    @click="onDelete(row)"
                  >
                    <span style="color: var(--el-color-danger)">
                      {{ t('perms.sys:user:delete') }}
                    </span>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
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
      :title="editMode === 'create' ? t('common.addUser') : t('common.editUser')"
      width="520px"
    >
      <el-form label-width="100px">
        <el-form-item v-if="editMode === 'create'" :label="t('common.username')">
          <el-input v-model="editForm.username" />
        </el-form-item>
        <el-form-item v-if="editMode === 'create'" :label="t('common.password')">
          <el-input v-model="editForm.password" type="password" show-password />
        </el-form-item>
        <el-form-item :label="t('common.nickname')">
          <el-input v-model="editForm.nickname" />
        </el-form-item>
        <el-form-item :label="t('common.enabled')">
          <el-select v-model="editForm.enabled" style="width: 100%">
            <el-option
              v-for="item in enabledFormOptions"
              :key="item.value"
              :label="enabledOptionLabel(item.value, item.code)"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('common.role')">
          <el-select
            v-model="editForm.roleIds"
            multiple
            filterable
            style="width: 100%"
            :loading="roleLoading"
            :no-data-text="t('common.noData')"
          >
            <el-option
              v-for="r in selectableRoles"
              :key="r.id"
              :label="r.name"
              :value="r.id"
            />
          </el-select>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="editVisible = false">
          {{ t('common.cancel') }}
        </el-button>
        <el-button
          v-perm="editMode === 'create' ? 'sys:user:add' : 'sys:user:update'"
          type="primary"
          :loading="editFormLoading"
          @click="submitEdit"
        >
          {{ t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="pwdVisible" :title="t('common.resetPassword')" width="420px">
      <el-form label-width="90px">
        <el-form-item :label="t('common.username')">
          <el-input :model-value="pwdForm.username" disabled />
        </el-form-item>
        <el-form-item :label="t('common.newPassword')">
          <el-input v-model="pwdForm.password" type="password" show-password />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="pwdVisible = false">
          {{ t('common.cancel') }}
        </el-button>
        <el-button
          v-perm="'sys:user:resetpwd'"
          type="primary"
          :loading="pwdSubmitLoading"
          @click="submitResetPwd"
        >
          {{ t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="roleVisible" :title="t('common.assignRoles')" width="520px">
      <el-form label-width="90px">
        <el-form-item :label="t('common.username')">
          <el-input :model-value="roleForm.username" disabled />
        </el-form-item>
        <el-form-item :label="t('common.role')">
          <el-select
            v-model="roleForm.roleIds"
            multiple
            filterable
            style="width: 100%"
            :loading="roleLoading"
          >
            <el-option
              v-for="r in roles"
              :key="r.id"
              :label="r.name"
              :value="r.id"
            />
          </el-select>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="roleVisible = false">
          {{ t('common.cancel') }}
        </el-button>
        <el-button
          v-perm="'sys:user:assignrole'"
          type="primary"
          :loading="roleAssignLoading"
          @click="submitAssignRoles"
        >
          {{ t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Google 2FA -->
    <el-dialog v-model="g2Visible" :title="t('common.google2faManage')" width="680px">
      <div style="display: flex; gap: 16px">
        <div style="flex: 1">
          <div style="margin-bottom: 8px; color: #666">
            {{ t('common.user') }}：{{ g2User.username }}（ID: {{ g2User.userId }}）
          </div>

          <div style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 12px">
            <el-button v-perm="'sys:user:2fa:init'" :loading="g2InitLoading" @click="doG2Init">
              {{ t('perms.sys:user:2fa:init') }}
            </el-button>
            <el-button
              v-perm="'sys:user:2fa:enable'"
              type="success"
              :loading="g2EnableLoading"
              @click="doG2Enable"
            >
              {{ t('perms.sys:user:2fa:enable') }}
            </el-button>
            <el-button
              v-perm="'sys:user:2fa:disable'"
              type="warning"
              :loading="g2DisableLoading"
              @click="doG2Disable"
            >
              {{ t('perms.sys:user:2fa:disable') }}
            </el-button>
            <el-button v-perm="'sys:user:2fa:reset'" type="danger" @click="doG2Reset">
              {{ t('perms.sys:user:2fa:reset') }}
            </el-button>
          </div>

          <el-form label-width="100px">
            <el-form-item :label="t('common.code')">
              <div style="display: flex; gap: 8px">
                <el-input
                  v-model="g2Form.code"
                  :placeholder="t('common.enterGoogleCode')"
                  style="flex: 1"
                />
                <el-button v-perm="'sys:user:2fa:bind'" @click="doG2Bind">
                  {{ t('perms.sys:user:2fa:bind') }}
                </el-button>
              </div>
            </el-form-item>

            <el-form-item :label="t('common.secret')">
              <div style="display: flex; gap: 8px">
                <el-input :model-value="g2Init.secret" readonly style="flex: 1" />
                <el-button :disabled="!g2Init.secret" @click="copySecret">
                  {{ t('common.copy') }}
                </el-button>
              </div>
            </el-form-item>

            <el-form-item :label="t('common.otpauthUrl')">
              <div style="display: flex; gap: 8px">
                <el-input :model-value="g2Init.otpauthUrl" readonly style="flex: 1" />
                <el-button :disabled="!g2Init.otpauthUrl" @click="copyOtpauthUrl">
                  {{ t('common.copy') }}
                </el-button>
              </div>
            </el-form-item>
          </el-form>
        </div>

        <div style="width: 260px">
          <div style="margin-bottom: 8px; color: #666">
            {{ t('common.qrCode') }}
          </div>
          <div
            style="
              background: #f7f8fa;
              border: 1px solid #eee;
              border-radius: 8px;
              padding: 12px;
              min-height: 240px;
            "
          >
            <img v-if="g2Init.qrCode" :src="g2Init.qrCode" style="width: 100%; height: auto">
            <div v-else style="color: #999">
              {{ t('common.click2faBindGenerateQrCode') }}
            </div>
          </div>
          <div v-if="g2Init.qrCode" style="margin-top: 8px; font-size: 12px; color: #666">
            {{ t('common.scanQrCodeWithGoogleAuthenticator') }}
          </div>
        </div>
      </div>

      <template #footer>
        <el-button @click="g2Visible = false">
          {{ t('common.cancel') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>
