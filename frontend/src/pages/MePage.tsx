import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import type { ReactNode } from 'react'
import { Archive, ChevronRight, Factory, UserRound } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { api } from '../api'
import { useAccess } from '../auth/permissions'
import { useSession } from '../auth/session'

function ManagementCard({ to, label, title, description, icon: Icon }: {
  to: string
  label: string
  title: string
  description: ReactNode
  icon: LucideIcon
}) {
  return (
    <Link to={to} aria-label={label} className="panel group flex min-w-0 items-start gap-4 p-4 transition-colors hover:border-accent active:bg-inset focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent sm:min-h-36 md:p-5">
      <span className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-accent-soft text-accent"><Icon className="size-6" aria-hidden /></span>
      <span className="min-w-0 flex-1">
        <span className="block text-base font-semibold text-ink">{title}</span>
        <span className="mt-2 block text-sm leading-6 text-muted">{description}</span>
      </span>
      <ChevronRight className="size-5 shrink-0 self-center text-subtle group-hover:text-accent" aria-hidden />
    </Link>
  )
}

export default function MePage() {
  const access = useAccess()
  const session = useSession().data
  const meta = useQuery({ queryKey: ['meta'], queryFn: api.meta, staleTime: Infinity })
  if (session?.state !== 'in') return null
  const { user } = session

  return (
    <main className="page-width min-w-0 space-y-6 px-4 pt-4 pb-8 md:px-6">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-ink">我的</h1>
          <p className="mt-1 text-sm text-muted">账号信息与管理入口</p>
        </div>
        {meta.data && <p className="rounded-md border border-line bg-surface px-3 py-2 text-xs text-muted">版本 {meta.data.version}</p>}
      </header>
      <div className="grid min-w-0 gap-4">
        <ManagementCard to="/me/account" label="账号管理" title="账号管理" icon={UserRound} description={<>
          <span className="block font-medium text-body">{user.displayName || user.username}</span>
          <span className="block">{user.username} · {user.role === 'admin' ? '管理员' : '普通账号'}</span>
        </>} />
      </div>
      <section aria-labelledby="management-heading" className="min-w-0 space-y-3">
        <h2 id="management-heading" className="text-sm font-semibold text-body">管理入口</h2>
        <div className={`grid min-w-0 gap-4 ${access.admin ? 'sm:grid-cols-2' : ''}`}>
          {access.can('read') && <ManagementCard to="/machines" label="机台管理" title="机台" description="管理机台，查看关联图纸和程序" icon={Factory} />}
          {access.admin ? <ManagementCard to="/me/backup" label="备份与迁移" title="备份与迁移" description="创建备份，导入迁移和恢复数据" icon={Archive} /> : <p className="text-sm leading-6 text-muted">整站备份与导入请联系管理员。</p>}
        </div>
      </section>
    </main>
  )
}
