import { useEffect, useState } from 'react'
import { Table, Button, Typography, Tag, Space, Modal, Form, Input, Switch, Popconfirm, message } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'

const { Title } = Typography

interface Tenant {
  id: string
  name: string
  enabled: boolean
  quota_limit: number
  created_at: string
}

export default function TenantList() {
  const [data, setData] = useState<Tenant[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [form] = Form.useForm()

  const fetchList = async () => {
    setLoading(true)
    try {
      const res = await api.get('/tenants')
      setData((res.data as { items: Tenant[] }).items || [])
    } catch {
      // 错误已由拦截器处理
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
    } catch {
      // 错误已由拦截器处理
    } finally {
      setCreateLoading(false)
    }
  }

  const handleToggle = async (id: string, enabled: boolean) => {
    try {
      await api.put(`/tenants/${id}`, { enabled })
      message.success(enabled ? '已启用' : '已禁用')
      fetchList()
    } catch {
      // 错误已由拦截器处理
    }
  }

  const columns: ColumnsType<Tenant> = [
    { title: '租户名称', dataIndex: 'name', key: 'name' },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'success' : 'default'}>{enabled ? '已启用' : '已禁用'}</Tag>
      ),
    },
    { title: '配额上限', dataIndex: 'quota_limit', key: 'quota_limit' },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (time: string) => new Date(time).toLocaleDateString('zh-CN'),
    },
    {
      title: '操作',
      key: 'actions',
      render: (_, record) => (
        <Popconfirm
          title={record.enabled ? '确认禁用？' : '确认启用？'}
          onConfirm={() => handleToggle(record.id, !record.enabled)}
        >
          <Button type="text" size="small">
            {record.enabled ? '禁用' : '启用'}
          </Button>
        </Popconfirm>
      ),
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
          <Form.Item name="quota_limit" label="配额上限">
            <Input type="number" placeholder="每月调用次数上限" />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked" initialValue={true}>
            <Switch />
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
