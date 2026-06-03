import { Spin } from 'antd'

/** 全屏页面加载状态 */
export default function PageLoading() {
  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        height: '100vh',
        width: '100%',
      }}
    >
      <Spin size="large" />
    </div>
  )
}
