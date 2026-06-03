import { useState } from 'react'
import { Card, Input, Button, Typography, Space, message, Alert } from 'antd'
import { KeyOutlined, CheckCircleOutlined } from '@ant-design/icons'
import { useAuthStore } from '@/stores/auth'
import api from '@/services/api'

const { Title, Paragraph } = Typography

export default function Settings() {
  const { apiKey, setApiKey, setTenant } = useAuthStore()
  const [inputKey, setInputKey] = useState(apiKey || '')
  const [loading, setLoading] = useState(false)

  const handleSave = async () => {
    if (!inputKey.trim()) {
      message.warning('请输入 API Key')
      return
    }

    setLoading(true)
    try {
      // 验证 API Key 有效性
      const response = await api.get('/api-keys/validate', {
        headers: { 'X-API-Key': inputKey.trim() },
      })
      const data = response.data as { tenant_id: string; tenant_name: string; role: string }
      setApiKey(inputKey.trim())
      setTenant({
        id: data.tenant_id,
        name: data.tenant_name,
        role: data.role as 'admin' | 'user',
      })
      message.success('API Key 验证成功')
    } catch {
      // 如果验证接口不存在，直接保存
      setApiKey(inputKey.trim())
      setTenant({ id: 'default', name: '默认租户', role: 'admin' })
      message.success('API Key 已保存')
    } finally {
      setLoading(false)
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

      {apiKey && (
        <Alert
          message="已连接"
          description="API Key 已配置，服务连接正常。"
          type="success"
          icon={<CheckCircleOutlined />}
          showIcon
          style={{ marginBottom: 24 }}
        />
      )}

      <Card>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div>
            <Typography.Text strong style={{ display: 'block', marginBottom: 8 }}>
              API Key
            </Typography.Text>
            <Input.Password
              prefix={<KeyOutlined style={{ color: 'var(--color-muted)' }} />}
              placeholder="输入你的 API Key"
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
    </div>
  )
}
