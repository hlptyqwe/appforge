<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import SideMenu from './side-menu.vue'
import TopBar from './top-bar.vue'
import { useSystemCore } from '@/composables/useSystemCore'
import { buildAssetUrl } from '@/utils/file-url'

const collapsed = ref(false)
const { systemCore, loadSystemCore } = useSystemCore()

onMounted(() => {
  loadSystemCore()
})

const asideWidth = ref(240)
const MIN_W = 200
const MAX_W = 360
const COLLAPSED_W = 64

const realAsideWidth = computed(() => (collapsed.value ? COLLAPSED_W : asideWidth.value))

let dragging = false
let startX = 0
let startW = 0

function toggleSider() {
  collapsed.value = !collapsed.value
}

function onDragStart(e: MouseEvent) {
  if (collapsed.value) return
  dragging = true
  startX = e.clientX
  startW = asideWidth.value
  document.body.style.userSelect = 'none'
  document.addEventListener('mousemove', onDragMove)
  document.addEventListener('mouseup', onDragEnd)
}

function onDragMove(e: MouseEvent) {
  if (!dragging) return
  const dx = e.clientX - startX
  const next = Math.min(MAX_W, Math.max(MIN_W, startW + dx))
  asideWidth.value = next
}

function onDragEnd() {
  dragging = false
  document.body.style.userSelect = ''
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup', onDragEnd)
}

onBeforeUnmount(() => {
	document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup', onDragEnd)
})
</script>

<template>
  <div class="layout">
    <aside class="sider" :style="{ width: realAsideWidth + 'px' }">
      <div class="brand">
        <img
          v-if="systemCore.siteLogo"
          :src="buildAssetUrl(systemCore.siteLogo)"
          alt="logo"
          class="brand-logo"
        />
        <span v-if="!collapsed" class="brand-text">{{ systemCore.siteName }}</span>
        <span v-else class="brand-text">{{
          systemCore.siteName ? systemCore.siteName.slice(0, 2).toUpperCase() : 'AI'
        }}</span>
      </div>

      <SideMenu :collapsed="collapsed" />

      <div v-if="!collapsed" class="resizer" @mousedown="onDragStart" />
    </aside>

    <main class="main">
      <TopBar :collapsed="collapsed" @toggle-sider="toggleSider" />

      <div class="content">
        <router-view />
      </div>
    </main>
  </div>
</template>

<style scoped>
.layout {
  display: flex;
  height: 100vh;
  overflow: hidden;
}

.sider {
  border-right: 1px solid #eee;
  display: flex;
  flex-direction: column;
  background: #fff;
  overflow: hidden;
  position: relative;
}

.brand {
  height: 56px;
  flex: 0 0 56px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.brand-logo {
  height: 32px;
  width: 32px;
  margin-right: 8px;
  border-radius: 4px;
  object-fit: contain;
}

.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
.content {
  padding: 16px;
  overflow: hidden;
  flex: 1;
  background: #f7f8fa;
  min-width: 0;
  min-height: 0;
}

.resizer {
  position: absolute;
  top: 0;
  right: 0;
  width: 6px;
  height: 100%;
  cursor: col-resize;
}
.resizer:hover {
  background: rgba(0, 0, 0, 0.06);
}
</style>
