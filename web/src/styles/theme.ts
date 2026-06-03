import type { ThemeConfig } from 'antd'

/**
 * Ant Design 主题配置
 * 基于 DESIGN.md 中定义的设计系统 token
 */
export const theme: ThemeConfig = {
  token: {
    // 品牌色
    colorPrimary: '#4a7c59',
    colorSuccess: '#3d8b5e',
    colorWarning: '#b8860b',
    colorError: '#c0392b',
    colorInfo: '#2980b9',

    // 字体
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif",
    fontFamilyCode: "'JetBrains Mono', 'Fira Code', 'SF Mono', monospace",
    fontSize: 14,

    // 圆角
    borderRadius: 6,
    borderRadiusSM: 4,
    borderRadiusLG: 8,

    // 间距
    marginXS: 4,
    marginSM: 8,
    margin: 16,
    marginMD: 20,
    marginLG: 24,
    marginXL: 32,

    // 线条
    colorBorder: '#e5e5e5',
    colorBorderSecondary: '#f0f0f0',

    // 背景
    colorBgContainer: '#ffffff',
    colorBgLayout: '#ffffff',
    colorBgElevated: '#ffffff',

    // 文字
    colorText: '#1a1a2e',
    colorTextSecondary: '#6b6b80',
    colorTextTertiary: '#8c8c9e',
    colorTextQuaternary: '#bfbfcf',
  },
  components: {
    Layout: {
      siderBg: '#1e1e2e',
      headerBg: '#ffffff',
      bodyBg: '#ffffff',
    },
    Menu: {
      darkItemBg: '#1e1e2e',
      darkItemColor: '#d4d4d4',
      darkItemHoverColor: '#ffffff',
      darkItemSelectedBg: 'rgba(74, 124, 89, 0.2)',
      darkItemSelectedColor: '#7cb88c',
    },
    Table: {
      headerBg: '#fafafa',
      headerColor: '#6b6b80',
      rowHoverBg: '#f8f9fa',
    },
    Button: {
      primaryShadow: 'none',
      defaultShadow: 'none',
    },
    Card: {
      paddingLG: 20,
    },
  },
}
