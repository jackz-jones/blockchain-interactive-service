import { useState } from 'react'
import { Layout, Alert } from 'antd'
import { Outlet } from 'react-router-dom'
import Sidebar from './Sidebar'
import HeaderBar from './HeaderBar'
import { useQuotaWarningStore } from '@/stores/quotaWarning'

const { Content } = Layout

export default function AppLayout() {
  const [collapsed, setCollapsed] = useState(false)
  const quotaWarning = useQuotaWarningStore((s) => s.message)
  const clearWarning = useQuotaWarningStore((s) => s.clearWarning)

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sidebar collapsed={collapsed} onCollapse={setCollapsed} />
      <Layout>
        <HeaderBar collapsed={collapsed} onToggleCollapse={() => setCollapsed(!collapsed)} />
        {quotaWarning && (
          <Alert
            type="warning"
            showIcon
            banner
            closable
            message={quotaWarning}
            onClose={clearWarning}
          />
        )}
        <Content
          style={{
            padding: '24px',
            overflow: 'auto',
            background: 'var(--color-bg)',
          }}
        >
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
