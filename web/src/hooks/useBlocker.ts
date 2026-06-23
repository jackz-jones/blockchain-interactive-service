import { useEffect } from 'react'
import { useBlocker as useRouterBlocker } from 'react-router-dom'
import { Modal } from 'antd'

/**
 * 未保存离开确认 hook
 * 当 when 为 true 时，拦截路由跳转并弹出确认框
 * 适配 react-router-dom v7 的 useBlocker API
 */
export function useBlocker(when: boolean) {
  const blocker = useRouterBlocker(when)

  useEffect(() => {
    if (blocker.state === 'blocked') {
      Modal.confirm({
        title: '未保存的更改',
        content: '您有未保存的更改，确定要离开吗？离开后数据将丢失。',
        okText: '离开',
        cancelText: '留下',
        okButtonProps: { danger: true },
        onOk: () => {
          blocker.proceed?.()
        },
        onCancel: () => {
          blocker.reset?.()
        },
      })
    }
  }, [blocker])
}
