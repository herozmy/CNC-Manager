import { useQuery, useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { api } from '../api'
import { ApiError, cancelAuthRequests, cancelBusinessRequests } from '../api/client'
import type { User } from '../api/types'

export type Session =
  | { state: 'setup' } // 还没有任何用户，需要创建管理员
  | { state: 'out'; notice?: string; username?: string } // 未登录
  | { state: 'in'; user: User }

export const SESSION_KEY = ['session'] as const

export function authorizationKey(session?: Session): string {
  return session?.state === 'in' ? JSON.stringify([session.user.id, session.user.role, session.user.enabled]) : session?.state ?? 'unknown'
}

export function clearBusinessCache(qc: QueryClient) {
  cancelBusinessRequests()
  void qc.cancelQueries({ predicate: (q) => q.queryKey[0] !== SESSION_KEY[0] })
  qc.removeQueries({ predicate: (q) => q.queryKey[0] !== SESSION_KEY[0] })
}

async function loadSession(qc: QueryClient): Promise<Session> {
  const status = await api.authStatus()
  if (status.setupRequired) return { state: 'setup' }
  try {
    const next: Session = { state: 'in', user: await api.me() }
    if (authorizationKey(next) !== authorizationKey(qc.getQueryData<Session>(SESSION_KEY))) {
      cancelAuthRequests()
      clearBusinessCache(qc)
    }
    return next
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return { state: 'out' }
    throw e
  }
}

export function useSession() {
  const qc = useQueryClient()
  return useQuery({ queryKey: SESSION_KEY, queryFn: () => loadSession(qc), retry: false, staleTime: 30_000, refetchInterval: 30_000, refetchOnWindowFocus: 'always' })
}

/**
 * 切换登录状态（登录、退出、会话过期）。
 *
 * 只清掉除会话以外的缓存，不能用 qc.clear()：clear 会把正在被组件观察的会话查询
 * 从缓存里摘掉，随后 setQueryData 写入的是一个没人观察的新查询，界面不会更新。
 */
export function setSession(qc: QueryClient, session: Session) {
  void qc.cancelQueries({ queryKey: SESSION_KEY, exact: true })
  cancelAuthRequests()
  clearBusinessCache(qc)
  qc.setQueryData(SESSION_KEY, session)
}
