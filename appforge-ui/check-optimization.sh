#!/bin/bash

# Admin UI 项目优化检查清单
# 运行此脚本验证优化是否完整

echo "📋 Admin UI 项目优化检查列表"
echo "================================"
echo ""

success_count=0
total_count=0

check_file() {
  local file=$1
  local name=$2
  total_count=$((total_count + 1))
  
  if [ -f "$file" ]; then
    echo "✅ $name"
    success_count=$((success_count + 1))
  else
    echo "❌ $name - 找不到文件: $file"
  fi
}

check_dir() {
  local dir=$1
  local name=$2
  total_count=$((total_count + 1))
  
  if [ -d "$dir" ]; then
    echo "✅ $name"
    success_count=$((success_count + 1))
  else
    echo "❌ $name - 找不到目录: $dir"
  fi
}

echo "🎯 目录结构"
check_dir "src/components" "📁 src/components"
check_dir "src/composables" "📁 src/composables"
check_dir "src/services" "📁 src/services"
check_dir "src/config" "📁 src/config"
check_dir "src/utils" "📁 src/utils"
echo ""

echo "📝 配置文件"
check_file ".env" ".env 基础环境变量"
check_file ".env.development" ".env.development 开发环境"
check_file ".env.production" ".env.production 生产环境"
check_file ".eslintrc.cjs" ".eslintrc.cjs ESLint 配置"
check_file ".prettierrc.cjs" ".prettierrc.cjs Prettier 配置"
check_file ".prettierignore" ".prettierignore"
echo ""

echo "🔧 工具文件"
check_file "src/utils/logger.ts" "logger.ts 日志工具"
check_file "src/utils/error.ts" "error.ts 错误处理"
check_file "src/utils/request.ts" "request.ts 增强请求工具"
check_file "src/config/environment.ts" "environment.ts 环境变量"
echo ""

echo "🎁 服务层"
check_file "src/services/BaseService.ts" "BaseService.ts 基础服务"
check_file "src/services/system/UserService.ts" "UserService.ts 用户服务"
check_file "src/services/system/RoleService.ts" "RoleService.ts 角色服务"
check_file "src/services/system/MenuService.ts" "MenuService.ts 菜单服务"
check_file "src/services/system/LogService.ts" "LogService.ts 日志服务"
check_file "src/services/system/UploadService.ts" "UploadService.ts 上传服务"
check_file "src/services/index.ts" "index.ts 服务统一导出"
echo ""

echo "🪝 Composables"
check_file "src/composables/usePagination.ts" "usePagination.ts 分页"
check_file "src/composables/useLoading.ts" "useLoading.ts 加载状态"
check_file "src/composables/useForm.ts" "useForm.ts 表单处理"
check_file "src/composables/useConfirm.ts" "useConfirm.ts 确认框"
check_file "src/composables/useAsync.ts" "useAsync.ts 异步处理"
check_file "src/composables/useLocalStorage.ts" "useLocalStorage.ts 本地存储"
echo ""

echo "🎨 组件"
check_file "src/components/common/ConfirmDialog.vue" "ConfirmDialog.vue 确认对话框"
check_file "src/components/table/DataTable.vue" "DataTable.vue 数据表格"
echo ""

echo "📚 文档"
check_file "OPTIMIZATION.md" "OPTIMIZATION.md 优化说明"
check_file "DEVELOPER_GUIDE.md" "DEVELOPER_GUIDE.md 开发者指南"
echo ""

echo "================================"
echo "检查结果: $success_count/$total_count ✨"
echo ""

if [ $success_count -eq $total_count ]; then
  echo "🎉 所有优化已完成！"
  exit 0
else
  echo "⚠️  还有 $((total_count - success_count)) 项未完成"
  exit 1
fi
