import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ConfigProvider, App as AntApp } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import './index.css'
import './lib/time' // 初始化 dayjs 插件/时区/locale
import App from './App.tsx'
import { AuthProvider } from './lib/auth'

async function bootstrap() {
  // mock-first：默认启用 MSW；切真接口联调时 .env 设 VITE_USE_MSW=false。
  if (import.meta.env.VITE_USE_MSW !== 'false') {
    const { startMockWorker } = await import('./mocks/browser')
    await startMockWorker()
  }

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <ConfigProvider locale={zhCN} theme={{ token: { colorPrimary: '#7c3aed' } }}>
        <AntApp>
          <AuthProvider>
            <App />
          </AuthProvider>
        </AntApp>
      </ConfigProvider>
    </StrictMode>,
  )
}

void bootstrap()
