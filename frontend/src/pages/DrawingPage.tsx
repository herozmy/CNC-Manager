import { useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { Link, useParams } from 'react-router-dom'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { Operation, Program } from '../api/types'
import { EmptyState, ErrorState, Spinner } from '../components/ui/State'

function ProgramRow({ p }: { p: Program }) {
  return (
    <li className="flex items-center justify-between gap-3 rounded-xl bg-slate-50 px-3 py-3">
      <div className="min-w-0">
        <p className="font-semibold">{p.programNo}</p>
        {(p.programName || p.controller) && <p className="truncate text-sm text-slate-500">{[p.programName, p.controller].filter(Boolean).join(' · ')}</p>}
      </div>
      <span className={p.currentVersionNo ? 'shrink-0 rounded-full bg-amber-100 px-3 py-1 text-sm font-semibold text-amber-800' : 'shrink-0 rounded-full bg-slate-200 px-3 py-1 text-sm text-slate-600'}>
        {p.currentVersionNo ? `V${p.currentVersionNo}` : '无版本'}
      </span>
    </li>
  )
}

function OperationCard({ op }: { op: Operation }) {
  const facts = [op.machineName, op.fixture && `装夹 ${op.fixture}`, op.zHeight > 0 && `垫高 ${op.zHeight} mm`].filter(Boolean)
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
      <h2 className="text-lg font-bold">
        {op.opNo}# {op.opName}
      </h2>
      {facts.length > 0 && <p className="mt-0.5 text-sm text-slate-600">{facts.join(' · ')}</p>}
      {op.remark && <p className="mt-1 text-sm text-slate-500">{op.remark}</p>}
      {op.programs.length > 0 ? (
        <ul className="mt-3 space-y-2">
          {op.programs.map((p) => (
            <ProgramRow key={p.id} p={p} />
          ))}
        </ul>
      ) : (
        <p className="mt-3 text-sm text-slate-400">暂无程序</p>
      )}
    </section>
  )
}

/** 图纸详情（只读）：图纸信息 → 工序 → 程序。 */
export default function DrawingPage() {
  const id = Number(useParams().id)
  const query = useQuery({ queryKey: ['drawing', id], queryFn: () => api.drawingDetail(id), enabled: Number.isInteger(id) && id > 0, retry: false })
  const notFound = query.error instanceof ApiError && query.error.status === 404

  return (
    <main className="mx-auto max-w-3xl px-4 pt-4 pb-8">
      <Link to="/" aria-label="返回总览" className="mb-2 inline-flex size-12 items-center justify-center rounded-xl hover:bg-slate-200">
        <ArrowLeft className="size-6" aria-hidden />
      </Link>
      {query.isPending && query.fetchStatus !== 'idle' && <Spinner />}
      {(notFound || !Number.isInteger(id) || id <= 0) && <EmptyState title="图纸不存在或已被删除" />}
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
          {query.data.operations.length === 0 ? (
            <EmptyState title="暂无工序" />
          ) : (
            <div className="space-y-3">
              {query.data.operations.map((op) => (
                <OperationCard key={op.id} op={op} />
              ))}
            </div>
          )}
        </>
      )}
    </main>
  )
}
