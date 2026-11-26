import { useEffect, useState } from 'react'
import { Card, Typography, Select, Alert, Table, Empty } from 'antd'
import ReactECharts from 'echarts-for-react'
import type { ColumnsType } from 'antd/es/table'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { formatNumber } from '@/utils/format'
import ErrorRetry from '@/components/ErrorRetry'

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
  // 图表下方数据表格所需
  daily_details: DailyDetail[]
}

interface DailyDetail {
  date: string
  total_calls: number
  invoke_calls: number
  query_calls: number
  success_count: number
  failed_count: number
}

export default function UsageStats() {
  const [data, setData] = useState<UsageData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const [period, setPeriod] = useState('30d')
  const { handleApiError } = useApiMessage()

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true)
      setError(null)
      try {
        const res = await api.get('/dashboard/usage-stats-trend', { params: { period } })
        setData(res.data as UsageData)
      } catch (err) {
        setError(err)
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
    dataZoom: [
      { type: 'inside' as const, xAxisIndex: 0 },
    ],
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

  // 数据表格列定义
  const detailColumns: ColumnsType<DailyDetail> = [
    { title: '日期', dataIndex: 'date', key: 'date', width: 120, align: 'center' as const },
    { title: '总调用', dataIndex: 'total_calls', key: 'total_calls', width: 100, align: 'center' as const, render: (v: number) => formatNumber(v) },
    { title: '写链(Invoke)', dataIndex: 'invoke_calls', key: 'invoke_calls', width: 120, align: 'center' as const, render: (v: number) => formatNumber(v) },
    { title: '读链(Query)', dataIndex: 'query_calls', key: 'query_calls', width: 120, align: 'center' as const, render: (v: number) => formatNumber(v) },
    { title: '成功', dataIndex: 'success_count', key: 'success_count', width: 80, align: 'center' as const, render: (v: number) => formatNumber(v) },
    { title: '失败', dataIndex: 'failed_count', key: 'failed_count', width: 80, align: 'center' as const, render: (v: number) => formatNumber(v) },
  ]

  // 加载失败时显示错误重试组件
  if (error && !loading && !data) {
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
        <ErrorRetry error={error} onRetry={() => { setPeriod(period) /* 触发 useEffect 重新请求 */ }} />
      </div>
    )
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
        {data ? (
          <ReactECharts option={chartOption} style={{ height: 360 }} opts={{ renderer: 'svg' }} />
        ) : (
          <Empty description="暂无用量数据" image={Empty.PRESENTED_IMAGE_SIMPLE} style={{ padding: '80px 0' }} />
        )}
      </Card>

      {/* 图表下方数据表格 */}
      {data?.daily_details && data.daily_details.length > 0 && (
        <Card title="每日明细" style={{ marginTop: 16 }}>
          <Table
            columns={detailColumns}
            dataSource={data.daily_details}
            rowKey="date"
            size="small"
            pagination={{ pageSize: 10, showTotal: (t) => `共 ${t} 天` }}
            scroll={{ x: 600 }}
          />
        </Card>
      )}
    </div>
  )
}
