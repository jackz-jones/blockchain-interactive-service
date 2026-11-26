import { useEffect, useState } from 'react'
import { Table, Button, Typography, Tag, Space, Modal, Form, Input, Popconfirm, Tooltip, Select } from 'antd'
import { PlusOutlined, SearchOutlined, EditOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { useGlobalMessage } from '@/components/GlobalMessage'
import { formatDateTime } from '@/utils/format'
import ErrorRetry from '@/components/ErrorRetry'

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

const planOptions = [
  { label: '免费版', value: 'free' },
  { label: '开发者版', value: 'developer' },
  { label: '企业版', value: 'enterprise' },
]

const planLabelMap: Record<string, string> = {
  free: '免费版',
  developer: '开发者版',
  enterprise: '企业版',
}

const planColorMap: Record<string, string> = {
  free: 'default',
  developer: 'blue',
  enterprise: 'purple',
}

export default function TenantList() {
  const [data, setData] = useState<Tenant[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<Tenant | null>(null)
  const [createLoading, setCreateLoading] = useState(false)
  const [editLoading, setEditLoading] = useState(false)
  const [search, setSearch] = useState('')
  const { message } = useGlobalMessage()
  const { handleApiError } = useApiMessage()
  const [createForm] = Form.useForm()
  const [editForm] = Form.useForm()

  const fetchList = async (searchOverride?: string) => {
    setLoading(true)
    setError(null)
    try {
      const params: Record<string, string> = {}
      const searchValue = searchOverride !== undefined ? searchOverride : search
      if (searchValue) params.search = searchValue
      const res = await api.get('/tenants', { params })
      setData((res.data as { items: Tenant[] }).items || [])
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

  const handleCreate = async (values: Record<string, unknown>) => {
    setCreateLoading(true)
    try {
      await api.post('/tenants', values)
      message.success('租户创建成功')
      setCreateOpen(false)
      createForm.resetFields()
      fetchList()
    } catch (err) {
      handleApiError(err)
    } finally {
      setCreateLoading(false)
    }
  }

  const handleEdit = async (values: Record<string, unknown>) => {
    if (!editRecord) return
    setEditLoading(true)
    try {
      await api.put(`/tenants/${editRecord.id}`, values)
      message.success('租户更新成功')
      setEditOpen(false)
      setEditRecord(null)
      editForm.resetFields()
      fetchList()
    } catch (err) {
      handleApiError(err)
    } finally {
      setEditLoading(false)
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

  const openEdit = (record: Tenant) => {
    setEditRecord(record)
    editForm.setFieldsValue({
      name: record.name,
      email: record.email,
      phone: record.phone,
      plan: record.plan,
    })
    setEditOpen(true)
  }

  const isEnabled = (record: Tenant) => record.status === 'active' || record.enabled

  const columns: ColumnsType<Tenant> = [
    {
      title: '租户名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      render: (name: string, record) => (
        <a onClick={() => openEdit(record)}>{name}</a>
      ),
    },
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
      width: 100,
      render: (_, record) => {
        const enabled = isEnabled(record)
        return <Tag color={enabled ? 'success' : 'default'}>{enabled ? '已启用' : '已禁用'}</Tag>
      },
    },
    {
      title: '套餐',
      dataIndex: 'plan',
      key: 'plan',
      width: 120,
      render: (plan: string) => (
        <Tag color={planColorMap[plan] || 'default'}>{planLabelMap[plan] || plan || '-'}</Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (time: string) => formatDateTime(time),
    },
    {
      title: '操作',
      key: 'actions',
      width: 160,
      render: (_, record) => {
        const enabled = isEnabled(record)
        return (
          <Space size="small">
            <Button type="text" size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} />
            <Popconfirm
              title={enabled ? '确认禁用？' : '确认启用？'}
              onConfirm={() => handleToggle(record.id, !enabled)}
            >
              <Button type="text" size="small" danger={enabled}>
                {enabled ? '禁用' : '启用'}
              </Button>
            </Popconfirm>
          </Space>
        )
      },
    },
  ]

  // 加载失败时显示错误重试组件
  if (error && !loading && data.length === 0) {
    return (
      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
          <Title level={4} style={{ margin: 0 }}>租户管理</Title>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            创建租户
          </Button>
        </div>
        <ErrorRetry error={error} onRetry={() => fetchList()} />
      </div>
    )
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <Title level={4} style={{ margin: 0 }}>租户管理</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          创建租户
        </Button>
      </div>

      <Space style={{ marginBottom: 16 }}>
        <Input
          placeholder="搜索租户名称"
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

      {/* 创建租户弹窗 */}
      <Modal title="创建租户" open={createOpen} onCancel={() => setCreateOpen(false)} footer={null}>
        <Form form={createForm} layout="vertical" onFinish={handleCreate}>
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
            <Select placeholder="选择套餐（选填）" options={planOptions} allowClear />
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={createLoading}>创建</Button>
            <Button onClick={() => setCreateOpen(false)}>取消</Button>
          </Space>
        </Form>
      </Modal>

      {/* 编辑租户弹窗 */}
      <Modal title="编辑租户" open={editOpen} onCancel={() => { setEditOpen(false); setEditRecord(null) }} footer={null}>
        <Form form={editForm} layout="vertical" onFinish={handleEdit}>
          <Form.Item name="name" label="租户名称" rules={[{ required: true, message: '请输入租户名称' }]}>
            <Input placeholder="租户名称" />
          </Form.Item>
          <Form.Item name="email" label="邮箱" rules={[{ required: true, message: '请输入邮箱' }, { type: 'email', message: '请输入有效的邮箱地址' }]}>
            <Input placeholder="管理员邮箱" />
          </Form.Item>
          <Form.Item name="phone" label="联系电话">
            <Input placeholder="联系电话（选填）" />
          </Form.Item>
          <Form.Item name="plan" label="套餐">
            <Select placeholder="选择套餐" options={planOptions} allowClear />
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={editLoading}>保存</Button>
            <Button onClick={() => { setEditOpen(false); setEditRecord(null) }}>取消</Button>
          </Space>
        </Form>
      </Modal>
    </div>
  )
}
