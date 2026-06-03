import { useState, useEffect } from 'react'
import { Card, Form, Select, Input, Button, Typography, Space, Alert } from 'antd'
import { SearchOutlined } from '@ant-design/icons'
import Editor from '@monaco-editor/react'
import api from '@/services/api'

const { Title, Text } = Typography

interface ChainOption {
  id: string
  chain_name: string
}

interface TxResult {
  tx_id: string
  status: string
  block_height?: number
  timestamp?: string
  content?: unknown
  error?: string
}

export default function TxQuery() {
  const [form] = Form.useForm()
  const [chains, setChains] = useState<ChainOption[]>([])
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<TxResult | null>(null)

  useEffect(() => {
    fetchChains()
  }, [])

  const fetchChains = async () => {
    try {
      const res = await api.get('/chain-configs')
      setChains((res.data as { items: ChainOption[] }).items || [])
    } catch {
      // 错误已由拦截器处理
    }
  }

  const handleQuery = async (values: Record<string, string>) => {
    setLoading(true)
    setResult(null)
    try {
      const res = await api.get('/chain/tx', {
        params: { chain_name: values.chain_name, tx_id: values.tx_id },
      })
      setResult(res.data as TxResult)
    } catch (err) {
      setResult({ tx_id: values.tx_id || '', status: 'ERROR', error: err instanceof Error ? err.message : '查询失败' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ maxWidth: 800 }}>
      <Title level={4} style={{ marginBottom: 24 }}>交易查询</Title>

      <Card style={{ marginBottom: 20 }}>
        <Form form={form} layout="inline" onFinish={handleQuery} style={{ gap: 12, flexWrap: 'wrap' }}>
          <Form.Item name="chain_name" rules={[{ required: true, message: '请选择链' }]}>
            <Select
              placeholder="选择链"
              style={{ width: 200 }}
              options={chains.map((c) => ({ label: c.chain_name, value: c.chain_name }))}
            />
          </Form.Item>
          <Form.Item name="tx_id" rules={[{ required: true, message: '请输入交易 ID' }]} style={{ flex: 1 }}>
            <Input placeholder="输入交易 ID / 交易哈希" style={{ fontFamily: 'monospace' }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} icon={<SearchOutlined />}>
              查询
            </Button>
          </Form.Item>
        </Form>
      </Card>

      {result?.error && (
        <Alert type="error" message="查询失败" description={result.error} showIcon style={{ marginBottom: 16 }} />
      )}

      {result && !result.error && (
        <Card title="交易详情">
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>交易 ID</Text>
                <div><Text code style={{ fontSize: 12, wordBreak: 'break-all' }}>{result.tx_id}</Text></div>
              </div>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>状态</Text>
                <div><Text>{result.status}</Text></div>
              </div>
              {result.block_height !== undefined && (
                <div>
                  <Text type="secondary" style={{ fontSize: 12 }}>区块高度</Text>
                  <div><Text>{String(result.block_height)}</Text></div>
                </div>
              )}
              {result.timestamp && (
                <div>
                  <Text type="secondary" style={{ fontSize: 12 }}>时间</Text>
                  <div><Text>{new Date(result.timestamp).toLocaleString('zh-CN')}</Text></div>
                </div>
              )}
            </div>

            {result.content !== undefined && result.content !== null && (
              <div>
                <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>交易内容</Text>
                <div style={{ border: '1px solid #d9d9d9', borderRadius: 6, overflow: 'hidden' }}>
                  <Editor
                    height="250px"
                    defaultLanguage="json"
                    value={JSON.stringify(result.content, null, 2)}
                    options={{
                      readOnly: true,
                      minimap: { enabled: false },
                      fontSize: 12,
                      lineNumbers: 'off',
                      scrollBeyondLastLine: false,
                      automaticLayout: true,
                    }}
                  />
                </div>
              </div>
            )}
          </Space>
        </Card>
      )}
    </div>
  )
}
