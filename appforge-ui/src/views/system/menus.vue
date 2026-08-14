<template>
  <div class="module-page">
    <CrudQueryCard :model="queryForm" @search="loadList" @reset="resetQuery">
      <el-form-item :label="t('common.keyword')">
        <el-input
          v-model="queryForm.keyword"
          :placeholder="t('system.pleaseInputMenuName')"
          clearable
          @keyup.enter="loadList"
        />
      </el-form-item>

      <el-form-item :label="t('system.menuType')">
        <el-select v-model="queryForm.menuType" clearable :placeholder="t('common.all')">
          <el-option
            v-for="item in menuTypeOptions"
            :key="item.value"
            :label="getOptionLabel(t, item.code, item.value)"
            :value="item.value"
          />
        </el-select>
      </el-form-item>

      <el-form-item :label="t('common.enabled')">
        <el-select v-model="queryForm.enabled" clearable :placeholder="t('common.all')">
          <el-option
            v-for="item in enabledSelectOptions"
            :key="item.value"
            :label="enabledOptionLabel(item.value, item.code)"
            :value="item.value"
          />
        </el-select>
      </el-form-item>

      <el-form-item :label="t('common.visible')">
        <el-select v-model="queryForm.visible" clearable :placeholder="t('common.all')">
          <el-option
            v-for="item in visibleSelectOptions"
            :key="item.value"
            :label="visibleOptionLabel(item.value, item.code)"
            :value="item.value"
          />
        </el-select>
      </el-form-item>

      <template #actions>
        <el-button v-perm="'sys:menu:add'" type="primary" @click="handleAdd(0)">
          {{ t('common.add') }}
        </el-button>
      </template>
    </CrudQueryCard>

    <el-card class="table-card" shadow="never">
      <el-table
        v-loading="loading"
        :data="tableData"
        row-key="id"
        border
        :tree-props="{ children: 'children' }"
      >
        <el-table-column :label="t('system.name')" prop="name" min-width="180">
          <template #default="{ row }">
            {{ getMenuTitle(row) }}
          </template>
        </el-table-column>

        <el-table-column :label="t('system.menuType')" width="100" align="center">
          <template #default="{ row }">
            <el-tag type="info">
              {{ getOptionValueLabel(optionGroups, 'menuType', row.menuType, t) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          :label="t('system.path')"
          prop="path"
          min-width="150"
          show-overflow-tooltip
        />
        <el-table-column
          :label="t('system.component')"
          prop="component"
          min-width="180"
          show-overflow-tooltip
        />

        <el-table-column :label="t('system.icon')" width="160">
          <template #default="{ row }">
            <div v-if="row.icon" class="menu-icon-cell">
              <el-icon v-if="resolveIconComponent(row.icon)" class="menu-icon-preview">
                <component :is="resolveIconComponent(row.icon)" />
              </el-icon>
              <span class="menu-icon-text">{{ row.icon }}</span>
            </div>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>

        <el-table-column
          :label="t('system.perms')"
          prop="perms"
          min-width="180"
          show-overflow-tooltip
        />
        <el-table-column :label="t('system.sort')" prop="sort" width="80" align="center" />

        <el-table-column :label="t('common.visible')" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.visible === 1 ? 'success' : 'info'">
              {{ visibleLabel(row.visible) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="t('common.enabled')" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.enabled === 1 ? 'success' : 'danger'">
              {{ enabledLabel(row.enabled) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="t('common.actions')" width="180" align="center" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.menuType !== 3"
              v-perm="'sys:menu:add'"
              link
              type="primary"
              @click="handleAdd(row.id)"
            >
              {{ t('system.addChild') }}
            </el-button>
            <el-button v-perm="'sys:menu:update'" link type="primary" @click="handleEdit(row)">
              {{ t('common.edit') }}
            </el-button>
            <el-button v-perm="'sys:menu:delete'" link type="danger" @click="handleDelete(row)">
              {{ t('common.delete') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? t('system.addMenu') : t('system.editMenu')"
      width="760px"
      destroy-on-close
    >
      <el-form ref="formRef" :model="formData" :rules="rules" label-width="100px">
        <el-form-item :label="t('system.parentMenu')" prop="parentId">
          <el-tree-select
            v-model="formData.parentId"
            :data="parentTreeOptions"
            node-key="id"
            check-strictly
            :render-after-expand="false"
            :props="{ label: 'name', children: 'children', value: 'id' }"
            :placeholder="t('system.pleaseSelectParentMenu')"
            style="width: 100%"
          />
        </el-form-item>

        <el-form-item :label="t('system.menuName')" prop="name">
          <el-input v-model="formData.name" :placeholder="t('system.pleaseInputMenuName')" />
        </el-form-item>

        <el-form-item :label="t('system.menuType')" prop="menuType">
          <el-select v-model="formData.menuType" style="width: 100%" @change="handleMenuTypeChange">
            <el-option
              v-for="item in menuTypeFormOptions"
              :key="item.value"
              :label="getOptionLabel(t, item.code, item.value)"
              :value="item.value"
            />
          </el-select>
        </el-form-item>

        <el-row v-if="formData.menuType !== 3" :gutter="16">
          <el-col :span="12">
            <el-form-item :label="t('system.path')" prop="path">
              <el-input v-model="formData.path" :placeholder="t('system.pleaseInputPath')" />
            </el-form-item>
          </el-col>

          <el-col v-if="formData.menuType === 2" :span="12">
            <el-form-item :label="t('system.component')" prop="component">
              <el-input
                v-model="formData.component"
                :placeholder="t('system.pleaseInputComponent')"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row v-if="formData.menuType !== 3" :gutter="16">
          <el-col :span="14">
            <el-form-item :label="t('system.icon')" prop="icon">
              <div class="icon-picker-box">
                <el-input
                  v-model="formData.icon"
                  :placeholder="t('system.pleaseInputIcon')"
                  clearable
                >
                  <template #prepend>
                    <el-icon v-if="currentIconComponent">
                      <component :is="currentIconComponent" />
                    </el-icon>
                    <span v-else class="text-muted">-</span>
                  </template>
                </el-input>

                <el-popover placement="bottom-start" :width="520" trigger="click">
                  <template #reference>
                    <el-button>{{ t('system.selectIcon') }}</el-button>
                  </template>

                  <div class="icon-panel">
                    <div
                      v-for="iconName in iconNames"
                      :key="iconName"
                      class="icon-item"
                      @click="selectIcon(iconName)"
                    >
                      <el-icon class="icon-item-preview">
                        <component :is="resolveIconComponent(iconName)" />
                      </el-icon>
                      <span class="icon-item-text">{{ iconName }}</span>
                    </div>
                  </div>
                </el-popover>

                <el-button @click="clearIcon">
                  {{ t('common.clear') }}
                </el-button>
              </div>
            </el-form-item>
          </el-col>

          <el-col :span="10">
            <el-form-item :label="t('system.iconPreview')">
              <div class="icon-preview-box">
                <el-icon v-if="currentIconComponent" class="icon-preview-large">
                  <component :is="currentIconComponent" />
                </el-icon>
                <span v-else class="text-muted">{{ t('system.noIcon') }}</span>
              </div>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item :label="t('system.sort')" prop="sort">
              <el-input-number
                v-model="formData.sort"
                :min="0"
                :max="999999"
                style="width: 100%"
                @change="handleSortChange"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item v-if="formData.menuType === 3" :label="t('system.perms')" prop="perms">
          <el-input v-model="formData.perms" :placeholder="t('system.pleaseInputPerms')" />
        </el-form-item>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item :label="t('common.visible')" prop="visible">
              <el-select v-model="formData.visible" style="width: 100%">
                <el-option
                  v-for="item in visibleFormOptions"
                  :key="item.value"
                  :label="visibleOptionLabel(item.value, item.code)"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item :label="t('common.enabled')" prop="enabled">
              <el-select v-model="formData.enabled" style="width: 100%">
                <el-option
                  v-for="item in enabledFormOptions"
                  :key="item.value"
                  :label="enabledOptionLabel(item.value, item.code)"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">
          {{ t('common.cancel') }}
        </el-button>
        <el-button
          v-perm="dialogType === 'add' ? 'sys:menu:add' : 'sys:menu:update'"
          type="primary"
          :loading="submitLoading"
          @click="handleSubmit"
        >
          {{ t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import { menuService, type OptionGroup } from '@/services'
import { useLoading } from '@/composables/useLoading'
import { useForm } from '@/composables/useForm'
import { useConfirm } from '@/composables/useConfirm'
import {
  findFormOptionGroup,
  findOptionGroup,
  getOptionLabel,
  getOptionValueLabel,
} from '@/utils/options'
import type {
  SysMenuCreateReq,
  SysMenuItem,
  SysMenuTreeItem,
  SysMenuUpdateReq,
} from '@/services/system/MenuService'
import CrudQueryCard from '@/components/common/CrudQueryCard.vue'

const { t, te } = useI18n()

type DialogType = 'add' | 'edit'

type MenuFormData = {
  id: number | undefined
  parentId: number
  name: string
  menuType: number
  path: string
  component: string
  icon: string
  sort: number
  visible: number
  enabled: number
  perms: string
  appScope: number
}

type QueryFormData = {
  keyword: string
  menuType: number | undefined
  enabled: number | undefined
  visible: number | undefined
  appScope: number | undefined
}

const iconMap = ElementPlusIconsVue as Record<string, Component>
const iconNames = Object.keys(iconMap).sort()
const optionGroups = ref<OptionGroup[]>([])
const menuTypeOptions = computed(() => findOptionGroup(optionGroups.value, 'menuType'))
const menuTypeFormOptions = computed(() => findFormOptionGroup(optionGroups.value, 'menuType'))
const enabledSelectOptions = computed(() => {
  const options = findOptionGroup(optionGroups.value, 'enabled')
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
const visibleSelectOptions = computed(() => {
  const options = findOptionGroup(optionGroups.value, 'visible')
  return options.length
    ? options
    : [
        { value: 0, code: 'SWITCH_UNKNOWN' },
        { value: 1, code: 'SWITCH_ON' },
        { value: 2, code: 'SWITCH_OFF' },
      ]
})
const visibleFormOptions = computed(() => {
  const options = findFormOptionGroup(optionGroups.value, 'visible')
  return options.length
    ? options
    : [
        { value: 1, code: 'SWITCH_ON' },
        { value: 2, code: 'SWITCH_OFF' },
      ]
})

// Composables
const { loading, withLoading: withMainLoading } = useLoading()
const { loading: submitLoading, withLoading: withSubmitLoading } = useLoading()
const { form: queryForm } = useForm<QueryFormData>({
  initialData: {
    keyword: '',
    menuType: undefined,
    enabled: undefined,
    visible: undefined,
    appScope: 1,
  },
})
const { confirm } = useConfirm()

// Dialog state
const dialogVisible = ref(false)
const dialogType = ref<DialogType>('add')
const formRef = ref<FormInstance>()

// Query pagination
const queryPage = { cursor: undefined, limit: 1000 }

// Menu tree data
const rawList = ref<SysMenuItem[]>([])
const tableData = ref<SysMenuTreeItem[]>([])

const createDefaultForm = (): MenuFormData => ({
  id: undefined,
  parentId: 0,
  name: '',
  menuType: 1,
  path: '',
  component: '',
  icon: '',
  sort: 0,
  visible: 1,
  enabled: 1,
  perms: '',
  appScope: 1,
})

const { form: formData } = useForm<MenuFormData>({
  initialData: createDefaultForm(),
})

const currentEditId = computed(() => formData.id ?? 0)
const isTopMenu = computed(() => Number(formData.parentId) === 0)

const currentIconComponent = computed(() => {
  return resolveIconComponent(formData.icon)
})

const childrenIdMap = computed(() => {
  const map = new Map<number, number[]>()

  const dfs = (node: SysMenuTreeItem): number[] => {
    const ids: number[] = []
    for (const child of node.children || []) {
      ids.push(child.id)
      ids.push(...dfs(child))
    }
    map.set(node.id, ids)
    return ids
  }

  for (const node of tableData.value) {
    dfs(node)
  }

  return map
})

const parentTreeOptions = computed(() => {
  const excludeIds = new Set<number>()

  if (dialogType.value === 'edit' && currentEditId.value) {
    excludeIds.add(currentEditId.value)
    const childIds = childrenIdMap.value.get(currentEditId.value) || []
    childIds.forEach((id) => excludeIds.add(id))
  }

  const filterNodes = (nodes: SysMenuTreeItem[]): SysMenuTreeItem[] => {
    return nodes
      .filter(
        (node) =>
          node.appScope === formData.appScope && node.menuType !== 3 && !excludeIds.has(node.id),
      )
      .map((node) => ({
        ...node,
        children: filterNodes(node.children || []),
      }))
  }

  return [
    {
      id: 0,
      name: t('system.topMenu'),
      children: filterNodes(tableData.value),
    },
  ]
})

const rules = computed<FormRules>(() => ({
  parentId: [{ required: true, message: t('system.pleaseSelectParentMenu'), trigger: 'change' }],
  name: [{ required: true, message: t('system.pleaseInputMenuName'), trigger: 'blur' }],
  menuType: [{ required: true, message: t('system.pleaseSelectMenuType'), trigger: 'change' }],
  path: [
    {
      validator: (_rule, value, callback) => {
        if (formData.menuType !== 3 && !isTopMenu.value && !String(value || '').trim()) {
          callback(new Error(t('system.pleaseInputPath')))
          return
        }
        callback()
      },
      trigger: 'blur',
    },
  ],
  component: [
    {
      validator: (_rule, value, callback) => {
        if (formData.menuType === 2 && !String(value || '').trim()) {
          callback(new Error(t('system.pleaseInputComponent')))
          return
        }
        callback()
      },
      trigger: 'blur',
    },
  ],
  perms: [
    {
      validator: (_rule, value, callback) => {
        if (formData.menuType === 3 && !String(value || '').trim()) {
          callback(new Error(t('system.pleaseInputPerms')))
          return
        }
        callback()
      },
      trigger: 'blur',
    },
  ],
  sort: [{ required: true, message: t('system.pleaseInputSort'), trigger: 'blur' }],
}))

function resolveIconComponent(iconName?: string) {
  if (!iconName) return null
  return iconMap[iconName] || null
}

function selectIcon(iconName: string) {
  formData.icon = iconName
}

function clearIcon() {
  formData.icon = ''
}

function getMenuTitle(row: SysMenuItem) {
  const key = `menu.${row.id}`
  if (te(key)) return t(key)
  return row.name
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

function visibleOptionLabel(value: number | string | undefined, code?: string) {
  if (Number(value) === 1) return t('common.visible')
  if (Number(value) === 2) return t('common.hidden')
  if (Number(value) === 0) return t('common.all')
  return getOptionLabel(t, code, Number(value))
}

function visibleLabel(value: number | undefined) {
  const label = getOptionValueLabel(optionGroups.value, 'visible', value, t)
  if (label && label !== value) return label
  return visibleOptionLabel(value)
}

function buildTree(list: SysMenuItem[]): SysMenuTreeItem[] {
  const map = new Map<number, SysMenuTreeItem>()
  const roots: SysMenuTreeItem[] = []

  list.forEach((item) => {
    map.set(item.id, {
      ...item,
      children: [],
    })
  })

  map.forEach((item) => {
    if (item.parentId === 0) {
      roots.push(item)
      return
    }

    const parent = map.get(item.parentId)
    if (parent) {
      parent.children ||= []
      parent.children.push(item)
    } else {
      roots.push(item)
    }
  })

  const sortTree = (nodes: SysMenuTreeItem[]) => {
    nodes.sort((a, b) => a.sort - b.sort)
    nodes.forEach((node) => {
      if (node.children?.length) {
        sortTree(node.children)
      }
    })
  }

  sortTree(roots)
  return roots
}

function normalizeFormByType() {
  if (formData.menuType === 1) {
    formData.component = ''
    formData.perms = ''
  } else if (formData.menuType === 2) {
    formData.perms = ''
  } else if (formData.menuType === 3) {
    formData.path = ''
    formData.component = ''
    formData.icon = ''
  }
}

function handleMenuTypeChange() {
  normalizeFormByType()
  nextTick(() => {
    formRef.value?.clearValidate(['path', 'component', 'perms'])
  })
}

function handleSortChange(value: number | undefined) {
  formData.sort = Number(value ?? 0)
}

function showError(error: unknown) {
  const msg =
    error instanceof Error ? error.message : typeof error === 'string' ? error : t('common.failed')

  ElMessage.error(msg || t('common.failed'))
}

async function getList() {
  await withMainLoading(async () => {
    try {
      const res = await menuService.getList({
        cursor: queryPage.cursor,
        limit: queryPage.limit,
        keyword: queryForm.keyword || '',
        menuType: queryForm.menuType ?? 0,
        enabled: queryForm.enabled ?? 0,
        visible: queryForm.visible ?? 0,
        appScope: queryForm.appScope,
      })

      if (res.code != 200) {
        ElMessage.error(res.msg || t('common.failed'))
        return
      }
      rawList.value = res?.data || []
      tableData.value = buildTree(rawList.value)
    } catch (error) {
      rawList.value = []
      tableData.value = []
      showError(error)
    }
  })
}

async function loadOptions() {
  const res = await menuService.getOptions()
  optionGroups.value = res.data || []
}

function loadList() {
  queryPage.cursor = undefined
  getList()
}

function resetQuery() {
  queryForm.keyword = ''
  queryForm.menuType = undefined
  queryForm.enabled = undefined
  queryForm.visible = undefined
  queryForm.appScope = 1
  queryPage.cursor = undefined
  queryPage.limit = 20
  getList()
}

function resetFormData() {
  Object.assign(formData, createDefaultForm())
  nextTick(() => {
    formRef.value?.clearValidate()
  })
}

function handleAdd(parentId = 0) {
  resetFormData()
  dialogType.value = 'add'
  formData.parentId = parentId
  dialogVisible.value = true
}

function handleEdit(row: SysMenuItem) {
  resetFormData()
  dialogType.value = 'edit'

  Object.assign(formData, {
    id: row.id,
    parentId: row.parentId,
    name: row.name,
    menuType: row.menuType,
    path: row.path,
    component: row.component,
    icon: row.icon,
    sort: row.sort,
    visible: row.visible,
    enabled: row.enabled,
    perms: row.perms,
    appScope: row.appScope || 1,
  })

  dialogVisible.value = true

  nextTick(() => {
    formRef.value?.clearValidate()
  })
}

async function handleDelete(row: SysMenuItem) {
  try {
    await confirm(t('system.confirmDeleteMenu', { name: getMenuTitle(row) }), { type: 'warning' })

    const res = await menuService.delete(row.id)
    if (res.code != 200) {
      ElMessage.error(res.msg || t('common.failed'))
      return
    }

    ElMessage.success(t('common.success'))
    await getList()
  } catch (error: unknown) {
    if (error === 'cancel') {
      return
    }
    showError(error)
  }
}

async function handleSubmit() {
  normalizeFormByType()
  await formRef.value?.validate()

  await withSubmitLoading(async () => {
    try {
      if (dialogType.value === 'add') {
        const payload: SysMenuCreateReq = {
          parentId: formData.parentId,
          name: formData.name.trim(),
          menuType: formData.menuType,
          path: formData.path.trim(),
          component: formData.component.trim(),
          icon: formData.icon.trim(),
          sort: Number(formData.sort || 0),
          visible: formData.visible,
          enabled: formData.enabled,
          perms: formData.perms.trim(),
          appScope: formData.appScope,
        }

        const res = await menuService.create(payload)
        if (res.code != 200) {
          ElMessage.error(res.msg || t('common.failed'))
        }
      } else {
        const payload: SysMenuUpdateReq = {
          id: formData.id as number,
          parentId: formData.parentId,
          name: formData.name.trim(),
          menuType: formData.menuType,
          path: formData.path.trim(),
          component: formData.component.trim(),
          icon: formData.icon.trim(),
          sort: Number(formData.sort || 0),
          visible: formData.visible,
          enabled: formData.enabled,
          perms: formData.perms.trim(),
          appScope: formData.appScope,
        }

        const res = await menuService.update(formData.id!, payload)
        if (res.code != 200) {
          ElMessage.error(res.msg || t('common.failed'))
        }
      }

      ElMessage.success(t('common.success'))
      dialogVisible.value = false
      await getList()
    } catch (error) {
      showError(error)
    }
  })
}

onMounted(() => {
  loadOptions()
  getList()
})
</script>

<style scoped>
.menu-icon-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.menu-icon-preview {
  font-size: 16px;
}

.menu-icon-text {
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.icon-picker-box {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.icon-picker-box :deep(.el-input) {
  flex: 1;
}

.icon-panel {
  max-height: 320px;
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.icon-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.icon-item:hover {
  border-color: var(--el-color-primary);
  background: var(--el-fill-color-light);
}

.icon-item-preview {
  font-size: 18px;
}

.icon-item-text {
  font-size: 12px;
  line-height: 1.2;
  word-break: break-all;
}

.icon-preview-box {
  width: 100%;
  min-height: 84px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed var(--el-border-color);
  border-radius: 8px;
  background: var(--el-fill-color-lighter);
}

.icon-preview-large {
  font-size: 32px;
}

.text-muted {
  color: var(--el-text-color-placeholder);
}
</style>
