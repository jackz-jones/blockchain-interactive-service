import { useEffect, useState } from 'react'
import { Table, Typography, Space, Select, DatePicker, Tag, Input } from 'antd'
import { SearchOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'

const { Title } = Typography
const { RangePicker } = DatePicker

interface CallLog {
  id: string
  chain_name: string
  contract_name: string
  method: string
  method_type: number
  method_type_label: string
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
  const { handleApiError } = useApiMessage()

  const fetchList = async (p = page) => {
    setLoading(true)
    try {
      const params = { ...filters, page: String(p), page_size: '20' }
      const res = await api.get('/dashboard/call-logs', { params })
      const result = res.data as { items: CallLog[]; total: number }
      setData(result.items || [])
      setTotal(result.total || 0)
    } catch (err) {
      handleApiError(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchList()
  }, [page, filters])

  const columns: ColumnsType<CallLog> = [
    { title: '链名称', dataIndex: 'chain_name', key: 'chain_name', width: 160 },
    { title: '合约', dataIndex: 'contract_name', key: 'contract_name', width: 200 },
    { title: '方法', dataIndex: 'method', key: 'method', width: 200, ellipsis: { showTitle: false } },
    {
      title: '操作类型',
      dataIndex: 'method_type_label',
      key: 'method_type_label',
      width: 100,
      render: (label: string, record: CallLog) => {
        const isInvoke = record.method_type === 1
        return (
          <Tag color={isInvoke ? 'orange' : 'blue'}>
            {label || (isInvoke ? '写链(Invoke)' : '读链(Query)')}
          </Tag>
        )
      },
    },
    {
      title: '链类型',
      dataIndex: 'call_type',
      key: 'call_type',
      width: 100,
      render: (type: string) => <Tag>{type}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => (
        <Tag color={status === 'success' ? 'success' : 'error'}>{status}</Tag>
      ),
    },
    {
      title: '耗时',
      dataIndex: 'duration_ms',
      key: 'duration_ms',
      width: 100,
      render: (ms: number) => `${ms}ms`,
    },
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 200,
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
          placeholder="操作类型"
          allowClear
          style={{ width: 140 }}
          onChange={(val) => setFilters((f) => ({ ...f, method_type: val || '' }))}
          options={[
            { label: '写链(Invoke)', value: '1' },
            { label: '读链(Query)', value: '2' },
          ]}
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
      />
    </div>
  )
}
