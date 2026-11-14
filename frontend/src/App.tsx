import { useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { UNAUTHORIZED_EVENT } from './api/client'
import { setSession, useSession } from './auth/session'
import { ErrorState, Spinner } from './components/ui/State'
import AppShell from './layout/AppShell'
import AuthPage from './pages/AuthPage'
import DrawingPage from './pages/DrawingPage'
import MePage from './pages/MePage'
import NewDrawingPage from './pages/NewDrawingPage'
import OverviewPage from './pages/OverviewPage'
import ProgramPage from './pages/ProgramPage'

export default function App() {
  const qc = useQueryClient()
  const session = useSession()

  // 任何接口返回 401（会话过期）都回到登录页
  useEffect(() => {
    const onUnauthorized = () => setSession(qc, { state: 'out' })
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
  }, [qc])

  if (session.isPending) return <Spinner />
  if (session.isError) return <ErrorState message="无法连接服务器" onRetry={() => session.refetch()} />
  if (session.data.state !== 'in') return <AuthPage setup={session.data.state === 'setup'} />

  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<OverviewPage />} />
        <Route path="drawings/new" element={<NewDrawingPage />} />
        <Route path="drawings/:id" element={<DrawingPage />} />
        <Route path="drawings/:id/programs/:programId" element={<ProgramPage />} />
        <Route path="me" element={<MePage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}
