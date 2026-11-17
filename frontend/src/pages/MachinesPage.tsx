import { useInfiniteQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { ArrowLeft, ChevronRight, Cpu, Plus, Search } from 'lucide-react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../api'
import { useAccess } from '../auth/permissions'
import MachineSheet from '../components/MachineSheet'
import MachineStatusBadge from '../components/MachineStatusBadge'
import OwnerSelect from '../components/OwnerSelect'
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
  return <main className="page-width min-w-0 space-y-4 px-4 pt-4 pb-8 md:px-6">
    <Link to="/me" className="inline-flex min-h-12 items-center gap-2 rounded-md px-2 text-sm text-muted hover:bg-hover focus-visible:outline-2 focus-visible:outline-accent"><ArrowLeft className="size-4" aria-hidden />返回高级管理</Link>
    <header className="flex flex-wrap items-start justify-between gap-3">
      <div className="flex min-w-0 items-center gap-3">
        <span className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-accent-soft text-accent"><Cpu className="size-6" aria-hidden /></span>
        <div className="min-w-0"><h1 className="text-xl font-semibold">机台管理</h1><p className="mt-1 text-sm text-muted">管理机台，查找关联图纸、工序和程序</p></div>
      </div>
      {access.can('create') && <Button className="shrink-0" onClick={() => setAdding(true)}><Plus className="size-4" aria-hidden />新增机台</Button>}
    </header>
    <div className="panel flex flex-wrap items-end gap-3 p-4">
      <label className="block min-w-0 flex-1 basis-64 space-y-1.5 text-sm font-medium text-body">
        <span>搜索机台</span>
        <span className="relative block"><Search className="pointer-events-none absolute top-1/2 left-3 size-5 -translate-y-1/2 text-subtle" aria-hidden /><input type="search" value={input} onChange={(e) => change('q', e.target.value)} placeholder="机台编号、名称、控制系统" enterKeyHint="search" className="h-12 w-full rounded-md border border-line-strong bg-surface pr-3 pl-10 text-base focus:outline-2 focus:outline-accent" /></span>
      </label>
      <label className="block w-full space-y-1.5 text-sm font-medium text-body sm:w-40">机台状态<select aria-label="机台状态" className="h-12 w-full rounded-md border border-line-strong bg-surface px-3 text-base focus:outline-2 focus:outline-accent" value={status} onChange={(e) => change('status', e.target.value)}><option value="">全部状态</option><option value="enabled">启用</option><option value="disabled">停用</option></select></label>
      {access.admin && <div className="w-full sm:w-48"><OwnerSelect value={ownerUserId ?? ''} onChange={(value) => change('owner', String(value))} includeDisabled includeUnassigned label="筛选所属账号" /></div>}
    </div>
    {query.isPending && <Spinner />}
    {query.isError && <ErrorState message="机台加载失败" onRetry={() => query.refetch()} />}
    {query.isSuccess && <p className="text-sm text-muted">共 {query.data.pages[0].total} 台机台</p>}
    {query.isSuccess && machines.length === 0 && <EmptyState title={q || status || ownerUserId !== undefined ? '没有符合筛选条件的机台' : '还没有机台'} />}
    <ul aria-label="机台台账" className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">{machines.map((m) => <li key={m.id} className="min-w-0"><Link to={`/machines/${m.id}`} className="group flex h-full min-w-0 flex-col rounded-lg border border-line bg-surface p-4 hover:border-accent active:bg-inset focus-visible:outline-2 focus-visible:outline-accent">
      <div className="flex items-start gap-3">
        <span className="flex size-10 shrink-0 items-center justify-center rounded-md bg-inset text-muted"><Cpu className="size-5" aria-hidden /></span>
        <div className="min-w-0 flex-1"><h2 className="break-all font-mono text-sm font-bold">{m.code}</h2><div className="mt-1.5"><MachineStatusBadge enabled={m.enabled} /></div></div>
        <div className="shrink-0 text-right"><p className="font-mono text-2xl font-semibold tabular-nums">{m.programCount}</p><p className="text-xs text-muted">关联程序</p></div>
      </div>
      {m.name && <p className="mt-3 break-words text-sm font-medium">{m.name}</p>}
      {m.controller && <p className="mt-1 break-words text-sm text-muted">控制系统：{m.controller}</p>}
      {access.admin && <p className="mt-1 break-words text-xs text-muted">所属账号：{m.ownerName}</p>}
      <p className="mt-4 flex items-center gap-2 border-t border-line pt-3 text-xs text-muted"><span className="min-w-0 flex-1"><span className="font-mono tabular-nums">{m.drawingCount}</span> 张图纸 · <span className="font-mono tabular-nums">{m.operationCount}</span> 道工序</span><ChevronRight className="size-4 shrink-0 text-accent" aria-hidden /></p>
    </Link></li>)}</ul>
    {query.hasNextPage && <Button variant="secondary" className="w-full" loading={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>加载更多机台</Button>}
    {adding && <MachineSheet onClose={() => setAdding(false)} onSaved={(m) => { setAdding(false); nav(`/machines/${m.id}`) }} />}
  </main>
}
