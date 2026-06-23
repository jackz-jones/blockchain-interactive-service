import dayjs from 'dayjs'

/**
 * 统一时间格式化函数
 * 将 ISO 8601 格式的时间字符串转换为 YYYY-MM-DD HH:mm:ss 格式
 * @param time ISO 8601 格式的时间字符串
 * @returns 格式化后的时间字符串，无效时间返回 '-'
 */
export function formatDateTime(time: string | null | undefined): string {
  if (!time) return '-'
  const parsed = dayjs(time)
  if (!parsed.isValid()) return '-'
  return parsed.format('YYYY-MM-DD HH:mm:ss')
}
