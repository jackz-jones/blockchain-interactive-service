import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/stores/auth'
import { message } from 'antd'

/** API 基础实例 */
const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

/** 请求拦截器：注入 API Key */
api.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const { apiKey } = useAuthStore.getState()
    if (apiKey) {
      config.headers['X-API-Key'] = apiKey
    }
    return config
  },
  (error) => Promise.reject(error)
)

/** 响应拦截器：统一错误处理 */
api.interceptors.response.use(
  (response) => response,
  (error: AxiosError<{ code?: number; message?: string }>) => {
    if (error.response) {
      const { status, data } = error.response
      const msg = data?.message || error.message

      switch (status) {
        case 401:
          message.error(msg || '认证失败，请检查 API Key 配置')
          // 仅对非公开接口触发 logout 和跳转
          const url = error.config?.url || ''
          if (!url.includes('/auth/')) {
            useAuthStore.getState().logout()
            window.location.href = '/settings'
          }
          break
        case 403:
          message.error('权限不足')
          break
        case 404:
          message.error('请求的资源不存在')
          break
        case 429:
          message.error('请求过于频繁，请稍后重试')
          break
        default:
          if (status >= 500) {
            message.error('服务器错误，请稍后重试')
          } else {
            message.error(msg || '请求失败')
          }
      }
    } else if (error.code === 'ECONNABORTED') {
      message.error('请求超时，请检查网络连接')
    } else {
      message.error('网络错误，请检查连接')
    }

    return Promise.reject(error)
  }
)

export default api
