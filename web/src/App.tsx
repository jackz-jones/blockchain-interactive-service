import { ConfigProvider } from 'antd'
import { BrowserRouter } from 'react-router-dom'
import { theme } from '@/styles/theme'
import AppRouter from '@/router'

function App() {
  return (
    <ConfigProvider theme={theme}>
      <BrowserRouter>
        <AppRouter />
      </BrowserRouter>
    </ConfigProvider>
  )
}

export default App