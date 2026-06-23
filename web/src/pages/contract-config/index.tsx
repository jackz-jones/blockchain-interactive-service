import { useEffect, useState } from 'react'
import { Table, Button, Space, Typography, Tag, Select, Popconfirm, Tooltip } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { useGlobalMessage } from '@/components/GlobalMessage'
import { formatDateTime } from '@/utils/format'

const { Title } = Typography

interface ChainOption {
  ID: number
  chain_name: string
}

interface ContractConfig {
  ID: number
  contract_name: string
  contract_addr: string
  chain_config_id: number
  chain_name: string
  enable_subscribe: boolean
  CreatedAt: string
}

export default function ContractConfigList() {
  const navigate = useNavigate()
  const { message } = useGlobalMessage()
  const { handleApiError } = useApiMessage()
  const [data, setData] = useState<ContractConfig[]>([])
  const [loading, setLoading] = useState(false)
  const [chains, setChains] = useState<ChainOption[]>([])
  const [selectedChainId, setSelectedChainId] = useState<string | undefined>()
  const [selectedChainName, setSelectedChainName] = useState<string | undefined>()

  // 获取链配置列表
  const fetchChains = async () => {
    try {
      const res = await api.get('/chain-configs')
      const items = (res.data as ChainOption[]) || []
      setChains(items)
      // 如果只有一个链配置，自动选中
      if (items.length === 1 && items[0]) {
        setSelectedChainId(String(items[0].ID))
        setSelectedChainName(items[0].chain_name)
      }
    } catch (err) {
      handleApiError(err)
    }
  }

  // 获取合约配置列表
  const fetchList = async () => {
    if (!selectedChainId) {
      setData([])
      return
    }
    setLoading(true)
    try {
      const res = await api.get(`/chain-configs/${selectedChainId}/contracts`)
      setData((res.data as { items: ContractConfig[] })?.items || [])
    } catch (err) {
      handleApiError(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchChains()
  }, [])

  useEffect(() => {
    fetchList()
  }, [selectedChainId])

  const handleDelete = async (record: ContractConfig) => {
    try {
      await api.delete(`/chain-configs/${record.chain_config_id}/contracts/${record.ID}`)
      message.success('删除成功')
      fetchList()
    } catch (err) {
      handleApiError(err)
    }
  }

  const columns: ColumnsType<ContractConfig> = [
    {
      title: '合约名称',
      dataIndex: 'contract_name',
      key: 'contract_name',
      width: 220,
      ellipsis: { showTitle: false },
      render: (text: string, record) => (
        <Tooltip title={text} placement="topLeft">
          <a onClick={() => navigate(`/chain-configs/${record.chain_config_id}/contracts/${record.ID}/edit`)}>{text}</a>
        </Tooltip>
      ),
    },
    {
      title: '合约地址',
      dataIndex: 'contract_addr',
      key: 'contract_addr',
      width: 340,
      ellipsis: { showTitle: false },
      render: (addr: string) => addr ? (
        <Tooltip title={addr} placement="topLeft">
          <Typography.Text code style={{ fontSize: 12 }}>{addr}</Typography.Text>
        </Tooltip>
      ) : '-',
    },
    {
      title: '所属链',
      dataIndex: 'chain_name',
      key: 'chain_name',
      width: 160,
      ellipsis: { showTitle: false },
      render: (chainName: string) => {
        const name = chainName || selectedChainName || '-'
        return (
          <Tooltip title={name} placement="topLeft">
            <span>{name}</span>
          </Tooltip>
        )
      },
    },
    {
      title: '事件订阅',
      dataIndex: 'enable_subscribe',
      key: 'enable_subscribe',
      width: 120,
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'processing' : 'default'}>
          {enabled ? '已开启' : '未开启'}
        </Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'CreatedAt',
      key: 'CreatedAt',
      width: 180,
      render: (time: string) => formatDateTime(time),
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      render: (_, record) => (
        <Space size="small">
          <Button
            type="text"
            size="small"
            icon={<EditOutlined />}
            onClick={() => navigate(`/chain-configs/${record.chain_config_id}/contracts/${record.ID}/edit`)}
          />
          <Popconfirm
            title="确认删除"
            description="删除后不可恢复，确定要删除吗？"
            onConfirm={() => handleDelete(record)}
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
        <Title level={4} style={{ margin: 0 }}>合约配置</Title>
        <Tooltip title={!selectedChainId ? '请先选择链配置' : undefined}>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            disabled={!selectedChainId}
            onClick={() => navigate(`/chain-configs/${selectedChainId}/contracts/create`)}
          >
            新建合约配置
          </Button>
        </Tooltip>
      </div>

      <Space style={{ marginBottom: 16 }}>
        <Select
          placeholder="请选择链配置"
          value={selectedChainId}
          onChange={(val, opt) => {
            setSelectedChainId(val)
            const option = opt as { label: string; value: string }
            setSelectedChainName(option?.label)
          }}
          allowClear
          onClear={() => {
            setSelectedChainId(undefined)
            setSelectedChainName(undefined)
          }}
          style={{ width: 250 }}
          options={chains.map((c) => ({ label: c.chain_name, value: String(c.ID) }))}
        />
      </Space>

      <Table
        columns={columns}
        dataSource={data}
        rowKey="ID"
        loading={loading}
        pagination={{ pageSize: 10, showSizeChanger: true, showTotal: (total) => `共 ${total} 条` }}
        size="middle"
        locale={{ emptyText: selectedChainId ? '暂无合约配置' : '请先选择链配置' }}
      />
    </div>
  )
}
