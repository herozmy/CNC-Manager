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
import OperationPhotos from '../components/OperationPhotos'
import OperationSheet from '../components/OperationSheet'
import ProgramSheet from '../components/ProgramSheet'
import { Button } from '../components/ui/Button'
import { Breadcrumbs } from '../components/ui/Breadcrumbs'
import { EmptyState, ErrorState, Spinner } from '../components/ui/State'

export function VersionBadge({ no }: { no: number | null }) {
  return (
    <span className={no ? 'shrink-0 rounded-md bg-success-soft px-2.5 py-1 font-mono text-xs font-semibold text-success' : 'shrink-0 rounded-md bg-inset px-2.5 py-1 text-xs text-muted'}>
      {no ? `V${no}` : '无版本'}
    </span>
  )
}

function ProgramRow({ drawingId, p }: { drawingId: number; p: Program }) {
  return (
    <li>
      <Link to={`/drawings/${drawingId}/programs/${p.id}`} className="flex min-h-14 items-center justify-between gap-3 px-3 py-3 hover:bg-accent-soft active:bg-hover">
        <div className="min-w-0">
          <p className="font-mono text-sm font-semibold text-accent">{p.programNo}</p>
          <p className="truncate text-sm text-muted">程序机台：{[p.machineCode, p.machineName].filter(Boolean).join(' · ') || '未关联机台'}</p>
          {(p.programName || p.controller) && <p className="truncate text-sm text-muted">{[p.programName, p.controller].filter(Boolean).join(' · ')}</p>}
        </div>
        <div className="flex shrink-0 items-center gap-1">
          <VersionBadge no={p.currentVersionNo} />
          <ChevronRight className="size-5 text-subtle" aria-hidden />
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
    <section className="overflow-hidden rounded-md border border-line bg-surface">
      <div className={`flex items-center gap-1 border-l-[3px] px-2 ${expanded ? 'border-accent bg-accent-soft' : 'border-transparent'}`}>
        <h2 aria-label={`${op.opNo}# ${op.opName}`} className="min-w-0 flex-1 text-base font-semibold">
          <button type="button" aria-label={`${expanded ? '收起' : '展开'}工序 ${op.opNo}`} aria-expanded={expanded} aria-controls={panelId} onClick={() => setExpanded(!expanded)} className="flex min-h-16 w-full items-center gap-2 rounded-md px-2 py-3 text-left hover:bg-canvas focus-visible:outline-2 focus-visible:outline-accent">
            <span className="min-w-0 flex-1 break-words text-ink" title={`${op.opNo}# ${op.opName}`}><span className="mr-2 font-mono text-accent">{op.opNo}#</span> {op.opName}</span>
            <span className="shrink-0 text-xs font-normal text-muted sm:text-sm">{op.programs.length} 个程序</span>
            <ChevronDown aria-hidden className={`size-5 shrink-0 text-muted ${expanded ? 'rotate-180' : ''}`} />
          </button>
        </h2>
        <div className="flex shrink-0 items-center gap-1">
          {access.can('edit') && <Button variant="ghost" onClick={onEdit} aria-label={`编辑工序 ${op.opNo}`} title="编辑工序" className="min-w-12 px-3">
            <Pencil className="size-4" aria-hidden /><span className="hidden sm:inline">编辑</span>
          </Button>}
          {access.can('delete') && <Button variant="ghost" onClick={onDelete} aria-label={`删除工序 ${op.opNo}`} title="删除工序" className="min-w-12 px-3 text-danger">
            <Trash2 className="size-4" aria-hidden /><span className="hidden sm:inline">删除</span>
          </Button>}
        </div>
      </div>
      <div id={panelId} role="region" aria-label={`工序 ${op.opNo} 详情`} hidden={!expanded}>
        {expanded && <div className="border-t border-line p-4">
          {facts.length > 0 && <p className="text-sm text-muted [overflow-wrap:anywhere]">{facts.join(' · ')}</p>}
          {op.centeringMethod && <p className="mt-1 text-sm text-muted [overflow-wrap:anywhere]">分中方式：{op.centeringMethod}</p>}
          {op.remark && <p className="mt-1 text-sm text-muted [overflow-wrap:anywhere]">{op.remark}</p>}
          <dl aria-label="坐标（mm）" className="mt-3 grid grid-cols-3 divide-x divide-line overflow-hidden rounded-md border border-line">
            {([['X', op.xCoordinate ?? 0], ['Y', op.yCoordinate ?? 0], ['Z', op.zHeight]] as const).map(([axis, value]) => (
              <div key={axis} className="min-w-0 bg-canvas px-3 py-3">
                <dt className="text-xs text-muted">{axis}（mm）</dt>
                <dd className="mt-1 break-all font-mono text-lg font-semibold tabular-nums">{value}</dd>
              </div>
            ))}
          </dl>
          <OperationPhotos operationId={op.id} editable={access.can('edit')} />
          {op.programs.length > 0 ? (
            <ul className="mt-4 divide-y divide-line border-y border-line">
              {op.programs.map((p) => (
                <ProgramRow key={p.id} drawingId={drawingId} p={p} />
              ))}
            </ul>
          ) : (
            <p className="mt-3 text-sm text-subtle">暂无程序</p>
          )}
          {access.can('create') && <Button variant="secondary" onClick={onAddProgram} className="mt-3 w-full sm:w-auto" aria-label={`在 ${op.opNo}# 下新增程序`}>
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
    <main className="page-width min-w-0 px-4 pt-4 pb-8 md:px-6">
      <div className="mb-3 flex min-w-0 items-center gap-1">
      <Link to="/" aria-label="返回总览" className="inline-flex size-12 shrink-0 items-center justify-center rounded-md hover:bg-hover">
        <ArrowLeft className="size-6" aria-hidden />
      </Link>
      {query.data && <Breadcrumbs className="mb-0 flex-1" items={[{ label: '总览', to: '/' }, { label: query.data.drawingNo, mono: true }]} />}
      </div>
      {valid && query.isPending && <Spinner />}
      {(notFound || !valid) && <EmptyState title="图纸不存在或已被删除" />}
      {query.isError && !notFound && <ErrorState message="图纸加载失败" onRetry={() => query.refetch()} />}
      {query.data && (
        <>
          <header className="panel mb-5 p-4 md:p-5">
            <p className="mb-2 text-sm text-muted">所属账号：{query.data.ownerName || '待分配'}</p>
            {access.admin && query.data.ownerUserId === 0 && <form className="mb-4 space-y-3 rounded-md bg-warning-soft p-3" onSubmit={async (e) => { e.preventDefault(); if (!assignOwner || assignBusy) return; setAssignBusy(true); setAssignError(''); try { await api.assignDrawingOwner(id, assignOwner); await Promise.all([qc.invalidateQueries({ queryKey: ['drawing', id] }), qc.invalidateQueries({ queryKey: ['drawings'] }), qc.invalidateQueries({ queryKey: ['admin-users'] })]) } catch (err) { setAssignError(err instanceof Error ? err.message : '分配失败') } finally { setAssignBusy(false) } }}><p>此图纸仅管理员可见。分配后，其工序、程序和版本将由目标账号管理。</p><OwnerSelect required value={assignOwner} onChange={setAssignOwner} /><Button type="submit" loading={assignBusy}>确认分配图纸</Button>{assignError && <p role="alert" className="text-danger">{assignError}</p>}</form>}
            <div className="flex items-start justify-between gap-3">
              <h1 className="min-w-0 break-words font-mono text-2xl font-semibold tracking-tight text-ink md:text-3xl">{query.data.drawingNo}</h1>
              {access.can('edit') && <Button variant="secondary" className="shrink-0 px-3" onClick={() => { setSaved(false); setSheet({ kind: 'drawing' }) }}>
                <Pencil className="size-4" aria-hidden />编辑图纸
              </Button>}
            </div>
            <div className="mt-4 flex flex-wrap items-center gap-x-6 gap-y-2 border-t border-line pt-3"><p className="w-fit max-w-full break-words rounded-lg bg-inset px-2 py-1 text-sm text-body">类型：{query.data.name || '未分类'}</p>
            {query.data.material && <p className="mt-1 break-words text-sm text-muted">{query.data.material}</p>}
            <p className="mt-1 break-words text-sm text-muted tabular-nums">价格：{query.data.price == null ? '未填写' : `${query.data.price} 元`}</p>
            </div>
            {query.data.remark && <p className="mt-2 rounded-md bg-warning-soft p-3 text-sm text-body">{query.data.remark}</p>}
            <Link to={`/drawings/${id}/preparation`} className="mt-3 inline-flex min-h-12 items-center rounded-md border border-line-strong px-4 text-sm font-semibold text-ink hover:bg-inset">加工准备单</Link>
          </header>
          {saved && <p role="status" className="mb-4 rounded-md bg-success-soft p-3 text-sm text-success">图纸已保存</p>}
          <div className="mb-3 flex items-center justify-between gap-3"><h2 className="text-lg font-semibold">加工工序 <span className="ml-1 font-mono text-sm text-muted">{ops.length}</span></h2>
            {ops.length > 0 && access.can('create') && <Button onClick={() => setSheet({ kind: 'op' })}><Plus className="size-4" />新增工序</Button>}
          </div>
          {ops.length === 0 ? (
            <EmptyState title="暂无工序">
              {access.can('create') && <Button onClick={() => setSheet({ kind: 'op' })}>新增第一道工序</Button>}
            </EmptyState>
          ) : (
            <div className="space-y-3">
              {ops.map((op) => (
                <OperationCard key={op.id} drawingId={id} op={op} onDelete={() => setDeleting(op)} onEdit={() => setSheet({ kind: 'op', op })} onAddProgram={() => setSheet({ kind: 'program', operationId: op.id })} />
              ))}

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
