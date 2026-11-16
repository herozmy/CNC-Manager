import { useInfiniteQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { ArrowLeft } from 'lucide-react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../api'
import { useAccess } from '../auth/permissions'
import MachineSheet from '../components/MachineSheet'
import OwnerSelect from '../components/OwnerSelect'
import { Field } from '../components/ui/Field'
import { Button } from '../components/ui/Button'
import { EmptyState, ErrorState, Spinner } from '../components/ui/State'
import { useDebounced } from '../lib/useDebounced'

export default function MachinesPage() {
  const access = useAccess()
  const nav = useNavigate()
  const [params, setParams] = useSearchParams()
  const input = params.get('q') ?? ''
  const status = params.get('status') ?? ''
  const ownerParam = params.get('owner') ?? ''
  const ownerUserId = /^\d+$/.test(ownerParam) ? Number(ownerParam) : undefined
  const change = (key: string, value: string) => setParams((previous) => { const next = new URLSearchParams(previous); if (value) next.set(key, value); else next.delete(key); return next }, { replace: true })
  const q = useDebounced(input.trim())
  const enabled = status === 'enabled' ? true : status === 'disabled' ? false : undefined
  const [adding, setAdding] = useState(false)
  const query = useInfiniteQuery({
    queryKey: ['machines', q, enabled, access.admin ? ownerUserId : undefined],
    queryFn: ({ pageParam }) => api.machines({ q, page: pageParam, size: 20, enabled, ownerUserId: access.admin ? ownerUserId : undefined }),
    initialPageParam: 1,
    enabled: access.can('read'),
    getNextPageParam: (last) => last.page * last.size < last.total ? last.page + 1 : undefined,
  })
  if (!access.can('read')) return <main className="p-6">账号不可用，请联系管理员。</main>
  const machines = query.data?.pages.flatMap((page) => page.items) ?? []
  return <main className="mx-auto max-w-5xl space-y-4 px-4 pt-4 pb-8">
    <Link to="/me" className="inline-flex min-h-12 items-center gap-2 rounded-md px-2 text-sm text-muted hover:bg-hover"><ArrowLeft className="size-5" aria-hidden />返回我的</Link>
    <header className="flex items-start justify-between gap-3"><div><h1 className="text-xl font-semibold">机台管理</h1><p className="mt-1 text-sm text-muted">管理机台，查找关联图纸、工序和程序</p></div>{access.can('create') && <Button className="shrink-0" onClick={() => setAdding(true)}>新增机台</Button>}</header>
    <Field type="search" label="搜索机台" value={input} onChange={(e) => change('q', e.target.value)} placeholder="机台编号、名称、控制系统" />
    <div className="grid gap-3 sm:grid-cols-2">
      <label className="block space-y-1.5 text-sm font-medium text-body">机台状态<select aria-label="机台状态" className="h-12 w-full rounded-md border border-line-strong bg-surface px-3 text-base" value={status} onChange={(e) => change('status', e.target.value)}><option value="">全部状态</option><option value="enabled">启用</option><option value="disabled">停用</option></select></label>
      {access.admin && <OwnerSelect value={ownerUserId ?? ''} onChange={(value) => change('owner', String(value))} includeDisabled includeUnassigned label="筛选所属账号" />}
    </div>
    {query.isPending && <Spinner />}
    {query.isError && <ErrorState message="机台加载失败" onRetry={() => query.refetch()} />}
    {query.isSuccess && <p className="text-sm text-muted">共 {query.data.pages[0].total} 台机台</p>}
    {query.isSuccess && machines.length === 0 && <EmptyState title={q || status || ownerUserId !== undefined ? '没有符合筛选条件的机台' : '还没有机台'} />}
    <ul className="grid grid-cols-1 gap-3 md:grid-cols-2">{machines.map((m) => <li key={m.id}><Link to={`/machines/${m.id}`} className="block rounded-lg border border-line bg-surface p-4 shadow-sm hover:border-accent">
      <div className="flex items-start justify-between gap-3"><h2 className="min-w-0 break-words text-lg font-bold">{m.code}</h2><span className={`shrink-0 rounded-full px-3 py-1 text-xs ${m.enabled ? 'bg-success-soft text-success' : 'bg-inset text-muted'}`}>{m.enabled ? '启用' : '已停用'}</span></div>
      {m.name && <p className="mt-1 break-words">{m.name}</p>}{m.controller && <p className="mt-1 text-sm text-muted">控制系统：{m.controller}</p>}
      {access.admin && <p className="mt-1 text-sm text-muted">所属账号：{m.ownerName}</p>}
      <p className="mt-3 text-sm text-muted">{m.drawingCount} 张图纸 · {m.operationCount} 道工序 · {m.programCount} 个程序</p>
    </Link></li>)}</ul>
    {query.hasNextPage && <Button variant="secondary" className="w-full" loading={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>加载更多机台</Button>}
    {adding && <MachineSheet onClose={() => setAdding(false)} onSaved={(m) => { setAdding(false); nav(`/machines/${m.id}`) }} />}
  </main>
}
