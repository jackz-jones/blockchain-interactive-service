import { useEffect, useState } from 'react'
import { Table, Button, Space, Typography, Tag, Select, Popconfirm, message } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'

const { Title } = Typography

interface ContractConfig {
  id: string
  contract_name: string
  contract_address: string
  chain_config_id: string
  chain_name: string
  subscribe_enabled: boolean
  created_at: string
}

export default function ContractConfigList() {
  const navigate = useNavigate()
  const [data, setData] = useState<ContractConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [chainFilter, setChainFilter] = useState<string | undefined>()

  const fetchList = async () => {
    setLoading(true)
    try {
      const params: Record<string, string> = {}
      if (chainFilter) params.chain_config_id = chainFilter
      const res = await api.get('/contract-configs', { params })
      setData((res.data as { items: ContractConfig[] }).items || [])
    } catch {
      // 错误已由拦截器处理
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchList()
  }, [chainFilter])

  const handleDelete = async (id: string) => {
    try {
      await api.delete(`/contract-configs/${id}`)
      message.success('删除成功')
      fetchList()
    } catch {
      // 错误已由拦截器处理
    }
  }

  const columns: ColumnsType<ContractConfig> = [
    {
      title: '合约名称',
      dataIndex: 'contract_name',
      key: 'contract_name',
      render: (text: string, record) => (
        <a onClick={() => navigate(`/contract-configs/${record.id}/edit`)}>{text}</a>
      ),
    },
    {
      title: '合约地址',
      dataIndex: 'contract_address',
      key: 'contract_address',
      render: (addr: string) => (
        <Typography.Text code style={{ fontSize: 12 }}>
          {addr?.length > 20 ? `${addr.slice(0, 10)}...${addr.slice(-8)}` : addr}
        </Typography.Text>
      ),
    },
    {
      title: '所属链',
      dataIndex: 'chain_name',
      key: 'chain_name',
    },
    {
      title: '事件订阅',
      dataIndex: 'subscribe_enabled',
      key: 'subscribe_enabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'processing' : 'default'}>
          {enabled ? '已开启' : '未开启'}
        </Tag>
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
      width: 150,
      render: (_, record) => (
        <Space size="small">
          <Button
            type="text"
            size="small"
            icon={<EditOutlined />}
            onClick={() => navigate(`/contract-configs/${record.id}/edit`)}
          />
          <Popconfirm
            title="确认删除"
            description="删除后不可恢复，确定要删除吗？"
            onConfirm={() => handleDelete(record.id)}
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
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => navigate('/contract-configs/create')}
        >
          新建合约配置
        </Button>
      </div>

      <Space style={{ marginBottom: 16 }}>
        <Select
          placeholder="筛选链"
          value={chainFilter}
          onChange={setChainFilter}
          allowClear
          style={{ width: 200 }}
        />
      </Space>

      <Table
        columns={columns}
        dataSource={data}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 10, showSizeChanger: true, showTotal: (total) => `共 ${total} 条` }}
        size="middle"
      />
    </div>
  )
}
