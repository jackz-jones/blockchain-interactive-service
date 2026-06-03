import { Layout, Menu } from 'antd'
import { useNavigate, useLocation } from 'react-router-dom'
import {
  DashboardOutlined,
  LinkOutlined,
  FileTextOutlined,
  CodeOutlined,
  SearchOutlined,
  BellOutlined,
  KeyOutlined,
  TeamOutlined,
  UserOutlined,
  BarChartOutlined,
  AuditOutlined,
  UnorderedListOutlined,
  DollarOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import type { MenuProps } from 'antd'
import { useAuthStore } from '@/stores/auth'

const { Sider } = Layout

interface SidebarProps {
  collapsed: boolean
  onCollapse: (collapsed: boolean) => void
}

type MenuItem = Required<MenuProps>['items'][number]

export default function Sidebar({ collapsed, onCollapse }: SidebarProps) {
  const navigate = useNavigate()
  const location = useLocation()
  const { tenant } = useAuthStore()
  const isAdmin = tenant?.role === 'admin'

  const menuItems: MenuItem[] = [
    {
      key: '/dashboard',
      icon: <DashboardOutlined />,
      label: '概览',
    },
    {
      type: 'divider',
    },
    {
      key: 'config-group',
      label: '配置管理',
      type: 'group',
      children: [
        {
          key: '/chain-configs',
          icon: <LinkOutlined />,
          label: '链配置',
        },
        {
          key: '/contract-configs',
          icon: <FileTextOutlined />,
          label: '合约配置',
        },
      ],
    },
    {
      key: 'interaction-group',
      label: '链交互',
      type: 'group',
      children: [
        {
          key: '/contract-call',
          icon: <CodeOutlined />,
          label: '合约调用',
        },
        {
          key: '/tx-query',
          icon: <SearchOutlined />,
          label: '交易查询',
        },
        {
          key: '/event-subscriptions',
          icon: <BellOutlined />,
          label: '事件订阅',
        },
      ],
    },
    {
      key: 'admin-group',
      label: '系统管理',
      type: 'group',
      children: [
        {
          key: '/api-keys',
          icon: <KeyOutlined />,
          label: 'API Key',
        },
        ...(isAdmin
          ? [
              {
                key: '/tenants',
                icon: <TeamOutlined />,
                label: '租户管理',
              },
              {
                key: '/users',
                icon: <UserOutlined />,
                label: '用户管理',
              },
            ]
          : []),
      ],
    },
    {
      key: 'logs-group',
      label: '日志与统计',
      type: 'group',
      children: [
        {
          key: '/call-logs',
          icon: <UnorderedListOutlined />,
          label: '调用日志',
        },
        ...(isAdmin
          ? [
              {
                key: '/audit-logs',
                icon: <AuditOutlined />,
                label: '审计日志',
              },
            ]
          : []),
        {
          key: '/usage-stats',
          icon: <BarChartOutlined />,
          label: '用量统计',
        },
        {
          key: '/bills',
          icon: <DollarOutlined />,
          label: '账单',
        },
      ],
    },
    {
      type: 'divider',
    },
    {
      key: '/settings',
      icon: <SettingOutlined />,
      label: '设置',
    },
  ]

  // 获取当前选中的菜单项
  const selectedKey = '/' + location.pathname.split('/')[1]

  return (
    <Sider
      collapsible
      collapsed={collapsed}
      onCollapse={onCollapse}
      width={240}
      collapsedWidth={64}
      theme="dark"
      style={{
        overflow: 'auto',
        height: '100vh',
        position: 'sticky',
        top: 0,
        left: 0,
      }}
    >
      {/* Logo */}
      <div
        style={{
          height: 56,
          display: 'flex',
          alignItems: 'center',
          justifyContent: collapsed ? 'center' : 'flex-start',
          padding: collapsed ? '0' : '0 20px',
          borderBottom: '1px solid rgba(255,255,255,0.06)',
          transition: 'all var(--duration-normal) var(--ease-out)',
        }}
      >
        <LinkOutlined style={{ fontSize: 20, color: '#7cb88c' }} />
        {!collapsed && (
          <span
            style={{
              marginLeft: 10,
              fontSize: 15,
              fontWeight: 600,
              color: '#ffffff',
              whiteSpace: 'nowrap',
            }}
          >
            Chain Service
          </span>
        )}
      </div>

      <Menu
        theme="dark"
        mode="inline"
        selectedKeys={[selectedKey]}
        items={menuItems}
        onClick={({ key }) => navigate(key)}
        style={{ borderRight: 0, marginTop: 8 }}
      />
    </Sider>
  )
}
