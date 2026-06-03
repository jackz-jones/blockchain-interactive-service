import { useEffect, useState } from 'react'
import { Table, Button, Typography, Tag, Space, Modal, Form, Input, Select, message, Alert } from 'antd'
import { PlusOutlined, CopyOutlined, WarningOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'

const { Title, Text, Paragraph } = Typography

interface ApiKey {
  id: string
  name: string
  key_prefix: string
  permissions: string[]
  ip_whitelist: string[]
  expires_at: string
  created_at: string
}

export default function ApiKeyList() {
  const [data, setData] = useState<ApiKey[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [newKey, setNewKey] = useState<string | null>(null)
  const [form] = Form.useForm()

  const fetchList = async () => {
    setLoading(true)
    try {
      const res = await api.get('/api-keys')
      setData((res.data as { items: ApiKey[] }).items || [])
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
      const res = await api.post('/api-keys', values)
      const data = res.data as { api_key: string }
      setNewKey(data.api_key)
      setCreateOpen(false)
      form.resetFields()
      fetchList()
    } catch {
      // 错误已由拦截器处理
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

  const isExpired = (date: string) => new Date(date) < new Date()
  const isExpiringSoon = (date: string) => {
    const diff = new Date(date).getTime() - Date.now()
    return diff > 0 && diff < 7 * 24 * 60 * 60 * 1000 // 7天内过期
  }

  const columns: ColumnsType<ApiKey> = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    {
      title: 'Key 前缀',
      dataIndex: 'key_prefix',
      key: 'key_prefix',
      render: (prefix: string) => <Text code>{prefix}...</Text>,
    },
    {
      title: '权限',
      dataIndex: 'permissions',
      key: 'permissions',
      render: (perms: string[]) => (
        <Space size={4} wrap>
          {perms?.map((p) => <Tag key={p}>{p}</Tag>)}
        </Space>
      ),
    },
    {
      title: 'IP 白名单',
      dataIndex: 'ip_whitelist',
      key: 'ip_whitelist',
      render: (ips: string[]) => ips?.length ? ips.join(', ') : <Text type="secondary">不限制</Text>,
    },
    {
      title: '过期时间',
      dataIndex: 'expires_at',
      key: 'expires_at',
      render: (date: string) => {
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
        rowKey="id"
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
