<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { Expand, Fold, Lock, Setting, User } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { setLocale, type Locale } from '@/i18n'
import { resetDynamicRoutes } from '@/router'
import { apiUpdateProfile, useAuthStore, type MenuNode } from '@/stores/auth'
import { logger } from '@/utils/logger'

const props = defineProps<{ collapsed: boolean }>()
const emit = defineEmits<{ (event: 'toggle-sider'): void }>()

const { t, te, locale } = useI18n()
const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const current = computed(() => locale.value as Locale)

type BreadcrumbItem = {
  key: string
  label: string
  node?: MenuNode
  children: MenuNode[]
}

function menuLabel(node: MenuNode) {
  const key = `menu.${node.id}`
  return te(key) ? t(key) : node.name
}

function visibleChildren(node?: MenuNode) {
  return (node?.children || [])
    .filter((child) => child.menuType !== 3 && child.visible !== 2 && child.enabled !== 2)
    .slice()
    .sort((a, b) => (a.sort ?? 0) - (b.sort ?? 0))
}

function firstPath(node?: MenuNode): string | undefined {
  if (!node || node.menuType === 3 || node.visible === 2 || node.enabled === 2) return undefined
  if (node.path) return node.path
  for (const child of visibleChildren(node)) {
    const path = firstPath(child)
    if (path) return path
  }
  return undefined
}

function findTrail(nodes: MenuNode[], path: string, parents: MenuNode[] = []): MenuNode[] {
  for (const node of nodes) {
    if (node.menuType !== 3 && node.path === path) return [...parents, node]
    const trail = findTrail(node.children || [], path, [...parents, node])
    if (trail.length) return trail
  }
  return []
}

const breadcrumbItems = computed<BreadcrumbItem[]>(() => {
  const items: BreadcrumbItem[] = [{ key: 'app', label: t('app.title'), children: [] }]
  const trail = findTrail(auth.menus, route.path)
  if (trail.length) {
    items.push(
      ...trail
        .filter((node) => node.menuType !== 3)
        .map((node) => ({
          key: String(node.id),
          label: menuLabel(node),
          node,
          children: visibleChildren(node),
        })),
    )
  } else {
    const key = route.meta?.titleKey
    const label =
      typeof key === 'string' && te(key)
        ? t(key)
        : typeof route.meta?.title === 'string'
          ? route.meta.title
          : ''
    if (label) items.push({ key: route.path, label, children: [] })
  }
  return items
})

function goBreadcrumb(node: MenuNode) {
  const path = firstPath(node)
  if (path && path !== route.path) router.push(path)
}

async function changePassword() {
  try {
    const result = await ElMessageBox.prompt(t('app.newPasswordPrompt'), t('app.changePassword'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      inputType: 'password',
      inputPattern: /^.{6,}$/,
      inputErrorMessage: t('app.passwordMinLength'),
    })
    const response = await apiUpdateProfile({ password: result.value })
    if (response.code !== 200) throw new Error(response.msg)
    ElMessage.success(t('app.passwordUpdated'))
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    logger.error('更新密码失败', error)
    ElMessage.error(t('app.updatePasswordFailed'))
  }
}

async function changeNickname() {
  try {
    const result = await ElMessageBox.prompt(t('app.newNicknamePrompt'), t('app.settings'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      inputValue: auth.user?.nickname || '',
    })
    const response = await apiUpdateProfile({ nickname: result.value })
    if (response.code !== 200) throw new Error(response.msg)
    if (auth.user) auth.user.nickname = result.value
    ElMessage.success(t('app.nicknameUpdated'))
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    logger.error('更新昵称失败', error)
    ElMessage.error(t('app.updateNicknameFailed'))
  }
}

function logout() {
  auth.logout()
  resetDynamicRoutes()
  router.push('/login')
}

function handleCommand(command: string) {
  if (command === 'password') changePassword()
  if (command === 'nickname') changeNickname()
  if (command === 'logout') logout()
}
</script>

<template>
  <header class="topbar">
    <div class="left">
      <el-button text class="collapse-btn" @click="emit('toggle-sider')">
        <el-icon><component :is="props.collapsed ? Expand : Fold" /></el-icon>
      </el-button>
      <el-breadcrumb separator=">">
        <el-breadcrumb-item v-for="item in breadcrumbItems" :key="item.key">
          <el-dropdown v-if="item.children.length" @command="goBreadcrumb">
            <span class="breadcrumb-trigger">{{ item.label }}</span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item
                  v-for="child in item.children"
                  :key="child.id"
                  :command="child"
                  :disabled="!firstPath(child)"
                >
                  {{ menuLabel(child) }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <span v-else>{{ item.label }}</span>
        </el-breadcrumb-item>
      </el-breadcrumb>
    </div>

    <div class="right">
      <el-select :model-value="current" class="locale" @update:model-value="setLocale">
        <el-option label="中文" value="zh-CN" />
        <el-option label="English" value="en-US" />
      </el-select>
      <el-dropdown @command="handleCommand">
        <div class="account">
          <el-avatar :size="32"><el-icon><User /></el-icon></el-avatar>
          <span>{{ auth.user?.nickname || auth.user?.username }}</span>
        </div>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="password">
              <el-icon><Lock /></el-icon>{{ t('app.changePassword') }}
            </el-dropdown-item>
            <el-dropdown-item command="nickname">
              <el-icon><Setting /></el-icon>{{ t('app.settings') }}
            </el-dropdown-item>
            <el-dropdown-item command="logout" divided>{{ t('app.logout') }}</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  height: 56px;
  flex: 0 0 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 18px 0 8px;
  border-bottom: 1px solid #ebeef5;
  background: #fff;
}
.left,
.right,
.account {
  display: flex;
  align-items: center;
  gap: 12px;
}
.collapse-btn {
  font-size: 18px;
}
.locale {
  width: 120px;
}
.account,
.breadcrumb-trigger {
  cursor: pointer;
}
</style>
