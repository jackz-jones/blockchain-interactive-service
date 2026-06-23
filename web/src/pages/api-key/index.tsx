import { useEffect, useState } from 'react'
import { Table, Button, Typography, Tag, Space, Modal, Form, Input, Select, Alert, Tooltip, Popconfirm, Switch } from 'antd'
import { PlusOutlined, CopyOutlined, WarningOutlined, DeleteOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { useGlobalMessage } from '@/components/GlobalMessage'
import { formatDateTime } from '@/utils/format'
import ErrorRetry from '@/components/ErrorRetry'

const { Title, Text, Paragraph } = Typography

interface ApiKey {
  ID: number
  name: string
  key: string
  key_masked: string
  permissions: string[]
  ip_whitelist: string[]
  status: string
  expires_at: string | null
  CreatedAt: string
  UpdatedAt: string
  last_used_at: string | null
}

export default function ApiKeyList() {
  const [data, setData] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [newKey, setNewKey] = useState<string | null>(null)
  const { message } = useGlobalMessage()
  const { handleApiError } = useApiMessage()
  const [form] = Form.useForm()

  const fetchList = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.get('/api-keys')
      const rawItems = (res.data as { items: any[] }).items || []
      const items = rawItems.map((item: any) => ({
        ...item,
        permissions: typeof item.permissions === 'string'
          ? (item.permissions ? item.permissions.split(',').filter(Boolean) : [])
          : (item.permissions || []),
        ip_whitelist: typeof item.ip_whitelist === 'string'
          ? (item.ip_whitelist ? item.ip_whitelist.split(',').filter(Boolean) : [])
          : (item.ip_whitelist || []),
      }))
      setData(items)
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
      const res = await api.post('/api-keys', values)
      const result = res.data as { id: number; key: string }
      message.success('创建成功')
      setNewKey(result.key)
      setCreateOpen(false)
      form.resetFields()
      fetchList()
    } catch (err) {
      handleApiError(err)
    } finally {
      setCreateLoading(false)
    }
  }

  const handleToggleStatus = async (record: ApiKey) => {
    const newStatus = record.status === 'active' ? 'disabled' : 'active'
    try {
      await api.put(`/api-keys/${record.ID}`, { status: newStatus })
      message.success(newStatus === 'active' ? '已启用' : '已禁用')
      fetchList()
    } catch (err) {
      handleApiError(err)
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await api.delete(`/api-keys/${id}`)
      message.success('删除成功')
      fetchList()
    } catch (err) {
      handleApiError(err)
    }
  }

  const copyKey = () => {
    if (newKey) {
      navigator.clipboard.writeText(newKey)
      message.success('已复制到剪贴板')
    }
  }

  const isExpired = (date: string | null | undefined) => {
    if (!date) return false
    return new Date(date) < new Date()
  }
  const isExpiringSoon = (date: string | null | undefined) => {
    if (!date) return false
    const diff = new Date(date).getTime() - Date.now()
    return diff > 0 && diff < 7 * 24 * 60 * 60 * 1000
  }

  const columns: ColumnsType<ApiKey> = [
    { title: '名称', dataIndex: 'name', key: 'name', width: 180 },
    {
      title: 'Key',
      dataIndex: 'key_masked',
      key: 'key_masked',
      width: 240,
      render: (masked: string) => <Text code>{masked || '-'}</Text>,
    },
    {
      title: '权限',
      dataIndex: 'permissions',
      key: 'permissions',
      width: 180,
      render: (perms: string[]) => {
        if (!perms?.length) return <Text type="secondary">不限制</Text>
        // 加载失败时显示错误重试组件
  if (error && !loading && data.length === 0) {
    return (
      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
          <Title level={4} style={{ margin: 0 }}>API Key 管理</Title>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            创建 API Key
          </Button>
        </div>
        <ErrorRetry error={error} onRetry={() => fetchList()} />
      </div>
    )
  }

  return (
          <Space size={4} wrap>
            {perms.map((p) => <Tag key={p}>{p}</Tag>)}
          </Space>
        )
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => (
        <Tag color={status === 'active' ? 'success' : 'default'}>{status === 'active' ? '启用' : '禁用'}</Tag>
      ),
    },
    {
      title: '过期时间',
      dataIndex: 'expires_at',
      key: 'expires_at',
      width: 180,
      render: (date: string | null | undefined) => {
        if (!date) return <Tag color="success">永不过期</Tag>
        if (isExpired(date)) return <Tooltip title={formatDateTime(date)}><Tag color="error">已过期</Tag></Tooltip>
        if (isExpiringSoon(date)) return <Tooltip title={formatDateTime(date)}><Tag icon={<WarningOutlined />} color="warning">即将过期</Tag></Tooltip>
        return <Tooltip title={formatDateTime(date)}><span>{formatDateTime(date)}</span></Tooltip>
      },
    },
    {
      title: '最后使用',
      dataIndex: 'last_used_at',
      key: 'last_used_at',
      width: 180,
      render: (time: string | null) => time ? formatDateTime(time) : <Text type="secondary">从未使用</Text>,
    },
    {
      title: '操作',
      key: 'actions',
      width: 160,
      render: (_, record) => (
        <Space size="small">
          <Tooltip title={record.status === 'active' ? '禁用' : '启用'}>
            <Switch
              size="small"
              checked={record.status === 'active'}
              onChange={() => handleToggleStatus(record)}
            />
          </Tooltip>
          <Popconfirm
            title="确认删除"
            description="删除后不可恢复，确定要删除此 API Key 吗？"
            onConfirm={() => handleDelete(record.ID)}
            okText="删除"
            cancelText="取消"
            okButtonProps={{ danger: true }}
          >
            <Button type="text" size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <Title level={4} style={{ margin: 0 }}>API Key 管理</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          创建 API Key
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={data}
        rowKey="ID"
        loading={loading}
        pagination={{ pageSize: 10 }}
        size="middle"
      />

      {/* 创建弹窗 */}
      <Modal title="创建 API Key" open={createOpen} onCancel={() => setCreateOpen(false)} footer={null}>
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="API Key 用途描述" />
          </Form.Item>
          <Form.Item name="permissions" label="权限">
            <Select
              mode="multiple"
              placeholder="选择权限"
              options={[
                { label: '读取', value: 'read' },
                { label: '写入', value: 'write' },
                { label: '管理', value: 'admin' },
              ]}
            />
          </Form.Item>
          <Form.Item name="ip_whitelist" label="IP 白名单">
            <Select mode="tags" placeholder="输入 IP 地址后回车（留空不限制）" />
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={createLoading}>创建</Button>
            <Button onClick={() => setCreateOpen(false)}>取消</Button>
          </Space>
        </Form>
      </Modal>

      {/* Key 展示弹窗 - 强化安全提示 */}
      <Modal
        title="API Key 已创建"
        open={!!newKey}
        onCancel={() => setNewKey(null)}
        footer={<Button type="primary" onClick={() => setNewKey(null)}>我已安全保存</Button>}
        maskClosable={false}
        closable={false}
      >
        <Alert
          type="warning"
          message="请立即保存此 API Key"
          description={
            <div>
              <p style={{ margin: '4px 0' }}>• 此 Key 仅展示一次，关闭后<strong>无法再次查看</strong></p>
              <p style={{ margin: '4px 0' }}>• 请勿将 Key 提交到代码仓库或分享给他人</p>
              <p style={{ margin: '4px 0' }}>• 如果 Key 泄露，请立即删除并重新创建</p>
            </div>
          }
          showIcon
          style={{ marginBottom: 16 }}
        />
        <div style={{ background: '#f5f5f5', padding: 12, borderRadius: 6, display: 'flex', alignItems: 'center', gap: 8 }}>
          <Paragraph code copyable style={{ margin: 0, flex: 1, wordBreak: 'break-all' }}>
            {newKey}
          </Paragraph>
          <Button icon={<CopyOutlined />} onClick={copyKey} size="small" />
        </div>
      </Modal>
    </div>
  )
}
