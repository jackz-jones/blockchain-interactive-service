import { useEffect, useState } from 'react'
import { Table, Button, Typography, Tag, Space, Modal, Form, Input, Select, Alert, Tooltip } from 'antd'
import { PlusOutlined, CopyOutlined, WarningOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { useGlobalMessage } from '@/components/GlobalMessage'

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
  const [createOpen, setCreateOpen] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [newKey, setNewKey] = useState<string | null>(null)
  const { message } = useGlobalMessage()
  const { handleApiError } = useApiMessage()
  const [form] = Form.useForm()

  const fetchList = async () => {
    setLoading(true)
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

  const copyKey = () => {
    if (newKey) {
      navigator.clipboard.writeText(newKey)
      message.success('已复制到剪贴板')
    }
  }

  const isExpired = (date: string | null | undefined) => {
    if (!date) return false // 永不过期的 Key（expires_at 为 null），不应判为已过期
    return new Date(date) < new Date()
  }
  const isExpiringSoon = (date: string | null | undefined) => {
    if (!date) return false
    const diff = new Date(date).getTime() - Date.now()
    return diff > 0 && diff < 7 * 24 * 60 * 60 * 1000 // 7天内过期
  }

  const columns: ColumnsType<ApiKey> = [
    { title: '名称', dataIndex: 'name', key: 'name', width: 200 },
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
      width: 200,
      render: (perms: string[]) => {
        if (!perms?.length) return <Text type="secondary">不限制</Text>
        return (
          <Space size={4} wrap>
            {perms.map((p) => <Tag key={p}>{p}</Tag>)}
          </Space>
        )
      },
    },
    {
      title: 'IP 白名单',
      dataIndex: 'ip_whitelist',
      key: 'ip_whitelist',
      width: 220,
      ellipsis: { showTitle: false },
      render: (ips: string[]) => {
        if (!ips?.length) return <Text type="secondary">不限制</Text>
        const text = ips.join(', ')
        return (
          <Tooltip title={text} placement="topLeft">
            <span>{text}</span>
          </Tooltip>
        )
      },
    },
    {
      title: '过期时间',
      dataIndex: 'expires_at',
      key: 'expires_at',
      width: 180,
      render: (date: string | null | undefined) => {
        if (!date) return <Tag color="success">永不过期</Tag>
        if (isExpired(date)) return <Tag color="error">已过期</Tag>
        if (isExpiringSoon(date)) return <Tag icon={<WarningOutlined />} color="warning">即将过期</Tag>
        return new Date(date).toLocaleDateString('zh-CN')
      },
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

      {/* Key 展示弹窗 */}
      <Modal
        title="API Key 已创建"
        open={!!newKey}
        onCancel={() => setNewKey(null)}
        footer={<Button type="primary" onClick={() => setNewKey(null)}>我已保存</Button>}
      >
        <Alert
          type="warning"
          message="请立即保存此 API Key"
          description="此 Key 仅展示一次，关闭后无法再次查看。"
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
