import { useEffect, useState } from 'react'
import { Table, Button, Typography, Tag, Space, Popconfirm, Input, Select } from 'antd'
import { PlusOutlined, SearchOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { useGlobalMessage } from '@/components/GlobalMessage'

const { Title } = Typography

interface ChainConfig {
  ID: number
  chain_name: string
  chain_type: string
  enable: boolean
  CreatedAt: string
}

export default function ChainConfigList() {
  const navigate = useNavigate()
  const { message } = useGlobalMessage()
  const { handleApiError } = useApiMessage()
  const [data, setData] = useState<ChainConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState<string | undefined>()

  const fetchList = async (searchOverride?: string) => {
    setLoading(true)
    try {
      const params: Record<string, string> = {}
      const searchValue = searchOverride !== undefined ? searchOverride : search
      if (searchValue) params.search = searchValue
      if (typeFilter) params.chain_type = typeFilter
      const res = await api.get('/chain-configs', { params })
      setData((res.data as ChainConfig[]) || [])
    } catch (err) {
      handleApiError(err)
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
    } catch (err) {
      handleApiError(err)
    }
  }

  const columns: ColumnsType<ChainConfig> = [
    {
      title: '链名称',
      dataIndex: 'chain_name',
      key: 'chain_name',
      width: 240,
      ellipsis: { showTitle: false },
      render: (text: string, record) => (
        <a onClick={() => navigate(`/chain-configs/${record.ID}/edit`)}>{text}</a>
      ),
    },
    {
      title: '链类型',
      dataIndex: 'chain_type',
      key: 'chain_type',
      width: 140,
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
      dataIndex: 'enable',
      key: 'enable',
      width: 120,
      render: (enable: boolean) => (
        <Tag color={enable ? 'success' : 'default'}>
          {enable ? '已启用' : '已禁用'}
        </Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'CreatedAt',
      key: 'CreatedAt',
      width: 180,
      render: (time: string) => time ? new Date(time).toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).replace(/\//g, '-') : '-',
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
            onClick={() => navigate(`/chain-configs/${record.ID}/edit`)}
          />
          <Popconfirm
            title="确认删除"
            description="删除后不可恢复，确定要删除吗？"
            onConfirm={() => handleDelete(String(record.ID))}
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
          onClear={() => { setSearch(''); fetchList('') }}
          style={{ width: 220 }}
          allowClear
        />
        <Button type="primary" icon={<SearchOutlined />} onClick={fetchList}>
          搜索
        </Button>
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
        rowKey="ID"
        loading={loading}
        pagination={{ pageSize: 10, showSizeChanger: true, showTotal: (total) => `共 ${total} 条` }}
        size="middle"
      />
    </div>
  )
}
