import { useState, useEffect, useRef, useCallback } from 'react'
import { Card, Form, Select, Input, Button, Typography, Space, Divider, Tag, Alert, Switch, InputNumber, Tooltip } from 'antd'
import { MinusCircleOutlined, PlusOutlined, SendOutlined, QuestionCircleOutlined, SyncOutlined, ClearOutlined, CopyOutlined } from '@ant-design/icons'
import Editor from '@monaco-editor/react'
import { Link } from 'react-router-dom'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { useGlobalMessage } from '@/components/GlobalMessage'

const { Title, Text } = Typography

interface ChainOption {
  ID: number
  chain_name: string
  chain_type: string
}

interface ContractOption {
  ID: number
  contract_name: string
  contract_addr: string
  abi_json: string
}

interface CallResult {
  tx_id?: string
  status?: string
  pending?: boolean
  content?: unknown
  error?: string
}

export default function ContractCall() {
  const [form] = Form.useForm()
  const [chains, setChains] = useState<ChainOption[]>([])
  const [contracts, setContracts] = useState<ContractOption[]>([])
  const [methodOptions, setMethodOptions] = useState<{ label: string; value: string }[]>([])
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<CallResult | null>(null)
  const [polling, setPolling] = useState(false)
  const pollingRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const { handleApiError } = useApiMessage()
  const { message } = useGlobalMessage()

  useEffect(() => {
    fetchChains()
    return () => {
      // 组件卸载时清除轮询
      if (pollingRef.current) clearTimeout(pollingRef.current)
    }
  }, [])

  // 轮询交易状态
  const pollTxStatus = useCallback((txId: string, chainName: string, attempt = 0) => {
    const MAX_ATTEMPTS = 20 // 最多轮询 20 次（约 60 秒）
    const INTERVAL = 3000  // 每 3 秒轮询一次

    if (attempt >= MAX_ATTEMPTS) {
      setPolling(false)
      setResult(prev => prev ? { ...prev, pending: true, status: 'TIMEOUT' } : prev)
      return
    }

    pollingRef.current = setTimeout(async () => {
      try {
        const res = await api.get(`/tx/${txId}`, { params: { chain_name: chainName } })
        const data = res.data as Record<string, unknown>
        const confirmed = data.confirmed as boolean

        if (confirmed) {
          // 交易已确认
          let content: unknown = data.result
          if (typeof data.result === 'string' && data.result) {
            try { content = JSON.parse(data.result as string) } catch { content = data.result }
          }
          setResult({
            tx_id: txId,
            status: 'CONFIRMED',
            pending: false,
            content: content !== undefined && content !== '' ? content : data,
          })
          setPolling(false)
        } else {
          // 继续轮询
          pollTxStatus(txId, chainName, attempt + 1)
        }
      } catch {
        // 查询失败不中断轮询，继续尝试
        pollTxStatus(txId, chainName, attempt + 1)
      }
    }, INTERVAL)
  }, [])

  const fetchChains = async () => {
    try {
      const res = await api.get('/chain-configs')
      setChains((res.data as ChainOption[]) || [])
    } catch (err) {
      handleApiError(err)
    }
  }

  // 6.1: 根据已选链加载合约列表
  const fetchContracts = async (chainName: string) => {
    try {
      const chain = chains.find(c => c.chain_name === chainName)
      if (!chain) return
      const res = await api.get(`/chain-configs/${chain.ID}/contracts`)
      const items = (res.data as { items: ContractOption[] })?.items || []
      setContracts(items)
    } catch (err) {
      handleApiError(err)
    }
  }

  // 6.2: 根据合约 ABI 解析方法名列表
  const parseAbiMethods = (abiJson: string) => {
    if (!abiJson || !abiJson.trim()) {
      setMethodOptions([])
      return
    }
    try {
      const abi = JSON.parse(abiJson)
      const methods: { label: string; value: string }[] = []
      if (Array.isArray(abi)) {
        abi.forEach((item: Record<string, unknown>) => {
          if (item.type === 'function' && item.name) {
            methods.push({ label: item.name as string, value: item.name as string })
          }
        })
      }
      setMethodOptions(methods)
    } catch {
      setMethodOptions([])
    }
  }

  // 当选择链变化时，加载合约列表
  const handleChainChange = (chainName: string) => {
    setContracts([])
    setMethodOptions([])
    form.setFieldsValue({ contract_name: undefined, method: undefined })
    if (chainName) {
      fetchContracts(chainName)
    }
  }

  // 当选择合约变化时，解析 ABI 方法
  const handleContractChange = (contractName: string) => {
    form.setFieldsValue({ method: undefined })
    const contract = contracts.find(c => c.contract_name === contractName)
    if (contract) {
      parseAbiMethods(contract.abi_json)
    } else {
      setMethodOptions([])
    }
  }

  // 6.3: 清除结果
  const handleClearResult = () => {
    if (pollingRef.current) {
      clearTimeout(pollingRef.current)
      pollingRef.current = null
    }
    setPolling(false)
    setResult(null)
  }

  // 复制交易 ID
  const handleCopyTxId = (txId: string) => {
    navigator.clipboard.writeText(txId).then(() => {
      message.success('交易 ID 已复制到剪贴板')
    }).catch(() => {
      message.error('复制失败')
    })
  }

  const handleCall = async (values: Record<string, unknown>) => {
    // 6.5: 重复参数名校验
    const paramList = values.params as Array<{ key: string; value: string }> | undefined
    if (paramList) {
      const keys = paramList.map(p => p.key).filter(Boolean)
      const duplicateKeys = keys.filter((key, index) => keys.indexOf(key) !== index)
      if (duplicateKeys.length > 0) {
        message.error(`参数名重复：${[...new Set(duplicateKeys)].join('、')}`)
        return
      }
    }

    setLoading(true)
    setResult(null)
    try {
      // 将 params 数组转为 map
      const params: Record<string, string> = {}
      if (paramList) {
        paramList.forEach((p) => {
          if (p.key) params[p.key] = p.value
        })
      }

      const payload = {
        chain_name: values.chain_name,
        contract_name: values.contract_name,
        method: values.method,
        method_type: values.method_type || 1, // 1-写链(Invoke) 2-读链(Query)
        params,
        sync: values.sync || false,
        tx_timeout: values.tx_timeout || 10,
        gas_limit: values.gas_limit || 0, // 0=自动估算
      }

      // 同步模式下动态设置 axios 超时：后端超时 + 5 秒余量
      const axiosTimeout = payload.sync
        ? ((payload.tx_timeout as number) + 5) * 1000
        : 30000

      const res = await api.post('/contract/call', payload, { timeout: axiosTimeout })
      // API 拦截器已自动解包 CommonResponse，res.data 直接是 {tx_id, result, duration}
      const resultData = res.data as Record<string, unknown>
      let content: unknown = resultData.result
      // 尝试解析 result 字符串为 JSON
      if (typeof resultData.result === 'string' && resultData.result) {
        try {
          content = JSON.parse(resultData.result as string)
        } catch {
          content = resultData.result
        }
      }
      const isPending = resultData.pending as boolean | undefined
      setResult({
        tx_id: resultData.tx_id as string,
        status: isPending ? 'PENDING' : 'SUCCESS',
        pending: isPending,
        content: content !== undefined && content !== '' ? content : resultData,
      })

      // 如果是异步模式且交易 pending，自动开始轮询
      if (isPending && resultData.tx_id) {
        setPolling(true)
        pollTxStatus(resultData.tx_id as string, values.chain_name as string)
      }
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
          <Form form={form} layout="vertical" onFinish={handleCall}>
            <Form.Item name="chain_name" label="选择链" rules={[{ required: true, message: '请选择链' }]}>
              <Select
                placeholder="选择链配置"
                options={chains.map((c) => ({ label: `${c.chain_name} (${c.chain_type})`, value: c.chain_name }))}
                onChange={handleChainChange}
              />
            </Form.Item>

            <Form.Item name="contract_name" label="合约名称" rules={[{ required: true, message: '请输入合约名称' }]}>
              {contracts.length > 0 ? (
                <Select
                  placeholder="选择合约"
                  showSearch
                  options={contracts.map((c) => ({ label: c.contract_name, value: c.contract_name }))}
                  onChange={handleContractChange}
                />
              ) : (
                <Input placeholder="请先选择链，或手动输入合约名称" />
              )}
            </Form.Item>

            <Form.Item name="method" label="方法名" rules={[{ required: true, message: '请输入方法名' }]}>
              {methodOptions.length > 0 ? (
                <Select
                  placeholder="选择方法"
                  showSearch
                  options={methodOptions}
                  // 允许手动输入不在列表中的方法名
                  filterOption={(input, option) =>
                    (option?.value as string)?.toLowerCase().includes(input.toLowerCase())
                  }
                />
              ) : (
                <Input placeholder="例如：transfer、balanceOf" />
              )}
            </Form.Item>

            <Form.Item name="method_type" label="调用类型" initialValue={1}>
              <Select
                options={[
                  { label: '写链 (Invoke) - 会改变链上状态', value: 1 },
                  { label: '读链 (Query) - 只读查询不上链', value: 2 },
                ]}
              />
            </Form.Item>

            <Form.Item noStyle shouldUpdate={(prev, cur) => prev.method_type !== cur.method_type || prev.sync !== cur.sync}>
              {({ getFieldValue }) => {
                const methodType = getFieldValue('method_type')
                // 读链(Query)不需要同步/异步选择、超时时间、Gas Limit
                if (methodType === 2) return null
                return (
                  <>
                    <Form.Item
                      name="sync"
                      valuePropName="checked"
                      initialValue={false}
                      label={
                        <span>
                          同步等待上链&nbsp;
                          <Tooltip title="开启后将等待交易被区块确认后再返回结果，关闭则提交交易后立即返回交易ID">
                            <QuestionCircleOutlined style={{ color: '#999' }} />
                          </Tooltip>
                        </span>
                      }
                    >
                      <Switch checkedChildren="同步" unCheckedChildren="异步" />
                    </Form.Item>

                    {getFieldValue('sync') && (
                      <Form.Item name="tx_timeout" label="超时时间(秒)" initialValue={10}>
                        <InputNumber min={5} max={60} style={{ width: '100%' }} />
                      </Form.Item>
                    )}

                    <Form.Item
                      name="gas_limit"
                      label={
                        <span>
                          Gas Limit&nbsp;
                          <Tooltip title="交易 Gas 上限。设为 0 或不填则自动估算（推荐）；手动填写可覆盖自动估算值">
                            <QuestionCircleOutlined style={{ color: '#999' }} />
                          </Tooltip>
                        </span>
                      }
                    >
                      <InputNumber min={0} placeholder="0（自动估算）" style={{ width: '100%' }} />
                    </Form.Item>
                  </>
                )
              }}
            </Form.Item>

            <Divider titlePlacement="left" plain>
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
        <Card
          title="调用结果"
          extra={result ? (
            <Button size="small" icon={<ClearOutlined />} onClick={handleClearResult}>
              清除结果
            </Button>
          ) : null}
        >
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
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 4, overflow: 'auto' }}>
                    <Text code style={{ fontSize: 12, whiteSpace: 'nowrap' }}>{result.tx_id}</Text>
                    <Button
                      type="text"
                      size="small"
                      icon={<CopyOutlined />}
                      onClick={() => handleCopyTxId(result.tx_id!)}
                      title="复制交易 ID"
                    />
                  </div>
                </div>
              )}
              {result.status && (
                <div>
                  <Text type="secondary" style={{ fontSize: 12 }}>状态</Text>
                  <div>
                    {result.status === 'CONFIRMED' && (
                      <Tag color="success">已确认上链</Tag>
                    )}
                    {result.status === 'SUCCESS' && (
                      <Tag color="success">调用成功</Tag>
                    )}
                    {result.status === 'PENDING' && (
                      <Tag icon={polling ? <SyncOutlined spin /> : undefined} color="processing">
                        {polling ? '轮询确认中...' : '交易已提交，等待区块确认'}
                      </Tag>
                    )}
                    {result.status === 'TIMEOUT' && (
                      <>
                        <Tag color="warning">轮询超时</Tag>
                        <Link to="/tx-query" style={{ fontSize: 12, marginLeft: 4 }}>
                          前往交易查询页查看 →
                        </Link>
                      </>
                    )}
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
