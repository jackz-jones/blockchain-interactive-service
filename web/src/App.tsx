import { ConfigProvider } from 'antd'
import { BrowserRouter } from 'react-router-dom'
import { theme } from '@/styles/theme'
import AppRouter from '@/router'
import { GlobalMessageProvider } from '@/components/GlobalMessage'

function App() {
  return (
    <ConfigProvider theme={theme}>
      <GlobalMessageProvider>
        <BrowserRouter>
          <AppRouter />
        </BrowserRouter>
      </GlobalMessageProvider>
    </ConfigProvider>
  )
}

export default App