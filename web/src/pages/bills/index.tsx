import { useEffect, useState } from 'react'
import { Table, Typography, Tag } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'

const { Title } = Typography

interface Bill {
  id: string
  period: string
  total_calls: number
  amount: number
  status: string
  created_at: string
}

export default function Bills() {
  const [data, setData] = useState<Bill[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchList = async () => {
      try {
        const res = await api.get('/dashboard/bills')
        setData((res.data as { items: Bill[] }).items || [])
      } catch {
        // 错误已由拦截器处理
      } finally {
        setLoading(false)
      }
    }
    fetchList()
  }, [])

  const columns: ColumnsType<Bill> = [
    { title: '账期', dataIndex: 'period', key: 'period' },
    { title: '总调用量', dataIndex: 'total_calls', key: 'total_calls' },
    {
      title: '金额',
      dataIndex: 'amount',
      key: 'amount',
      render: (amount: number) => `¥${(amount / 100).toFixed(2)}`,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const colorMap: Record<string, string> = { paid: 'success', pending: 'warning', overdue: 'error' }
        const labelMap: Record<string, string> = { paid: '已支付', pending: '待支付', overdue: '逾期' }
        return <Tag color={colorMap[status] || 'default'}>{labelMap[status] || status}</Tag>
      },
    },
    {
      title: '生成时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (time: string) => new Date(time).toLocaleDateString('zh-CN'),
    },
  ]

  return (
    <div>
      <Title level={4} style={{ marginBottom: 20 }}>账单</Title>
      <Table columns={columns} dataSource={data} rowKey="id" loading={loading} pagination={{ pageSize: 10 }} size="middle" />
    </div>
  )
}
