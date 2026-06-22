import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, Input, Button, Typography, Space, Alert, Divider, Form } from 'antd'
import { KeyOutlined, CheckCircleOutlined, UserAddOutlined } from '@ant-design/icons'
import { useAuthStore } from '@/stores/auth'
import api from '@/services/api'
import { useApiMessage } from '@/hooks/useApiMessage'
import { useGlobalMessage } from '@/components/GlobalMessage'

const { Title, Paragraph, Text } = Typography

export default function Settings() {
  const navigate = useNavigate()
  const { apiKey, setApiKey, setTenant } = useAuthStore()
  const { message } = useGlobalMessage()
  const { handleApiError } = useApiMessage()
  const [inputKey, setInputKey] = useState(apiKey || '')
  const [loading, setLoading] = useState(false)
  const [registerMode, setRegisterMode] = useState(false)
  const [registerLoading, setRegisterLoading] = useState(false)
  const [registerForm] = Form.useForm()

  // 验证并保存 API Key
  const handleSave = async () => {
    if (!inputKey.trim()) {
      message.warning('请输入 API Key')
      return
    }

    setLoading(true)
    try {
      const response = await api.post('/auth/validate', {
        api_key: inputKey.trim(),
      })
      // 响应拦截器已解包：response.data = body.data = {tenant_id, tenant_name, role, plan}
      const info = response.data as { tenant_id: number; tenant_name: string; role: string }
      setApiKey(inputKey.trim())
      setTenant({
        id: String(info.tenant_id),
        name: info.tenant_name,
        role: (info.role as 'admin' | 'user') || 'admin',
      })
      message.success('API Key 验证成功')
      // 验证成功后自动跳转到概览页
      navigate('/dashboard')
    } catch {
      message.error('API Key 验证失败，请检查输入')
    } finally {
      setLoading(false)
    }
  }

  // 注册新租户
  const handleRegister = async (values: { name: string; email: string; password: string; phone?: string }) => {
    setRegisterLoading(true)
    try {
      const response = await api.post('/auth/register', {
        name: values.name,
        email: values.email,
        password: values.password,
        phone: values.phone,
      })
      // 响应拦截器已解包：response.data = body.data = {api_key, tenant_id, tenant_name, username}
      const info = response.data as { api_key: string; tenant_id: number; tenant_name: string; username: string }

      // 注册成功后自动填充 API Key
      setInputKey(info.api_key)
      setApiKey(info.api_key)
      setTenant({
        id: String(info.tenant_id),
        name: info.tenant_name,
        role: 'admin',
      })
      message.success('注册成功！API Key 已自动配置')
      setRegisterMode(false)
      registerForm.resetFields()
      // 注册成功后自动跳转到概览页
      navigate('/dashboard')
    } catch (err) {
      handleApiError(err)
    } finally {
      setRegisterLoading(false)
    }
  }

  return (
    <div style={{ maxWidth: 560, margin: '0 auto', paddingTop: 40 }}>
      <Title level={3} style={{ marginBottom: 8 }}>
        连接设置
      </Title>
      <Paragraph type="secondary" style={{ marginBottom: 32 }}>
        配置 API Key 以连接到 Chain Interactive Service 后端服务。
      </Paragraph>

      {!apiKey && (
        <Alert
          title="首次使用"
          description="你还没有配置 API Key。请在下方输入已有 Key，或点击「注册新租户」创建一个新账号。"
          type="info"
          showIcon
          style={{ marginBottom: 24 }}
        />
      )}

      {apiKey && (
        <Alert
          title="已连接"
          description={`API Key 已配置，当前租户：${useAuthStore.getState().tenant?.name || '未知'}`}
          type="success"
          icon={<CheckCircleOutlined />}
          showIcon
          style={{ marginBottom: 24 }}
        />
      )}

      {/* 输入已有 API Key */}
      <Card title={<><KeyOutlined style={{ marginRight: 8 }} />使用已有 API Key</>}>
<Space orientation="vertical" size="middle" style={{ width: '100%' }}>
          <div>
            <Text strong style={{ display: 'block', marginBottom: 8 }}>
              API Key
            </Text>
            <Input.Password
              prefix={<KeyOutlined style={{ color: 'var(--color-muted)' }} />}
              placeholder="输入你的 API Key（以 cis_ 开头）"
              value={inputKey}
              onChange={(e) => setInputKey(e.target.value)}
              onPressEnter={handleSave}
              size="large"
            />
          </div>

          <Button
            type="primary"
            onClick={handleSave}
            loading={loading}
            block
            size="large"
          >
            验证并保存
          </Button>
        </Space>
      </Card>

      <Divider style={{ margin: '24px 0' }}>或</Divider>

      {/* 注册新租户 */}
      {!registerMode ? (
        <Card>
<Space orientation="vertical" size="middle" style={{ width: '100%', alignItems: 'center' }}>
            <UserAddOutlined style={{ fontSize: 32, color: 'var(--color-primary)' }} />
            <Text type="secondary">还没有 API Key？创建一个新租户，系统将自动为你生成初始 API Key。</Text>
            <Button
              type="default"
              onClick={() => setRegisterMode(true)}
              block
              size="large"
            >
              注册新租户
            </Button>
          </Space>
        </Card>
      ) : (
        <Card title={<><UserAddOutlined style={{ marginRight: 8 }} />注册新租户</>}>
          <Form
            form={registerForm}
            layout="vertical"
            onFinish={handleRegister}
            initialValues={{ name: '', email: '', password: '' }}
          >
            <Form.Item
              name="name"
              label="租户名称"
              rules={[{ required: true, message: '请输入租户名称' }]}
            >
              <Input placeholder="例如：MyCompany" size="large" />
            </Form.Item>

            <Form.Item
              name="email"
              label="邮箱"
              rules={[
                { required: true, message: '请输入邮箱' },
                { type: 'email', message: '请输入有效的邮箱地址' },
              ]}
            >
              <Input placeholder="admin@example.com" size="large" />
            </Form.Item>

            <Form.Item name="phone" label="手机号（可选）">
              <Input placeholder="138xxxxxxxx" size="large" />
            </Form.Item>

            <Form.Item
              name="password"
              label="管理员密码"
              rules={[
                { required: true, message: '请输入密码' },
                { min: 6, message: '密码至少 6 位' },
              ]}
            >
              <Input.Password placeholder="至少 6 位" size="large" />
            </Form.Item>

            <Space style={{ width: '100%' }}>
              <Button type="primary" htmlType="submit" loading={registerLoading} size="large">
                注册并获取 API Key
              </Button>
              <Button onClick={() => { setRegisterMode(false); registerForm.resetFields() }} size="large">
                取消
              </Button>
            </Space>
          </Form>
        </Card>
      )}
    </div>
  )
}
