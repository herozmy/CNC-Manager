import { useInfiniteQuery } from '@tanstack/react-query'
import { Layers, Pencil, Search, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api'
import type { Drawing } from '../api/types'
import DeleteSheet from '../components/DeleteSheet'
import DrawingEditSheet from '../components/DrawingEditSheet'
import { Button } from '../components/ui/Button'
import { EmptyState, ErrorState, Spinner } from '../components/ui/State'
import { relativeTime } from '../lib/format'
import { useAccess } from '../auth/permissions'
import OwnerSelect from '../components/OwnerSelect'
import { useDebounced } from '../lib/useDebounced'

function DrawingCard({ d, onEdit, onDelete }: { d: Drawing; onEdit?: () => void; onDelete?: () => void }) {
  return (
    <article className="flex items-stretch rounded-2xl border border-slate-200 bg-white shadow-sm md:hover:border-amber-400">
      <Link to={`/drawings/${d.id}`} className="min-w-0 flex-1 rounded-l-2xl p-4 active:bg-slate-100">
        <div className="flex items-start justify-between gap-3">
          <p className="min-w-0 break-words text-lg font-bold">{d.drawingNo}</p>
          <span className="shrink-0 text-xs text-slate-500">{relativeTime(d.updatedAt)}</span>
        </div>
        <p className="mt-2 w-fit max-w-full break-words rounded-lg bg-slate-100 px-2 py-1 text-sm text-slate-700">类型：{d.name || '未分类'}</p>
        {d.ownerName && <p className="mt-1 text-sm text-slate-500">所属账号：{d.ownerName}</p>}
        {d.material && <p className="mt-1 break-words text-sm text-slate-500">{d.material}</p>}
        <p className="mt-3 flex items-center gap-1.5 text-sm text-slate-600">
          <Layers className="size-4" aria-hidden />
          {d.operationCount} 道工序 · {d.programCount} 个程序
        </p>
      </Link>
      {(onEdit || onDelete) && <div className="flex w-14 shrink-0 flex-col overflow-hidden rounded-r-2xl border-l border-slate-100 md:w-20">
        {onEdit && <button
          type="button"
          onClick={onEdit}
          aria-label={`编辑图纸 ${d.drawingNo}`}
          title="编辑图纸"
          className="flex min-h-12 flex-1 flex-col items-center justify-center gap-1 text-sm text-slate-700 hover:bg-amber-50 active:bg-amber-100 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-amber-500 md:flex-row"
        >
          <Pencil className="size-4" aria-hidden /><span>编辑</span>
        </button>}
        {onDelete && <button
          type="button"
          onClick={onDelete}
          aria-label={`删除图纸 ${d.drawingNo}`}
          title="删除图纸"
          className={`flex min-h-12 flex-1 flex-col items-center justify-center gap-1 text-sm text-red-600 hover:bg-red-50 active:bg-red-100 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-amber-500 md:flex-row ${onEdit ? 'border-t border-slate-100' : ''}`}
        >
          <Trash2 className="size-4" aria-hidden /><span>删除</span>
        </button>}
      </div>}
    </article>
  )
}

/** 总览：首页，按最近更新排序的图纸，可按图纸号/类型/材料/程序号搜索。 */
export default function OverviewPage() {
  const access = useAccess()
  const [ownerUserId, setOwnerUserId] = useState<number | ''>('')
  const [input, setInput] = useState('')
  const [deleting, setDeleting] = useState<Drawing | null>(null)
  const [editing, setEditing] = useState<Drawing | null>(null)
  const [saved, setSaved] = useState(false)
  const q = useDebounced(input.trim())
  const query = useInfiniteQuery({
    queryKey: ['drawings', q, ownerUserId],
    queryFn: ({ pageParam }) => api.listDrawings(q, pageParam, 20, access.admin && ownerUserId !== '' ? ownerUserId : undefined),
    enabled: access.can('read'),
    initialPageParam: 1,
    getNextPageParam: (last) => (last.page * last.size < last.total ? last.page + 1 : undefined),
  })
  const items = query.data?.pages.flatMap((p) => p.items) ?? []
  const total = query.data?.pages[0]?.total ?? 0
  if (!access.can('read')) return <main className="p-6"><h1 className="text-2xl font-bold">总览</h1><p className="mt-4">账号不可用，请联系管理员。</p>{access.can('create') && <Link to="/drawings/new" className="mt-4 inline-block">新建图纸</Link>}</main>

  return (
    <main className="mx-auto max-w-3xl px-4 pt-4 pb-8">
      <header className="mb-4 flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-bold">总览</h1>
          {query.isSuccess && <p className="text-sm text-slate-500">{q ? `找到 ${total} 张图纸` : `共 ${total} 张图纸`}</p>}
        </div>
        {access.can('create') && <Link to="/drawings/new" className="hidden md:block">
          <Button>新建图纸</Button>
        </Link>}
      </header>
      {access.admin && <div className="mb-3"><OwnerSelect value={ownerUserId} onChange={setOwnerUserId} includeUnassigned includeDisabled label="筛选所属账号" /></div>}

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

      {saved && <p role="status" className="mb-4 rounded-xl bg-green-50 p-3 text-sm text-green-900">图纸已保存</p>}
      {query.isPending && <Spinner />}
      {query.isError && <ErrorState message="图纸加载失败" onRetry={() => query.refetch()} />}
      {query.isSuccess && items.length === 0 && (
        <EmptyState title={q ? '没有找到匹配的图纸' : '还没有图纸'}>
          {!q && access.can('create') && (
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
              <DrawingCard d={d} onEdit={access.can('edit') ? () => { setSaved(false); setEditing(d) } : undefined} onDelete={access.can('delete') ? () => setDeleting(d) : undefined} />
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
      {editing && <DrawingEditSheet key={editing.id} drawing={editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); setSaved(true) }} />}
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
