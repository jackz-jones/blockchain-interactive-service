import { useEffect } from 'react'
import { Modal } from 'antd'

/**
 * 未保存离开确认 hook
 * 当 when 为 true 时，拦截浏览器关闭/刷新操作，弹出浏览器原生确认框
 * 注意：程序式导航（如点击取消按钮、侧边栏导航）需要在组件中自行处理确认逻辑
 */
export function useBlocker(when: boolean) {
  useEffect(() => {
    if (!when) return

    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault()
      // 现代浏览器会忽略自定义消息，但保留这一行以兼容旧浏览器
      e.returnValue = ''
    }

    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload)
    }
  }, [when])
}

/**
 * 离开确认弹窗
 * 用于程序式导航（如取消按钮、侧边栏菜单）的确认
 */
export function confirmLeave(callback: () => void, isModified: boolean) {
  if (isModified) {
    Modal.confirm({
      title: '未保存的更改',
      content: '您有未保存的更改，确定要离开吗？离开后数据将丢失。',
      okText: '离开',
      cancelText: '留下',
      okButtonProps: { danger: true },
      onOk: () => {
        callback()
      },
    })
  } else {
    callback()
  }
}