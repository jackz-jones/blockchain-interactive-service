import { useEffect, useState } from 'react'
import { Table, Button, Space, Typography, Tag, Input, Select, Popconfirm, message } from 'antd'
import { PlusOutlined, SearchOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'

const { Title } = Typography

interface ChainConfig {
  id: string
  chain_name: string
  chain_type: string
  enabled: boolean
  node_count: number
  created_at: string
}

export default function ChainConfigList() {
  const navigate = useNavigate()
  const [data, setData] = useState<ChainConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState<string | undefined>()

  const fetchList = async () => {
    setLoading(true)
    try {
      const params: Record<string, string> = {}
      if (search) params.search = search
      if (typeFilter) params.chain_type = typeFilter
      const res = await api.get('/chain-configs', { params })
      setData((res.data as { items: ChainConfig[] }).items || [])
    } catch {
      // 错误已由拦截器处理
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchList()
  }, [typeFilter])

  const handleDelete = async (id: string) => {
    try {
      await api.delete(`/chain-configs/${id}`)
      message.success('删除成功')
      fetchList()
    } catch {
      // 错误已由拦截器处理
    }
  }

  const columns: ColumnsType<ChainConfig> = [
    {
      title: '链名称',
      dataIndex: 'chain_name',
      key: 'chain_name',
      render: (text: string, record) => (
        <a onClick={() => navigate(`/chain-configs/${record.id}/edit`)}>{text}</a>
      ),
    },
    {
      title: '链类型',
      dataIndex: 'chain_type',
      key: 'chain_type',
      render: (type: string) => {
        const colorMap: Record<string, string> = {
          ethereum: 'blue',
          chainmaker: 'green',
          solana: 'purple',
        }
        return <Tag color={colorMap[type] || 'default'}>{type}</Tag>
      },
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'success' : 'default'}>
          {enabled ? '已启用' : '已禁用'}
        </Tag>
      ),
    },
    {
      title: '节点数量',
      dataIndex: 'node_count',
      key: 'node_count',
      width: 100,
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
            onClick={() => navigate(`/chain-configs/${record.id}/edit`)}
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
        <Title level={4} style={{ margin: 0 }}>链配置</Title>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => navigate('/chain-configs/create')}
        >
          新建链配置
        </Button>
      </div>

      <Space style={{ marginBottom: 16 }}>
        <Input
          placeholder="搜索链名称"
          prefix={<SearchOutlined />}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onPressEnter={fetchList}
          style={{ width: 220 }}
          allowClear
        />
        <Select
          placeholder="链类型"
          value={typeFilter}
          onChange={setTypeFilter}
          allowClear
          style={{ width: 140 }}
          options={[
            { label: 'Ethereum', value: 'ethereum' },
            { label: 'ChainMaker', value: 'chainmaker' },
            { label: 'Solana', value: 'solana' },
          ]}
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
