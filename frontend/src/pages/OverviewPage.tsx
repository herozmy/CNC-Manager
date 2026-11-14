import { useInfiniteQuery } from '@tanstack/react-query'
import { Layers, Search, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api'
import type { Drawing } from '../api/types'
import DeleteSheet from '../components/DeleteSheet'
import { Button } from '../components/ui/Button'
import { EmptyState, ErrorState, Spinner } from '../components/ui/State'
import { relativeTime } from '../lib/format'
import { useDebounced } from '../lib/useDebounced'

function DrawingCard({ d, onDelete }: { d: Drawing; onDelete: () => void }) {
  return (
    <article className="flex items-stretch rounded-2xl border border-slate-200 bg-white shadow-sm md:hover:border-amber-400">
      <Link to={`/drawings/${d.id}`} className="min-w-0 flex-1 rounded-l-2xl p-4 active:bg-slate-100">
        <div className="flex items-start justify-between gap-3">
          <p className="min-w-0 break-words text-lg font-bold">{d.drawingNo}</p>
          <span className="shrink-0 text-xs text-slate-500">{relativeTime(d.updatedAt)}</span>
        </div>
        <p className="mt-2 w-fit max-w-full break-words rounded-lg bg-slate-100 px-2 py-1 text-sm text-slate-700">类型：{d.name || '未分类'}</p>
        {d.material && <p className="mt-1 break-words text-sm text-slate-500">{d.material}</p>}
        <p className="mt-3 flex items-center gap-1.5 text-sm text-slate-600">
          <Layers className="size-4" aria-hidden />
          {d.operationCount} 道工序 · {d.programCount} 个程序
        </p>
      </Link>
      <button
        type="button"
        onClick={onDelete}
        aria-label={`删除图纸 ${d.drawingNo}`}
        className="flex w-14 shrink-0 items-center justify-center rounded-r-2xl border-l border-slate-100 text-red-600 hover:bg-red-50 active:bg-red-100"
      >
        <Trash2 className="size-5" aria-hidden />
      </button>
    </article>
  )
}

/** 总览：首页，按最近更新排序的图纸，可按图纸号/类型/材料/程序号搜索。 */
export default function OverviewPage() {
  const [input, setInput] = useState('')
  const [deleting, setDeleting] = useState<Drawing | null>(null)
  const q = useDebounced(input.trim())
  const query = useInfiniteQuery({
    queryKey: ['drawings', q],
    queryFn: ({ pageParam }) => api.listDrawings(q, pageParam),
    initialPageParam: 1,
    getNextPageParam: (last) => (last.page * last.size < last.total ? last.page + 1 : undefined),
  })
  const items = query.data?.pages.flatMap((p) => p.items) ?? []
  const total = query.data?.pages[0]?.total ?? 0

  return (
    <main className="mx-auto max-w-3xl px-4 pt-4 pb-8">
      <header className="mb-4 flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-bold">总览</h1>
          {query.isSuccess && <p className="text-sm text-slate-500">{q ? `找到 ${total} 张图纸` : `共 ${total} 张图纸`}</p>}
        </div>
        <Link to="/drawings/new" className="hidden md:block">
          <Button>新建图纸</Button>
        </Link>
      </header>

      <div className="sticky top-0 z-[5] -mx-4 mb-4 bg-slate-50/95 px-4 py-2 backdrop-blur">
        <label className="relative block">
          <span className="sr-only">搜索图纸</span>
          <Search className="pointer-events-none absolute top-1/2 left-4 size-5 -translate-y-1/2 text-slate-400" aria-hidden />
          <input
            type="search"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="图纸号、类型、材料、程序号"
            enterKeyHint="search"
            className="h-12 w-full rounded-xl border border-slate-300 bg-white pr-4 pl-12 text-base focus:outline-2 focus:outline-amber-500"
          />
        </label>
      </div>

      {query.isPending && <Spinner />}
      {query.isError && <ErrorState message="图纸加载失败" onRetry={() => query.refetch()} />}
      {query.isSuccess && items.length === 0 && (
        <EmptyState title={q ? '没有找到匹配的图纸' : '还没有图纸'}>
          {!q && (
            <Link to="/drawings/new">
              <Button>新建第一张图纸</Button>
            </Link>
          )}
        </EmptyState>
      )}
      {items.length > 0 && (
        <ul className="space-y-3">
          {items.map((d) => (
            <li key={d.id}>
              <DrawingCard d={d} onDelete={() => setDeleting(d)} />
            </li>
          ))}
        </ul>
      )}
      {query.hasNextPage && (
        <div className="mt-4 text-center">
          <Button variant="secondary" loading={query.isFetchingNextPage} onClick={() => query.fetchNextPage()}>
            加载更多
          </Button>
        </div>
      )}
      {deleting && (
        <DeleteSheet
          title="删除图纸"
          message={`将删除图纸「${deleting.drawingNo}」及其下的 ${deleting.operationCount} 道工序、${deleting.programCount} 个程序和全部版本，无法恢复。`}
          remove={() => api.deleteDrawing(deleting.id)}
          onDeleted={() => setDeleting(null)}
          onClose={() => setDeleting(null)}
        />
      )}
    </main>
  )
}
