<script setup lang='ts'>
import { computed } from 'vue'
import type { MenuNode } from '@/stores/auth'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'

const props = defineProps<{
  node: MenuNode
}>()

const router = useRouter()
const { t, te } = useI18n()
const iconMap = ElementPlusIconsVue as Record<string, any>

function labelById(id: number, fallback: string) {
  const key = `menu.${id}`
  return te(key) ? t(key) : fallback
}

function iconComp(icon?: string) {
  if (!icon) return iconMap.Menu
  return iconMap[icon] || iconMap.Menu
}

function go(path?: string) {
  if (path) router.push(path)
}

const children = computed(() =>
  (props.node.children || [])
    .filter((x) => x.menuType !== 3 && x.visible !== 0 && x.enabled !== 0)
    .slice()
    .sort((a, b) => (a.sort ?? 0) - (b.sort ?? 0)),
)
</script>

<template>
  <!-- 目录 -->
  <el-sub-menu v-if="node.menuType === 1" :index="String(node.id)">
    <template #title>
      <el-icon>
        <component :is="iconComp(node.icon)" />
      </el-icon>
      <span>{{ labelById(node.id, node.name) }}</span>
    </template>

    <MenuItem v-for="c in children" :key="c.id" :node="c" />
  </el-sub-menu>

  <!-- 菜单 -->
  <el-menu-item v-else-if="node.menuType === 2" :index="node.path" @click="go(node.path)">
    <el-icon>
      <component :is="iconComp(node.icon)" />
    </el-icon>
    <template #title>
      {{ labelById(node.id, node.name) }}
    </template>
  </el-menu-item>
</template>
