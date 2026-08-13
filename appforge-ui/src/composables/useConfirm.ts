/**
 * 确认对话框 Hook
 */

import { ElMessageBox } from 'element-plus'

export function useConfirm() {
  const confirm = async (
    message: string,
    options?: {
      title?: string
      confirmButtonText?: string
      cancelButtonText?: string
      type?: 'warning' | 'info' | 'success' | 'error'
    },
  ): Promise<void> => {
    try {
      await ElMessageBox.confirm(message, options?.title || 'Confirm', {
        confirmButtonText: options?.confirmButtonText || 'OK',
        cancelButtonText: options?.cancelButtonText || 'Cancel',
        type: options?.type || 'warning',
      })
    } catch (error) {
      // 捕获取消动作，当用户点击取消时会抛出错误
      if (error !== 'cancel') {
        throw error
      }
      // 取消时重新抛出 'cancel' 标记
      throw 'cancel'
    }
  }

  const confirmDelete = (name?: string): Promise<void> => {
    return confirm(`Are you sure to delete${name ? ` "${name}"` : ''} permanently?`, {
      title: 'Delete Confirmation',
      type: 'warning',
    })
  }

  return {
    confirm,
    confirmDelete,
  }
}
