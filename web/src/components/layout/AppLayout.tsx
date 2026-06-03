import { useState } from 'react'
import { Layout } from 'antd'
import { Outlet } from 'react-router-dom'
import Sidebar from './Sidebar'
import HeaderBar from './HeaderBar'

const { Content } = Layout

export default function AppLayout() {
  const [collapsed, setCollapsed] = useState(false)

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sidebar collapsed={collapsed} onCollapse={setCollapsed} />
      <Layout>
        <HeaderBar collapsed={collapsed} onToggleCollapse={() => setCollapsed(!collapsed)} />
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
