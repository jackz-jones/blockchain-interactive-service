import { useEffect, useState, useRef } from 'react'
import { Form, Input, Select, Switch, Button, Card, Typography, Space, Divider, InputNumber, Modal, Tooltip } from 'antd'
import { useNavigate, useParams } from 'react-router-dom'
import { MinusCircleOutlined, PlusOutlined, ApiOutlined, EyeOutlined, EyeInvisibleOutlined } from '@ant-design/icons'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { useGlobalMessage } from '@/components/GlobalMessage'

const { Title } = Typography
const { TextArea } = Input

type ChainType = 'ethereum' | 'chainmaker' | 'solana'

export default function ChainConfigForm() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { message } = useGlobalMessage()
  const { handleApiError } = useApiMessage()
  const [form] = Form.useForm()
  const [loading, setLoading] = useState(false)
  const [testLoading, setTestLoading] = useState(false)
  const [chainType, setChainType] = useState<ChainType>('ethereum')
  const isEdit = !!id

  // 脱敏字段的可见性状态
  const [privateKeyVisible, setPrivateKeyVisible] = useState(false)
  const [signKeyVisible, setSignKeyVisible] = useState(false)
  const [solPrivateKeyVisible, setSolPrivateKeyVisible] = useState(false)
  const [userTlsKeyVisible, setUserTlsKeyVisible] = useState(false)
  const [userEncKeyVisible, setUserEncKeyVisible] = useState(false)

  // 存储脱敏前的原始值（用于提交对比和显示）
  const originalSensitiveValues = useRef<Record<string, string>>({})
  // 存储脱敏后的值（用于表单回显）
  const maskedSensitiveValues = useRef<Record<string, string>>({})

  useEffect(() => {
    if (isEdit) {
      fetchDetail()
    }
  }, [id])

  const fetchDetail = async () => {
    try {
      const res = await api.get(`/chain-configs/${id}`)
      const detail = res.data as { config: Record<string, unknown>; nodes: Array<Record<string, unknown>> }
      const config = detail?.config || {}
      const nodes = detail?.nodes || []

      // 敏感字段处理：保留脱敏值用于表单回显，保存原始值用于提交
      const sensitiveFields = ['private_key', 'sign_key', 'sol_private_key', 'user_tls_key', 'user_enc_key']
      const formValues: Record<string, unknown> = { ...config, nodes }
      for (const field of sensitiveFields) {
        const value = formValues[field] as string
        if (value && value !== '' && value !== null && value !== undefined) {
          // 保存脱敏后的值用于表单显示
          maskedSensitiveValues.current[field] = value
          // 保存脱敏前的原始值（这里后端返回的就是脱敏值，原始值不出域）
          // 由于后端只返回脱敏值，我们将其作为"原始值"存储
          // 后端脱敏格式如 "0x1234****abcd"，提交时如果不修改则发送此值，
          // 后端检测到脱敏格式则不更新该字段
          originalSensitiveValues.current[field] = value
        } else {
          // 字段为空则删除以显示 placeholder
          delete formValues[field]
        }
      }
      form.setFieldsValue(formValues)
      setChainType(config.chain_type as ChainType)
    } catch (err) {
      handleApiError(err)
    }
  }

  // 判断值是否为脱敏格式（包含 ****）
  const isMaskedValue = (value: string) => {
    return value && value.includes('****')
  }

  // 二次确认弹框
  const confirmSensitiveChange = (fieldName: string, newValue: string): Promise<boolean> => {
    return new Promise((resolve) => {
      Modal.confirm({
        title: '确认修改私钥',
        content: (
          <div>
            <p>您正在修改「{fieldName}」，私钥修改后可能导致合约调用失败，确定要修改吗？</p>
            <p style={{ marginTop: 8, color: '#888', fontSize: 12, wordBreak: 'break-all' }}>
              新值：{newValue}
            </p>
          </div>
        ),
        okText: '确认修改',
        cancelText: '取消',
        okButtonProps: { danger: true },
        onOk: () => resolve(true),
        onCancel: () => resolve(false),
      })
    })
  }

  const handleSubmit = async (values: Record<string, unknown>) => {

    // 检查私钥字段是否被用户实际修改
    const sensitiveFieldMap: Record<string, string> = {
      'private_key': '私钥',
      'sign_key': '签名私钥',
      'sol_private_key': 'Solana 私钥',
      'user_tls_key': 'TLS 私钥',
      'user_enc_key': '国密加密私钥',
    }

    // 处理私钥字段的提交值：脱敏值或空值不提交，保留数据库原值
    // 仅编辑模式下才需要二次确认私钥变更，新建时直接提交
    let needsConfirm = false
    let confirmFieldName = ''
    let confirmNewValue = ''
    for (const [field, label] of Object.entries(sensitiveFieldMap)) {
      const currentValue = values[field] as string
      // 如果是脱敏值原样保留或字段为空，则不提交该字段
      if (isMaskedValue(currentValue) || currentValue === undefined || currentValue === null || currentValue === '') {
        delete values[field]
      } else if (isEdit && currentValue !== originalSensitiveValues.current[field]) {
        // 编辑模式下，用户输入了非脱敏格式的新值（与原始值不同），需要二次确认
        needsConfirm = true
        confirmFieldName = label
        confirmNewValue = currentValue
        break
      } else {
        // 值未变化，正常处理
      }
    }

    if (needsConfirm) {
      const confirmed = await confirmSensitiveChange(confirmFieldName, confirmNewValue)
      if (!confirmed) {
        return
      }
    }

    setLoading(true)
    try {
      if (isEdit) {
        await api.put(`/chain-configs/${id}`, values)
        message.success('更新成功')
      } else {
        await api.post('/chain-configs', values)
        message.success('创建成功')
      }
      navigate('/chain-configs')
    } catch (err) {
      handleApiError(err)
    } finally {
      setLoading(false)
    }
  }

  const handleTestConnection = async () => {
    if (!id) {
      message.warning('请先保存配置后再测试连接')
      return
    }
    setTestLoading(true)
    try {
      // 获取当前表单值，发送给后端以测试用户实际填写的配置
      const formValues = form.getFieldsValue()
      const res = await api.post(`/chain-configs/${id}/test-connection`, formValues)
      const data = res.data as { success?: boolean; error?: string; chain_name?: string; chain_type?: string }
      if (data.success) {
        message.success(`连接测试成功（${data.chain_type}: ${data.chain_name}）`)
      } else {
        message.error(`连接测试失败：${data.error || '未知错误'}`)
      }
    } catch (err) {
      handleApiError(err)
    } finally {
      setTestLoading(false)
    }
  }

  return (
    <div style={{ maxWidth: 720 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <Title level={4} style={{ margin: 0 }}>
          {isEdit ? '编辑链配置' : '新建链配置'}
        </Title>
        {isEdit && (
          <Button icon={<ApiOutlined />} loading={testLoading} onClick={handleTestConnection}>
            测试连接
          </Button>
        )}
      </div>

      <Card>
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{ chain_type: 'ethereum', enable: true }}
        >
          {/* 基础信息 */}
          <Form.Item
            name="chain_name"
            label="链名称"
            rules={[{ required: true, message: '请输入链名称' }]}
          >
            <Input placeholder="例如：my-ethereum-mainnet" />
          </Form.Item>

          <Form.Item
            name="chain_type"
            label="链类型"
            rules={[{ required: true }]}
          >
            <Select
              onChange={(val) => setChainType(val)}
              options={[
                { label: 'Ethereum', value: 'ethereum' },
                { label: 'ChainMaker', value: 'chainmaker' },
                { label: 'Solana', value: 'solana' },
              ]}
            />
          </Form.Item>

          <Form.Item name="enable" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>

          <Divider />

          {/* Ethereum 字段 */}
          {chainType === 'ethereum' && (
            <>
              <Form.Item
                name="http_url"
                label="HTTP RPC URL"
                rules={[{ required: true, message: '请输入 HTTP RPC URL' }]}
              >
                <Input placeholder="https://mainnet.infura.io/v3/YOUR_KEY" />
              </Form.Item>
              <Form.Item name="websocket_url" label="WebSocket URL">
                <Input placeholder="wss://mainnet.infura.io/ws/v3/YOUR_KEY" />
              </Form.Item>
              <Form.Item name="eth_chain_id" label="Chain ID">
                <InputNumber placeholder="1" style={{ width: '100%' }} />
              </Form.Item>
              <Form.Item name="private_key" label="私钥">
                <Input
                  type={privateKeyVisible ? 'text' : 'password'}
                  placeholder={isEdit ? '不修改请留空，保留原私钥' : '0x...'}
                  addonAfter={
                    isEdit ? (
                      <Tooltip title={privateKeyVisible ? '隐藏私钥' : '显示原私钥'}>
                        {privateKeyVisible ? (
                          <EyeInvisibleOutlined onClick={() => setPrivateKeyVisible(false)} />
                        ) : (
                          <EyeOutlined onClick={() => setPrivateKeyVisible(true)} />
                        )}
                      </Tooltip>
                    ) : undefined
                  }
                />
              </Form.Item>
            </>
          )}

          {/* ChainMaker 字段 */}
          {chainType === 'chainmaker' && (
            <>
              <Form.Item
                name="chain_id"
                label="Chain ID"
                rules={[{ required: true, message: '请输入 Chain ID' }]}
              >
                <Input placeholder="chain1" />
              </Form.Item>
              <Form.Item
                name="org_id"
                label="组织 ID"
                rules={[{ required: true, message: '请输入组织 ID' }]}
              >
                <Input placeholder="wx-org1.chainmaker.org" />
              </Form.Item>
              <Form.Item name="auth_type" label="认证类型">
                <Select
                  options={[
                    { label: 'PermissionedWithCert', value: 'permissionedWithCert' },
                    { label: 'PermissionedWithKey', value: 'permissionedWithKey' },
                    { label: 'Public', value: 'public' },
                  ]}
                />
              </Form.Item>
              <Form.Item name="hash_type" label="哈希类型">
                <Select
                  options={[
                    { label: 'SHA256', value: 'SHA256' },
                    { label: 'SM3', value: 'SM3' },
                  ]}
                />
              </Form.Item>
              <Form.Item name="sign_key" label="签名私钥">
                <Input
                  type={signKeyVisible ? 'text' : 'password'}
                  placeholder={isEdit ? '不修改请留空，保留原私钥' : 'PEM 格式私钥'}
                  addonAfter={
                    isEdit ? (
                      <Tooltip title={signKeyVisible ? '隐藏私钥' : '显示原私钥'}>
                        {signKeyVisible ? (
                          <EyeInvisibleOutlined onClick={() => setSignKeyVisible(false)} />
                        ) : (
                          <EyeOutlined onClick={() => setSignKeyVisible(true)} />
                        )}
                      </Tooltip>
                    ) : undefined
                  }
                />
              </Form.Item>
              <Form.Item name="sign_cert" label="签名证书">
                <TextArea rows={3} placeholder="PEM 格式证书" />
              </Form.Item>
              <Form.Item name="user_tls_key" label="TLS 私钥">
                <Input
                  type={userTlsKeyVisible ? 'text' : 'password'}
                  placeholder={isEdit ? '不修改请留空，保留原私钥' : 'Base64 编码 TLS 私钥'}
                  addonAfter={
                    isEdit ? (
                      <Tooltip title={userTlsKeyVisible ? '隐藏私钥' : '显示原私钥'}>
                        {userTlsKeyVisible ? (
                          <EyeInvisibleOutlined onClick={() => setUserTlsKeyVisible(false)} />
                        ) : (
                          <EyeOutlined onClick={() => setUserTlsKeyVisible(true)} />
                        )}
                      </Tooltip>
                    ) : undefined
                  }
                />
              </Form.Item>
              <Form.Item name="user_enc_key" label="国密加密私钥">
                <Input
                  type={userEncKeyVisible ? 'text' : 'password'}
                  placeholder={isEdit ? '不修改请留空，保留原私钥' : 'Base64 编码国密加密私钥'}
                  addonAfter={
                    isEdit ? (
                      <Tooltip title={userEncKeyVisible ? '隐藏私钥' : '显示原私钥'}>
                        {userEncKeyVisible ? (
                          <EyeInvisibleOutlined onClick={() => setUserEncKeyVisible(false)} />
                        ) : (
                          <EyeOutlined onClick={() => setUserEncKeyVisible(true)} />
                        )}
                      </Tooltip>
                    ) : undefined
                  }
                />
              </Form.Item>
              <Form.Item name="proxy_url" label="代理 URL">
                <Input placeholder="http://proxy:8080" />
              </Form.Item>

              {/* 节点列表 */}
              <Form.List name="nodes">
                {(fields, { add, remove }) => (
                  <>
                    <Typography.Text strong style={{ display: 'block', marginBottom: 8 }}>
                      节点列表
                    </Typography.Text>
                    {fields.map(({ key, name, ...restField }) => (
                      <Space key={key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                        <Form.Item
                          {...restField}
                          name={[name, 'node_addr']}
                          rules={[{ required: true, message: '请输入节点地址' }]}
                        >
                          <Input placeholder="节点地址 (host:port)" style={{ width: 300 }} />
                        </Form.Item>
                        <Form.Item {...restField} name={[name, 'tls_host_name']}>
                          <Input placeholder="TLS hostname" style={{ width: 200 }} />
                        </Form.Item>
                        <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#c0392b' }} />
                      </Space>
                    ))}
                    <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                      添加节点
                    </Button>
                  </>
                )}
              </Form.List>
            </>
          )}

          {/* Solana 字段 */}
          {chainType === 'solana' && (
            <>
              <Form.Item
                name="sol_rpc_url"
                label="RPC URL"
                rules={[{ required: true, message: '请输入 RPC URL' }]}
              >
                <Input placeholder="https://api.mainnet-beta.solana.com" />
              </Form.Item>
              <Form.Item name="sol_private_key" label="私钥">
                <Input
                  type={solPrivateKeyVisible ? 'text' : 'password'}
                  placeholder={isEdit ? '不修改请留空，保留原私钥' : 'Base58 编码私钥'}
                  addonAfter={
                    isEdit ? (
                      <Tooltip title={solPrivateKeyVisible ? '隐藏私钥' : '显示原私钥'}>
                        {solPrivateKeyVisible ? (
                          <EyeInvisibleOutlined onClick={() => setSolPrivateKeyVisible(false)} />
                        ) : (
                          <EyeOutlined onClick={() => setSolPrivateKeyVisible(true)} />
                        )}
                      </Tooltip>
                    ) : undefined
                  }
                />
              </Form.Item>
              <Form.Item name="commitment_level" label="Commitment Level">
                <Select
                  options={[
                    { label: 'Finalized', value: 'finalized' },
                    { label: 'Confirmed', value: 'confirmed' },
                    { label: 'Processed', value: 'processed' },
                  ]}
                />
              </Form.Item>
              <Form.Item name="skip_preflight" label="Skip Preflight" valuePropName="checked">
                <Switch />
              </Form.Item>
              <Form.Item name="max_retries" label="Max Retries">
                <InputNumber min={0} max={10} style={{ width: '100%' }} />
              </Form.Item>
            </>
          )}

          <Divider />

          <Space>
            <Button type="primary" htmlType="submit" loading={loading}>
              {isEdit ? '保存修改' : '创建'}
            </Button>
            <Button onClick={() => navigate('/chain-configs')}>取消</Button>
          </Space>
        </Form>
      </Card>
    </div>
  )
}