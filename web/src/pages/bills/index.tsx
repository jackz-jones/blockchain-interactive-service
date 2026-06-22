import { useEffect, useState } from 'react'
import { Table, Typography, Tag, Segmented, Empty } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'

const { Title } = Typography

interface Bill {
  id: string
  bill_type: string
  period: string
  total_calls: number
  amount: number
  status: string
  created_at: string
}

const billTypeOptions = [
  { label: '日账单', value: 'daily' },
  { label: '月度汇总', value: 'monthly' },
]

// 格式化数字为千位分隔符
function formatNumber(num: number): string {
  return num.toLocaleString('zh-CN')
}

// 格式化账期显示
function formatPeriod(period: string, billType: string): string {
  if (!period) return '-'
  if (billType === 'monthly') {
    // 月度汇总：2026-06 → 2026年06月
    const parts = period.split('-')
    if (parts.length >= 2) {
      return `${parts[0]}年${parts[1]}月`
    }
    return period
  }
  // 日账单：2026-06-10 → 2026-06-10
  return period
}

export default function Bills() {
  const [data, setData] = useState<Bill[]>([])
  const [loading, setLoading] = useState(true)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [billType, setBillType] = useState<string>('daily')
  const { handleApiError } = useApiMessage()

  const fetchList = async (p = page, bt = billType) => {
    setLoading(true)
    try {
      const res = await api.get('/dashboard/bills', {
        params: { page: p, page_size: 10, bill_type: bt },
      })
      const result = res.data as { items: Bill[]; total: number }
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
  }, [page, billType])

  const handleBillTypeChange = (value: string) => {
    setBillType(value)
    setPage(1)
  }

  const statusConfig: Record<string, { color: string; label: string }> = {
    paid: { color: 'green', label: '已支付' },
    unpaid: { color: 'orange', label: '待支付' },
    overdue: { color: 'red', label: '逾期' },
  }

  const columns: ColumnsType<Bill> = [
    {
      title: '账期',
      dataIndex: 'period',
      key: 'period',
      width: 180,
      align: 'center',
      render: (period: string, record: Bill) => formatPeriod(period, record.bill_type),
    },
    {
      title: '类型',
      dataIndex: 'bill_type',
      key: 'bill_type',
      width: 120,
      align: 'center',
      render: (type: string) => (
        <Tag color={type === 'daily' ? 'blue' : 'purple'}>
          {type === 'daily' ? '日账单' : '月度汇总'}
        </Tag>
      ),
    },
    {
      title: '总调用量',
      dataIndex: 'total_calls',
      key: 'total_calls',
      width: 140,
      align: 'center',
      render: (calls: number) => formatNumber(calls),
    },
    {
      title: '金额',
      dataIndex: 'amount',
      key: 'amount',
      width: 140,
      align: 'center',
      render: (amount: number) => `¥${amount.toFixed(2)}`,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 120,
      align: 'center',
      render: (status: string) => {
        const config = statusConfig[status]
        return <Tag color={config?.color || 'default'}>{config?.label || status}</Tag>
      },
    },
    {
      title: '生成时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 200,
      align: 'center',
      render: (time: string) => (time ? new Date(time).toLocaleString('zh-CN') : '-'),
    },
  ]

  return (
    <div>
      <Title level={4} style={{ marginBottom: 20 }}>账单</Title>

      <div style={{ marginBottom: 16 }}>
        <Segmented
          options={billTypeOptions}
          value={billType}
          onChange={(val) => handleBillTypeChange(val as string)}
        />
      </div>

      <Table
        columns={columns}
        dataSource={data}
        rowKey="id"
        loading={loading}
        pagination={{
          current: page,
          total,
          pageSize: 10,
          onChange: setPage,
          showTotal: (t) => `共 ${t} 条`,
        }}
        size="middle"
        locale={{
          emptyText: (
            <Empty
              description="暂无账单记录，账单将在每日零点自动生成"
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
          ),
        }}
      />
    </div>
  )
}
