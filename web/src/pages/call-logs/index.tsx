import { useEffect, useState } from 'react'
import { Table, Typography, Space, Select, DatePicker, Tag, Input } from 'antd'
import { SearchOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'

const { Title } = Typography
const { RangePicker } = DatePicker

interface CallLog {
  id: string
  chain_name: string
  contract_name: string
  method: string
  call_type: string
  status: string
  duration_ms: number
  created_at: string
}

export default function CallLogs() {
  const [data, setData] = useState<CallLog[]>([])
  const [loading, setLoading] = useState(true)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState<Record<string, string>>({})

  const fetchList = async (p = page) => {
    setLoading(true)
    try {
      const params = { ...filters, page: String(p), page_size: '20' }
      const res = await api.get('/dashboard/call-logs', { params })
      const result = res.data as { items: CallLog[]; total: number }
      setData(result.items || [])
      setTotal(result.total || 0)
    } catch {
      // 错误已由拦截器处理
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchList()
  }, [page, filters])

  const columns: ColumnsType<CallLog> = [
    { title: '链名称', dataIndex: 'chain_name', key: 'chain_name', width: 120 },
    { title: '合约', dataIndex: 'contract_name', key: 'contract_name', width: 120 },
    { title: '方法', dataIndex: 'method', key: 'method', width: 120 },
    {
      title: '类型',
      dataIndex: 'call_type',
      key: 'call_type',
      width: 80,
      render: (type: string) => <Tag>{type}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 80,
      render: (status: string) => (
        <Tag color={status === 'success' ? 'success' : 'error'}>{status}</Tag>
      ),
    },
    {
      title: '耗时',
      dataIndex: 'duration_ms',
      key: 'duration_ms',
      width: 80,
      render: (ms: number) => `${ms}ms`,
    },
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (time: string) => new Date(time).toLocaleString('zh-CN'),
    },
  ]

  return (
    <div>
      <Title level={4} style={{ marginBottom: 20 }}>调用日志</Title>

      <Space style={{ marginBottom: 16 }} wrap>
        <Input
          placeholder="链名称"
          prefix={<SearchOutlined />}
          style={{ width: 160 }}
          allowClear
          onChange={(e) => setFilters((f) => ({ ...f, chain_name: e.target.value }))}
        />
        <Select
          placeholder="状态"
          allowClear
          style={{ width: 120 }}
          onChange={(val) => setFilters((f) => ({ ...f, status: val || '' }))}
          options={[
            { label: '成功', value: 'success' },
            { label: '失败', value: 'failed' },
          ]}
        />
        <RangePicker
          onChange={(_, dateStrings) => {
            setFilters((f) => ({ ...f, start_time: dateStrings[0] || '', end_time: dateStrings[1] || '' }))
          }}
        />
      </Space>

      <Table
        columns={columns}
        dataSource={data}
        rowKey="id"
        loading={loading}
        pagination={{
          current: page,
          total,
          pageSize: 20,
          onChange: setPage,
          showTotal: (t) => `共 ${t} 条`,
        }}
        size="middle"
        scroll={{ x: 800 }}
      />
    </div>
  )
}
