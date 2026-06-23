import { useEffect, useState } from 'react'
import { Table, Typography, Skeleton, Tag } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
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

export default function UserList() {
  const [data, setData] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const { handleApiError } = useApiMessage()

  const fetchList = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.get('/users')
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

  const columns: ColumnsType<User> = [
    { title: '用户名', dataIndex: 'username', key: 'username', width: 180 },
    { title: '所属租户', dataIndex: 'tenant_name', key: 'tenant_name', width: 200 },
    {
      title: '角色',
      dataIndex: 'role',
      key: 'role',
      width: 120,
      render: (role: string) => (
        <Tag color={role === 'admin' ? 'blue' : 'default'}>{role}</Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (status: string) => (
        <Tag color={status === 'active' ? 'success' : 'default'}>{status}</Tag>
      ),
    },
    {
      title: '最后登录',
      dataIndex: 'last_login',
      key: 'last_login',
      width: 200,
      render: (time: string) => time ? formatDateTime(time) : '-',
    },
  ]

  if (loading) {
    return (
      <div>
        <Title level={4} style={{ marginBottom: 20 }}>用户管理</Title>
        <Skeleton active paragraph={{ rows: 8 }} />
      </div>
    )
  }

  // 加载失败时显示错误重试组件
  if (error && data.length === 0) {
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
      <Table columns={columns} dataSource={data} rowKey="id" pagination={{ pageSize: 10 }} size="middle" />
    </div>
  )
}
