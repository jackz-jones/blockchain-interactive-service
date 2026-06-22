import { useEffect, useState } from 'react'
import { Table, Button, Typography, Tag, Space, Modal, Form, Input, Popconfirm, Tooltip } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { useGlobalMessage } from '@/components/GlobalMessage'

const { Title } = Typography

interface Tenant {
  id: string
  name: string
  enabled: boolean
  status: string
  email: string
  phone: string
  plan: string
  created_at: string
}

export default function TenantList() {
  const [data, setData] = useState<Tenant[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const { message } = useGlobalMessage()
  const { handleApiError } = useApiMessage()
  const [form] = Form.useForm()

  const fetchList = async () => {
    setLoading(true)
    try {
      const res = await api.get('/tenants')
      setData((res.data as { items: Tenant[] }).items || [])
    } catch (err) {
      handleApiError(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchList()
  }, [])

  const handleCreate = async (values: Record<string, unknown>) => {
    setCreateLoading(true)
    try {
      await api.post('/tenants', values)
      message.success('租户创建成功')
      setCreateOpen(false)
      form.resetFields()
      fetchList()
    } catch (err) {
      handleApiError(err)
    } finally {
      setCreateLoading(false)
    }
  }

  const handleToggle = async (id: string, enable: boolean) => {
    try {
      if (enable) {
        await api.post(`/tenants/${id}/enable`)
        message.success('已启用')
      } else {
        await api.post(`/tenants/${id}/disable`)
        message.success('已禁用')
      }
      fetchList()
    } catch (err) {
      handleApiError(err)
    }
  }

  const isEnabled = (record: Tenant) => record.status === 'active' || record.enabled

  const columns: ColumnsType<Tenant> = [
    { title: '租户名称', dataIndex: 'name', key: 'name', width: 200 },
    {
      title: '邮箱',
      dataIndex: 'email',
      key: 'email',
      width: 260,
      ellipsis: { showTitle: false },
      render: (email: string) => (
        <Tooltip title={email} placement="topLeft"><span>{email || '-'}</span></Tooltip>
      ),
    },
    {
      title: '状态',
      key: 'status',
      width: 120,
      render: (_, record) => {
        const enabled = isEnabled(record)
        return <Tag color={enabled ? 'success' : 'default'}>{enabled ? '已启用' : '已禁用'}</Tag>
      },
    },
    { title: '套餐', dataIndex: 'plan', key: 'plan', width: 120 },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (time: string) => new Date(time).toLocaleDateString('zh-CN'),
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      render: (_, record) => {
        const enabled = isEnabled(record)
        return (
          <Popconfirm
            title={enabled ? '确认禁用？' : '确认启用？'}
            onConfirm={() => handleToggle(record.id, !enabled)}
          >
            <Button type="text" size="small">
              {enabled ? '禁用' : '启用'}
            </Button>
          </Popconfirm>
        )
      },
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <Title level={4} style={{ margin: 0 }}>租户管理</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          创建租户
        </Button>
      </div>

      <Table columns={columns} dataSource={data} rowKey="id" loading={loading} pagination={{ pageSize: 10 }} size="middle" />

      <Modal title="创建租户" open={createOpen} onCancel={() => setCreateOpen(false)} footer={null}>
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <Form.Item name="name" label="租户名称" rules={[{ required: true, message: '请输入租户名称' }]}>
            <Input placeholder="租户名称" />
          </Form.Item>
          <Form.Item name="email" label="邮箱" rules={[{ required: true, message: '请输入邮箱' }, { type: 'email', message: '请输入有效的邮箱地址' }]}>
            <Input placeholder="管理员邮箱" />
          </Form.Item>
          <Form.Item name="password" label="初始密码" rules={[{ required: true, message: '请输入初始密码' }, { min: 6, message: '密码至少6位' }]}>
            <Input.Password placeholder="管理员初始密码" />
          </Form.Item>
          <Form.Item name="phone" label="联系电话">
            <Input placeholder="联系电话（选填）" />
          </Form.Item>
          <Form.Item name="plan" label="套餐">
            <Input placeholder="套餐计划（选填）" />
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={createLoading}>创建</Button>
            <Button onClick={() => setCreateOpen(false)}>取消</Button>
          </Space>
        </Form>
      </Modal>
    </div>
  )
}
