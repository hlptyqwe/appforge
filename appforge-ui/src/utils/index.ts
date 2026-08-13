/**
 * 格式化日期
 * @param timestamp 时间戳（秒或毫秒）
 * @returns 格式化后的日期字符串
 */
export function formatDate(timestamp: number): string {
  if (!timestamp) return '-'
  const normalizedTimestamp = timestamp < 1e12 ? timestamp * 1000 : timestamp
  const date = new Date(normalizedTimestamp)
  return date.toLocaleString()
}

/**
 * 格式化日期（日期部分）
 * @param timestamp 时间戳（秒或毫秒）
 * @returns 格式化后的日期字符串
 */
export function formatDateOnly(timestamp: number): string {
  if (!timestamp) return '-'
  const normalizedTimestamp = timestamp < 1e12 ? timestamp * 1000 : timestamp
  const date = new Date(normalizedTimestamp)
  return date.toLocaleDateString()
}

/**
 * 格式化时间（时间部分）
 * @param timestamp 时间戳（秒或毫秒）
 * @returns 格式化后的时间字符串
 */
export function formatTimeOnly(timestamp: number): string {
  if (!timestamp) return '-'
  const normalizedTimestamp = timestamp < 1e12 ? timestamp * 1000 : timestamp
  const date = new Date(normalizedTimestamp)
  return date.toLocaleTimeString()
}
