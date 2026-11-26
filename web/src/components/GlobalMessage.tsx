import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'

/** 消息类型 */
type MessageType = 'success' | 'info' | 'warning' | 'error' | 'loading'

/** 消息配置 */
interface MessageConfig {
  /** 消息内容 */
  content: React.ReactNode
  /** 消息类型 */
  type?: MessageType
  /** 自动关闭延时，单位秒，默认 3，设为 0 则不自动关闭 */
  duration?: number
  /** 消息唯一标识 */
  key?: string
  /** 关闭回调 */
  onClose?: () => void
}

/** 消息项（内部使用） */
interface MessageItem {
  _id: string
  content: React.ReactNode
  type: MessageType
  duration: number
  key?: string
  onClose?: () => void
  _createdAt: number
  _timer?: ReturnType<typeof setTimeout>
}

/** 消息 API 接口 */
interface MessageApi {
  open: (config: MessageConfig) => void
  success: (content: React.ReactNode, duration?: number, onClose?: () => void) => void
  info: (content: React.ReactNode, duration?: number, onClose?: () => void) => void
  warning: (content: React.ReactNode, duration?: number, onClose?: () => void) => void
  error: (content: React.ReactNode, duration?: number, onClose?: () => void) => void
  loading: (content: React.ReactNode, duration?: number, onClose?: () => void) => void
  destroy: (key?: string) => void
  /** 自身引用，方便解构使用 const { message } = useGlobalMessage() */
  message: MessageApi
}

// ============================
// Context
// ============================

const MessageApiContext = createContext<MessageApi | null>(null)
const MessageDispatchContext = createContext<((items: MessageItem[]) => void) | null>(null)

// ============================
// 默认配置
// ============================
const DEFAULT_MAX_COUNT = 5
const DEFAULT_DURATION = 3
let keyIndex = 0
const generateId = () => `global-msg-${++keyIndex}`

// ============================
// 消息列表组件
// ============================

interface GlobalMessageListProps {
  items: MessageItem[]
  onRemove: (id: string) => void
}

const typeIconMap: Record<MessageType, string> = {
  success: '✅',
  info: 'ℹ️',
  warning: '⚠️',
  error: '❌',
  loading: '⏳',
}

const typeClassMap: Record<MessageType, string> = {
  success: 'ant-message-success',
  info: 'ant-message-info',
  warning: 'ant-message-warning',
  error: 'ant-message-error',
  loading: 'ant-message-loading',
}

