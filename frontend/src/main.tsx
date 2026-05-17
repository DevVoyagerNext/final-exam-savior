import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ConfigProvider, App as AntdApp, theme } from 'antd'
import zhCN from 'antd/locale/zh_CN'

import { AuthProvider } from './auth.tsx'
import './index.css'
import App from './App.tsx'

const queryClient = new QueryClient()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ConfigProvider
      locale={zhCN}
      form={{
        validateMessages: {
          required: '请输入${label}',
          types: {
            email: '${label}格式不正确',
          },
          string: {
            min: '${label}至少需要${min}个字符',
          },
        },
      }}
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          colorPrimary: '#7c4dff',
          borderRadius: 14,
        },
      }}
    >
      <AntdApp>
        <QueryClientProvider client={queryClient}>
          <AuthProvider>
            <App />
          </AuthProvider>
        </QueryClientProvider>
      </AntdApp>
    </ConfigProvider>
  </StrictMode>,
)
