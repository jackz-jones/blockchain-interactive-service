import { useEffect, useState } from 'react'
import { Form, Input, Select, Switch, Button, Card, Typography, Space, message, Divider } from 'antd'
import { useNavigate, useParams } from 'react-router-dom'
import Editor from '@monaco-editor/react'
import api from '@/services/api'

const { Title } = Typography

interface ChainOption {
  id: string
  chain_name: string
}

export default function ContractConfigForm() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [form] = Form.useForm()
  const [loading, setLoading] = useState(false)
  const [chains, setChains] = useState<ChainOption[]>([])
  const [abiValue, setAbiValue] = useState('')
  const isEdit = !!id

  useEffect(() => {
    fetchChains()
    if (isEdit) {
      fetchDetail()
    }
  }, [id])

  const fetchChains = async () => {
    try {
      const res = await api.get('/chain-configs')
      setChains((res.data as { items: ChainOption[] }).items || [])
    } catch {
      // 错误已由拦截器处理
    }
  }

  const fetchDetail = async () => {
    try {
      const res = await api.get(`/contract-configs/${id}`)
      const data = res.data as Record<string, unknown>
      form.setFieldsValue(data)
      if (data.abi) {
        setAbiValue(typeof data.abi === 'string' ? data.abi : JSON.stringify(data.abi, null, 2))
      }
    } catch {
      // 错误已由拦截器处理
    }
  }

  const handleSubmit = async (values: Record<string, unknown>) => {
    // 将 ABI 字符串解析为 JSON
    let parsedAbi = null
    if (abiValue.trim()) {
      try {
        parsedAbi = JSON.parse(abiValue)
      } catch {
        message.error('ABI JSON 格式不正确')
        return
      }
    }

    const payload = { ...values, abi: parsedAbi }
    setLoading(true)
    try {
      if (isEdit) {
        await api.put(`/contract-configs/${id}`, payload)
        message.success('更新成功')
      } else {
        await api.post('/contract-configs', payload)
        message.success('创建成功')
      }
      navigate('/contract-configs')
    } catch {
      // 错误已由拦截器处理
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
          initialValues={{ subscribe_enabled: false }}
        >
          <Form.Item
            name="chain_config_id"
            label="所属链"
            rules={[{ required: true, message: '请选择所属链' }]}
          >
            <Select
              placeholder="选择链配置"
              options={chains.map((c) => ({ label: c.chain_name, value: c.id }))}
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
            name="contract_address"
            label="合约地址"
            rules={[{ required: true, message: '请输入合约地址' }]}
          >
            <Input placeholder="0x..." style={{ fontFamily: 'var(--font-mono, monospace)' }} />
          </Form.Item>

          <Form.Item label="ABI (JSON)">
            <div style={{ border: '1px solid #d9d9d9', borderRadius: 6, overflow: 'hidden' }}>
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

          <Form.Item name="subscribe_enabled" label="开启事件订阅" valuePropName="checked">
            <Switch />
          </Form.Item>

          <Form.Item name="extend_config" label="扩展配置">
            <Input.TextArea rows={3} placeholder="JSON 格式扩展配置（可选）" />
          </Form.Item>

          <Divider />

          <Space>
            <Button type="primary" htmlType="submit" loading={loading}>
              {isEdit ? '保存修改' : '创建'}
            </Button>
            <Button onClick={() => navigate('/contract-configs')}>取消</Button>
          </Space>
        </Form>
      </Card>
    </div>
  )
}