const GlobalMessageList: React.FC<GlobalMessageListProps> = ({ items, onRemove }) => {
  return (
    <div
      className="ant-message"
      style={{
        position: 'fixed',
        top: 8,
        left: 0,
        right: 0,
        zIndex: 1010,
        pointerEvents: 'none',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
      }}
    >
      {items.map((item) => (
        <div
          key={item._id}
          className={`ant-message-notice ${typeClassMap[item.type]}`}
          style={{
            pointerEvents: 'auto',
            display: 'inline-block',
            margin: '0 auto 8px',
            padding: '9px 12px',
            borderRadius: 8,
            background: '#fff',
            boxShadow:
              '0 6px 16px 0 rgba(0, 0, 0, 0.08), 0 3px 6px -4px rgba(0, 0, 0, 0.12), 0 9px 28px 8px rgba(0, 0, 0, 0.05)',
            animation: 'messageFadeIn 0.3s ease-out',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span className="ant-message-notice-icon">{typeIconMap[item.type]}</span>
            <span style={{ fontSize: 14, color: 'rgba(0,0,0,0.88)', lineHeight: '22px' }}>
              {item.content}
            </span>
            {item.duration === 0 && (
              <button
                onClick={() => onRemove(item._id)}
                style={{
                  background: 'none',
                  border: 'none',
                  cursor: 'pointer',
                  marginLeft: 4,
                  fontSize: 14,
                  color: 'rgba(0,0,0,0.45)',
                }}
              >
                ✕
              </button>
            )}
          </div>
        </div>
      ))}
      <style>{`
        @keyframes messageFadeIn {
          from { opacity: 0; transform: translateY(-100%); }
          to { opacity: 1; transform: translateY(0); }
        }
      `}</style>
    </div>
  )
}

// ============================
// Provider 组件
// ============================

interface GlobalMessageProviderProps {
  children?: React.ReactNode
  maxCount?: number
  duration?: number
}

export const GlobalMessageProvider: React.FC<GlobalMessageProviderProps> = ({
  children,
  maxCount = DEFAULT_MAX_COUNT,
  duration = DEFAULT_DURATION,
}) => {
  const [items, setItems] = useState<MessageItem[]>([])
  const timersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())

  const removeItem = useCallback(
    (id: string) => {
      setItems((prev) => {
        const item = prev.find((i) => i._id === id)
        if (item) {
          const timer = timersRef.current.get(id)
          if (timer) {
            clearTimeout(timer)
            timersRef.current.delete(id)
          }
          item.onClose?.()
        }
        return prev.filter((i) => i._id !== id)
      })
    },
    []
  )

  const startCloseTimer = useCallback(
    (item: MessageItem) => {
      const oldTimer = timersRef.current.get(item._id)
      if (oldTimer) clearTimeout(oldTimer)

      if (item.duration > 0) {
        const timer = setTimeout(() => {
          removeItem(item._id)
        }, item.duration * 1000)
        timersRef.current.set(item._id, timer)
      }
    },
    [removeItem]
  )

  // 清理所有计时器
  useEffect(() => {
    return () => {
      timersRef.current.forEach((timer) => clearTimeout(timer))
    }
  }, [])

  const api = useMemo(() => {
    const open = (config: MessageConfig) => {
      setItems((prev) => {
        const id = config.key || generateId()
        const dur = config.duration ?? duration
        const now = Date.now()

        let newItems: MessageItem[]

        const existingIndex = prev.findIndex((item) => item.key && item.key === config.key)
        if (existingIndex >= 0) {
          // 更新已有消息
          const existing = prev[existingIndex]!
          const oldTimer = timersRef.current.get(existing._id)
          if (oldTimer) clearTimeout(oldTimer)

          const updated: MessageItem = {
            _id: existing._id,
            key: config.key,
            type: config.type || existing.type,
            content: config.content,
            duration: dur,
            onClose: config.onClose,
            _createdAt: now,
          }
          newItems = [...prev]
          newItems[existingIndex] = updated
          startCloseTimer(updated)
        } else {
          const item: MessageItem = {
            _id: id,
            key: config.key || id,
            type: config.type || 'info',
            content: config.content,
            duration: dur,
            onClose: config.onClose,
            _createdAt: now,
          }
          newItems = [...prev, item]
          startCloseTimer(item)
        }

        // 超出最大数量
        if (newItems.length > maxCount) {
          const removed = newItems.slice(0, newItems.length - maxCount)
          removed.forEach((r) => {
            const timer = timersRef.current.get(r._id)
            if (timer) clearTimeout(timer)
            timersRef.current.delete(r._id)
          })
          newItems = newItems.slice(newItems.length - maxCount)
        }

        return newItems
      })
    }

    const success = (content: React.ReactNode, dur?: number, onClose?: () => void) =>
      open({ type: 'success', content, duration: dur, onClose })
    const info = (content: React.ReactNode, dur?: number, onClose?: () => void) =>
      open({ type: 'info', content, duration: dur, onClose })
    const warning = (content: React.ReactNode, dur?: number, onClose?: () => void) =>
      open({ type: 'warning', content, duration: dur, onClose })
    const error = (content: React.ReactNode, dur?: number, onClose?: () => void) =>
      open({ type: 'error', content, duration: dur, onClose })
    const loading = (content: React.ReactNode, dur?: number, onClose?: () => void) =>
      open({ type: 'loading', content, duration: dur ?? 0, onClose })
    const destroy = (key?: string) => {
      if (key) {
        removeItem(key)
      } else {
        setItems([])
        timersRef.current.forEach((timer) => clearTimeout(timer))
        timersRef.current.clear()
      }
    }

    return { open, success, info, warning, error, loading, destroy } as MessageApi
  }, [maxCount, duration, removeItem, startCloseTimer])

  // 添加 message 自引用，支持 const { message } = useGlobalMessage() 解构使用
  api.message = api

  return (
    <MessageApiContext.Provider value={api}>
      <MessageDispatchContext.Provider value={setItems}>
        {children}
        <GlobalMessageList items={items} onRemove={removeItem} />
      </MessageDispatchContext.Provider>
    </MessageApiContext.Provider>
  )
}

// ============================
// Hook
// ============================

/**
 * 全局消息 Hook
 * 提供与 antd message API 相同的接口，但不依赖 App.useApp()
 * 必须在 GlobalMessageProvider 内部使用
 */
export function useGlobalMessage(): MessageApi {
  const api = useContext(MessageApiContext)
  if (!api) {
    throw new Error('useGlobalMessage must be used within a <GlobalMessageProvider>')
  }
  return api
}

export default { GlobalMessageProvider, useGlobalMessage }