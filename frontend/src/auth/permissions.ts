import { skipToken, useQuery } from '@tanstack/react-query'
import type { Permission, User } from '../api/types'
import { SESSION_KEY, type Session } from './session'

export const permissionLabels: Record<Permission, string> = { read: '查看', download: '下载', create: '新建', edit: '编辑', delete: '删除' }
export function can(user: User | undefined, permission: Permission): boolean {
  return !!user?.enabled && (user.role === 'admin' || user.role === 'editor') && Object.hasOwn(permissionLabels, permission)
}
export function useAccess() {
  const { data } = useQuery<Session>({ queryKey: SESSION_KEY, queryFn: skipToken })
  const user = data?.state === 'in' ? data.user : undefined
  return { user, admin: !!user?.enabled && user.role === 'admin', can: (permission: Permission) => can(user, permission) }
}
