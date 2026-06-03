import { useEffect, useState } from 'react'
import { Table, Typography, Tag, Space } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'

const { Title, Text, Paragraph } = Typography

interface AuditLog {
  id: string
  operator: string
  action: string
  resource_type: string
  resource_id: string
  changes?: { before: unknown; after: unknown }
  created_at: string
}

export default function AuditLogs() {
  const [data, setData] = useState<AuditLog[]>([])
  const [loading, setLoading] = useState(true)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)

  const fetchList = async (p = page) => {
    setLoading(true)
    try {
      const res = await api.get('/dashboard/audit-logs', { params: { page: p, page_size: 20 } })
      const result = res.data as { items: AuditLog[]; total: number }
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
  }, [page])

  const actionColorMap: Record<string, string> = {
    create: 'green',
    update: 'blue',
    delete: 'red',
  }

  const columns: ColumnsType<AuditLog> = [
    { title: '操作人', dataIndex: 'operator', key: 'operator', width: 120 },
    {
      title: '操作',
      dataIndex: 'action',
      key: 'action',
      width: 80,
      render: (action: string) => <Tag color={actionColorMap[action] || 'default'}>{action}</Tag>,
    },
    { title: '资源类型', dataIndex: 'resource_type', key: 'resource_type', width: 120 },
    {
      title: '资源 ID',
      dataIndex: 'resource_id',
      key: 'resource_id',
      render: (id: string) => <Text code style={{ fontSize: 12 }}>{id}</Text>,
    },
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (time: string) => new Date(time).toLocaleString('zh-CN'),
    },
  ]

  return (
    <div>
      <Title level={4} style={{ marginBottom: 20 }}>审计日志</Title>

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
        expandable={{
          expandedRowRender: (record) =>
            record.changes ? (
              <Space direction="vertical" style={{ width: '100%' }}>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
                  <div>
                    <Text type="secondary" style={{ fontSize: 12 }}>变更前</Text>
                    <Paragraph code style={{ fontSize: 11, maxHeight: 200, overflow: 'auto' }}>
                      {JSON.stringify(record.changes.before, null, 2)}
                    </Paragraph>
                  </div>
                  <div>
                    <Text type="secondary" style={{ fontSize: 12 }}>变更后</Text>
                    <Paragraph code style={{ fontSize: 11, maxHeight: 200, overflow: 'auto' }}>
                      {JSON.stringify(record.changes.after, null, 2)}
                    </Paragraph>
                  </div>
                </div>
              </Space>
            ) : (
              <Text type="secondary">无变更详情</Text>
            ),
          rowExpandable: (record) => !!record.changes,
        }}
      />
    </div>
  )
}
