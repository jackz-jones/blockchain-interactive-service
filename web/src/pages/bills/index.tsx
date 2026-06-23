import { useEffect, useState } from 'react'
import { Table, Typography, Tag, Segmented, Empty, Button, Card, Row, Col, Skeleton, Space, Tooltip } from 'antd'
import { DollarOutlined, ReloadOutlined, InfoCircleOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { formatDateTime, formatNumber } from '@/utils/format'

const { Title, Text } = Typography

interface Bill {
  id: string
  bill_type: string
  period: string
  total_calls: number
  amount: number
  status: string
  created_at: string
}

interface RealtimeCost {
  plan: string
  month_calls: number
  monthly_limit: number
  monthly_used: number
  current_cost: number
  currency: string
  cost_breakdown: string
  usage_percent: number
  calculated_at: string
}

const billTypeOptions = [
  { label: '日账单', value: 'daily' },
  { label: '月度汇总', value: 'monthly' },
]

// 套餐中文名映射
const planLabelMap: Record<string, string> = {
  free: '免费版',
  developer: '开发者版',
  enterprise: '企业版',
}

// 套餐对应的 Tag 颜色
const planColorMap: Record<string, string> = {
  free: 'default',
  developer: 'blue',
  enterprise: 'purple',
}

// 格式化账期显示
function formatPeriod(period: string, billType: string): string {
  if (!period) return '-'
  if (billType === 'monthly') {
    const parts = period.split('-')
    if (parts.length >= 2) {
      return `${parts[0]}年${parts[1]}月`
    }
    return period
  }
  return period
}

export default function Bills() {
  const [data, setData] = useState<Bill[]>([])
  const [loading, setLoading] = useState(true)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [billType, setBillType] = useState<string>('daily')
  const [realtimeCost, setRealtimeCost] = useState<RealtimeCost | null>(null)
  const [costLoading, setCostLoading] = useState(false)
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

  const fetchRealtimeCost = async () => {
    setCostLoading(true)
    try {
      const res = await api.get('/dashboard/realtime-cost')
      setRealtimeCost(res.data as RealtimeCost)
    } catch (err) {
      handleApiError(err)
    } finally {
      setCostLoading(false)
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

  // 计费公式说明
  const getFormulaText = (plan: string): string => {
    switch (plan) {
      case 'free':
        return '计费公式：前 1,000 次 Invoke 免费，超出部分 ¥0.01/次'
      case 'developer':
        return '计费公式：月费 ¥99 含 50,000 次，超出部分 ¥0.005/次'
      case 'enterprise':
        return '计费公式：月费 ¥999，无限调用'
      default:
        return '计费公式：按量计费 ¥0.01/次'
    }
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
      render: (time: string) => (time ? formatDateTime(time) : '-'),
    },
  ]

  return (
    <div>
      <Title level={4} style={{ marginBottom: 20 }}>账单</Title>

      {/* 实时计费 */}
      <Card
        style={{ marginBottom: 20, borderRadius: 'var(--radius-lg)' }}
        title={
          <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <DollarOutlined style={{ color: 'var(--color-primary)' }} />
            实时计费
          </span>
        }
        extra={
          <Button
            type="primary"
            icon={<ReloadOutlined />}
            loading={costLoading}
            onClick={fetchRealtimeCost}
            size="small"
          >
            查看实时费用
          </Button>
        }
      >
        {costLoading && !realtimeCost ? (
          <Row gutter={[16, 16]}>
            {[1, 2, 3, 4].map((i) => (
              <Col xs={12} sm={12} md={6} key={i}>
                <Skeleton active paragraph={{ rows: 1 }} />
              </Col>
            ))}
          </Row>
        ) : realtimeCost ? (
          <>
            {/* 核心指标行：4 个等宽指标 */}
            <Row gutter={[16, 16]} style={{ marginBottom: 12 }}>
              <Col xs={12} sm={12} md={6}>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ color: 'var(--color-muted)', fontSize: 13, marginBottom: 4 }}>
                    当前套餐
                  </div>
                  <div>
                    <Tag
                      color={planColorMap[realtimeCost.plan] || 'default'}
                      style={{ fontSize: 14, padding: '2px 12px' }}
                    >
                      {planLabelMap[realtimeCost.plan] || realtimeCost.plan}
                    </Tag>
                  </div>
                </div>
              </Col>
              <Col xs={12} sm={12} md={6}>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ color: 'var(--color-muted)', fontSize: 13, marginBottom: 4 }}>
                    <span>计费调用量</span>
                    <Tooltip title="仅统计 Invoke 写链调用次数，用于计算费用">
                      <InfoCircleOutlined style={{ marginLeft: 4, fontSize: 11, color: 'var(--color-muted)' }} />
                    </Tooltip>
                  </div>
                  <div style={{ fontSize: 28, fontWeight: 600, color: 'var(--color-ink)' }}>
                    {formatNumber(realtimeCost.month_calls)}
                    <span style={{ fontSize: 13, fontWeight: 400, color: 'var(--color-muted)', marginLeft: 4 }}>次</span>
                  </div>
                </div>
              </Col>
              <Col xs={12} sm={12} md={6}>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ color: 'var(--color-muted)', fontSize: 13, marginBottom: 4 }}>
                    <span>配额用量</span>
                    <Tooltip title="统计所有调用（含读链 + 写链），用于判断配额是否用尽">
                      <InfoCircleOutlined style={{ marginLeft: 4, fontSize: 11, color: 'var(--color-muted)' }} />
                    </Tooltip>
                  </div>
                  <div style={{ fontSize: 28, fontWeight: 600, color: 'var(--color-ink)' }}>
                    {formatNumber(realtimeCost.monthly_used)}
                    <span style={{ fontSize: 13, fontWeight: 400, color: 'var(--color-muted)', marginLeft: 4 }}>
                      / {realtimeCost.monthly_limit === 0 ? '∞' : formatNumber(realtimeCost.monthly_limit)} 次
                    </span>
                  </div>
                  {realtimeCost.monthly_limit > 0 && (
                    <div style={{ fontSize: 12, color: realtimeCost.usage_percent > 80 ? 'var(--color-error)' : 'var(--color-muted)', marginTop: 2 }}>
                      {realtimeCost.usage_percent.toFixed(1)}% 已用
                    </div>
                  )}
                </div>
              </Col>
              <Col xs={12} sm={12} md={6}>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ color: 'var(--color-muted)', fontSize: 13, marginBottom: 4 }}>
                    当前费用
                  </div>
                  <div style={{ fontSize: 28, fontWeight: 600, color: 'var(--color-primary)' }}>
                    <span style={{ fontSize: 16, fontWeight: 500 }}>¥</span>
                    {realtimeCost.current_cost.toFixed(2)}
                  </div>
                </div>
              </Col>
            </Row>

            {/* 底部说明：套餐说明 + 计费公式合并为一行，计算时间右对齐 */}
            <div style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              paddingTop: 12,
              borderTop: '1px solid var(--color-surface-border)',
            }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {realtimeCost.cost_breakdown}　·　{getFormulaText(realtimeCost.plan)}
              </Text>
              <Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap', marginLeft: 16 }}>
                {realtimeCost.calculated_at}
              </Text>
            </div>
          </>
        ) : (
          <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--color-muted)' }}>
            点击「查看实时费用」按钮，获取当前实时计费信息
          </div>
        )}
      </Card>

      <div style={{ marginBottom: 16 }}>
        <Space>
          <Segmented
            options={billTypeOptions}
            value={billType}
            onChange={(val) => handleBillTypeChange(val as string)}
          />
        </Space>
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
