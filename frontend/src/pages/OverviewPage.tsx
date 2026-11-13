import { useInfiniteQuery } from '@tanstack/react-query'
import { Layers, Search } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api'
import type { Drawing } from '../api/types'
import { Button } from '../components/ui/Button'
import { EmptyState, ErrorState, Spinner } from '../components/ui/State'
import { relativeTime } from '../lib/format'
import { useDebounced } from '../lib/useDebounced'

function DrawingCard({ d }: { d: Drawing }) {
  const meta = [d.customer, d.material, d.drawingVersion && `版本 ${d.drawingVersion}`].filter(Boolean)
  return (
    <Link
      to={`/drawings/${d.id}`}
      className="block rounded-2xl border border-slate-200 bg-white p-4 shadow-sm active:bg-slate-100 md:hover:border-amber-400"
    >
      <div className="flex items-start justify-between gap-3">
        <p className="min-w-0 break-words text-lg font-bold">{d.drawingNo}</p>
        <span className="shrink-0 text-xs text-slate-500">{relativeTime(d.updatedAt)}</span>
      </div>
      {d.name && <p className="mt-0.5 text-slate-700">{d.name}</p>}
      {meta.length > 0 && <p className="mt-1 text-sm text-slate-500">{meta.join(' · ')}</p>}
      <p className="mt-3 flex items-center gap-1.5 text-sm text-slate-600">
        <Layers className="size-4" aria-hidden />
        {d.operationCount} 道工序 · {d.programCount} 个程序
      </p>
    </Link>
  )
}

/** 总览：首页，按最近更新排序的图纸，可按图纸号/名称/客户/材料/程序号搜索。 */
export default function OverviewPage() {
  const [input, setInput] = useState('')
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
            placeholder="图纸号、名称、客户、程序号"
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
              <DrawingCard d={d} />
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
    </main>
  )
}
