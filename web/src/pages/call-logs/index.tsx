import { useEffect, useState, useCallback, useRef } from 'react'
import { Table, Typography, Space, Select, DatePicker, Tag, Input, Button } from 'antd'
import { SearchOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { formatDateTime } from '@/utils/format'
import ErrorRetry from '@/components/ErrorRetry'

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
  input_params: string
  output_data: string
  error_message: string
  created_at: string
}

export default function CallLogs() {
  const [data, setData] = useState<CallLog[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState<Record<string, string>>({})
  const [contractSearch, setContractSearch] = useState('')
  const [expandedRowId, setExpandedRowId] = useState<string | null>(null)
  const { handleApiError } = useApiMessage()

  // 搜索防抖
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const debouncedFilterUpdate = useCallback((key: string, value: string) => {
    if (debounceTimerRef.current) clearTimeout(debounceTimerRef.current)
    debounceTimerRef.current = setTimeout(() => {
      setFilters((f) => ({ ...f, [key]: value }))
    }, 300)
  }, [])

  // 当 filters 变化时触发列表刷新
  useEffect(() => {
    fetchList()
  }, [page, filters])

  const fetchList = async (p = page) => {
    setLoading(true)
    setError(null)
    try {
      const params: Record<string, string> = { page: String(p), page_size: '20' }
      if (contractSearch) params.contract_name = contractSearch
      const res = await api.get('/dashboard/call-logs', { params })
      const result = res.data as { items: CallLog[]; total: number }
      setData(result.items || [])
      setTotal(result.total || 0)
    } catch (err) {
      setError(err)
      handleApiError(err)
    } finally {
      setLoading(false)
    }
  }

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
      render: (time: string) => formatDateTime(time),
    },
  ]

  // 展开行渲染：显示调用详情
  const expandedRowRender = useCallback((record: CallLog) => {
    return (
      <div style={{ padding: '8px 16px', fontSize: 13 }}>
        <div style={{ marginBottom: 8 }}>
          <strong>输入参数：</strong>
          <pre style={{ margin: 0, padding: 8, background: 'var(--color-bg-secondary)', borderRadius: 4, overflow: 'auto', maxHeight: 200 }}>
            {record.input_params || '-'}
          </pre>
        </div>
        <div style={{ marginBottom: 8 }}>
          <strong>输出数据：</strong>
          <pre style={{ margin: 0, padding: 8, background: 'var(--color-bg-secondary)', borderRadius: 4, overflow: 'auto', maxHeight: 200 }}>
            {record.output_data || '-'}
          </pre>
        </div>
        {record.error_message && (
          <div>
            <strong>错误信息：</strong>
            <pre style={{ margin: 0, padding: 8, background: '#fff2f0', borderRadius: 4, color: '#cf1322', overflow: 'auto', maxHeight: 200 }}>
              {record.error_message}
            </pre>
          </div>
        )}
      </div>
    )
  }, [])

  // 加载失败时显示错误重试组件
  if (error && !loading && data.length === 0) {
    return (
      <div>
        <Title level={4} style={{ marginBottom: 20 }}>调用日志</Title>
        <ErrorRetry error={error} onRetry={() => fetchList()} />
      </div>
    )
  }

  return (
    <div>
      <Title level={4} style={{ marginBottom: 20 }}>调用日志</Title>

      <Space style={{ marginBottom: 16 }} wrap>
        <Input
          placeholder="链名称"
          prefix={<SearchOutlined />}
          style={{ width: 160 }}
          allowClear
          onChange={(e) => debouncedFilterUpdate('chain_name', e.target.value)}
        />
        <Input
          placeholder="合约名称"
          style={{ width: 160 }}
          allowClear
          value={contractSearch}
          onChange={(e) => setContractSearch(e.target.value)}
          onPressEnter={() => fetchList()}
          onClear={() => { setContractSearch(''); fetchList() }}
        />
        <Button type="primary" icon={<SearchOutlined />} onClick={() => fetchList()}>
          搜索
        </Button>
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
        expandable={{
          expandedRowRender,
          expandedRowKeys: expandedRowId ? [expandedRowId] : [],
          onExpandedRowsChange: (keys) => setExpandedRowId(keys.length > 0 ? String(keys[0]) : null),
        }}
        size="middle"
      />
    </div>
  )
}
