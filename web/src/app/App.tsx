import { useEffect } from 'react'
import { QueryClient, QueryClientProvider, useQuery, useQueryClient } from '@tanstack/react-query'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { api } from '../api/client'
import { ConnectionError, LoadingPage } from '../components/Feedback'
import { AuthPage } from '../features/auth/AuthPage'
import { DashboardPage } from '../features/dashboard/DashboardPage'
import { MonitoringPage } from '../features/monitoring/MonitoringPage'
import { AddonsPage } from '../features/addons/AddonsPage'
import { HomeLayoutPage } from '../features/layout/HomeLayoutPage'
import { SettingsPage } from '../features/settings/SettingsPage'
import { ImportPage } from '../features/imports/ImportPage'
import { AppShell } from './AppShell'

const client = new QueryClient({ defaultOptions: { queries: { staleTime: 30_000, retry: false, refetchOnWindowFocus: true }, mutations: { retry: false } } })

export function useAuth() { return useQuery({ queryKey: ['auth'], queryFn: api.me }) }

function Bootstrap() {
  const qc = useQueryClient()
  const setup = useQuery({ queryKey: ['setup'], queryFn: api.setupStatus })
  const auth = useAuth()
  useEffect(() => {
    const onUnauthorized = () => {
      qc.setQueryData(['auth'], null)
      qc.removeQueries({ predicate: query => !['auth', 'setup'].includes(String(query.queryKey[0])) })
    }
    window.addEventListener('dashboard:unauthorized', onUnauthorized)
    return () => window.removeEventListener('dashboard:unauthorized', onUnauthorized)
  }, [qc])
  if (setup.isPending || auth.isPending) return <LoadingPage />
  if (setup.isError || auth.isError) return <ConnectionError error={setup.error ?? auth.error} onRetry={() => { void setup.refetch(); void auth.refetch() }} />
  const authenticated = !!auth.data
  const needsSetup = !!setup.data.needsSetup
  return <Routes>
    <Route path="/setup" element={authenticated ? <Navigate to="/" replace /> : needsSetup ? <AuthPage setup /> : <Navigate to="/login" replace />} />
    <Route path="/login" element={needsSetup ? <Navigate to="/setup" replace /> : authenticated ? <Navigate to="/" replace /> : <AuthPage />} />
    <Route element={needsSetup ? <Navigate to="/setup" replace /> : authenticated ? <AppShell /> : <Navigate to="/login" replace />}>
      <Route index element={<DashboardPage />} />
      <Route path="imports/new" element={<Navigate to="/settings/imports" replace />} />
      <Route path="connections" element={<MonitoringPage kind="docker" />} />
      <Route path="hosts" element={<MonitoringPage kind="host" />} />
      <Route path="settings" element={<SettingsPage />} />
      <Route path="settings/addons" element={<AddonsPage />} />
      <Route path="settings/layout" element={<HomeLayoutPage />} />
      <Route path="settings/services" element={<DashboardPage manage />} />
      <Route path="settings/imports" element={<ImportPage />} />
      <Route path="*" element={<NotFound />} />
    </Route>
  </Routes>
}

function NotFound() { return <div className="page-heading"><h1>페이지를 찾을 수 없습니다</h1><a href="/">대시보드로 돌아가기</a></div> }

export function App() {
  return <QueryClientProvider client={client}><BrowserRouter><Bootstrap /></BrowserRouter></QueryClientProvider>
}
