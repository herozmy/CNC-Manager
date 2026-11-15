import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { FORBIDDEN_EVENT, UNAUTHORIZED_EVENT } from './api/client'
import { authorizationKey, clearBusinessCache, SESSION_KEY, setSession, useSession } from './auth/session'
import type { Session } from './auth/session'
import { ErrorState, Spinner } from './components/ui/State'
import { api } from './api'
import AdminPage from './pages/AdminPage'
import AuditPage from './pages/AuditPage'
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
  const location = useLocation()
  const [checking, setChecking] = useState(false)
  const [denied, setDenied] = useState<{ locationKey: string; authorization: string } | null>(null)

  // 任何接口返回 401（会话过期）都回到登录页
  useEffect(() => {
    const onUnauthorized = () => { setDenied(null); setSession(qc, { state: 'out' }) }
    let refreshing = false
    const onForbidden = () => {
      if (refreshing) return
      refreshing = true
      const previous = authorizationKey(qc.getQueryData<Session>(SESSION_KEY))
      clearBusinessCache(qc)
      setChecking(true)
      void api.me().then((user) => {
        const next: Session = { state: 'in', user }
        // An unchanged identity cannot fix a persistent denial. Wait for an
        // explicit retry/navigation instead of remounting the rejected query.
        setDenied(authorizationKey(next) === previous ? { locationKey: location.key, authorization: previous } : null)
        setSession(qc, next)
      }).catch((err: unknown) => {
        if ((err instanceof Error || err instanceof DOMException) && err.name === 'AbortError') return
        setSession(qc, { state: 'out', notice: '权限已改变，请重新登录或联系管理员。' })
      }).finally(() => {
        refreshing = false; setChecking(false)
      })
    }
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    window.addEventListener(FORBIDDEN_EVENT, onForbidden)
    return () => { window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized); window.removeEventListener(FORBIDDEN_EVENT, onForbidden) }
  }, [qc, location.key])

  // Inline tool drafts survive internal navigation; warn before losing the login's in-memory drafts.
  useEffect(() => {
    const warn = (e: BeforeUnloadEvent) => {
      if (!qc.getQueriesData<{ dirty: boolean }>({ queryKey: ['version-tools-draft'] }).some(([, draft]) => draft?.dirty)) return
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [qc])

  if (checking || session.isPending) return <Spinner />
  if (session.isError) return <ErrorState message="无法连接服务器" onRetry={() => session.refetch()} />
  if (session.data.state !== 'in') return <AuthPage setup={session.data.state === 'setup'} notice={session.data.state === 'out' ? session.data.notice : undefined} initialUsername={session.data.state === 'out' ? session.data.username : undefined} />
  if (denied?.locationKey === location.key && denied.authorization === authorizationKey(session.data)) return (
    <Routes><Route element={<AppShell />}><Route path="*" element={<ErrorState message="没有此操作的权限，请联系管理员。" onRetry={() => setDenied(null)} />} /></Route></Routes>
  )

  return (
    <Routes key={authorizationKey(session.data)}>
      <Route element={<AppShell />}>
        <Route index element={<OverviewPage />} />
        <Route path="drawings/new" element={<NewDrawingPage />} />
        <Route path="drawings/:id" element={<DrawingPage />} />
        <Route path="drawings/:id/programs/:programId" element={<ProgramPage />} />
        <Route path="me" element={<MePage />} />
        <Route path="admin" element={<AdminPage />} />
        <Route path="admin/audit" element={<AuditPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}
