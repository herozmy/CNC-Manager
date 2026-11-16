import { ChevronRight, Layers, Pencil, Trash2 } from 'lucide-react'
import { Link } from 'react-router-dom'
import type { Drawing } from '../api/types'
import { relativeTime } from '../lib/format'

interface Props {
  items: Drawing[]
  mode: 'grid' | 'table'
  onEdit?: (drawing: Drawing) => void
  onDelete?: (drawing: Drawing) => void
}

function Actions({ drawing, onEdit, onDelete }: { drawing: Drawing } & Pick<Props, 'onEdit' | 'onDelete'>) {
  return <>
    {onEdit && <button type="button" onClick={() => onEdit(drawing)} aria-label={`编辑图纸 ${drawing.drawingNo}`} title="编辑图纸" className="flex min-h-12 flex-col items-center justify-center gap-0.5 rounded-md px-2 text-xs whitespace-nowrap text-body hover:bg-accent-soft focus-visible:outline-2 focus-visible:outline-accent"><Pencil className="size-4 shrink-0" aria-hidden /><span>编辑</span></button>}
    {onDelete && <button type="button" onClick={() => onDelete(drawing)} aria-label={`删除图纸 ${drawing.drawingNo}`} title="删除图纸" className="flex min-h-12 flex-col items-center justify-center gap-0.5 rounded-md px-2 text-xs whitespace-nowrap text-danger hover:bg-danger-soft focus-visible:outline-2 focus-visible:outline-accent"><Trash2 className="size-4 shrink-0" aria-hidden /><span>删除</span></button>}
  </>
}

export default function DrawingCatalog({ items, mode, onEdit, onDelete }: Props) {
  if (mode === 'table') return <div className="max-w-full overflow-x-auto rounded-lg border border-line bg-surface">
    <table aria-label="图纸台账表" className="w-full min-w-[760px] border-collapse text-left text-sm">
      <thead className="border-b border-line bg-inset text-xs text-muted"><tr><th className="px-4 py-3">图纸号</th><th className="px-4 py-3">类型 / 材料</th><th className="px-4 py-3">所属账号</th><th className="px-4 py-3">工序 / 程序</th><th className="px-4 py-3">最近更新</th>{(onEdit || onDelete) && <th className="px-4 py-3">操作</th>}</tr></thead>
      <tbody className="divide-y divide-line">{items.map(d => <tr key={d.id} className="hover:bg-accent-soft/50">
        <td className="px-4 py-2"><Link to={`/drawings/${d.id}`} className="inline-flex min-h-12 items-center font-mono font-semibold text-accent hover:underline">{d.drawingNo}</Link></td>
        <td className="px-4 py-3"><p>{d.name || '未分类'}</p><p className="mt-1 text-xs text-muted">{d.material || '未填写材料'}</p></td>
        <td className="px-4 py-3 text-muted">{d.ownerName || '待分配'}</td>
        <td className="px-4 py-3 font-mono tabular-nums">{d.operationCount} 序 / {d.programCount} 程序</td>
        <td className="px-4 py-3 text-xs text-muted">{relativeTime(d.updatedAt)}</td>
        {(onEdit || onDelete) && <td className="px-2 py-2"><div className="flex gap-1"><Actions drawing={d} onEdit={onEdit} onDelete={onDelete} /></div></td>}
      </tr>)}</tbody>
    </table>
  </div>

  return <ul className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">{items.map(d => <li key={d.id} className="min-w-0">
    <article className="flex h-full items-stretch rounded-lg border border-line bg-surface hover:border-accent">
      <Link to={`/drawings/${d.id}`} className="min-w-0 flex-1 rounded-l-lg p-4 focus-visible:outline-2 focus-visible:outline-accent active:bg-hover">
        <div className="flex flex-wrap items-start justify-between gap-x-2 gap-y-1"><p className="min-w-0 break-all font-mono text-sm font-semibold text-accent">{d.drawingNo}</p><span className="shrink-0 text-xs text-muted">{relativeTime(d.updatedAt)}</span></div>
        <p className="mt-3 break-words text-sm font-semibold">类型：{d.name || '未分类'}</p>
        {d.ownerName && <p className="mt-2 text-xs text-muted">所属账号：{d.ownerName}</p>}
        {d.material && <p className="mt-1 break-words text-sm text-muted">{d.material}</p>}
        <p className="mt-4 flex flex-wrap items-center gap-1.5 border-t border-line pt-3 text-xs text-muted"><Layers className="size-4 shrink-0" aria-hidden />{d.operationCount} 道工序 · {d.programCount} 个程序<ChevronRight className="ml-auto size-4 text-accent" aria-hidden /></p>
      </Link>
      {(onEdit || onDelete) && <div className="flex w-14 shrink-0 flex-col justify-center gap-2 border-l border-line p-1"><Actions drawing={d} onEdit={onEdit} onDelete={onDelete} /></div>}
    </article>
  </li>)}</ul>
}
