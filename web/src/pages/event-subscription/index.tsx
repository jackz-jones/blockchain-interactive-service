import { useEffect, useState } from 'react'
import { Table, Button, Card, Typography, Space, Tag, Form, Input, Modal, Popconfirm, message } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'

const { Title, Text } = Typography

interface Subscription {
  subscription_id: string
  chain_name: string
  contract_name?: string
  contract_address?: string
  status: string
  created_at: string
}

interface EventItem {
  event_id: string
  event_name: string
  data: unknown
  timestamp: string
}

export default function EventSubscription() {
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [pollLoading, setPollLoading] = useState<string | null>(null)
  const [events, setEvents] = useState<EventItem[]>([])
  const [eventsModalOpen, setEventsModalOpen] = useState(false)
  const [form] = Form.useForm()

  const fetchList = async () => {
    setLoading(true)
    try {
      const res = await api.get('/event/subscriptions')
      setSubscriptions((res.data as { items: Subscription[] }).items || [])
    } catch {
      // 错误已由拦截器处理
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchList()
  }, [])

  const handleCreate = async (values: Record<string, string>) => {
    setCreateLoading(true)
    try {
      await api.post('/event/subscribe', values)
      message.success('订阅创建成功')
      setCreateOpen(false)
      form.resetFields()
      fetchList()
    } catch {
      // 错误已由拦截器处理
    } finally {
      setCreateLoading(false)
    }
  }

  const handlePoll = async (subscriptionId: string) => {
    setPollLoading(subscriptionId)
    try {
      const res = await api.post('/event/poll', { subscription_id: subscriptionId })
      setEvents((res.data as { events: EventItem[] }).events || [])
      setEventsModalOpen(true)
    } catch {
      // 错误已由拦截器处理
    } finally {
      setPollLoading(null)
    }
  }

  const handleUnsubscribe = async (subscriptionId: string) => {
    try {
      await api.post('/event/unsubscribe', { subscription_id: subscriptionId })
      message.success('已取消订阅')
      fetchList()
    } catch {
      // 错误已由拦截器处理
    }
  }

  const columns: ColumnsType<Subscription> = [
    {
      title: 'Subscription ID',
      dataIndex: 'subscription_id',
      key: 'subscription_id',
      render: (id: string) => <Text code style={{ fontSize: 12 }}>{id.slice(0, 12)}...</Text>,
    },
    { title: '链名称', dataIndex: 'chain_name', key: 'chain_name' },
    { title: '合约名称', dataIndex: 'contract_name', key: 'contract_name', render: (v: string) => v || '-' },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={status === 'active' ? 'success' : 'default'}>{status}</Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (time: string) => new Date(time).toLocaleDateString('zh-CN'),
    },
    {
      title: '操作',
      key: 'actions',
      width: 180,
      render: (_, record) => (
        <Space size="small">
          <Button
            type="text"
            size="small"
            icon={<ReloadOutlined />}
            loading={pollLoading === record.subscription_id}
            onClick={() => handlePoll(record.subscription_id)}
          >
            轮询
          </Button>
          <Popconfirm
            title="确认取消订阅？"
            onConfirm={() => handleUnsubscribe(record.subscription_id)}
            okText="确认"
            cancelText="取消"
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
        <Title level={4} style={{ margin: 0 }}>事件订阅</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          新建订阅
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={subscriptions}
        rowKey="subscription_id"
        loading={loading}
        pagination={{ pageSize: 10 }}
        size="middle"
      />

      {/* 创建订阅弹窗 */}
      <Modal
        title="新建事件订阅"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        footer={null}
      >
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <Form.Item name="chain_name" label="链名称" rules={[{ required: true, message: '请输入链名称' }]}>
            <Input placeholder="链名称" />
          </Form.Item>
          <Form.Item name="contract_name" label="合约名称">
            <Input placeholder="合约名称（可选）" />
          </Form.Item>
          <Form.Item name="contract_address" label="合约地址">
            <Input placeholder="合约地址（可选）" />
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={createLoading}>创建</Button>
            <Button onClick={() => setCreateOpen(false)}>取消</Button>
          </Space>
        </Form>
      </Modal>

      {/* 事件列表弹窗 */}
      <Modal
        title="轮询事件"
        open={eventsModalOpen}
        onCancel={() => setEventsModalOpen(false)}
        footer={null}
        width={700}
      >
        {events.length === 0 ? (
          <div style={{ textAlign: 'center', padding: 40, color: 'var(--color-muted)' }}>
            暂无新事件
          </div>
        ) : (
          <div style={{ maxHeight: 400, overflow: 'auto' }}>
            {events.map((evt) => (
              <Card key={evt.event_id} size="small" style={{ marginBottom: 8 }}>
                <Space direction="vertical" size="small" style={{ width: '100%' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <Text strong>{evt.event_name}</Text>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {new Date(evt.timestamp).toLocaleString('zh-CN')}
                    </Text>
                  </div>
                  <Text code style={{ fontSize: 11, wordBreak: 'break-all' }}>
                    {JSON.stringify(evt.data)}
                  </Text>
                </Space>
              </Card>
            ))}
          </div>
        )}
      </Modal>
    </div>
  )
}
