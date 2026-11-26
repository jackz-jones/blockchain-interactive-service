import { Layout, Button, Space, Typography, Dropdown } from 'antd'
import {
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  LogoutOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { useAuthStore } from '@/stores/auth'
import type { MenuProps } from 'antd'

const { Header } = Layout
const { Text } = Typography

interface HeaderBarProps {
  collapsed: boolean
  onToggleCollapse: () => void
}

export default function HeaderBar({ collapsed, onToggleCollapse }: HeaderBarProps) {
  const { tenant, logout } = useAuthStore()

  const dropdownItems: MenuProps['items'] = [
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      onClick: () => {
        logout()
        window.location.href = '/settings'
      },
    },
  ]

  return (
    <Header
      style={{
        padding: '0 24px',
        background: '#ffffff',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        borderBottom: '1px solid var(--color-surface-border)',
        height: 56,
        lineHeight: '56px',
      }}
    >
      <Button
        type="text"
        icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
        onClick={onToggleCollapse}
        style={{ fontSize: 16 }}
      />

      <Space size="middle">
        {tenant && (
          <Dropdown menu={{ items: dropdownItems }} placement="bottomRight">
            <Space style={{ cursor: 'pointer' }}>
              <UserOutlined />
              <Text style={{ fontSize: 13 }}>
                {tenant.name}
                <Text type="secondary" style={{ marginLeft: 6, fontSize: 12 }}>
                  {tenant.role === 'admin' ? '管理员' : '用户'}
                </Text>
              </Text>
            </Space>
          </Dropdown>
        )}
      </Space>
    </Header>
  )
}
