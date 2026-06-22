import { useCallback, useRef } from 'react'
import type { AxiosError } from 'axios'
import { useGlobalMessage } from '@/components/GlobalMessage'

/**
 * API 消息处理 Hook
 * 基于 useGlobalMessage() 获取 message 实例，提供统一的错误消息显示
 * 必须在 <GlobalMessageProvider> 组件内部使用
 */
export function useApiMessage() {
  const { message } = useGlobalMessage()
  const quotaWarningShown = useRef(false)

  /** 显示配额预警（同一周期内只提示一次，10 分钟后重置） */
  const showQuotaWarning = useCallback((warningText?: string) => {
    if (!quotaWarningShown.current) {
      quotaWarningShown.current = true
      message.warning(warningText || '调用配额即将用尽，请及时联系管理员提升限额，避免服务中断', 8)
      setTimeout(() => { quotaWarningShown.current = false }, 10 * 60 * 1000)
    }
  }, [message])

  /** 处理 API 错误并显示对应消息 */
  const handleApiError = useCallback(
    (error: unknown) => {
      if (error && typeof error === 'object' && 'businessMessage' in error) {
        const axiosErr = error as AxiosError & { businessMessage?: string; quotaWarning?: string }

        // 配额预警
        if (axiosErr.quotaWarning) {
          showQuotaWarning(axiosErr.quotaWarning)
        }

        // 显示业务错误消息
        message.error(axiosErr.businessMessage || '请求失败')
      } else if (error instanceof Error) {
        message.error(error.message || '请求失败')
      } else {
        message.error('请求失败')
      }
    },
    [message, showQuotaWarning]
  )

  /** 处理成功的 API 响应，检查配额预警响应头 */
  const handleSuccessResponse = useCallback(
    (response: { headers?: Record<string, string> }) => {
      const quotaWarning = response.headers?.['x-quota-warning'] as string | undefined
      if (quotaWarning) {
        showQuotaWarning()
      }
    },
    [showQuotaWarning]
  )

  return { message, handleApiError, handleSuccessResponse, showQuotaWarning }
}
