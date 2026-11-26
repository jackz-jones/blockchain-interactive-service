import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/stores/auth'
import { useQuotaWarningStore } from '@/stores/quotaWarning'

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

/**
 * 扩展 AxiosError 类型，添加业务错误信息
 * 组件层可通过 error.businessMessage 和 error.quotaWarning 获取
 */
declare module 'axios' {
  interface AxiosError {
    businessMessage?: string
    quotaWarning?: string
  }
}

/** 响应拦截器：统一错误处理 + 解包业务数据 */
api.interceptors.response.use(
  (response) => {
    // 检查配额预警响应头，通过自定义属性传递，组件层可据此显示提示
    const quotaWarning = response.headers['x-quota-warning'] as string | undefined

    // 后端统一返回格式: { code: 0, message: "success", data: ... }
    // 自动解包，使 res.data 直接为业务数据部分
    const body = response.data
    if (body && typeof body === 'object' && 'code' in body) {
      if (body.code !== 0) {
        // 业务错误（HTTP 200 但 code 非 0）
        // 不再在此处调用 message，而是将错误信息附加到 Error 上
// 组件层使用 useGlobalMessage() 获取 message 实例后显示提示
        const businessError = new Error(body.message || '请求失败')
        const axiosErr = new AxiosError(businessError.message, undefined, undefined, undefined, response)
        axiosErr.businessMessage = body.message || '请求失败'
        if (quotaWarning) {
          axiosErr.quotaWarning = '调用配额即将用尽，请及时联系管理员提升限额，避免服务中断'
        }
        return Promise.reject(axiosErr)
      }
      // 解包：将 response.data 替换为 body.data
      response.data = body.data
    }

    // 如果有配额预警但没有业务错误，通过 response header 传递
    if (quotaWarning) {
      // 更新全局配额预警状态，AppLayout 横幅会自动显示
      useQuotaWarningStore.getState().setWarning('调用配额即将用尽，请及时联系管理员提升限额，避免服务中断')
    }

    return response
  },
  (error: AxiosError<{ code?: number; message?: string }>) => {
    if (error.response) {
      const { status, data } = error.response
      const msg = data?.message || error.message

      // 不再在此处调用 message，改为通过 Error 对象传递错误信息
// 组件层使用 useGlobalMessage() 获取 message 实例后显示提示
      const axiosErr = error as AxiosError
      axiosErr.businessMessage = msg

      switch (status) {
        case 401:
          axiosErr.businessMessage = msg || '认证失败，请检查 API Key 配置'
          // 仅对非公开接口触发 logout 和跳转
          const url = error.config?.url || ''
          if (!url.includes('/auth/')) {
            useAuthStore.getState().logout()
            window.location.href = '/settings'
          }
          break
        case 403:
          if (msg && msg.includes('quota exceeded')) {
            axiosErr.businessMessage = '调用配额已用尽，请联系管理员提升限额或升级套餐'
          } else if (msg && msg.includes('ip not in whitelist')) {
            axiosErr.businessMessage = '当前 IP 不在白名单内，请检查 API Key 的 IP 限制配置'
          } else if (msg && msg.includes('tenant is disabled')) {
            axiosErr.businessMessage = '租户账号已被禁用，请联系管理员'
          } else {
            axiosErr.businessMessage = '权限不足，无法访问该资源'
          }
          break
        case 404:
          axiosErr.businessMessage = '请求的资源不存在'
          break
        case 429:
          if (msg && msg.includes('quota exceeded')) {
            axiosErr.businessMessage = '调用配额已用尽，请联系管理员提升限额或升级套餐'
          } else {
            axiosErr.businessMessage = '请求过于频繁，请稍后重试'
          }
          break
        default:
          if (status >= 500) {
            axiosErr.businessMessage = '服务器错误，请稍后重试'
          } else {
            axiosErr.businessMessage = msg || '请求失败'
          }
      }
    } else if (error.code === 'ECONNABORTED') {
      error.businessMessage = '请求超时，请检查网络连接'
    } else {
      error.businessMessage = '网络错误，请检查连接'
    }

    return Promise.reject(error)
  }
)

export default api
