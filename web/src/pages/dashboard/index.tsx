import { useEffect, useState } from 'react'
import { Row, Col, Card, Statistic, Skeleton, Typography, Button } from 'antd'
import {
  ThunderboltOutlined,
  CalendarOutlined,
  LinkOutlined,
  CheckCircleOutlined,
  PercentageOutlined,
  ReloadOutlined,
} from '@ant-design/icons'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import ErrorRetry from '@/components/ErrorRetry'

const { Title } = Typography

interface OverviewData {
  today_calls: number
  month_calls: number
  usage_percent: number
  active_chains: number
  success_rate: number
}

export default function Dashboard() {
  const [data, setData] = useState<OverviewData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const { handleApiError } = useApiMessage()

  const fetchOverview = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.get('/dashboard/overview')
      setData(res.data as OverviewData)
    } catch (err) {
      setError(err)
      handleApiError(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchOverview()
  }, [])

  const statCards = [
    {
      title: '今日调用量',
      value: data?.today_calls ?? null,
      icon: <ThunderboltOutlined />,
      color: '#4a7c59',
    },
    {
      title: '本月调用量',
      value: data?.month_calls ?? null,
      icon: <CalendarOutlined />,
      color: '#2980b9',
    },
    {
      title: '配额使用率',
      value: data?.usage_percent ?? null,
      suffix: '%',
      icon: <PercentageOutlined />,
      color: (data?.usage_percent ?? 0) > 80 ? '#c0392b' : '#b8860b',
    },
    {
      title: '活跃链数',
      value: data?.active_chains ?? null,
      icon: <LinkOutlined />,
      color: '#6c5ce7',
    },
    {
      title: '成功率',
      value: data?.success_rate ?? null,
      suffix: '%',
      precision: 1,
      icon: <CheckCircleOutlined />,
      color: '#3d8b5e',
    },
  ]

  // 加载失败时显示错误重试组件
  if (error && !loading && !data) {
    return (
      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
          <Title level={4} style={{ margin: 0 }}>概览</Title>
        </div>
        <ErrorRetry error={error} onRetry={() => fetchOverview()} />
      </div>
    )
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <Title level={4} style={{ margin: 0 }}>概览</Title>
        <Button
          type="text"
          icon={<ReloadOutlined />}
          onClick={() => fetchOverview()}
          loading={loading}
          size="small"
        >
          刷新
        </Button>
      </div>

      <Row gutter={[16, 16]}>
        {statCards.map((card) => (
          <Col xs={24} sm={12} lg={8} xl={6} xxl={4} key={card.title}>
            {loading ? (
              <Card>
                <Skeleton active paragraph={{ rows: 1 }} />
              </Card>
            ) : (
              <Card hoverable style={{ borderRadius: 'var(--radius-lg)' }}>
                <Statistic
                  title={
                    <span style={{ color: 'var(--color-muted)', fontSize: 13 }}>
                      {card.title}
                    </span>
                  }
                  value={card.value !== null ? card.value : '-'}
                  suffix={card.value !== null ? card.suffix : undefined}
                  precision={card.value !== null ? card.precision : undefined}
                  prefix={
                    <span style={{ color: card.color, marginRight: 4 }}>
                      {card.icon}
                    </span>
                  }
                  valueStyle={{ fontSize: 28, fontWeight: 600 }}
                />
              </Card>
            )}
          </Col>
        ))}
      </Row>
    </div>
  )
}
