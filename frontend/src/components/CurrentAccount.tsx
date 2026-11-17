import { useState } from 'react'
import type { ReactNode } from 'react'
import { UserRound } from 'lucide-react'
import { useSession } from '../auth/session'
import AccountSheet from './AccountSheet'
import { Button } from './ui/Button'

export default function CurrentAccount({ children }: { children?: ReactNode }) {
  const session = useSession().data
  const user = session?.state === 'in' ? session.user : undefined
  const [sheet, setSheet] = useState<'name' | 'password' | null>(null)
  const [saved, setSaved] = useState(false)
  if (!user) return null

  return <section aria-labelledby="account-heading" className="panel min-w-0 p-4 md:p-5">
    <div className="flex min-w-0 flex-col gap-5 xl:flex-row xl:items-center xl:justify-between">
      <div className="min-w-0 flex-1">
        <h2 id="account-heading" className="text-sm font-semibold text-body">当前账号</h2>
        <div className="mt-4 flex items-start gap-3">
          <span className="flex size-12 shrink-0 items-center justify-center rounded-xl bg-accent-soft text-accent"><UserRound className="size-6" aria-hidden /></span>
          <div className="min-w-0 flex-1">
            <p className="break-words text-lg font-semibold leading-7 text-ink">{user.displayName || user.username}</p>
            <p className="mt-1 break-words text-sm leading-6 text-muted">{user.username} · {user.role === 'admin' ? '管理员' : '普通账号'}</p>
          </div>
        </div>
      </div>
      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" onClick={() => { setSaved(false); setSheet('name') }}>修改名称</Button>
        <Button variant="secondary" onClick={() => { setSaved(false); setSheet('password') }}>修改密码</Button>
        {children}
      </div>
    </div>
    {saved && <p role="status" className="mt-4 rounded-md bg-success-soft p-3 text-sm text-success">名称已保存</p>}
    {sheet && <AccountSheet kind={sheet} user={user} onClose={() => setSheet(null)} onSaved={() => { setSheet(null); setSaved(true) }} />}
  </section>
}
