import { useEffect, useState } from 'react'
import { Card, Typography, Select, Alert } from 'antd'
import ReactECharts from 'echarts-for-react'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'

const { Title } = Typography

interface UsageData {
  dates: string[]
  calls: number[]
  success: number[]
  failed: number[]
  invoke: number[]
  query: number[]
  quota_limit: number
  quota_used: number
}

export default function UsageStats() {
  const [data, setData] = useState<UsageData | null>(null)
  const [loading, setLoading] = useState(true)
  const [period, setPeriod] = useState('30d')
  const { handleApiError } = useApiMessage()

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true)
      try {
        const res = await api.get('/dashboard/usage-stats-trend', { params: { period } })
        setData(res.data as UsageData)
      } catch (err) {
        handleApiError(err)
      } finally {
        setLoading(false)
      }
    }
    fetchData()
  }, [period])

  const quotaPercent = data?.quota_limit ? Math.round(((data.quota_used || 0) / data.quota_limit) * 100) : 0

  const chartOption = {
    tooltip: { trigger: 'axis' as const },
    legend: { data: ['总调用', 'Invoke(写链)', 'Query(读链)', '成功', '失败'], bottom: 0 },
    grid: { left: 50, right: 20, top: 20, bottom: 40 },
    xAxis: {
      type: 'category' as const,
      data: data?.dates || [],
      axisLabel: { fontSize: 11 },
    },
    yAxis: { type: 'value' as const },
    series: [
      {
        name: '总调用',
        type: 'line',
        data: data?.calls || [],
        smooth: true,
        lineStyle: { width: 2, color: '#2f74c0', type: 'dashed' },
        itemStyle: { color: '#2f74c0' },
      },
      {
        name: 'Invoke(写链)',
        type: 'line',
        data: data?.invoke || [],
        smooth: true,
        lineStyle: { width: 2, color: '#e67e22' },
        itemStyle: { color: '#e67e22' },
        areaStyle: { color: 'rgba(230,126,34,0.08)' },
      },
      {
        name: 'Query(读链)',
        type: 'line',
        data: data?.query || [],
        smooth: true,
        lineStyle: { width: 2, color: '#8e44ad' },
        itemStyle: { color: '#8e44ad' },
        areaStyle: { color: 'rgba(142,68,173,0.08)' },
      },
      {
        name: '成功',
        type: 'line',
        data: data?.success || [],
        smooth: true,
        lineStyle: { width: 1.5, color: '#3d8b5e' },
        itemStyle: { color: '#3d8b5e' },
      },
      {
        name: '失败',
        type: 'line',
        data: data?.failed || [],
        smooth: true,
        lineStyle: { width: 1.5, color: '#c0392b' },
        itemStyle: { color: '#c0392b' },
      },
    ],
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <Title level={4} style={{ margin: 0 }}>用量统计</Title>
        <Select
          value={period}
          onChange={setPeriod}
          style={{ width: 120 }}
          options={[
            { label: '近 7 天', value: '7d' },
            { label: '近 30 天', value: '30d' },
            { label: '近 90 天', value: '90d' },
          ]}
        />
      </div>

      {quotaPercent > 80 && (
        <Alert
          type="warning"
          title={`配额使用率已达 ${quotaPercent}%`}
          description="当前用量接近配额上限，请及时扩容或联系管理员。"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      <Card loading={loading}>
        <ReactECharts option={chartOption} style={{ height: 360 }} />
      </Card>
    </div>
  )
}
