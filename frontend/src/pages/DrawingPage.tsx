import { useAccess } from '../auth/permissions'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, ChevronDown, ChevronRight, Pencil, Plus, Trash2 } from 'lucide-react'
import { useId, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api'
import OwnerSelect from '../components/OwnerSelect'
import { ApiError } from '../api/client'
import type { Operation, Program } from '../api/types'
import DeleteSheet from '../components/DeleteSheet'
import DrawingEditSheet from '../components/DrawingEditSheet'
import OperationSheet from '../components/OperationSheet'
import ProgramSheet from '../components/ProgramSheet'
import { Button } from '../components/ui/Button'
import { EmptyState, ErrorState, Spinner } from '../components/ui/State'

export function VersionBadge({ no }: { no: number | null }) {
  return (
    <span className={no ? 'shrink-0 rounded-full bg-amber-100 px-3 py-1 text-sm font-semibold text-amber-800' : 'shrink-0 rounded-full bg-slate-200 px-3 py-1 text-sm text-slate-600'}>
      {no ? `V${no}` : '无版本'}
    </span>
  )
}

function ProgramRow({ drawingId, p }: { drawingId: number; p: Program }) {
  return (
    <li>
      <Link to={`/drawings/${drawingId}/programs/${p.id}`} className="flex min-h-14 items-center justify-between gap-3 rounded-xl bg-slate-50 px-3 py-2 active:bg-slate-200">
        <div className="min-w-0">
          <p className="font-semibold">{p.programNo}</p>
          <p className="truncate text-sm text-slate-500">程序机台：{[p.machineCode, p.machineName].filter(Boolean).join(' · ') || '未关联机台'}</p>
          {(p.programName || p.controller) && <p className="truncate text-sm text-slate-500">{[p.programName, p.controller].filter(Boolean).join(' · ')}</p>}
        </div>
        <div className="flex shrink-0 items-center gap-1">
          <VersionBadge no={p.currentVersionNo} />
          <ChevronRight className="size-5 text-slate-400" aria-hidden />
        </div>
      </Link>
    </li>
  )
}

function OperationCard({ drawingId, op, onEdit, onDelete, onAddProgram }: { drawingId: number; op: Operation; onEdit: () => void; onDelete: () => void; onAddProgram: () => void }) {
  const access = useAccess()
  const [expanded, setExpanded] = useState(false)
  const panelId = useId()
  const machine = [op.machineCode, op.machineName].filter(Boolean).join(' · ')
  const facts = [machine && `机台：${machine}`, op.fixture && `装夹 ${op.fixture}`].filter(Boolean)
  return (
    <section className="rounded-2xl border border-slate-200 bg-white shadow-sm">
      <h2 aria-label={`${op.opNo}# ${op.opName}`} className="text-lg font-bold">
        <button type="button" aria-label={`${expanded ? '收起' : '展开'}工序 ${op.opNo}`} aria-expanded={expanded} aria-controls={panelId} onClick={() => setExpanded(!expanded)} className="flex min-h-16 w-full items-center gap-3 rounded-2xl px-4 py-3 text-left hover:bg-slate-50 focus-visible:outline-2 focus-visible:outline-amber-500">
          <span className="min-w-0 flex-1 [overflow-wrap:anywhere]">{op.opNo}# {op.opName}</span>
          <span className="shrink-0 text-sm font-normal text-slate-500">{op.programs.length} 个程序</span>
          <ChevronDown aria-hidden className={`size-5 shrink-0 text-slate-500 ${expanded ? 'rotate-180' : ''}`} />
        </button>
      </h2>
      <div id={panelId} role="region" aria-label={`工序 ${op.opNo} 详情`} hidden={!expanded}>
        {expanded && <div className="border-t border-slate-100 p-4">
          <div className="flex flex-wrap justify-end gap-1">
            {access.can('edit') && <Button variant="ghost" onClick={onEdit} aria-label={`编辑工序 ${op.opNo}`} className="px-3">
              <Pencil className="size-4" aria-hidden />编辑
            </Button>}
            {access.can('delete') && <Button variant="ghost" onClick={onDelete} aria-label={`删除工序 ${op.opNo}`} className="px-3 text-red-600">
              <Trash2 className="size-4" aria-hidden />删除
            </Button>}
          </div>
          {facts.length > 0 && <p className="text-sm text-slate-600 [overflow-wrap:anywhere]">{facts.join(' · ')}</p>}
          {op.centeringMethod && <p className="mt-1 text-sm text-slate-600 [overflow-wrap:anywhere]">分中方式：{op.centeringMethod}</p>}
          {op.remark && <p className="mt-1 text-sm text-slate-500 [overflow-wrap:anywhere]">{op.remark}</p>}
          <dl aria-label="坐标（mm）" className="mt-3 grid grid-cols-3 gap-2">
            {([['X', op.xCoordinate ?? 0], ['Y', op.yCoordinate ?? 0], ['Z', op.zHeight]] as const).map(([axis, value]) => (
              <div key={axis} className="min-w-0 rounded-xl bg-slate-50 px-3 py-2">
                <dt className="text-xs text-slate-500">{axis}（mm）</dt>
                <dd className="break-all font-semibold tabular-nums">{value}</dd>
              </div>
            ))}
          </dl>
          {op.programs.length > 0 ? (
            <ul className="mt-3 space-y-2">
              {op.programs.map((p) => (
                <ProgramRow key={p.id} drawingId={drawingId} p={p} />
              ))}
            </ul>
          ) : (
            <p className="mt-3 text-sm text-slate-400">暂无程序</p>
          )}
          {access.can('create') && <Button variant="secondary" onClick={onAddProgram} className="mt-3 w-full" aria-label={`在 ${op.opNo}# 下新增程序`}>
            <Plus className="size-5" aria-hidden />
            新增程序
          </Button>}
        </div>}
      </div>
    </section>
  )
}

type Sheet = { kind: 'drawing' } | { kind: 'op'; op?: Operation } | { kind: 'program'; operationId: number } | null

/** 图纸详情：图纸信息 → 工序 → 程序。 */
export default function DrawingPage() {
  const access = useAccess()
  const qc = useQueryClient()
  const [assignOwner, setAssignOwner] = useState<number | ''>('')
  const [assignBusy, setAssignBusy] = useState(false)
  const [assignError, setAssignError] = useState('')
  const [deleting, setDeleting] = useState<Operation | null>(null)
  const id = Number(useParams().id)
  const valid = Number.isInteger(id) && id > 0
  const query = useQuery({ queryKey: ['drawing', id], queryFn: () => api.drawingDetail(id), enabled: valid && access.can('read'), retry: false })
  const [sheet, setSheet] = useState<Sheet>(null)
  const [saved, setSaved] = useState(false)
  const notFound = query.error instanceof ApiError && query.error.status === 404
  const ops = query.data?.operations ?? []
  const nextNo = (ops.reduce((m, o) => Math.max(m, o.opNo), 0) || 0) + 10
  if (!access.can('read')) return <main className="p-6"><p>账号不可用，请联系管理员。</p><Link to="/" className="mt-4 inline-block">返回总览</Link></main>

  return (
    <main className="mx-auto max-w-3xl px-4 pt-4 pb-8">
      <Link to="/" aria-label="返回总览" className="mb-2 inline-flex size-12 items-center justify-center rounded-xl hover:bg-slate-200">
        <ArrowLeft className="size-6" aria-hidden />
      </Link>
      {valid && query.isPending && <Spinner />}
      {(notFound || !valid) && <EmptyState title="图纸不存在或已被删除" />}
      {query.isError && !notFound && <ErrorState message="图纸加载失败" onRetry={() => query.refetch()} />}
      {query.data && (
        <>
          <header className="mb-4">
            <p className="mb-2 text-sm text-slate-500">所属账号：{query.data.ownerName || '待分配'}</p>
            {access.admin && query.data.ownerUserId === 0 && <form className="mb-4 space-y-3 rounded-xl bg-amber-50 p-3" onSubmit={async (e) => { e.preventDefault(); if (!assignOwner || assignBusy) return; setAssignBusy(true); setAssignError(''); try { await api.assignDrawingOwner(id, assignOwner); await Promise.all([qc.invalidateQueries({ queryKey: ['drawing', id] }), qc.invalidateQueries({ queryKey: ['drawings'] }), qc.invalidateQueries({ queryKey: ['admin-users'] })]) } catch (err) { setAssignError(err instanceof Error ? err.message : '分配失败') } finally { setAssignBusy(false) } }}><p>此图纸仅管理员可见。分配后，其工序、程序和版本将由目标账号管理。</p><OwnerSelect required value={assignOwner} onChange={setAssignOwner} /><Button type="submit" loading={assignBusy}>确认分配图纸</Button>{assignError && <p role="alert" className="text-red-700">{assignError}</p>}</form>}
            <div className="flex items-start justify-between gap-3">
              <h1 className="min-w-0 break-words text-2xl font-bold">{query.data.drawingNo}</h1>
              {access.can('edit') && <Button variant="secondary" className="shrink-0 px-3" onClick={() => { setSaved(false); setSheet({ kind: 'drawing' }) }}>
                <Pencil className="size-4" aria-hidden />编辑图纸
              </Button>}
            </div>
            <p className="mt-2 w-fit max-w-full break-words rounded-lg bg-slate-100 px-2 py-1 text-sm text-slate-700">类型：{query.data.name || '未分类'}</p>
            {query.data.material && <p className="mt-1 break-words text-sm text-slate-500">{query.data.material}</p>}
            <p className="mt-1 break-words text-sm text-slate-600 tabular-nums">价格：{query.data.price == null ? '未填写' : `${query.data.price} 元`}</p>
            {query.data.remark && <p className="mt-2 rounded-xl bg-amber-50 p-3 text-sm text-slate-700">{query.data.remark}</p>}
          </header>
          {saved && <p role="status" className="mb-4 rounded-xl bg-green-50 p-3 text-sm text-green-900">图纸已保存</p>}
          {ops.length === 0 ? (
            <EmptyState title="暂无工序">
              {access.can('create') && <Button onClick={() => setSheet({ kind: 'op' })}>新增第一道工序</Button>}
            </EmptyState>
          ) : (
            <div className="space-y-3">
              {ops.map((op) => (
                <OperationCard key={op.id} drawingId={id} op={op} onDelete={() => setDeleting(op)} onEdit={() => setSheet({ kind: 'op', op })} onAddProgram={() => setSheet({ kind: 'program', operationId: op.id })} />
              ))}
              {access.can('create') && <Button variant="secondary" className="w-full" onClick={() => setSheet({ kind: 'op' })}>
                <Plus className="size-5" aria-hidden />
                新增工序
              </Button>}
            </div>
          )}
        </>
      )}
      {deleting && <DeleteSheet
        title="删除工序"
        message={`将删除「${deleting.opNo}# ${deleting.opName}」及其下的 ${deleting.programs.length} 个程序和全部版本，无法恢复。`}
        remove={() => api.deleteOperation(deleting.id)}
        onDeleted={() => setDeleting(null)}
        onClose={() => setDeleting(null)}
      />}
      {sheet?.kind === 'op'  && <OperationSheet drawingId={id} ownerUserId={query.data?.ownerUserId} operation={sheet.op} suggestedNo={nextNo} onClose={() => setSheet(null)} />}
      {sheet?.kind === 'program' && <ProgramSheet drawingId={id} operationId={sheet.operationId} ownerUserId={query.data?.ownerUserId} onClose={() => setSheet(null)} />}
      {sheet?.kind === 'drawing' && query.data && <DrawingEditSheet key={id} drawing={query.data} onClose={() => setSheet(null)} onSaved={() => { setSheet(null); setSaved(true) }} />}
    </main>
  )
}
