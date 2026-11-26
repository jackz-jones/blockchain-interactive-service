import { useState, useCallback } from 'react'
import type { AxiosResponse } from 'axios'

interface RequestState<T> {
  data: T | null
  loading: boolean
  error: string | null
}

interface UseRequestReturn<T> extends RequestState<T> {
  run: (...args: unknown[]) => Promise<T | null>
  reset: () => void
}

/**
 * 通用请求 Hook
 * 封装 loading/error/data 状态管理
 */
export function useRequest<T>(
  requestFn: (...args: unknown[]) => Promise<AxiosResponse<T>>
): UseRequestReturn<T> {
  const [state, setState] = useState<RequestState<T>>({
    data: null,
    loading: false,
    error: null,
  })

  const run = useCallback(
    async (...args: unknown[]): Promise<T | null> => {
      setState((prev) => ({ ...prev, loading: true, error: null }))
      try {
        const response = await requestFn(...args)
        setState({ data: response.data, loading: false, error: null })
        return response.data
      } catch (err) {
        const errorMsg = err instanceof Error ? err.message : '请求失败'
        setState((prev) => ({ ...prev, loading: false, error: errorMsg }))
        return null
      }
    },
    [requestFn]
  )

  const reset = useCallback(() => {
    setState({ data: null, loading: false, error: null })
  }, [])

  return { ...state, run, reset }
}
