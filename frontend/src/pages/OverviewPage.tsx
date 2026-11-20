import { useInfiniteQuery } from '@tanstack/react-query'
import { LayoutGrid, List, Search } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api'
import type { Drawing } from '../api/types'
import DeleteSheet from '../components/DeleteSheet'
import DrawingEditSheet from '../components/DrawingEditSheet'
import { Button } from '../components/ui/Button'
import { EmptyState, ErrorState, Spinner } from '../components/ui/State'
import DrawingCatalog from '../components/DrawingCatalog'
import { useAccess } from '../auth/permissions'
import OwnerSelect from '../components/OwnerSelect'
import { useDebounced } from '../lib/useDebounced'

/** 总览：首页，按最近更新排序的图纸，可按图纸号/类型/材料/程序号搜索。 */
export default function OverviewPage() {
  const access = useAccess()
  const [ownerUserId, setOwnerUserId] = useState<number | ''>('')
  const [input, setInput] = useState('')
  const [deleting, setDeleting] = useState<Drawing | null>(null)
  const [editing, setEditing] = useState<Drawing | null>(null)
  const [saved, setSaved] = useState(false)
  const [displayMode, setDisplayMode] = useState<'grid' | 'table'>(() => window.matchMedia?.('(min-width: 1024px)').matches ? 'table' : 'grid')
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
  if (!access.can('read')) return <main className="p-6"><h1 className="text-2xl font-semibold tracking-tight">图纸台账</h1><p className="mt-4">账号不可用，请联系管理员。</p>{access.can('create') && <Link to="/drawings/new" className="mt-4 inline-block">新建图纸</Link>}</main>

  return (
    <main className="page-width min-w-0 px-4 pt-5 pb-8 md:px-6">
      <header className="mb-5 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">图纸台账</h1>
          {query.isSuccess && <p className="text-sm text-muted">{q ? `找到 ${total} 张图纸` : `共 ${total} 张图纸`}</p>}
        </div>
        {access.can('create') && <Link to="/drawings/new" className="hidden min-h-12 items-center rounded-md bg-action px-5 text-sm font-semibold text-on-action hover:brightness-110 md:inline-flex">
          新建图纸
        </Link>}
      </header>
      {access.admin && <div className="mb-3"><OwnerSelect value={ownerUserId} onChange={setOwnerUserId} includeUnassigned includeDisabled label="筛选所属账号" /></div>}

      <div className="sticky top-0 z-[5] mb-4 flex flex-wrap items-center gap-3 bg-canvas/95 py-2 backdrop-blur">
        <label className="relative block min-w-0 flex-1 basis-64">
          <span className="sr-only">搜索图纸</span>
          <Search className="pointer-events-none absolute top-1/2 left-4 size-5 -translate-y-1/2 text-subtle" aria-hidden />
          <input
            type="search"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="图纸号、类型、材料、程序号"
            enterKeyHint="search"
            className="h-12 w-full rounded-md border border-line-strong bg-surface pr-4 pl-12 text-base focus:outline-2 focus:outline-accent"
          />
        </label>
        <div role="group" aria-label="图纸显示方式" className="inline-flex shrink-0 gap-1 rounded-md border border-line bg-surface p-1">
          <button type="button" aria-pressed={displayMode === 'grid'} onClick={() => setDisplayMode('grid')} className={`flex min-h-10 items-center gap-1.5 rounded px-3 text-sm ${displayMode === 'grid' ? 'bg-accent-soft text-accent' : 'text-muted hover:bg-hover'}`}><LayoutGrid className="size-4" aria-hidden />卡片</button>
          <button type="button" aria-pressed={displayMode === 'table'} onClick={() => setDisplayMode('table')} className={`flex min-h-10 items-center gap-1.5 rounded px-3 text-sm ${displayMode === 'table' ? 'bg-accent-soft text-accent' : 'text-muted hover:bg-hover'}`}><List className="size-4" aria-hidden />台账表</button>
        </div>
      </div>

      {saved && <p role="status" className="mb-4 rounded-md bg-success-soft p-3 text-sm text-success">图纸已保存</p>}
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
        <DrawingCatalog items={items} mode={displayMode} onEdit={access.can('edit') ? d => { setSaved(false); setEditing(d) } : undefined} onDelete={access.can('delete') ? setDeleting : undefined} />
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
