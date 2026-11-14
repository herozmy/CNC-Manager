import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, ChevronRight, Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api'
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
  const facts = [op.machineName, op.fixture && `装夹 ${op.fixture}`, op.zHeight > 0 && `垫高 ${op.zHeight} mm`].filter(Boolean)
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <h2 className="text-lg font-bold">
            {op.opNo}# {op.opName}
          </h2>
          {facts.length > 0 && <p className="mt-0.5 text-sm text-slate-600">{facts.join(' · ')}</p>}
          {op.remark && <p className="mt-1 text-sm text-slate-500">{op.remark}</p>}
        </div>
        <div className="flex shrink-0">
          <button onClick={onEdit} aria-label={`编辑工序 ${op.opNo}`} className="flex size-12 shrink-0 items-center justify-center rounded-xl text-slate-600 hover:bg-slate-100">
            <Pencil className="size-5" aria-hidden />
          </button>
          <button onClick={onDelete} aria-label={`删除工序 ${op.opNo}`} className="flex size-12 items-center justify-center rounded-xl text-red-600 hover:bg-red-50"><Trash2 className="size-5" aria-hidden /></button>
        </div>
      </div>
      {op.programs.length > 0 ? (
        <ul className="mt-3 space-y-2">
          {op.programs.map((p) => (
            <ProgramRow key={p.id} drawingId={drawingId} p={p} />
          ))}
        </ul>
      ) : (
        <p className="mt-3 text-sm text-slate-400">暂无程序</p>
      )}
      <Button variant="secondary" onClick={onAddProgram} className="mt-3 w-full" aria-label={`在 ${op.opNo}# 下新增程序`}>
        <Plus className="size-5" aria-hidden />
        新增程序
      </Button>
    </section>
  )
}

type Sheet = { kind: 'drawing' } | { kind: 'op'; op?: Operation } | { kind: 'program'; operationId: number } | null

/** 图纸详情：图纸信息 → 工序 → 程序。 */
export default function DrawingPage() {
  const [deleting, setDeleting] = useState<Operation | null>(null)
  const id = Number(useParams().id)
  const valid = Number.isInteger(id) && id > 0
  const query = useQuery({ queryKey: ['drawing', id], queryFn: () => api.drawingDetail(id), enabled: valid, retry: false })
  const [sheet, setSheet] = useState<Sheet>(null)
  const [saved, setSaved] = useState(false)
  const notFound = query.error instanceof ApiError && query.error.status === 404
  const ops = query.data?.operations ?? []
  const nextNo = (ops.reduce((m, o) => Math.max(m, o.opNo), 0) || 0) + 10

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
            <div className="flex items-start justify-between gap-3">
              <h1 className="min-w-0 break-words text-2xl font-bold">{query.data.drawingNo}</h1>
              <Button variant="secondary" className="shrink-0 px-3" onClick={() => { setSaved(false); setSheet({ kind: 'drawing' }) }}>
                <Pencil className="size-4" aria-hidden />编辑图纸
              </Button>
            </div>
            <p className="mt-2 w-fit max-w-full break-words rounded-lg bg-slate-100 px-2 py-1 text-sm text-slate-700">类型：{query.data.name || '未分类'}</p>
            {query.data.material && <p className="mt-1 break-words text-sm text-slate-500">{query.data.material}</p>}
            {query.data.remark && <p className="mt-2 rounded-xl bg-amber-50 p-3 text-sm text-slate-700">{query.data.remark}</p>}
          </header>
          {saved && <p role="status" className="mb-4 rounded-xl bg-green-50 p-3 text-sm text-green-900">图纸已保存</p>}
          {ops.length === 0 ? (
            <EmptyState title="暂无工序">
              <Button onClick={() => setSheet({ kind: 'op' })}>新增第一道工序</Button>
            </EmptyState>
          ) : (
            <div className="space-y-3">
              {ops.map((op) => (
                <OperationCard key={op.id} drawingId={id} op={op} onDelete={() => setDeleting(op)} onEdit={() => setSheet({ kind: 'op', op })} onAddProgram={() => setSheet({ kind: 'program', operationId: op.id })} />
              ))}
              <Button variant="secondary" className="w-full" onClick={() => setSheet({ kind: 'op' })}>
                <Plus className="size-5" aria-hidden />
                新增工序
              </Button>
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
      {sheet?.kind === 'op'  && <OperationSheet drawingId={id} operation={sheet.op} suggestedNo={nextNo} onClose={() => setSheet(null)} />}
      {sheet?.kind === 'program' && <ProgramSheet drawingId={id} operationId={sheet.operationId} onClose={() => setSheet(null)} />}
      {sheet?.kind === 'drawing' && query.data && <DrawingEditSheet key={id} drawing={query.data} onClose={() => setSheet(null)} onSaved={() => { setSheet(null); setSaved(true) }} />}
    </main>
  )
}
