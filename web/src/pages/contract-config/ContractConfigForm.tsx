import { useEffect, useState, useCallback } from 'react'
import { Form, Input, Switch, Button, Card, Typography, Space, Divider, InputNumber, Tooltip, message as antMessage } from 'antd'
import { useNavigate, useParams } from 'react-router-dom'
import { QuestionCircleOutlined, FormatPainterOutlined } from '@ant-design/icons'
import Editor from '@monaco-editor/react'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { useGlobalMessage } from '@/components/GlobalMessage'
import { useBlocker, confirmLeave } from '@/hooks/useBlocker'

const { Title, Text } = Typography

interface ChainOption {
  ID: number
  chain_name: string
  chain_type: string
}

type ChainType = 'ethereum' | 'chainmaker' | 'solana' | ''

export default function ContractConfigForm() {
  const { chainConfigId, id } = useParams()
  const navigate = useNavigate()
  const { message } = useGlobalMessage()
  const { handleApiError } = useApiMessage()
  const [form] = Form.useForm()
  const [loading, setLoading] = useState(false)
  const [chains, setChains] = useState<ChainOption[]>([])
  const [abiValue, setAbiValue] = useState('')
  const [chainType, setChainType] = useState<ChainType>('')
  const [enableSubscribe, setEnableSubscribe] = useState(false)
  const [solanaMethodsValue, setSolanaMethodsValue] = useState('')
  const isEdit = !!id

  // 记录表单是否被修改（用于未保存离开确认）
  const [formModified, setFormModified] = useState(false)
  // 记录是否正在提交（提交后跳转不需要确认）
  const [isSubmitting, setIsSubmitting] = useState(false)

  // 未保存离开确认
  useBlocker(formModified && !isSubmitting)

  // 监听表单值变化
  const handleValuesChange = useCallback(() => {
    setFormModified(true)
  }, [])

  useEffect(() => {
    fetchChains()
    if (isEdit) {
      fetchDetail()
    }
  }, [id])

  // 当链列表加载完成后，根据 chainConfigId 确定链类型和链名称
  useEffect(() => {
    if (chains.length > 0 && chainConfigId) {
      const chain = chains.find((c) => String(c.ID) === chainConfigId)
      if (chain) {
        setChainType(chain.chain_type as ChainType)
      }
    }
  }, [chains, chainConfigId])

  const fetchChains = async () => {
    try {
      const res = await api.get('/chain-configs')
      setChains((res.data as ChainOption[]) || [])
    } catch (err) {
      handleApiError(err)
    }
  }

  const fetchDetail = async () => {
    try {
      const res = await api.get(`/chain-configs/${chainConfigId}/contracts/${id}`)
      const data = res.data as Record<string, unknown>
      form.setFieldsValue({
        contract_name: data.contract_name,
        contract_addr: data.contract_addr,
        enable_subscribe: data.enable_subscribe,
      })
      setEnableSubscribe(!!data.enable_subscribe)

      if (data.abi_json) {
        const abiStr = typeof data.abi_json === 'string' ? data.abi_json : JSON.stringify(data.abi_json, null, 2)
        setAbiValue(abiStr)
      }

      // 解析 extra_conf 并填充到对应字段
      if (data.extra_conf && typeof data.extra_conf === 'string') {
        try {
          const extraConf = JSON.parse(data.extra_conf as string)
          if (extraConf.DeployBlockHeight) {
            form.setFieldsValue({ deploy_block_height: extraConf.DeployBlockHeight })
          }
          if (extraConf.GetHistoryEventInterval) {
            form.setFieldsValue({ get_history_event_interval: extraConf.GetHistoryEventInterval })
          }
          if (extraConf.GetHistoryEventHeightWindow) {
            form.setFieldsValue({ get_history_event_height_window: extraConf.GetHistoryEventHeightWindow })
          }
          if (extraConf.SolanaMethods) {
            setSolanaMethodsValue(JSON.stringify(extraConf.SolanaMethods, null, 2))
          }
        } catch {
          // extra_conf 解析失败，忽略
        }
      }
      // 编辑模式加载完数据后，标记为未修改
      setTimeout(() => setFormModified(false), 0)
    } catch (err) {
      handleApiError(err)
    }
  }

  // ABI 格式化处理
  const handleFormatAbi = () => {
    if (!abiValue.trim()) return
    try {
      const parsed = JSON.parse(abiValue)
      const formatted = JSON.stringify(parsed, null, 2)
      setAbiValue(formatted)
      antMessage.success('格式化成功')
    } catch {
      antMessage.error('JSON 格式不正确，无法格式化')
    }
  }

  // Solana 方法配置格式化处理
  const handleFormatSolanaMethods = () => {
    if (!solanaMethodsValue.trim()) return
    try {
      const parsed = JSON.parse(solanaMethodsValue)
      const formatted = JSON.stringify(parsed, null, 2)
      setSolanaMethodsValue(formatted)
      antMessage.success('格式化成功')
    } catch {
      antMessage.error('JSON 格式不正确，无法格式化')
    }
  }

  const handleSubmit = async (values: Record<string, unknown>) => {
    // 验证 ABI JSON 格式
    let abiJson = ''
    if (abiValue.trim()) {
      try {
        JSON.parse(abiValue)
        abiJson = abiValue.trim()
      } catch {
        message.error('ABI JSON 格式不正确')
        return
      }
    }

    // 验证合约地址格式（如果填写了）
    const addrValue = values.contract_addr as string | undefined
    if (addrValue && addrValue.trim()) {
      const addr = addrValue
      if (chainType === 'ethereum' && !/^0x[0-9a-fA-F]{40}$/.test(addr)) {
        message.error('合约地址格式不正确，需为 0x 开头的 40 位十六进制字符串')
        return
      }
      if (chainType === 'solana' && !/^[1-9A-HJ-NP-Za-km-z]{32,44}$/.test(addr)) {
        message.error('合约地址格式不正确，需为 Base58 编码的 Solana 地址')
        return
      }
    }

    // 组装 extra_conf
    const extraConf: Record<string, unknown> = {}

    if (values.deploy_block_height) {
      extraConf.DeployBlockHeight = Number(values.deploy_block_height)
    }

    if (chainType === 'ethereum') {
      if (values.get_history_event_interval) {
        extraConf.GetHistoryEventInterval = Number(values.get_history_event_interval)
      }
      if (values.get_history_event_height_window) {
        extraConf.GetHistoryEventHeightWindow = Number(values.get_history_event_height_window)
      }
    }

    if (chainType === 'solana' && solanaMethodsValue.trim()) {
      try {
        extraConf.SolanaMethods = JSON.parse(solanaMethodsValue)
      } catch {
        message.error('Solana 方法配置 JSON 格式不正确')
        return
      }
    }

    const extraConfStr = Object.keys(extraConf).length > 0 ? JSON.stringify(extraConf) : ''

    const payload = {
      contract_name: values.contract_name,
      contract_addr: values.contract_addr || '',
      abi_json: abiJson,
      enable_subscribe: values.enable_subscribe || false,
      extra_conf: extraConfStr,
    }

    setIsSubmitting(true)
    setLoading(true)
    try {
      if (isEdit) {
        await api.put(`/chain-configs/${chainConfigId}/contracts/${id}`, payload)
        message.success('更新成功')
      } else {
        await api.post(`/chain-configs/${chainConfigId}/contracts`, payload)
        message.success('创建成功')
      }
      navigate('/contract-configs')
    } catch (err) {
      handleApiError(err)
      setIsSubmitting(false)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ maxWidth: 800 }}>
      <Title level={4} style={{ marginBottom: 24 }}>
        {isEdit ? '编辑合约配置' : '新建合约配置'}
      </Title>

      <Card>
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          onValuesChange={handleValuesChange}
          initialValues={{ enable_subscribe: false }}
        >
          <Form.Item
            label="所属链"
          >
            <Input
              value={chains.length > 0 && chainConfigId
                ? (() => {
                    const chain = chains.find((c) => String(c.ID) === chainConfigId)
                    return chain ? `${chain.chain_name} (${chain.chain_type})` : chainConfigId
                  })()
                : '加载中...'}
              disabled
              style={{ color: 'rgba(0, 0, 0, 0.88)' }}
            />
          </Form.Item>

          <Form.Item
            name="contract_name"
            label="合约名称"
            rules={[{ required: true, message: '请输入合约名称' }]}
          >
            <Input placeholder="例如：MyToken" />
          </Form.Item>

          <Form.Item
            name="contract_addr"
            label="合约地址"
            rules={[
              {
                validator: (_, value) => {
                  if (!value || !value.trim()) return Promise.resolve()
                  if (chainType === 'ethereum' && !/^0x[0-9a-fA-F]{40}$/.test(value)) {
                    return Promise.reject(new Error('合约地址格式不正确，需为 0x 开头的 40 位十六进制字符串'))
                  }
                  if (chainType === 'solana' && !/^[1-9A-HJ-NP-Za-km-z]{32,44}$/.test(value)) {
                    return Promise.reject(new Error('合约地址格式不正确，需为 Base58 编码的 Solana 地址'))
                  }
                  return Promise.resolve()
                },
              },
            ]}
            extra={
              chainType === 'ethereum'
                ? '以太坊合约地址格式：0x 开头的 40 位十六进制字符串'
                : chainType === 'solana'
                ? 'Solana 合约地址格式：Base58 编码的 32-44 位字符串'
                : '请输入合约地址'
            }
          >
            <Input
              placeholder={
                chainType === 'ethereum'
                  ? '0x1234567890abcdef1234567890abcdef12345678'
                  : chainType === 'solana'
                  ? 'Base58 编码的 Solana 地址'
                  : '0x...'
              }
              style={{ fontFamily: 'var(--font-mono, monospace)' }}
            />
          </Form.Item>

          <Form.Item label="ABI (JSON)">
            <div style={{ border: '1px solid #d9d9d9', borderRadius: 6, overflow: 'hidden' }}>
              <div style={{ display: 'flex', justifyContent: 'flex-end', padding: '4px 8px', borderBottom: '1px solid #d9d9d9', background: '#fafafa' }}>
                <Button
                  size="small"
                  icon={<FormatPainterOutlined />}
                  onClick={handleFormatAbi}
                >
                  格式化
                </Button>
              </div>
              <Editor
                height="300px"
                defaultLanguage="json"
                value={abiValue}
                onChange={(val) => setAbiValue(val || '')}
                options={{
                  minimap: { enabled: false },
                  fontSize: 13,
                  lineNumbers: 'on',
                  scrollBeyondLastLine: false,
                  formatOnPaste: true,
                  automaticLayout: true,
                }}
              />
            </div>
          </Form.Item>

          <Form.Item
            name="enable_subscribe"
            label="开启事件订阅"
            valuePropName="checked"
          >
            <Switch onChange={(checked) => setEnableSubscribe(checked)} />
          </Form.Item>

          {/* ========== 订阅相关扩展配置（开启订阅时展示） ========== */}
          {enableSubscribe && (
            <>
              <Divider titlePlacement="left" plain>
                <Text type="secondary" style={{ fontSize: 13 }}>订阅配置</Text>
              </Divider>

              {/* 通用字段：合约部署区块高度（以太坊 & 长安链都需要） */}
              {(chainType === 'ethereum' || chainType === 'chainmaker') && (
                <Form.Item
                  name="deploy_block_height"
                  label={
                    <span>
                      合约部署区块高度&nbsp;
                      <Tooltip title="从该区块高度开始扫描历史事件，设为 0 则从最新区块开始">
                        <QuestionCircleOutlined style={{ color: '#999' }} />
                      </Tooltip>
                    </span>
                  }
                >
                  <InputNumber
                    min={0}
                    placeholder="0（从最新区块开始）"
                    style={{ width: '100%' }}
                  />
                </Form.Item>
              )}

              {/* 以太坊专属：轮询间隔 & 区块窗口 */}
              {chainType === 'ethereum' && (
                <>
                  <Form.Item
                    name="get_history_event_interval"
                    label={
                      <span>
                        事件轮询间隔 (ms)&nbsp;
                        <Tooltip title="每隔多少毫秒轮询一次链上历史事件，默认 12000ms（约一个出块周期）">
                          <QuestionCircleOutlined style={{ color: '#999' }} />
                        </Tooltip>
                      </span>
                    }
                  >
                    <InputNumber
                      min={1000}
                      step={1000}
                      placeholder="12000（默认）"
                      style={{ width: '100%' }}
                    />
                  </Form.Item>

                  <Form.Item
                    name="get_history_event_height_window"
                    label={
                      <span>
                        区块扫描窗口&nbsp;
                        <Tooltip title="每次轮询扫描多少个区块的事件，默认 100">
                          <QuestionCircleOutlined style={{ color: '#999' }} />
                        </Tooltip>
                      </span>
                    }
                  >
                    <InputNumber
                      min={1}
                      max={10000}
                      placeholder="100（默认）"
                      style={{ width: '100%' }}
                    />
                  </Form.Item>
                </>
              )}

              {/* Solana 专属：合约部署高度 */}
              {chainType === 'solana' && (
                <Form.Item
                  name="deploy_block_height"
                  label={
                    <span>
                      合约部署 Slot 高度&nbsp;
                      <Tooltip title="从该 Slot 开始扫描历史事件，设为 0 则从最新 Slot 开始">
                        <QuestionCircleOutlined style={{ color: '#999' }} />
                      </Tooltip>
                    </span>
                  }
                >
                  <InputNumber
                    min={0}
                    placeholder="0（从最新 Slot 开始）"
                    style={{ width: '100%' }}
                  />
                </Form.Item>
              )}
            </>
          )}

          {/* ========== Solana 方法调用规范（Solana 链始终展示） ========== */}
          {chainType === 'solana' && (
            <>
              <Divider titlePlacement="left" plain>
                <Text type="secondary" style={{ fontSize: 13 }}>
                  Solana 方法调用规范&nbsp;
                  <Tooltip title="定义合约方法的 Discriminator、参数类型和账户列表，用于 Borsh 序列化调用">
                    <QuestionCircleOutlined style={{ color: '#999' }} />
                  </Tooltip>
                </Text>
              </Divider>

              <Form.Item
                label={
                  <span>
                    方法配置 (JSON)&nbsp;
                    <Tooltip
                      title={
                        <div style={{ fontSize: 12 }}>
                          <p>格式示例：</p>
                          <pre style={{ margin: 0, fontSize: 11 }}>{`{
  "transfer": {
    "Discriminator": "hex16位",
    "ArgSchema": [
      {"Name": "amount", "Type": "u64"}
    ],
    "Accounts": [
      {"Pubkey": "$fromAddress", "IsSigner": true, "IsWritable": true}
    ]
  }
}`}</pre>
                          <p style={{ marginTop: 4 }}>支持类型: u8, u16, u32, u64, i64, bool, string, pubkey, bytes</p>
                        </div>
                      }
                      overlayStyle={{ maxWidth: 420 }}
                    >
                      <QuestionCircleOutlined style={{ color: '#999' }} />
                    </Tooltip>
                  </span>
                }
              >
                <div style={{ border: '1px solid #d9d9d9', borderRadius: 6, overflow: 'hidden' }}>
                  <div style={{ display: 'flex', justifyContent: 'flex-end', padding: '4px 8px', borderBottom: '1px solid #d9d9d9', background: '#fafafa' }}>
                    <Button
                      size="small"
                      icon={<FormatPainterOutlined />}
                      onClick={handleFormatSolanaMethods}
                    >
                      格式化
                    </Button>
                  </div>
                  <Editor
                    height="200px"
                    defaultLanguage="json"
                    value={solanaMethodsValue}
                    onChange={(val) => setSolanaMethodsValue(val || '')}
                    options={{
                      minimap: { enabled: false },
                      fontSize: 13,
                      lineNumbers: 'on',
                      scrollBeyondLastLine: false,
                      formatOnPaste: true,
                      automaticLayout: true,
                    }}
                  />
                </div>
              </Form.Item>
            </>
          )}

          <Divider />

          <Space>
            <Button type="primary" htmlType="submit" loading={loading}>
              {isEdit ? '保存修改' : '创建'}
            </Button>
            <Button onClick={() => confirmLeave(() => navigate('/contract-configs'), formModified)}>取消</Button>
          </Space>
        </Form>
      </Card>
    </div>
  )
}
