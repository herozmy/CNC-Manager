import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import { ArrowLeft, ChevronRight, Cpu, FileCode2, FolderKanban, Layers, Pencil, Power, Search, Trash2 } from 'lucide-react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api } from '../api'
import { ApiError } from '../api/client'
import { useAccess } from '../auth/permissions'
import MachineSheet from '../components/MachineSheet'
import MachineStatusBadge from '../components/MachineStatusBadge'
import DeleteSheet from '../components/DeleteSheet'
import { Button } from '../components/ui/Button'
import { Sheet } from '../components/ui/Sheet'
import { EmptyState, ErrorState, Spinner } from '../components/ui/State'
import { useDebounced } from '../lib/useDebounced'
import { VersionBadge } from './DrawingPage'

export default function MachinePage() {
  const access = useAccess()
  const nav = useNavigate()
  const id = Number(useParams().id)
  const valid = Number.isSafeInteger(id) && id > 0
  const qc = useQueryClient()
  const query = useQuery({ queryKey: ['machine', id], queryFn: () => api.machine(id), enabled: valid && access.can('read'), retry: false })
  const [input, setInput] = useState('')
  const q = useDebounced(input.trim())
  const drawings = useInfiniteQuery({ queryKey: ['machine-drawings', id, q], queryFn: ({ pageParam }) => api.machineDrawings(id, q, pageParam), enabled: query.isSuccess && access.can('read'), initialPageParam: 1, getNextPageParam: (last) => last.page * last.size < last.total ? last.page + 1 : undefined, retry: false })
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [targetEnabled, setTargetEnabled] = useState<boolean | null>(null)
  const [busy, setBusy] = useState(false)
  const pending = useRef(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const closeStatus = () => { if (!pending.current) setTargetEnabled(null) }
  async function changeStatus() {
    if (pending.current || targetEnabled === null) return
    pending.current = true; setBusy(true); setError('')
    try {
      const saved = await api.setMachineEnabled(id, targetEnabled)
      qc.setQueryData(['machine', id], saved)
      await qc.invalidateQueries({ predicate: (q) => ['machines', 'drawing-machines'].includes(String(q.queryKey[0])) })
      setNotice(saved.enabled ? '机台已启用，可供工序和程序选用。' : '机台已停用，已有工序及程序历史保留。')
      setTargetEnabled(null)
    } catch (err) { setError(err instanceof ApiError ? err.message : '操作失败，请检查网络后重试') }
    finally { pending.current = false; setBusy(false) }
  }
  if (!access.can('read')) return <main className="p-6">账号不可用，请联系管理员。</main>
  const missing = !valid || (query.error instanceof ApiError && query.error.status === 404)
  const machine = query.data
  return <main className="page-width min-w-0 space-y-4 px-4 pt-4 pb-8 md:px-6">
    <Link to="/machines" className="inline-flex min-h-12 items-center gap-2 rounded-md px-2 text-sm text-muted hover:bg-hover focus-visible:outline-2 focus-visible:outline-accent"><ArrowLeft className="size-4" aria-hidden />返回机台管理</Link>
    {valid && query.isPending && <Spinner />}
    {missing && <EmptyState title="机台不存在或无权访问" />}
    {query.isError && !missing && <ErrorState message="机台加载失败" onRetry={() => query.refetch()} />}
    {machine && !missing && <>
      <header className="panel overflow-hidden">
        <div className="grid gap-5 p-4 md:p-5 lg:grid-cols-[minmax(0,1fr)_minmax(280px,0.7fr)] lg:items-center">
          <div className="min-w-0">
            <div className="flex items-start gap-3">
              <span className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-accent-soft text-accent"><Cpu className="size-6" aria-hidden /></span>
              <div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><h1 className="min-w-0 break-all font-mono text-xl font-semibold">{machine.code}</h1><MachineStatusBadge enabled={machine.enabled} /></div>{machine.name && <p className="mt-1 break-words text-sm text-body">{machine.name}</p>}</div>
            </div>
            <dl className="mt-4 grid min-w-0 gap-3 sm:grid-cols-2">
              <div className="min-w-0"><dt className="text-xs text-muted">控制系统</dt><dd className="mt-1 break-words text-sm">{machine.controller || '未填写'}</dd></div>
              <div className="min-w-0"><dt className="text-xs text-muted">所属账号</dt><dd className="mt-1 break-words text-sm">{machine.ownerName || '待分配'}</dd></div>
            </dl>
          </div>
          <dl aria-label="机台关联统计" className="grid grid-cols-3 divide-x divide-line rounded-lg border border-line bg-canvas py-4 text-center">
            <div className="px-2"><dt className="flex items-center justify-center gap-1 text-xs text-muted"><FolderKanban className="hidden size-3.5 sm:block" aria-hidden />图纸</dt><dd className="mt-2 font-mono text-2xl font-semibold tabular-nums">{machine.drawingCount}</dd></div>
            <div className="px-2"><dt className="flex items-center justify-center gap-1 text-xs text-muted"><Layers className="hidden size-3.5 sm:block" aria-hidden />工序</dt><dd className="mt-2 font-mono text-2xl font-semibold tabular-nums">{machine.operationCount}</dd></div>
            <div className="px-2"><dt className="flex items-center justify-center gap-1 text-xs text-muted"><FileCode2 className="hidden size-3.5 sm:block" aria-hidden />程序</dt><dd className="mt-2 font-mono text-2xl font-semibold tabular-nums">{machine.programCount}</dd></div>
          </dl>
        </div>
        {machine.remark && <div className="mx-4 mb-4 rounded-md bg-inset p-3 md:mx-5"><p className="mb-1 text-xs text-muted">备注</p><p className="whitespace-pre-wrap break-words text-sm text-body">{machine.remark}</p></div>}
        {!machine.enabled && <p className="mx-4 mb-4 rounded-md bg-warning-soft p-3 text-sm text-warning md:mx-5">此机台已停用。已有工序、程序版本和刀具表仍可查看，新关联不能选用。</p>}
        {(access.can('edit') || access.can('delete')) && <div className="flex flex-wrap gap-2 border-t border-line bg-canvas px-4 py-3 md:px-5">
          {access.can('edit') && <><Button variant="secondary" onClick={() => setEditing(true)}><Pencil className="size-4" aria-hidden />编辑机台</Button><Button variant="ghost" onClick={() => { setError(''); setTargetEnabled(!machine.enabled) }}><Power className="size-4" aria-hidden />{machine.enabled ? '停用机台' : '启用机台'}</Button></>}
          {access.can('delete') && <Button variant="ghost" className="text-danger sm:ml-auto" onClick={() => setDeleting(true)}><Trash2 className="size-4" aria-hidden />删除机台</Button>}
        </div>}
      </header>
      {notice && <p role="status" className="rounded-md bg-success-soft p-3 text-sm text-success">{notice}</p>}
      <section aria-label="关联图纸、工序和程序" className="space-y-3">
        <div className="panel space-y-3 p-4">
          <div><h2 className="text-base font-semibold">关联图纸、工序和程序</h2><p className="mt-1 text-xs text-muted">显示匹配图纸的相关工序及独立关联此机台的程序。</p></div>
          <label className="relative block"><span className="sr-only">搜索关联记录</span><Search className="pointer-events-none absolute top-1/2 left-3 size-5 -translate-y-1/2 text-subtle" aria-hidden /><input type="search" value={input} onChange={(e) => setInput(e.target.value)} placeholder="图纸号、类型、材料、工序、程序号或名称" enterKeyHint="search" className="h-12 w-full rounded-md border border-line-strong bg-surface pr-3 pl-10 text-base focus:outline-2 focus:outline-accent" /></label>
        </div>
        {drawings.isPending && <Spinner />}
        {drawings.isError && <ErrorState message="关联记录加载失败" onRetry={() => drawings.refetch()} />}
        {drawings.isSuccess && <p className="text-sm text-muted">{drawings.data.pages[0].total} 张关联图纸</p>}
        {drawings.isSuccess && drawings.data.pages[0].total === 0 && <EmptyState title={q ? '没有匹配的关联记录' : '此机台尚未关联工序或程序'} />}
        {drawings.data?.pages.flatMap((p) => p.items).map((d) => <article key={d.id} className="panel min-w-0 overflow-hidden">
          <h3 className="border-b border-line"><Link to={`/drawings/${d.id}`} className="flex min-h-14 items-center gap-3 px-4 py-3 text-accent hover:bg-accent-soft focus-visible:outline-2 focus-visible:outline-accent"><FolderKanban className="size-5 shrink-0" aria-hidden /><span className="min-w-0 flex-1 break-words text-sm font-semibold"><span className="font-mono">{d.drawingNo}</span>{d.name ? ` · ${d.name}` : ''}</span><ChevronRight className="size-4 shrink-0" aria-hidden /></Link></h3>
          <div className="grid min-w-0 gap-3 p-4 lg:grid-cols-2">{d.operations.map((op) => <section key={op.id} className="min-w-0 rounded-md border border-line bg-canvas p-3">
            <h4 className="flex items-start gap-2 text-sm font-semibold"><Layers className="mt-0.5 size-4 shrink-0 text-muted" aria-hidden /><span className="min-w-0 break-words">{op.opNo}# {op.opName}</span></h4>
            {op.programs.length === 0 && <p className="mt-2 text-sm text-muted">暂无程序</p>}
            <ul className="mt-3 space-y-2">{op.programs.map((p) => <li key={p.id}><Link to={`/drawings/${d.id}/programs/${p.id}`} className="flex min-h-14 items-center gap-2 rounded-md border border-line bg-surface p-3 hover:border-accent focus-visible:outline-2 focus-visible:outline-accent"><FileCode2 className="size-4 shrink-0 text-accent" aria-hidden /><div className="min-w-0 flex-1 break-words"><p className="font-mono text-sm font-medium">{p.programNo}</p>{p.programName && <p className="mt-0.5 text-xs text-muted">{p.programName}</p>}</div><VersionBadge no={p.currentVersionNo} /><ChevronRight className="size-4 shrink-0 text-muted" aria-hidden /></Link></li>)}</ul>
          </section>)}</div>
        </article>)}
        {drawings.hasNextPage && <Button variant="secondary" className="w-full" loading={drawings.isFetchingNextPage} onClick={() => void drawings.fetchNextPage()}>加载更多关联图纸</Button>}
      </section>
      {editing && <MachineSheet machine={machine} onClose={() => setEditing(false)} onSaved={() => { setEditing(false); setNotice('机台信息已保存。') }} />}
      {deleting && <DeleteSheet title="删除机台" message={`将永久删除「${machine.code}」及其机台刀库，并解除相关工序和程序的机台关联。图纸、工序、程序、NC 文件、全部版本及版本刀具表保留。删除无法恢复；若需保留机台及刀库，请使用停用。`} remove={() => api.deleteMachine(id)} onDeleted={() => nav('/machines', { replace: true })} onClose={() => setDeleting(false)} />}
      {targetEnabled !== null && <Sheet open title={targetEnabled ? '启用机台' : '停用机台'} onClose={closeStatus}>
        <p className="mb-4">{targetEnabled ? `启用「${machine.code}」后，工序和程序可以选用此机台。` : `停用「${machine.code}」后，新关联不能选用此机台；已有工序、程序版本和刀具表保留。`}</p>
        {error && <p role="alert" className="mb-3 rounded-md bg-danger-soft p-3 text-sm text-danger">{error}</p>}
        <div className="grid grid-cols-2 gap-3"><Button variant="secondary" disabled={busy} onClick={closeStatus}>取消</Button><Button loading={busy} onClick={() => void changeStatus()}>{targetEnabled ? '确认启用' : '确认停用'}</Button></div>
      </Sheet>}
    </>}
  </main>
}
