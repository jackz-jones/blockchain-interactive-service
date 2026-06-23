import { Button, Result } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'

interface ErrorRetryProps {
  error?: unknown
  onRetry?: () => void
  title?: string
  subTitle?: string
}

/**
 * 错误重试组件
 * 用于数据加载失败时显示错误提示和重试按钮
 * 替换各页面的加载失败空白态
 */
export default function ErrorRetry({ error, onRetry, title, subTitle }: ErrorRetryProps) {
  const errorTitle = title || '加载失败'
  const errorSubTitle = subTitle || (error instanceof Error ? error.message : '数据加载出现问题，请稍后重试')

  return (
    <Result
      status="error"
      title={errorTitle}
      subTitle={errorSubTitle}
      extra={
        onRetry ? (
          <Button
            type="primary"
            icon={<ReloadOutlined />}
            onClick={onRetry}
          >
            重新加载
          </Button>
        ) : undefined
      }
    />
  )
}
