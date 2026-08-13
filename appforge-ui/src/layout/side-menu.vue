<script setup lang='ts'>
import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import MenuItem from './menu-item.vue'

const props = defineProps<{
  collapsed: boolean
}>()

const auth = useAuthStore()
const route = useRoute()
const { t } = useI18n()

const iconMap = ElementPlusIconsVue as Record<string, any>

const menuTree = computed(() => {
  return auth.menus
})

function iconComp(icon?: string) {
  if (!icon) return iconMap.Menu
  return iconMap[icon] || iconMap.Menu
}
</script>

<template>
  <el-menu
    class="aside-menu"
    router
    :default-active="route.path"
    :collapse="props.collapsed"
    :collapse-transition="false"
  >
    <el-menu-item index="/home">
      <el-icon>
        <component :is="iconComp('House')" />
      </el-icon>
      <template #title>
        {{ t('route.home') }}
      </template>
    </el-menu-item>

    <MenuItem v-for="m in menuTree" :key="m.id" :node="m" />
  </el-menu>
</template>

<style scoped>
.aside-menu {
  flex: 1;
  min-height: 0;
  border-right: none;
  overflow-x: hidden;
  overflow-y: auto;
}

:deep(.el-menu--collapse) {
  width: 100%;
}
</style>
