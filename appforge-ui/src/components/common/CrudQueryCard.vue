<template>
  <el-card v-if="card" shadow="never" class="query-card crud-query-card">
    <template v-if="$slots.header" #header>
      <slot name="header" />
    </template>

    <el-form
      :model="model"
      inline
      :label-width="formLabelWidth"
      class="crud-query-form"
      :class="{
        'crud-query-form--auto-label': props.labelWidth === 'auto',
        'crud-query-form--has-extra-actions': !!$slots.actions,
      }"
    >
      <slot />

      <el-form-item
        v-if="showActions && (showSearch || showReset)"
        class="crud-query-actions crud-query-search-actions"
      >
        <div class="crud-query-action-inner">
          <el-button v-if="showSearch" type="primary" @click="emit('search')">
            {{ searchText || t('common.search') }}
          </el-button>

          <el-button v-if="showReset" @click="emit('reset')">
            {{ resetText || t('common.reset') }}
          </el-button>
        </div>
      </el-form-item>

      <el-form-item
        v-if="showActions && $slots.actions"
        class="crud-query-actions crud-query-extra-actions"
      >
        <div class="crud-query-action-inner">
          <slot name="actions" />
        </div>
      </el-form-item>
    </el-form>
  </el-card>

  <div v-else class="query-card crud-query-card crud-query-card--plain">
    <template v-if="$slots.header">
      <div class="crud-query-card__header">
        <slot name="header" />
      </div>
    </template>

    <el-form
      :model="model"
      inline
      :label-width="formLabelWidth"
      class="crud-query-form"
      :class="{
        'crud-query-form--auto-label': props.labelWidth === 'auto',
        'crud-query-form--has-extra-actions': !!$slots.actions,
      }"
    >
      <slot />

      <el-form-item
        v-if="showActions && (showSearch || showReset)"
        class="crud-query-actions crud-query-search-actions"
      >
        <div class="crud-query-action-inner">
          <el-button v-if="showSearch" type="primary" @click="emit('search')">
            {{ searchText || t('common.search') }}
          </el-button>

          <el-button v-if="showReset" @click="emit('reset')">
            {{ resetText || t('common.reset') }}
          </el-button>
        </div>
      </el-form-item>

      <el-form-item
        v-if="showActions && $slots.actions"
        class="crud-query-actions crud-query-extra-actions"
      >
        <div class="crud-query-action-inner">
          <slot name="actions" />
        </div>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = withDefaults(
  defineProps<{
    model?: Record<string, unknown>
    labelWidth?: string
    showSearch?: boolean
    showReset?: boolean
    showActions?: boolean
    card?: boolean
    searchText?: string
    resetText?: string
  }>(),
  {
    model: () => ({}),
    labelWidth: 'auto',
    showSearch: true,
    showReset: true,
    showActions: true,
    card: true,
    searchText: '',
    resetText: '',
  },
)

const emit = defineEmits<{
  search: []
  reset: []
}>()

const { t } = useI18n()
const formLabelWidth = computed(() => (props.labelWidth === 'auto' ? undefined : props.labelWidth))
</script>

<style scoped>
.query-card {
  width: 100%;
  box-sizing: border-box;
}

.crud-query-card {
  --crud-query-control-width: 160px;
  margin-bottom: 12px;
  border-radius: 16px;
  border: 1px solid #e5e8ef;
}

/* el-card body */
.crud-query-card :deep(.el-card__body) {
  padding: 18px 20px;
}

/* el-card header */
.crud-query-card :deep(.el-card__header) {
  padding: 16px 28px;
  border-bottom: 1px solid #ebeef5;
}

/* 不使用 el-card 时 */
.crud-query-card--plain {
  padding: 24px 28px;
  background: #fff;
  border: 1px solid #e5e8ef;
  border-radius: 16px;
}

/* plain 模式 header */
.crud-query-card__header {
  margin: -4px 0 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid #ebeef5;
}

/* 表单核心布局 */
.crud-query-form {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px 24px;
  width: 100%;
}

/* 清掉 Element Plus inline 的默认 margin */
.crud-query-form :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 0;
}

/* 普通筛选项 */
.crud-query-form :deep(.el-form-item:not(.crud-query-actions)) {
  width: auto;
  display: flex;
  align-items: center;
  flex-shrink: 0;
}

/* label */
.crud-query-form :deep(.el-form-item__label) {
  font-size: 14px;
  font-weight: 600;
  color: #606266;
  line-height: 36px;
}

/* labelWidth = auto 时 */
.crud-query-form--auto-label :deep(.el-form-item:not(.crud-query-actions)) {
  width: auto;
}

.crud-query-form--auto-label :deep(.el-form-item__label) {
  flex: 0 0 auto;
  width: auto !important;
  min-width: max-content;
}

.crud-query-form--auto-label :deep(.el-form-item__label-wrap) {
  flex: 0 0 auto;
  width: auto !important;
  margin-left: 0 !important;
}

/* 内容区域 */
.crud-query-form :deep(.el-form-item:not(.crud-query-actions) .el-form-item__content) {
  flex: 0 0 var(--crud-query-control-width);
  width: var(--crud-query-control-width);
  min-width: 0;
  line-height: 36px;
}

.crud-query-actions :deep(.el-form-item__content) {
  width: auto;
  line-height: 36px;
}

/* 输入框、选择框统一撑满 */
.crud-query-form :deep(.el-input),
.crud-query-form :deep(.el-select),
.crud-query-form :deep(.el-date-editor),
.crud-query-form :deep(.el-input-number),
.crud-query-form :deep(.el-cascader) {
  width: 100%;
}

/* 日期范围组件特殊处理 */
.crud-query-form :deep(.el-date-editor.el-input__wrapper) {
  width: 100%;
}

/* 按钮项 */
.crud-query-actions {
  flex-shrink: 0;
}

/* 搜索、重置跟着筛选项自然流动 */
.crud-query-search-actions {
  width: auto;
}

/* 没有额外 actions 时，搜索、重置靠右 */
.crud-query-form:not(.crud-query-form--has-extra-actions) .crud-query-search-actions {
  margin-left: auto !important;
}

/* 有额外 actions 时，额外 actions 靠右 */
.crud-query-extra-actions {
  margin-left: auto !important;
  width: auto;
}

/* 按钮内部布局 */
.crud-query-action-inner {
  display: flex;
  align-items: center;
  gap: 12px;
  white-space: nowrap;
}

/* 去掉 Element Plus 按钮默认相邻 margin */
.crud-query-action-inner :deep(.el-button + .el-button) {
  margin-left: 0;
}

/* 按钮高度统一 */
.crud-query-action-inner :deep(.el-button) {
  height: 36px;
  padding: 0 20px;
  font-size: 14px;
}

/* 输入框高度统一 */
.crud-query-form :deep(.el-input__wrapper),
.crud-query-form :deep(.el-select__wrapper),
.crud-query-form :deep(.el-input-number .el-input__wrapper),
.crud-query-form :deep(.el-cascader .el-input__wrapper) {
  min-height: 36px;
  border-radius: 6px;
}

/* 小屏适配 */
@media (max-width: 768px) {
  .crud-query-card :deep(.el-card__body) {
    padding: 16px;
  }

  .crud-query-card--plain {
    padding: 16px;
  }

  .crud-query-form {
    gap: 16px;
  }

  .crud-query-form :deep(.el-form-item:not(.crud-query-actions)) {
    width: 100%;
  }

  .crud-query-search-actions,
  .crud-query-extra-actions {
    width: 100%;
    margin-left: 0 !important;
  }

  .crud-query-action-inner {
    width: 100%;
  }
}
</style>
