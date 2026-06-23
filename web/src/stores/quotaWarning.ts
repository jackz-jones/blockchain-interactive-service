import { create } from 'zustand'

interface QuotaWarningState {
  /** 配额预警消息，为空表示无预警 */
  message: string | null
  /** 设置预警消息 */
  setWarning: (msg: string | null) => void
  /** 清除预警 */
  clearWarning: () => void
}

/**
 * 配额预警全局状态
 * API 拦截器检测到 x-quota-warning 响应头时写入
 * AppLayout 顶部横幅消费此状态
 */
export const useQuotaWarningStore = create<QuotaWarningState>()((set) => ({
  message: null,
  setWarning: (msg) => set({ message: msg }),
  clearWarning: () => set({ message: null }),
}))
