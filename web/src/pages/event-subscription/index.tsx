import { useEffect, useState } from 'react'
import { Table, Button, Typography, Space, Tag, Modal, Select, Form, Tooltip, Popconfirm, Empty, InputNumber, Divider, Switch } from 'antd'
import { PlusOutlined, DeleteOutlined, EyeOutlined, EditOutlined, QuestionCircleOutlined } from '@ant-design/icons'
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
  chain_type: string
  contract_name: string
  contract_addr: string
  status: string
  created_at: string
  extra_conf?: Record<string, unknown>
}

interface AvailableContract {
  contract_config_id: number
  chain_config_id: number
  chain_name: string
  chain_type: string
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
  const [selectedChainName, setSelectedChainName] = useState<string | null>(null)
  const [selectedContractId, setSelectedContractId] = useState<number | null>(null)
  const [selectedChainType, setSelectedChainType] = useState<string>('')
  const [form] = Form.useForm()

  // 查看最新事件相关状态
  const [eventsOpen, setEventsOpen] = useState(false)
  const [eventsLoading, setEventsLoading] = useState(false)
  const [recentEvents, setRecentEvents] = useState<any[]>([])
  const [eventsTitle, setEventsTitle] = useState('')

  // 编辑订阅配置相关状态
  const [editOpen, setEditOpen] = useState(false)
  const [editLoading, setEditLoading] = useState(false)
  const [editRecord, setEditRecord] = useState<SubscriptionItem | null>(null)
  const [editForm] = Form.useForm()

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
    setSelectedChainName(null)
    setSelectedContractId(null)
    setSelectedChainType('')
    form.resetFields()
    setCreateOpen(true)
  }

  // 获取当前选中合约的链类型
  const getSelectedContractChainType = (contractId: number | null): string => {
    if (!contractId) return ''
    const contract = availableContracts.find(c => c.contract_config_id === contractId)
    return contract?.chain_type || ''
  }

  // 当选中的链改变时，重置合约选择和表单
  const handleChainChange = (chainName: string | null) => {
    setSelectedChainName(chainName)
    setSelectedContractId(null)
    setSelectedChainType('')
    form.resetFields(['contract', 'deploy_block_height', 'get_history_event_interval', 'get_history_event_height_window'])
  }

  // 当选中的合约改变时，更新链类型
  const handleContractChange = (contractId: number | null) => {
    setSelectedContractId(contractId)
    const chainType = getSelectedContractChainType(contractId)
    setSelectedChainType(chainType)
    // 切换合约时重置订阅参数
    form.resetFields(['deploy_block_height', 'get_history_event_interval', 'get_history_event_height_window'])
  }

  const handleCreate = async () => {
    if (!selectedContractId) {
      message.warning('请选择要订阅的合约')
      return
    }

    const values = form.getFieldsValue()
    const payload: Record<string, unknown> = {
      contract_config_id: selectedContractId,
    }

    // 根据链类型组装订阅参数
    if (selectedChainType === 'ethereum' || selectedChainType === 'chainmaker') {
      if (values.deploy_block_height !== undefined && values.deploy_block_height !== null) {
        payload.deploy_block_height = Number(values.deploy_block_height)
      }
    }
    if (selectedChainType === 'solana') {
      if (values.deploy_block_height !== undefined && values.deploy_block_height !== null) {
        payload.deploy_block_height = Number(values.deploy_block_height)
      }
    }
    if (selectedChainType === 'ethereum') {
      if (values.get_history_event_interval !== undefined && values.get_history_event_interval !== null) {
        payload.get_history_event_interval = Number(values.get_history_event_interval)
      }
      if (values.get_history_event_height_window !== undefined && values.get_history_event_height_window !== null) {
        payload.get_history_event_height_window = Number(values.get_history_event_height_window)
      }
    }

    setCreateLoading(true)
    try {
      const res = await api.post('/events/subscribe-by-contract', payload)
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

  // 打开编辑订阅配置弹窗
  const handleOpenEdit = (record: SubscriptionItem) => {
    setEditRecord(record)
    const extraConf = record.extra_conf || {}
    editForm.setFieldsValue({
      enable_subscribe: record.status === 'active',
      deploy_block_height: extraConf.DeployBlockHeight ?? undefined,
      get_history_event_interval: extraConf.GetHistoryEventInterval ?? undefined,
      get_history_event_height_window: extraConf.GetHistoryEventHeightWindow ?? undefined,
    })
    setEditOpen(true)
  }

  // 提交编辑订阅配置
  const handleEditSubmit = async () => {
    if (!editRecord) return
    const values = editForm.getFieldsValue()
    const payload: Record<string, unknown> = {
      enable_subscribe: values.enable_subscribe ?? true,
    }

    if (values.deploy_block_height !== undefined && values.deploy_block_height !== null) {
      payload.deploy_block_height = Number(values.deploy_block_height)
    }
    if (editRecord.chain_type === 'ethereum') {
      if (values.get_history_event_interval !== undefined && values.get_history_event_interval !== null) {
        payload.get_history_event_interval = Number(values.get_history_event_interval)
      }
      if (values.get_history_event_height_window !== undefined && values.get_history_event_height_window !== null) {
        payload.get_history_event_height_window = Number(values.get_history_event_height_window)
      }
    }

    setEditLoading(true)
    try {
      await api.put(`/events/subscribe-by-contract/${editRecord.contract_config_id}`, payload)
      message.success('订阅配置更新成功')
      setEditOpen(false)
      fetchList()
    } catch (err) {
      handleApiError(err)
    } finally {
      setEditLoading(false)
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
      width: 260,
      render: (_, record) => (
        <Space size="small">
          <Button type="text" size="small" icon={<EyeOutlined />} onClick={() => handleViewEvents(record)}>
            查看最近事件
          </Button>
          <Button type="text" size="small" icon={<EditOutlined />} onClick={() => handleOpenEdit(record)}>
            编辑配置
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
  // 根据选中的链过滤合约
  const filteredContracts = selectedChainName
    ? availableContracts.filter(c => c.chain_name === selectedChainName)
    : []

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
        okText="创建订阅"
        cancelText="取消"
        width={640}
      >
        <Form form={form} layout="vertical">
          {/* 第一步：选择链 */}
          <Form.Item label="1. 选择链" required>
            <Select
              placeholder="请选择链"
              value={selectedChainName}
              onChange={handleChainChange}
              style={{ width: '100%' }}
              notFoundContent={availableContracts.length === 0 ? '暂无可用的链' : '无匹配结果'}
            >
              {chainNames.map(chainName => (
                <Select.Option key={chainName} value={chainName}>
                  {chainName}
                </Select.Option>
              ))}
            </Select>
          </Form.Item>

          {/* 第二步：选择合约 */}
          <Form.Item label="2. 选择合约" required>
            <Select
              placeholder={selectedChainName ? '请选择要订阅的合约' : '请先选择链'}
              value={selectedContractId}
              onChange={handleContractChange}
              style={{ width: '100%' }}
              showSearch
              optionFilterProp="label"
              disabled={!selectedChainName}
              notFoundContent={
                !selectedChainName
                  ? '请先选择链'
                  : filteredContracts.length === 0
                    ? '该链下所有合约均已开启订阅'
                    : '无匹配结果'
              }
            >
              {filteredContracts.map(c => (
                <Select.Option
                  key={c.contract_config_id}
                  value={c.contract_config_id}
                  label={c.contract_name}
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
            </Select>
          </Form.Item>

          {/* 第三步：订阅参数配置 */}
          {selectedContractId && (
            <>
              <Divider style={{ margin: '8px 0 16px' }} />
              <div style={{ marginBottom: 12, color: 'var(--color-muted)', fontSize: 13 }}>
                3. 订阅参数配置
                <Tooltip title="配置事件订阅的参数，不同链类型支持的参数可能不同">
                  <QuestionCircleOutlined style={{ marginLeft: 4, color: '#bbb' }} />
                </Tooltip>
              </div>

              {/* 通用参数：合约初始订阅开始高度 */}
              {(selectedChainType === 'ethereum' || selectedChainType === 'chainmaker' || selectedChainType === 'solana') && (
                <Form.Item
                  label="合约初始订阅开始高度"
                  name="deploy_block_height"
                  tooltip="首次订阅时从该区块高度开始扫描事件，通常设为合约部署高度；设为 0 则从最新区块开始"
                >
                  <InputNumber
                    style={{ width: '100%' }}
                    min={0}
                    placeholder="0（从最新区块开始）"
                  />
                </Form.Item>
              )}

              {/* Ethereum 特有参数 */}
              {selectedChainType === 'ethereum' && (
                <>
                  <Form.Item
                    label="事件轮询间隔（ms）"
                    name="get_history_event_interval"
                    tooltip="轮询链上事件的时间间隔，单位毫秒"
                  >
                    <InputNumber
                      style={{ width: '100%' }}
                      min={1000}
                      placeholder="12000（默认）"
                    />
                  </Form.Item>
                  <Form.Item
                    label="区块扫描窗口"
                    name="get_history_event_height_window"
                    tooltip="每次轮询扫描的区块数量窗口"
                  >
                    <InputNumber
                      style={{ width: '100%' }}
                      min={1}
                      placeholder="100（默认）"
                    />
                  </Form.Item>
                </>
              )}

              {/* ChainMaker 特有参数 */}
              {selectedChainType === 'chainmaker' && (
                <div style={{ color: 'var(--color-muted)', fontSize: 13, padding: '8px 0' }}>
                  长安链订阅使用节点长连接推送模式，无需配置轮询间隔和扫描窗口
                </div>
              )}

              {/* Solana 特有参数 */}
              {selectedChainType === 'solana' && (
                <div style={{ color: 'var(--color-muted)', fontSize: 13, padding: '8px 0' }}>
                  Solana 使用 WebSocket 订阅模式，无需配置轮询间隔和扫描窗口
                </div>
              )}
            </>
          )}
        </Form>
      </Modal>

      {/* 编辑订阅配置弹窗 */}
      <Modal
        title={`编辑订阅配置 - ${editRecord?.chain_name || ''} / ${editRecord?.contract_name || ''}`}
        open={editOpen}
        onCancel={() => setEditOpen(false)}
        onOk={handleEditSubmit}
        confirmLoading={editLoading}
        okText="保存修改"
        cancelText="取消"
        width={560}
      >
        <Form form={editForm} layout="vertical">
          <Form.Item
            name="enable_subscribe"
            label="订阅开关"
            valuePropName="checked"
          >
            <Switch checkedChildren="开启" unCheckedChildren="关闭" />
          </Form.Item>

          <Divider style={{ margin: '8px 0 16px' }} />
          <div style={{ marginBottom: 12, color: 'var(--color-muted)', fontSize: 13 }}>
            订阅参数配置
            <Tooltip title="修改订阅参数后，系统将自动重启订阅以应用新配置">
              <QuestionCircleOutlined style={{ marginLeft: 4, color: '#bbb' }} />
            </Tooltip>
          </div>

          {/* 合约初始订阅开始高度 - 所有链通用 */}
          <Form.Item
            name="deploy_block_height"
            label={
              <span>
                {editRecord?.chain_type === 'solana' ? '合约初始订阅开始 Slot 高度' : '合约初始订阅开始高度'}&nbsp;
                <Tooltip title={editRecord?.chain_type === 'solana' ? '首次订阅时从该 Slot 开始扫描事件，通常设为合约部署 Slot；设为 0 则从最新 Slot 开始' : '首次订阅时从该区块高度开始扫描事件，通常设为合约部署高度；设为 0 则从最新区块开始'}>
                  <QuestionCircleOutlined style={{ color: '#999' }} />
                </Tooltip>
              </span>
            }
          >
            <InputNumber
              style={{ width: '100%' }}
              min={0}
              placeholder="0（从最新区块开始）"
            />
          </Form.Item>

          {/* Ethereum 特有参数 */}
          {editRecord?.chain_type === 'ethereum' && (
            <>
              <Form.Item
                name="get_history_event_interval"
                label={
                  <span>
                    事件轮询间隔（ms）&nbsp;
                    <Tooltip title="每隔多少毫秒轮询一次链上历史事件，默认 12000ms">
                      <QuestionCircleOutlined style={{ color: '#999' }} />
                    </Tooltip>
                  </span>
                }
              >
                <InputNumber
                  style={{ width: '100%' }}
                  min={1000}
                  placeholder="12000（默认）"
                />
              </Form.Item>
              <Form.Item
                name="get_history_event_height_window"
                label={
                  <span>
                    区块扫描窗口&nbsp;
                    <Tooltip title="每次轮询扫描多少个区块的事件，默认 100">
                      <QuestionCircleOutlined style={{ color: '#999' }} />
                    </Tooltip>
                  </span>
                }
              >
                <InputNumber
                  style={{ width: '100%' }}
                  min={1}
                  placeholder="100（默认）"
                />
              </Form.Item>
            </>
          )}

          {/* ChainMaker 特有说明 */}
          {editRecord?.chain_type === 'chainmaker' && (
            <div style={{ color: 'var(--color-muted)', fontSize: 13, padding: '8px 0' }}>
              长安链订阅使用节点长连接推送模式，无需配置轮询间隔和扫描窗口
            </div>
          )}

          {/* Solana 特有说明 */}
          {editRecord?.chain_type === 'solana' && (
            <div style={{ color: 'var(--color-muted)', fontSize: 13, padding: '8px 0' }}>
              Solana 使用 WebSocket 订阅模式，无需配置轮询间隔和扫描窗口
            </div>
          )}
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
                title: '消息ID',
                dataIndex: 'message_id',
                key: 'message_id',
                width: 200,
                render: (id: string, record: any) => (
                  <div>
                    <Tooltip title={id} placement="topLeft">
                      <Text style={{ fontSize: 12 }} type="secondary">{id}</Text>
                    </Tooltip>
                    {record.timestamp > 0 && (
                      <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 2 }}>
                        {formatDateTime(new Date(record.timestamp).toISOString())}
                      </Text>
                    )}
                  </div>
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
                    textAlign: 'left',
                  }}>
                    {typeof data === 'object' ? JSON.stringify(data, null, 2) : String(data || '')}
                  </pre>
                ),
              },
              {
                title: '事件名称',
                dataIndex: 'event_name',
                key: 'event_name',
                width: 140,
                render: (name: string) => (
                  <Tag color="blue">{name || '未知事件'}</Tag>
                ),
              },
            ]}
          />
        )}
      </Modal>
    </div>
  )
}
