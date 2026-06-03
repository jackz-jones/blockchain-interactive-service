import { useEffect, useState } from 'react'
import { Row, Col, Card, Statistic, Skeleton, Typography } from 'antd'
import {
  ThunderboltOutlined,
  CalendarOutlined,
  LinkOutlined,
  CheckCircleOutlined,
  PercentageOutlined,
} from '@ant-design/icons'
import api from '@/services/api'

const { Title } = Typography

interface OverviewData {
  today_calls: number
  month_calls: number
  quota_usage_percent: number
  active_chains: number
  success_rate: number
}

export default function Dashboard() {
  const [data, setData] = useState<OverviewData | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchOverview = async () => {
      try {
        const res = await api.get('/dashboard/overview')
        setData(res.data as OverviewData)
      } catch {
        // 错误已由拦截器处理
      } finally {
        setLoading(false)
      }
    }
    fetchOverview()
  }, [])

  const statCards = [
    {
      title: '今日调用量',
      value: data?.today_calls ?? 0,
      icon: <ThunderboltOutlined />,
      color: '#4a7c59',
    },
    {
      title: '本月调用量',
      value: data?.month_calls ?? 0,
      icon: <CalendarOutlined />,
      color: '#2980b9',
    },
    {
      title: '配额使用率',
      value: data?.quota_usage_percent ?? 0,
      suffix: '%',
      icon: <PercentageOutlined />,
      color: (data?.quota_usage_percent ?? 0) > 80 ? '#c0392b' : '#b8860b',
    },
    {
      title: '活跃链数',
      value: data?.active_chains ?? 0,
      icon: <LinkOutlined />,
      color: '#6c5ce7',
    },
    {
      title: '成功率',
      value: data?.success_rate ?? 0,
      suffix: '%',
      precision: 1,
      icon: <CheckCircleOutlined />,
      color: '#3d8b5e',
    },
  ]

  return (
    <div>
      <Title level={4} style={{ marginBottom: 24 }}>
        概览
      </Title>

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
                  value={card.value}
                  suffix={card.suffix}
                  precision={card.precision}
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
