import { useEffect, useState } from 'react'
import { Table, Typography, Skeleton, Tag } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'

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
  const { handleApiError } = useApiMessage()

  useEffect(() => {
    const fetchList = async () => {
      try {
        const res = await api.get('/users')
        setData((res.data as { items: User[] }).items || [])
      } catch (err) {
        handleApiError(err)
      } finally {
        setLoading(false)
      }
    }
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
      render: (time: string) => time ? new Date(time).toLocaleString('zh-CN') : '-',
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

  return (
    <div>
      <Title level={4} style={{ marginBottom: 20 }}>用户管理</Title>
      <Table columns={columns} dataSource={data} rowKey="id" pagination={{ pageSize: 10 }} size="middle" />
    </div>
  )
}
