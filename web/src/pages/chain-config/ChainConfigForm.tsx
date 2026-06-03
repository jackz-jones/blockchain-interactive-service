import { useEffect, useState } from 'react'
import { Form, Input, Select, Switch, Button, Card, Typography, Space, message, Divider, InputNumber } from 'antd'
import { useNavigate, useParams } from 'react-router-dom'
import { MinusCircleOutlined, PlusOutlined, ApiOutlined } from '@ant-design/icons'
import api from '@/services/api'

const { Title } = Typography
const { TextArea } = Input

type ChainType = 'ethereum' | 'chainmaker' | 'solana'

export default function ChainConfigForm() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [form] = Form.useForm()
  const [loading, setLoading] = useState(false)
  const [testLoading, setTestLoading] = useState(false)
  const [chainType, setChainType] = useState<ChainType>('ethereum')
  const isEdit = !!id

  useEffect(() => {
    if (isEdit) {
      fetchDetail()
    }
  }, [id])

  const fetchDetail = async () => {
    try {
      const res = await api.get(`/chain-configs/${id}`)
      const data = res.data as Record<string, unknown>
      form.setFieldsValue(data)
      setChainType(data.chain_type as ChainType)
    } catch {
      // 错误已由拦截器处理
    }
  }

  const handleSubmit = async (values: Record<string, unknown>) => {
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
    } catch {
      // 错误已由拦截器处理
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
      await api.post(`/chain-configs/${id}/test-connection`)
      message.success('连接测试成功')
    } catch {
      // 错误已由拦截器处理
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
          initialValues={{ chain_type: 'ethereum', enabled: true }}
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

          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>

          <Divider />

          {/* Ethereum 字段 */}
          {chainType === 'ethereum' && (
            <>
              <Form.Item
                name={['sdk_config', 'http_url']}
                label="HTTP RPC URL"
                rules={[{ required: true, message: '请输入 HTTP RPC URL' }]}
              >
                <Input placeholder="https://mainnet.infura.io/v3/YOUR_KEY" />
              </Form.Item>
              <Form.Item name={['sdk_config', 'ws_url']} label="WebSocket URL">
                <Input placeholder="wss://mainnet.infura.io/ws/v3/YOUR_KEY" />
              </Form.Item>
              <Form.Item name={['sdk_config', 'chain_id']} label="Chain ID">
                <InputNumber placeholder="1" style={{ width: '100%' }} />
              </Form.Item>
              <Form.Item name={['sdk_config', 'private_key']} label="私钥">
                <Input.Password placeholder="0x..." />
              </Form.Item>
              <Form.Item name={['sdk_config', 'gas_limit']} label="Gas Limit">
                <InputNumber placeholder="21000" style={{ width: '100%' }} />
              </Form.Item>
            </>
          )}

          {/* ChainMaker 字段 */}
          {chainType === 'chainmaker' && (
            <>
              <Form.Item
                name={['sdk_config', 'chain_id']}
                label="Chain ID"
                rules={[{ required: true, message: '请输入 Chain ID' }]}
              >
                <Input placeholder="chain1" />
              </Form.Item>
              <Form.Item
                name={['sdk_config', 'org_id']}
                label="组织 ID"
                rules={[{ required: true, message: '请输入组织 ID' }]}
              >
                <Input placeholder="wx-org1.chainmaker.org" />
              </Form.Item>
              <Form.Item name={['sdk_config', 'auth_type']} label="认证类型">
                <Select
                  options={[
                    { label: 'PermissionedWithCert', value: 'permissionedWithCert' },
                    { label: 'PermissionedWithKey', value: 'permissionedWithKey' },
                    { label: 'Public', value: 'public' },
                  ]}
                />
              </Form.Item>
              <Form.Item name={['sdk_config', 'hash_type']} label="哈希类型">
                <Select
                  options={[
                    { label: 'SHA256', value: 'SHA256' },
                    { label: 'SM3', value: 'SM3' },
                  ]}
                />
              </Form.Item>
              <Form.Item name={['sdk_config', 'sign_key']} label="签名私钥">
                <TextArea rows={3} placeholder="PEM 格式私钥" />
              </Form.Item>
              <Form.Item name={['sdk_config', 'sign_cert']} label="签名证书">
                <TextArea rows={3} placeholder="PEM 格式证书" />
              </Form.Item>
              <Form.Item name={['sdk_config', 'tls_ca_cert']} label="TLS CA 证书">
                <TextArea rows={3} placeholder="PEM 格式 CA 证书" />
              </Form.Item>
              <Form.Item name={['sdk_config', 'proxy_url']} label="代理 URL">
                <Input placeholder="http://proxy:8080" />
              </Form.Item>

              {/* 节点列表 */}
              <Form.List name={['sdk_config', 'nodes']}>
                {(fields, { add, remove }) => (
                  <>
                    <Typography.Text strong style={{ display: 'block', marginBottom: 8 }}>
                      节点列表
                    </Typography.Text>
                    {fields.map(({ key, name, ...restField }) => (
                      <Space key={key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                        <Form.Item
                          {...restField}
                          name={[name, 'addr']}
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
                name={['sdk_config', 'rpc_url']}
                label="RPC URL"
                rules={[{ required: true, message: '请输入 RPC URL' }]}
              >
                <Input placeholder="https://api.mainnet-beta.solana.com" />
              </Form.Item>
              <Form.Item name={['sdk_config', 'private_key']} label="私钥">
                <Input.Password placeholder="Base58 编码私钥" />
              </Form.Item>
              <Form.Item name={['sdk_config', 'commitment']} label="Commitment Level">
                <Select
                  options={[
                    { label: 'Finalized', value: 'finalized' },
                    { label: 'Confirmed', value: 'confirmed' },
                    { label: 'Processed', value: 'processed' },
                  ]}
                />
              </Form.Item>
              <Form.Item name={['sdk_config', 'skip_preflight']} label="Skip Preflight" valuePropName="checked">
                <Switch />
              </Form.Item>
              <Form.Item name={['sdk_config', 'max_retries']} label="Max Retries">
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
