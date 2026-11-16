import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api } from '../api'
import { ApiError } from '../api/client'
import { useAccess } from '../auth/permissions'
import MachineSheet from '../components/MachineSheet'
import DeleteSheet from '../components/DeleteSheet'
import { Button } from '../components/ui/Button'
import { Field } from '../components/ui/Field'
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
  return <main className="mx-auto max-w-3xl space-y-4 px-4 pt-4 pb-8">
    <Link to="/machines" className="inline-flex min-h-12 items-center text-sm text-muted underline">返回机台管理</Link>
    {valid && query.isPending && <Spinner />}
    {missing && <EmptyState title="机台不存在或无权访问" />}
    {query.isError && !missing && <ErrorState message="机台加载失败" onRetry={() => query.refetch()} />}
    {machine && !missing && <>
      <header className="space-y-3 rounded-lg border border-line bg-surface p-4">
        <div className="flex items-start justify-between gap-3"><h1 className="min-w-0 break-words text-2xl font-bold">{machine.code}</h1><span className="shrink-0 rounded-full bg-inset px-3 py-1 text-sm">{machine.enabled ? '启用' : '已停用'}</span></div>
        {machine.name && <p className="break-words text-lg">{machine.name}</p>}
        <p className="text-sm text-muted">所属账号：{machine.ownerName}</p>
        {machine.controller && <p className="text-sm">控制系统：{machine.controller}</p>}
        {machine.remark && <p className="whitespace-pre-wrap break-words text-sm text-muted">{machine.remark}</p>}
        <p className="text-sm text-muted">{machine.drawingCount} 张图纸 · {machine.operationCount} 道工序 · {machine.programCount} 个程序</p>
        {!machine.enabled && <p className="rounded-md bg-warning-soft p-3 text-sm text-warning">此机台已停用。已有工序、程序版本和刀具表仍可查看，新关联不能选用。</p>}
        {access.can('edit') && <div className="flex flex-wrap gap-2"><Button variant="secondary" onClick={() => setEditing(true)}>编辑机台</Button><Button variant="ghost" onClick={() => { setError(''); setTargetEnabled(!machine.enabled) }}>{machine.enabled ? '停用机台' : '启用机台'}</Button></div>}
        {access.can('delete') && <Button variant="ghost" className="text-danger" onClick={() => setDeleting(true)}>删除机台</Button>}
      </header>
      {notice && <p role="status" className="rounded-md bg-success-soft p-3 text-sm text-success">{notice}</p>}
      <section aria-label="关联图纸、工序和程序" className="space-y-3">
        <h2 className="text-xl font-bold">关联图纸、工序和程序</h2>
        <Field type="search" label="搜索关联记录" value={input} onChange={(e) => setInput(e.target.value)} placeholder="图纸号、类型、材料、工序、程序号或名称" />
        <p className="text-xs text-muted">显示匹配图纸的相关工序及独立关联此机台的程序。</p>
        {drawings.isPending && <Spinner />}
        {drawings.isError && <ErrorState message="关联记录加载失败" onRetry={() => drawings.refetch()} />}
        {drawings.isSuccess && <p className="text-sm text-muted">{drawings.data.pages[0].total} 张关联图纸</p>}
        {drawings.isSuccess && drawings.data.pages[0].total === 0 && <EmptyState title={q ? '没有匹配的关联记录' : '此机台尚未关联工序或程序'} />}
        {drawings.data?.pages.flatMap((p) => p.items).map((d) => <article key={d.id} className="space-y-3 rounded-lg border border-line bg-surface p-4">
          <h3><Link to={`/drawings/${d.id}`} className="inline-flex min-h-12 items-center break-words text-lg font-bold text-warning underline">{d.drawingNo}{d.name ? ` · ${d.name}` : ''}</Link></h3>
          {d.operations.map((op) => <section key={op.id} className="rounded-md bg-canvas p-3">
            <h4 className="font-semibold">{op.opNo}# {op.opName}</h4>
            {op.programs.length === 0 && <p className="mt-2 text-sm text-muted">暂无程序</p>}
            <ul className="mt-2 space-y-2">{op.programs.map((p) => <li key={p.id}><Link to={`/drawings/${d.id}/programs/${p.id}`} className="flex min-h-14 items-center justify-between gap-3 rounded-md border border-line bg-surface p-3 hover:border-accent"><div className="min-w-0 break-words"><p className="font-medium">{p.programNo}</p>{p.programName && <p className="text-sm text-muted">{p.programName}</p>}</div><VersionBadge no={p.currentVersionNo} /></Link></li>)}</ul>
          </section>)}
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
