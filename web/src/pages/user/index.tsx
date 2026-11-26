import { useEffect, useState } from 'react'
import { Table, Typography, Tag, Space, Input, Button, Popconfirm } from 'antd'
import { SearchOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { useGlobalMessage } from '@/components/GlobalMessage'
import { formatDateTime } from '@/utils/format'
import ErrorRetry from '@/components/ErrorRetry'

const { Title } = Typography

interface User {
  id: string
  username: string
  tenant_name: string
  role: string
  status: string
  last_login: string
}

const roleLabelMap: Record<string, string> = {
  admin: '管理员',
  user: '普通用户',
}

const roleColorMap: Record<string, string> = {
  admin: 'blue',
  user: 'default',
}

export default function UserList() {
  const [data, setData] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const [search, setSearch] = useState('')
  const { message } = useGlobalMessage()
  const { handleApiError } = useApiMessage()

  const fetchList = async (searchOverride?: string) => {
    setLoading(true)
    setError(null)
    try {
      const params: Record<string, string> = {}
      const searchValue = searchOverride !== undefined ? searchOverride : search
      if (searchValue) params.search = searchValue
      const res = await api.get('/users', { params })
      setData((res.data as { items: User[] }).items || [])
    } catch (err) {
      setError(err)
      handleApiError(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchList()
  }, [])

  const handleToggle = async (id: string, currentStatus: string) => {
    try {
      const enable = currentStatus !== 'active'
      if (enable) {
        await api.post(`/users/${id}/enable`)
        message.success('已启用')
      } else {
        await api.post(`/users/${id}/disable`)
        message.success('已禁用')
      }
      fetchList()
    } catch (err) {
      handleApiError(err)
    }
  }

  const columns: ColumnsType<User> = [
    { title: '用户名', dataIndex: 'username', key: 'username', width: 180 },
    { title: '所属租户', dataIndex: 'tenant_name', key: 'tenant_name', width: 200 },
    {
      title: '角色',
      dataIndex: 'role',
      key: 'role',
      width: 120,
      render: (role: string) => (
        <Tag color={roleColorMap[role] || 'default'}>{roleLabelMap[role] || role}</Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (status: string) => (
        <Tag color={status === 'active' ? 'success' : 'default'}>{status === 'active' ? '已启用' : '已禁用'}</Tag>
      ),
    },
    {
      title: '最后登录',
      dataIndex: 'last_login',
      key: 'last_login',
      width: 200,
      render: (time: string) => time ? formatDateTime(time) : '-',
    },
    {
      title: '操作',
      key: 'actions',
      width: 100,
      render: (_, record) => {
        const isActive = record.status === 'active'
        return (
          <Popconfirm
            title={isActive ? '确认禁用该用户？' : '确认启用该用户？'}
            onConfirm={() => handleToggle(record.id, record.status)}
          >
            <Button type="text" size="small" danger={isActive}>
              {isActive ? '禁用' : '启用'}
            </Button>
          </Popconfirm>
        )
      },
    },
  ]

  // 加载失败时显示错误重试组件
  if (error && !loading && data.length === 0) {
    return (
      <div>
        <Title level={4} style={{ marginBottom: 20 }}>用户管理</Title>
        <ErrorRetry error={error} onRetry={() => fetchList()} />
      </div>
    )
  }

  return (
    <div>
      <Title level={4} style={{ marginBottom: 20 }}>用户管理</Title>

      <Space style={{ marginBottom: 16 }}>
        <Input
          placeholder="搜索用户名"
          prefix={<SearchOutlined />}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onPressEnter={() => fetchList()}
          onClear={() => { setSearch(''); fetchList('') }}
          style={{ width: 220 }}
          allowClear
        />
        <Button type="primary" icon={<SearchOutlined />} onClick={() => fetchList()}>
          搜索
        </Button>
      </Space>

      <Table columns={columns} dataSource={data} rowKey="id" loading={loading} pagination={{ pageSize: 10 }} size="middle" />
    </div>
  )
}
