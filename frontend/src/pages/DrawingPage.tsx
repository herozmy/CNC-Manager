import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, ChevronRight, Pencil, Plus } from 'lucide-react'
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { Operation, Program } from '../api/types'
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

function OperationCard({ drawingId, op, onEdit, onAddProgram }: { drawingId: number; op: Operation; onEdit: () => void; onAddProgram: () => void }) {
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
        <button onClick={onEdit} aria-label={`编辑工序 ${op.opNo}`} className="flex size-12 shrink-0 items-center justify-center rounded-xl text-slate-600 hover:bg-slate-100">
          <Pencil className="size-5" aria-hidden />
        </button>
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

type Sheet = { kind: 'op'; op?: Operation } | { kind: 'program'; operationId: number } | null

/** 图纸详情：图纸信息 → 工序 → 程序。 */
export default function DrawingPage() {
  const id = Number(useParams().id)
  const valid = Number.isInteger(id) && id > 0
  const query = useQuery({ queryKey: ['drawing', id], queryFn: () => api.drawingDetail(id), enabled: valid, retry: false })
  const [sheet, setSheet] = useState<Sheet>(null)
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
            <h1 className="break-words text-2xl font-bold">{query.data.drawingNo}</h1>
            {query.data.name && <p className="text-lg text-slate-700">{query.data.name}</p>}
            <p className="mt-1 text-sm text-slate-500">
              {[query.data.customer, query.data.material, query.data.drawingVersion && `版本 ${query.data.drawingVersion}`].filter(Boolean).join(' · ')}
            </p>
            {query.data.remark && <p className="mt-2 rounded-xl bg-amber-50 p-3 text-sm text-slate-700">{query.data.remark}</p>}
          </header>
          {ops.length === 0 ? (
            <EmptyState title="暂无工序">
              <Button onClick={() => setSheet({ kind: 'op' })}>新增第一道工序</Button>
            </EmptyState>
          ) : (
            <div className="space-y-3">
              {ops.map((op) => (
                <OperationCard key={op.id} drawingId={id} op={op} onEdit={() => setSheet({ kind: 'op', op })} onAddProgram={() => setSheet({ kind: 'program', operationId: op.id })} />
              ))}
              <Button variant="secondary" className="w-full" onClick={() => setSheet({ kind: 'op' })}>
                <Plus className="size-5" aria-hidden />
                新增工序
              </Button>
            </div>
          )}
        </>
      )}
      {sheet?.kind === 'op' && <OperationSheet drawingId={id} operation={sheet.op} suggestedNo={nextNo} onClose={() => setSheet(null)} />}
      {sheet?.kind === 'program' && <ProgramSheet drawingId={id} operationId={sheet.operationId} onClose={() => setSheet(null)} />}
    </main>
  )
}
