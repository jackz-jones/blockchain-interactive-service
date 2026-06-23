import { useEffect, useState } from 'react'
import { Table, Button, Typography, Space, Tag, Modal, Select, Form, Tooltip, Popconfirm, Empty } from 'antd'
import { PlusOutlined, DeleteOutlined, EyeOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { useGlobalMessage } from '@/components/GlobalMessage'
import { formatDateTime } from '@/utils/format'
import ErrorRetry from '@/components/ErrorRetry'

const { Title, Text } = Typography

interface SubscriptionItem {
  contract_config_id: number
  chain_config_id: number
  chain_name: string
  contract_name: string
  contract_addr: string
  status: string
  created_at: string
}

interface AvailableContract {
  contract_config_id: number
  chain_config_id: number
  chain_name: string
  contract_name: string
  contract_addr: string
}

export default function EventSubscription() {
  const [subscriptions, setSubscriptions] = useState<SubscriptionItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const { message } = useGlobalMessage()
  const { handleApiError } = useApiMessage()
  const [availableContracts, setAvailableContracts] = useState<AvailableContract[]>([])
  const [selectedContractId, setSelectedContractId] = useState<number | null>(null)
  const [form] = Form.useForm()

  // 查看最新事件相关状态
  const [eventsOpen, setEventsOpen] = useState(false)
  const [eventsLoading, setEventsLoading] = useState(false)
  const [recentEvents, setRecentEvents] = useState<any[]>([])
  const [eventsTitle, setEventsTitle] = useState('')

  const fetchList = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.get('/events/subscriptions')
      setSubscriptions((res.data as { items: SubscriptionItem[] }).items || [])
    } catch (err) {
      setError(err)
      handleApiError(err)
    } finally {
      setLoading(false)
    }
  }

  const fetchAvailableContracts = async () => {
    try {
      const res = await api.get('/events/available-contracts')
      setAvailableContracts((res.data as { items: AvailableContract[] }).items || [])
    } catch (err) {
      handleApiError(err)
    }
  }

  useEffect(() => {
    fetchList()
  }, [])

  const handleOpenCreate = () => {
    fetchAvailableContracts()
    setSelectedContractId(null)
    form.resetFields()
    setCreateOpen(true)
  }

  const handleCreate = async () => {
    if (!selectedContractId) {
      message.warning('请选择要订阅的合约')
      return
    }

    setCreateLoading(true)
    try {
      const res = await api.post('/events/subscribe-by-contract', {
        contract_config_id: selectedContractId,
      })
      const data = res as { code?: number; message?: string }
      if (data.code === 409) {
        message.warning(data.message || '该合约已存在订阅，请勿重复创建')
      } else {
        message.success('订阅创建成功')
        setCreateOpen(false)
        fetchList()
      }
    } catch (err: unknown) {
      const error = err as { response?: { data?: { message?: string; code?: number } } }
      if (error?.response?.data?.code === 409) {
        message.warning(error.response.data.message || '该合约已存在订阅，请勿重复创建')
      }
    } finally {
      setCreateLoading(false)
    }
  }

  const handleUnsubscribe = async (contractConfigId: number) => {
    try {
      await api.delete(`/events/subscribe-by-contract/${contractConfigId}`)
      message.success('已取消订阅')
      fetchList()
    } catch (err) {
      handleApiError(err)
    }
  }

  const handleViewEvents = async (record: SubscriptionItem) => {
    setEventsTitle(`${record.chain_name} / ${record.contract_name}`)
    setEventsOpen(true)
    setEventsLoading(true)
    setRecentEvents([])
    try {
      const res = await api.get(`/events/recent/${record.contract_config_id}`)
      const data = res.data as { items: any[] }
      setRecentEvents(data.items || [])
    } catch (err) {
      handleApiError(err)
    } finally {
      setEventsLoading(false)
    }
  }

  // 加载失败时显示错误重试组件
  if (error && !loading && subscriptions.length === 0) {
    return (
      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
          <Title level={4} style={{ margin: 0 }}>事件订阅管理</Title>
        </div>
        <ErrorRetry error={error} onRetry={() => fetchList()} />
      </div>
    )
  }

  const columns: ColumnsType<SubscriptionItem> = [
    {
      title: '链名称',
      dataIndex: 'chain_name',
      key: 'chain_name',
      width: 160,
      ellipsis: { showTitle: false },
      render: (name: string) => (
        <Tooltip title={name} placement="topLeft"><span>{name || '-'}</span></Tooltip>
      ),
    },
    {
      title: '合约名称',
      dataIndex: 'contract_name',
      key: 'contract_name',
      width: 200,
      ellipsis: { showTitle: false },
      render: (name: string) => (
        <Tooltip title={name} placement="topLeft"><span>{name || '-'}</span></Tooltip>
      ),
    },
    {
      title: '合约地址',
      dataIndex: 'contract_addr',
      key: 'contract_addr',
      width: 300,
      ellipsis: { showTitle: false },
      render: (addr: string) => addr ? (
        <Tooltip title={addr} placement="topLeft">
          <Text code style={{ fontSize: 12 }}>{addr}</Text>
        </Tooltip>
      ) : '-',
    },
    {
      title: '订阅状态',
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (status: string) => (
        <Tag color={status === 'active' ? 'success' : 'default'}>{status === 'active' ? '运行中' : status}</Tag>
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
      width: 200,
      render: (_, record) => (
        <Space size="small">
          <Button type="text" size="small" icon={<EyeOutlined />} onClick={() => handleViewEvents(record)}>
            查看最近事件
          </Button>
          <Popconfirm
            title="确认取消订阅？"
            description="取消后将停止监听该合约的链上事件"
            onConfirm={() => handleUnsubscribe(record.contract_config_id)}
            okText="确认"
            cancelText="取消"
          >
            <Button type="text" size="small" danger icon={<DeleteOutlined />}>
              取消订阅
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // 按链名称分组可选合约
  const chainNames = [...new Set(availableContracts.map(c => c.chain_name))]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <Title level={4} style={{ margin: 0 }}>事件订阅管理</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenCreate}>
          新建订阅
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={subscriptions}
        rowKey="contract_config_id"
        loading={loading}
        pagination={{ pageSize: 10 }}
        size="middle"
        locale={{
          emptyText: (
            <Empty
              description="暂无订阅"
              style={{ padding: '40px 0' }}
            >
              <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
                点击"新建订阅"为合约开启链上事件监听
              </Text>
              <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenCreate}>
                新建订阅
              </Button>
            </Empty>
          ),
        }}
      />

      {/* 创建订阅弹窗 */}
      <Modal
        title="新建事件订阅"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={handleCreate}
        confirmLoading={createLoading}
        okText="创建"
        cancelText="取消"
      >
        <div style={{ marginBottom: 16, color: 'var(--color-muted)', fontSize: 13 }}>
          选择一个未开启订阅的合约，系统将自动启动链上事件监听。
        </div>
        <Form form={form} layout="vertical">
          <Form.Item label="选择合约" required>
            <Select
              placeholder="请选择要订阅的合约"
              value={selectedContractId}
              onChange={(val) => setSelectedContractId(val)}
              style={{ width: '100%' }}
              showSearch
              optionFilterProp="label"
              notFoundContent={availableContracts.length === 0 ? '所有合约均已开启订阅' : '无匹配结果'}
            >
              {chainNames.map(chainName => (
                <Select.OptGroup key={chainName} label={chainName}>
                  {availableContracts
                    .filter(c => c.chain_name === chainName)
                    .map(c => (
                      <Select.Option
                        key={c.contract_config_id}
                        value={c.contract_config_id}
                        label={`${c.chain_name} - ${c.contract_name}`}
                      >
                        <div>
                          <span style={{ fontWeight: 500 }}>{c.contract_name}</span>
                          {c.contract_addr && (
                            <span style={{ marginLeft: 8, fontSize: 12, color: '#999' }}>
                              {c.contract_addr.slice(0, 10)}...
                            </span>
                          )}
                        </div>
                      </Select.Option>
                    ))}
                </Select.OptGroup>
              ))}
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      {/* 查看最近事件弹窗 */}
      <Modal
        title={`最近事件 - ${eventsTitle}${!eventsLoading ? ` (最多10条)` : ''}`}
        open={eventsOpen}
        onCancel={() => setEventsOpen(false)}
        footer={[
          <Button key="close" onClick={() => setEventsOpen(false)}>关闭</Button>
        ]}
        width={900}
      >
        {eventsLoading ? (
          <div style={{ textAlign: 'center', padding: '40px 0' }}>加载中...</div>
        ) : recentEvents.length === 0 ? (
          <Empty
            description="暂无事件数据"
            style={{ padding: '40px 0' }}
          >
            <Text type="secondary">可能订阅刚开启还未收到链上事件</Text>
          </Empty>
        ) : (
          <Table
            dataSource={recentEvents}
            rowKey={(record, idx) => record.message_id || idx}
            pagination={false}
            size="small"
            scroll={{ y: 480 }}
            columns={[
              {
                title: '事件名称',
                dataIndex: 'event_name',
                key: 'event_name',
                width: 140,
                render: (name: string) => (
                  <Tag color="blue">{name || '未知事件'}</Tag>
                ),
              },
              {
                title: '事件数据',
                dataIndex: 'data',
                key: 'data',
                render: (data: unknown) => (
                  <pre style={{
                    margin: 0,
                    fontSize: 12,
                    lineHeight: 1.5,
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-all',
                  }}>
                    {typeof data === 'object' ? JSON.stringify(data, null, 2) : String(data || '')}
                  </pre>
                ),
              },
              {
                title: '消息ID',
                dataIndex: 'message_id',
                key: 'message_id',
                width: 180,
                ellipsis: { showTitle: false },
                render: (id: string) => (
                  <Tooltip title={id} placement="topLeft">
                    <Text style={{ fontSize: 12 }} type="secondary">{id}</Text>
                  </Tooltip>
                ),
              },
            ]}
          />
        )}
      </Modal>
    </div>
  )
}
