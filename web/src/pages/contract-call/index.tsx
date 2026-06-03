import { useState, useEffect } from 'react'
import { Card, Form, Select, Input, Button, Typography, Space, Divider, Tag, Alert } from 'antd'
import { MinusCircleOutlined, PlusOutlined, SendOutlined } from '@ant-design/icons'
import Editor from '@monaco-editor/react'
import api from '@/services/api'

const { Title, Text } = Typography

interface ChainOption {
  id: string
  chain_name: string
  chain_type: string
}

interface CallResult {
  tx_id?: string
  status?: string
  content?: unknown
  error?: string
}

export default function ContractCall() {
  const [form] = Form.useForm()
  const [chains, setChains] = useState<ChainOption[]>([])
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<CallResult | null>(null)

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

  const handleCall = async (values: Record<string, unknown>) => {
    setLoading(true)
    setResult(null)
    try {
      // 将 params 数组转为 map
      const params: Record<string, string> = {}
      const paramList = values.params as Array<{ key: string; value: string }> | undefined
      if (paramList) {
        paramList.forEach((p) => {
          if (p.key) params[p.key] = p.value
        })
      }

      const payload = {
        chain_name: values.chain_name,
        contract_name: values.contract_name,
        method: values.method,
        call_type: values.call_type,
        params,
      }

      const res = await api.post('/chain/call-contract', payload)
      setResult(res.data as CallResult)
    } catch (err) {
      setResult({ error: err instanceof Error ? err.message : '调用失败' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ maxWidth: 900 }}>
      <Title level={4} style={{ marginBottom: 24 }}>合约调用</Title>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20 }}>
        {/* 左侧：调用表单 */}
        <Card title="调用参数">
          <Form form={form} layout="vertical" onFinish={handleCall} initialValues={{ call_type: 'invoke' }}>
            <Form.Item name="chain_name" label="选择链" rules={[{ required: true, message: '请选择链' }]}>
              <Select
                placeholder="选择链配置"
                options={chains.map((c) => ({ label: `${c.chain_name} (${c.chain_type})`, value: c.chain_name }))}
              />
            </Form.Item>

            <Form.Item name="contract_name" label="合约名称" rules={[{ required: true, message: '请输入合约名称' }]}>
              <Input placeholder="合约名称或地址" />
            </Form.Item>

            <Form.Item name="method" label="方法名" rules={[{ required: true, message: '请输入方法名' }]}>
              <Input placeholder="例如：transfer、balanceOf" />
            </Form.Item>

            <Form.Item name="call_type" label="调用类型">
              <Select
                options={[
                  { label: 'Invoke（写入）', value: 'invoke' },
                  { label: 'Query（查询）', value: 'query' },
                ]}
              />
            </Form.Item>

            <Divider orientation="left" plain>
              <Text type="secondary" style={{ fontSize: 13 }}>参数列表</Text>
            </Divider>

            <Form.List name="params">
              {(fields, { add, remove }) => (
                <>
                  {fields.map(({ key, name, ...restField }) => (
                    <Space key={key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                      <Form.Item {...restField} name={[name, 'key']} style={{ marginBottom: 0 }}>
                        <Input placeholder="参数名" style={{ width: 140 }} />
                      </Form.Item>
                      <Form.Item {...restField} name={[name, 'value']} style={{ marginBottom: 0 }}>
                        <Input placeholder="参数值" style={{ width: 200 }} />
                      </Form.Item>
                      <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#c0392b' }} />
                    </Space>
                  ))}
                  <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />} size="small">
                    添加参数
                  </Button>
                </>
              )}
            </Form.List>

            <Divider />

            <Button type="primary" htmlType="submit" loading={loading} icon={<SendOutlined />} block>
              执行调用
            </Button>
          </Form>
        </Card>

        {/* 右侧：结果展示 */}
        <Card title="调用结果">
          {!result && (
            <div style={{ textAlign: 'center', padding: '60px 0', color: 'var(--color-muted)' }}>
              <Text type="secondary">执行合约调用后，结果将在此处展示</Text>
            </div>
          )}

          {result?.error && (
            <Alert type="error" message="调用失败" description={result.error} showIcon />
          )}

          {result && !result.error && (
            <Space direction="vertical" style={{ width: '100%' }} size="middle">
              {result.tx_id && (
                <div>
                  <Text type="secondary" style={{ fontSize: 12 }}>交易 ID</Text>
                  <div>
                    <Text code style={{ fontSize: 12, wordBreak: 'break-all' }}>{result.tx_id}</Text>
                  </div>
                </div>
              )}
              {result.status && (
                <div>
                  <Text type="secondary" style={{ fontSize: 12 }}>状态</Text>
                  <div>
                    <Tag color={result.status === 'SUCCESS' ? 'success' : 'warning'}>
                      {result.status}
                    </Tag>
                  </div>
                </div>
              )}
              {result.content !== undefined && (
                <div>
                  <Text type="secondary" style={{ fontSize: 12 }}>返回内容</Text>
                  <div style={{ marginTop: 8, border: '1px solid #d9d9d9', borderRadius: 6, overflow: 'hidden' }}>
                    <Editor
                      height="200px"
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
          )}
        </Card>
      </div>
    </div>
  )
}
