import { useQuery } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { User } from '../api/types'

export type Session =
  | { state: 'setup' } // 还没有任何用户，需要创建管理员
  | { state: 'out'; notice?: string; username?: string } // 未登录
  | { state: 'in'; user: User }

export const SESSION_KEY = ['session'] as const

async function loadSession(): Promise<Session> {
  const status = await api.authStatus()
  if (status.setupRequired) return { state: 'setup' }
  try {
    return { state: 'in', user: await api.me() }
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return { state: 'out' }
    throw e
  }
}

export function useSession() {
  return useQuery({ queryKey: SESSION_KEY, queryFn: loadSession, retry: false, staleTime: Infinity })
}

/**
 * 切换登录状态（登录、退出、会话过期）。
 *
 * 只清掉除会话以外的缓存，不能用 qc.clear()：clear 会把正在被组件观察的会话查询
 * 从缓存里摘掉，随后 setQueryData 写入的是一个没人观察的新查询，界面不会更新。
 */
export function setSession(qc: QueryClient, session: Session) {
  qc.removeQueries({ predicate: (q) => q.queryKey[0] !== SESSION_KEY[0] })
  qc.setQueryData(SESSION_KEY, session)
}
