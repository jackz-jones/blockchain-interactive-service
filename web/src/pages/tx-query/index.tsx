import { useState, useEffect } from 'react'
import { Card, Form, Select, Input, Button, Typography, Space, Alert } from 'antd'
import { SearchOutlined } from '@ant-design/icons'
import Editor from '@monaco-editor/react'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'

const { Title, Text } = Typography

interface ChainOption {
  ID: number
  chain_name: string
}

interface TxResult {
  tx_id: string
  confirmed?: boolean
  result?: string
  error?: string
  status?: string
}

export default function TxQuery() {
  const [form] = Form.useForm()
  const [chains, setChains] = useState<ChainOption[]>([])
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<TxResult | null>(null)
  const { handleApiError } = useApiMessage()

  useEffect(() => {
    fetchChains()
  }, [])

  const fetchChains = async () => {
    try {
      const res = await api.get('/chain-configs')
      setChains((res.data as ChainOption[]) || [])
    } catch (err) {
      handleApiError(err)
    }
  }

  const handleQuery = async (values: Record<string, string>) => {
    const txId = values.tx_id || ''
    const chainName = values.chain_name || ''
    setLoading(true)
    setResult(null)
    try {
      const res = await api.get(`/tx/${encodeURIComponent(txId)}`, {
        params: { chain_name: chainName },
      })
      const data = res.data as { result?: string; confirmed?: boolean }
      setResult({ tx_id: txId, result: data.result, confirmed: data.confirmed })
    } catch (err) {
      setResult({ tx_id: txId, status: 'ERROR', error: err instanceof Error ? err.message : '查询失败' })
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
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap', flexShrink: 0 }}>交易 ID</Text>
                <div style={{ overflow: 'auto', flex: 1 }}>
                  <Text code style={{ fontSize: 12, whiteSpace: 'nowrap' }}>{result.tx_id}</Text>
                </div>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap', flexShrink: 0 }}>确认状态</Text>
                <Text>{result.confirmed ? '✅ 已确认' : '⏳ 待确认 / Pending'}</Text>
              </div>
            </div>

            {result.result !== undefined && result.result !== null && (
              <div>
                <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>交易详情</Text>
                <div style={{ border: '1px solid #d9d9d9', borderRadius: 6, overflow: 'hidden' }}>
                  <Editor
                    height="300px"
                    defaultLanguage="json"
                    value={(() => {
                      try {
                        const obj = JSON.parse(result.result!)
                        // logs 字段是 []byte 经 JSON 序列化的 base64 字符串，自动解码为 JSON
                        if (obj && typeof obj.logs === 'string' && obj.logs.length > 0) {
                          try {
                            const decoded = atob(obj.logs)
                            obj.logs = JSON.parse(decoded)
                          } catch {
                            // 解码失败保持原样
                          }
                        }
                        return JSON.stringify(obj, null, 2)
                      } catch {
                        return result.result
                      }
                    })()}
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
