import { useEffect, useState, useCallback } from 'react'
import { Table, Typography, Tag, Tooltip, Space, Input, Select, DatePicker } from 'antd'
import { SearchOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { formatDateTime } from '@/utils/format'
import ErrorRetry from '@/components/ErrorRetry'

const { Title, Text } = Typography
const { RangePicker } = DatePicker

interface AuditLog {
  ID: number
  operator: string
  action: string
  resource: string
  resource_label: string
  resource_id: string
  resource_name: string
  detail: string
  ip: string
  user_agent: string
  created_at: string
}

// 格式化 JSON 显示，对过长内容截断
function formatDetail(raw: unknown): string {
  try {
    const str = JSON.stringify(raw, null, 2)
    if (str.length > 2000) {
      return str.substring(0, 2000) + '\n... (内容过长已截断)'
    }
    return str
  } catch {
    return String(raw)
  }
}

// 解析 detail JSON，提取变更前后数据；无 before/after 时返回原始 detail
function parseDetail(detail: string): { before?: unknown; after?: unknown; raw?: unknown } | null {
  if (!detail) return null
  try {
    const parsed = JSON.parse(detail)
    if (parsed.before !== undefined && parsed.after !== undefined) {
      return { before: parsed.before, after: parsed.after }
    }
    return { raw: parsed }
  } catch {
    return null
  }
}

export default function AuditLogs() {
  const [data, setData] = useState<AuditLog[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [expandedKeys, setExpandedKeys] = useState<number[]>([])
  const [operatorSearch, setOperatorSearch] = useState('')
  const [actionFilter, setActionFilter] = useState<string | undefined>()
  const [resourceFilter, setResourceFilter] = useState<string | undefined>()
  const [dateRange, setDateRange] = useState<[string, string] | null>(null)
  const { handleApiError } = useApiMessage()

  const fetchList = async (p = page) => {
    setLoading(true)
    setError(null)
    try {
      const params: Record<string, string> = { page: String(p), page_size: '20' }
      if (operatorSearch) params.operator = operatorSearch
      if (actionFilter) params.action = actionFilter
      if (resourceFilter) params.resource = resourceFilter
      if (dateRange) {
        params.start_time = dateRange[0]
        params.end_time = dateRange[1]
      }
      const res = await api.get('/dashboard/audit-logs', { params })
      const result = res.data as { items: AuditLog[]; total: number }
      setData(result.items || [])
      setTotal(result.total || 0)
    } catch (err) {
      setError(err)
      handleApiError(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchList()
  }, [page, actionFilter, resourceFilter, dateRange])

  const actionColorMap: Record<string, string> = {
    create: 'green',
    update: 'blue',
    delete: 'red',
    call: 'orange',
  }

  const columns: ColumnsType<AuditLog> = [
    { title: '操作人', dataIndex: 'operator', key: 'operator', width: 160, align: 'center' },
    {
      title: '操作',
      dataIndex: 'action',
      key: 'action',
      width: 120,
      align: 'center',
      render: (action: string) => <Tag color={actionColorMap[action] || 'default'}>{action}</Tag>,
    },
    { title: '资源类型', dataIndex: 'resource_label', key: 'resource_label', width: 160, align: 'center' },
    {
      title: '资源',
      dataIndex: 'resource_name',
      key: 'resource_name',
      width: 240,
      align: 'center',
      ellipsis: { showTitle: false },
      render: (name: string, record) => {
        if (!name && !record.resource_id) {
          return <span style={{ color: '#aaa' }}>-</span>
        }
        if (name && name !== record.resource_id) {
          return (
            <Tooltip title={name} placement="topLeft">
              <span>{name}</span>
            </Tooltip>
          )
        }
        if (record.resource_id) {
          return (
            <Tooltip title={`ID: ${record.resource_id}`} placement="topLeft">
              <span style={{ color: '#888' }}>ID: {record.resource_id}</span>
            </Tooltip>
          )
        }
        return <span style={{ color: '#aaa' }}>-</span>
      },
    },
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 200,
      align: 'center',
      render: (time: string) => formatDateTime(time),
    },
  ]

  const expandedRowRender = useCallback((record: AuditLog) => {
    const changes = parseDetail(record.detail)
    if (!changes) {
      return <Text type="secondary">无详情</Text>
    }

    if (changes.before !== undefined && changes.after !== undefined) {
      return (
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
          <div style={{ textAlign: 'left' }}>
            <Text type="secondary" style={{ fontSize: 12, fontWeight: 500 }}>变更前</Text>
            <pre style={{
              fontSize: 12, maxHeight: 240, overflow: 'auto',
              background: '#fafafa', padding: 8, borderRadius: 4,
              margin: '4px 0 0', whiteSpace: 'pre-wrap', wordBreak: 'break-all',
              textAlign: 'left',
            }}>
              {formatDetail(changes.before)}
            </pre>
          </div>
          <div style={{ textAlign: 'left' }}>
            <Text type="secondary" style={{ fontSize: 12, fontWeight: 500 }}>变更后</Text>
            <pre style={{
              fontSize: 12, maxHeight: 240, overflow: 'auto',
              background: '#fafafa', padding: 8, borderRadius: 4,
              margin: '4px 0 0', whiteSpace: 'pre-wrap', wordBreak: 'break-all',
              textAlign: 'left',
            }}>
              {formatDetail(changes.after)}
            </pre>
          </div>
        </div>
      )
    }

    // 中间件自动记录的 HTTP 请求信息（含 method/path/status_code/duration 等字段）
    const raw = changes.raw as Record<string, unknown> | null
    if (raw && typeof raw === 'object' && ('method' in raw || 'status_code' in raw)) {
      const methodTagColor: Record<string, string> = {
        GET: 'green', POST: 'blue', PUT: 'orange', PATCH: 'orange', DELETE: 'red',
      }
      const method = (raw.method as string) || ''
      const path = (raw.path as string) || ''
      const statusCode = raw.status_code as number | undefined
      const duration = raw.duration as number | undefined

      // 提取除 method/path/status_code/duration 之外的额外字段，过滤掉空值字段
      const extraFields = Object.entries(raw).filter(
        ([k, v]) => !['method', 'path', 'status_code', 'duration'].includes(k)
          && v !== '' && v !== null && v !== undefined
      )

      return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {/* 请求路径 + 方法/状态码/耗时 - 左对齐一排展示 */}
          <div style={{
            display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'nowrap',
          }}>
            {path && (
              <>
                <Text type="secondary" style={{ fontSize: 12, fontWeight: 500, whiteSpace: 'nowrap' }}>请求路径</Text>
                <Text code style={{ fontSize: 13, whiteSpace: 'nowrap' }}>{path}</Text>
              </>
            )}
            {method && <Tag color={methodTagColor[method] || 'default'} style={{ margin: 0, flexShrink: 0 }}>{method}</Tag>}
            {statusCode !== undefined && (
              <Tag color={statusCode < 400 ? 'success' : 'error'} style={{ margin: 0, flexShrink: 0 }}>
                {statusCode}
              </Tag>
            )}
            {duration !== undefined && (
              <Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap' }}>耗时 {duration}ms</Text>
            )}
          </div>
          {/* 附加信息 - 仅展示非空额外字段，左对齐JSON结构 */}
          {extraFields.length > 0 && (
            <div style={{ textAlign: 'left' }}>
              <Text type="secondary" style={{ fontSize: 12, fontWeight: 500 }}>附加信息</Text>
              <pre style={{
                fontSize: 12, maxHeight: 200, overflow: 'auto',
                background: '#fafafa', padding: 8, borderRadius: 4,
                margin: '4px 0 0', whiteSpace: 'pre-wrap', wordBreak: 'break-all',
                textAlign: 'left',
              }}>
                {formatDetail(Object.fromEntries(extraFields))}
              </pre>
            </div>
          )}
        </div>
      )
    }

    // 其他未知格式的 detail，原始 JSON 展示
    return (
      <div>
        <Text type="secondary" style={{ fontSize: 12, fontWeight: 500 }}>操作详情</Text>
        <pre style={{
          fontSize: 12, maxHeight: 300, overflow: 'auto',
          background: '#fafafa', padding: 8, borderRadius: 4,
          margin: '4px 0 0', whiteSpace: 'pre-wrap', wordBreak: 'break-all',
        }}>
          {formatDetail(changes.raw)}
        </pre>
      </div>
    )
  }, [])

  // 加载失败时显示错误重试组件
  if (error && !loading && data.length === 0) {
    return (
      <div>
        <Title level={4} style={{ marginBottom: 20 }}>审计日志</Title>
        <ErrorRetry error={error} onRetry={() => fetchList()} />
      </div>
    )
  }

  return (
    <div>
      <Title level={4} style={{ marginBottom: 20 }}>审计日志</Title>

      <Space style={{ marginBottom: 16 }} wrap>
        <Input
          placeholder="搜索操作人"
          prefix={<SearchOutlined />}
          style={{ width: 160 }}
          allowClear
          value={operatorSearch}
          onChange={(e) => setOperatorSearch(e.target.value)}
          onPressEnter={() => fetchList()}
          onClear={() => { setOperatorSearch(''); fetchList() }}
        />
        <Select
          placeholder="操作类型"
          allowClear
          style={{ width: 140 }}
          value={actionFilter}
          onChange={(val) => setActionFilter(val || undefined)}
          options={[
            { label: '创建', value: 'create' },
            { label: '更新', value: 'update' },
            { label: '删除', value: 'delete' },
            { label: '调用', value: 'call' },
          ]}
        />
        <Select
          placeholder="资源类型"
          allowClear
          style={{ width: 160 }}
          value={resourceFilter}
          onChange={(val) => setResourceFilter(val || undefined)}
          options={[
            { label: '链配置', value: 'chain_config' },
            { label: '合约配置', value: 'contract_config' },
            { label: 'API Key', value: 'api_key' },
            { label: '租户', value: 'tenant' },
            { label: '用户', value: 'user' },
          ]}
        />
        <RangePicker
          value={dateRange ? [dayjs(dateRange[0]), dayjs(dateRange[1])] : null}
          onChange={(_, dateStrings) => {
            if (dateStrings[0] && dateStrings[1]) {
              setDateRange([dateStrings[0], dateStrings[1]])
            } else {
              setDateRange(null)
            }
          }}
        />
      </Space>

      <Table
        columns={columns}
        dataSource={data}
        rowKey="ID"
        loading={loading}
        pagination={{
          current: page,
          total,
          pageSize: 20,
          onChange: (p: number) => setPage(p),
          showTotal: (t: number) => `共 ${t} 条`,
        } as any}
        size="middle"
        expandable={{
          expandedRowRender,
          expandedRowKeys: expandedKeys,
          onExpand: (expanded: boolean, record: AuditLog) => {
            setExpandedKeys(
              expanded
                ? [...expandedKeys, record.ID]
                : expandedKeys.filter((k) => k !== record.ID)
            )
          },
        } as any}
      />
    </div>
  )
}
