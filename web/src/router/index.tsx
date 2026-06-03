import { Routes, Route, Navigate } from 'react-router-dom'
import AppLayout from '@/components/layout/AppLayout'
import { useAuthStore } from '@/stores/auth'

// 页面懒加载
import { lazy, Suspense } from 'react'
import PageLoading from '@/components/common/PageLoading'

const Dashboard = lazy(() => import('@/pages/dashboard'))
const ChainConfigList = lazy(() => import('@/pages/chain-config'))
const ChainConfigForm = lazy(() => import('@/pages/chain-config/ChainConfigForm'))
const ContractConfigList = lazy(() => import('@/pages/contract-config'))
const ContractConfigForm = lazy(() => import('@/pages/contract-config/ContractConfigForm'))
const ContractCall = lazy(() => import('@/pages/contract-call'))
const TxQuery = lazy(() => import('@/pages/tx-query'))
const EventSubscription = lazy(() => import('@/pages/event-subscription'))
const ApiKeyList = lazy(() => import('@/pages/api-key'))
const TenantList = lazy(() => import('@/pages/tenant'))
const UserList = lazy(() => import('@/pages/user'))
const CallLogs = lazy(() => import('@/pages/call-logs'))
const AuditLogs = lazy(() => import('@/pages/audit-logs'))
const UsageStats = lazy(() => import('@/pages/usage-stats'))
const Bills = lazy(() => import('@/pages/bills'))
const Settings = lazy(() => import('@/pages/settings'))

/** 路由守卫：未认证时重定向到设置页 */
function RequireAuth({ children }: { children: React.ReactNode }) {
  const { apiKey } = useAuthStore()
  if (!apiKey) {
    return <Navigate to="/settings" replace />
  }
  return <>{children}</>
}

function AppRouter() {
  return (
    <Suspense fallback={<PageLoading />}>
      <Routes>
        {/* 设置页不需要认证 */}
        <Route path="/settings" element={<AppLayout />}>
          <Route index element={<Settings />} />
        </Route>

        {/* 需要认证的路由 */}
        <Route
          path="/"
          element={
            <RequireAuth>
              <AppLayout />
            </RequireAuth>
          }
        >
          <Route index element={<Navigate to="/dashboard" replace />} />
          <Route path="dashboard" element={<Dashboard />} />

          {/* 链配置 */}
          <Route path="chain-configs" element={<ChainConfigList />} />
          <Route path="chain-configs/create" element={<ChainConfigForm />} />
          <Route path="chain-configs/:id/edit" element={<ChainConfigForm />} />

          {/* 合约配置 */}
          <Route path="contract-configs" element={<ContractConfigList />} />
          <Route path="contract-configs/create" element={<ContractConfigForm />} />
          <Route path="contract-configs/:id/edit" element={<ContractConfigForm />} />

          {/* 合约交互 */}
          <Route path="contract-call" element={<ContractCall />} />
          <Route path="tx-query" element={<TxQuery />} />

          {/* 事件订阅 */}
          <Route path="event-subscriptions" element={<EventSubscription />} />

          {/* 管理 */}
          <Route path="api-keys" element={<ApiKeyList />} />
          <Route path="tenants" element={<TenantList />} />
          <Route path="users" element={<UserList />} />

          {/* 日志与统计 */}
          <Route path="call-logs" element={<CallLogs />} />
          <Route path="audit-logs" element={<AuditLogs />} />
          <Route path="usage-stats" element={<UsageStats />} />
          <Route path="bills" element={<Bills />} />
        </Route>

        {/* 404 */}
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </Suspense>
  )
}

export default AppRouter
