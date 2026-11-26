import { useId, useRef, useState, type PointerEvent, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api'
import type { PhotoAnnotations, PhotoMark } from '../api/types'
import { Sheet } from './ui/Sheet'
import { Button } from './ui/Button'

const tools = { ellipse: '圈选', arrow: '箭头', text: '文字' } as const
type Tool = keyof typeof tools

function point(e: PointerEvent<SVGSVGElement>) {
  const rect = e.currentTarget.getBoundingClientRect()
  const unit = (value: number) => Math.round(Math.max(0, Math.min(1, value)) * 1000)
  return { x: unit((e.clientX - rect.left) / rect.width), y: unit((e.clientY - rect.top) / rect.height) }
}

export default function PhotoPreview({ operationId, digest, src, editable, onClose, children }: {
  operationId: number; digest: string; src: string; editable: boolean; onClose: () => void; children: ReactNode
}) {
  const key = ['photo-annotations', operationId, digest]
  const cache = useQueryClient()
  const query = useQuery({ queryKey: key, queryFn: () => api.photoAnnotations(operationId, digest) })
  const [draft, setDraft] = useState<PhotoAnnotations | null>(null)
  const [tool, setTool] = useState<Tool>('ellipse')
  const [note, setNote] = useState('')
  const [pending, setPending] = useState<PhotoMark | null>(null)
  const pointer = useRef<number | null>(null)
  const [saving, setSaving] = useState(false)
  const [original, setOriginal] = useState(false)
  const [error, setError] = useState('')
  const [ratio, setRatio] = useState(0)
  const marker = useId()
  const marks = draft?.marks ?? query.data?.marks ?? []
  const drawn = original && !draft ? [] : pending ? [...marks, pending] : marks
  const height = ratio ? 1000 / ratio : 1000
  const unit = Math.min(1000, height)
  const red = '#dc2626'
  const dirty = draft && (draft.marks !== query.data?.marks || note.trim() !== '')

  function discard() {
    if (dirty && !window.confirm('放弃尚未保存的标注？')) return false
    setDraft(null); setPending(null); setNote(''); setError('')
    pointer.current = null
    return true
  }
  function start(e: PointerEvent<SVGSVGElement>) {
    if (!draft || saving || !editable || pointer.current !== null || e.button !== 0) return
    if (marks.length >= 80) { setError('每张照片最多 80 处标注'); return }
    if (tool === 'text' && !note.trim()) { setError('先输入说明，再点照片上的位置'); return }
    const { x, y } = point(e)
    pointer.current = e.pointerId
    e.currentTarget.setPointerCapture(e.pointerId)
    setPending({ kind: tool, x, y, x2: x, y2: y, ...(tool === 'text' ? { text: note.trim() } : {}) })
    setError('')
  }
  function finish(e: PointerEvent<SVGSVGElement>) {
    if (pointer.current !== e.pointerId || !pending || !draft) return
    pointer.current = null
    const { x, y } = point(e)
    setPending(null)
    if (tool !== 'text' && Math.abs(x - pending.x) + Math.abs(y - pending.y) < 6) return
    setDraft({ ...draft, marks: [...marks, { ...pending, x2: x, y2: y }] })
    if (tool === 'text') setNote('')
  }
  async function save() {
    if (!editable || !draft || saving) return
    setError(''); setSaving(true)
    try {
      const result = await api.savePhotoAnnotations(operationId, digest, draft)
      cache.setQueryData(key, result)
      setDraft(null); setNote('')
    } catch (err) { setError(err instanceof Error ? err.message : '标注保存失败') }
    finally { setSaving(false) }
  }

  return <Sheet wide open title="装夹照片" onClose={() => { if (!saving && discard()) onClose() }}>
    <div className="flex justify-center">
      <div className="relative inline-block max-w-full leading-none">
        <img className="block max-h-[56vh] max-w-full object-contain" src={src} alt="装夹照片大图" onLoad={e => setRatio(e.currentTarget.naturalWidth / e.currentTarget.naturalHeight)} />
        {ratio > 0 && <svg className={`absolute inset-0 size-full ${draft ? 'touch-none cursor-crosshair' : 'pointer-events-none'}`}
          viewBox={`0 0 1000 ${height}`} preserveAspectRatio="none" aria-label="照片标注层"
          onPointerDown={start} onPointerMove={e => {
            if (pointer.current === e.pointerId && pending && tool !== 'text') {
              const { x, y } = point(e)
              setPending({ ...pending, x2: x, y2: y })
            }
          }} onPointerUp={finish} onPointerCancel={() => { pointer.current = null; setPending(null) }}>
          <defs><marker id={marker} markerWidth="4" markerHeight="4" refX="3.8" refY="2" orient="auto"><path d="M0 0 L4 2 L0 4 Z" fill={red} /></marker></defs>
          {drawn.map((m, i) => {
            const y = m.y / 1000 * height, y2 = m.y2 / 1000 * height
            if (m.kind === 'text') return <g key={i}>
              <circle cx={m.x} cy={y} r={unit * .034} fill={red} stroke="white" strokeWidth={unit * .004} />
              <text x={m.x} y={y} dominantBaseline="central" textAnchor="middle" fill="white" fontSize={unit * .044} fontWeight="bold">{drawn.slice(0, i + 1).filter(m => m.kind === 'text').length}</text>
            </g>
            if (m.kind === 'arrow') return <line key={i} x1={m.x} y1={y} x2={m.x2} y2={y2} stroke={red} strokeWidth={unit * .012} markerEnd={`url(#${marker})`} />
            return <ellipse key={i} cx={(m.x + m.x2) / 2} cy={(y + y2) / 2} rx={Math.max(unit * .007, Math.abs(m.x2 - m.x) / 2)} ry={Math.max(unit * .007, Math.abs(y2 - y) / 2)} fill="none" stroke={red} strokeWidth={unit * .01} />
          })}
        </svg>}
      </div>
    </div>
    {(!original || draft) && marks.some(m => m.kind === 'text') && <ol aria-label="照片说明" className="mt-3 list-decimal space-y-1 pl-6 text-sm">{marks.filter(m => m.kind === 'text').map((m, i) => <li key={i} className="break-words">{m.text}</li>)}</ol>}
    {query.isError && <button className="mt-2 underline" onClick={() => query.refetch()}>标注读取失败，点此重试</button>}
    {error && <p role="alert" className="mt-2 text-danger">{error}</p>}
    {draft && editable ? <fieldset disabled={saving} className="mt-3 space-y-2">
      <div className="flex flex-wrap gap-2">{(Object.keys(tools) as Tool[]).map(t => <Button key={t} aria-pressed={tool === t} variant={tool === t ? 'primary' : 'secondary'} onClick={() => {setTool(t); setError(''); if (t !== 'text') setNote('')}}>{tools[t]}</Button>)}</div>
      {tool === 'text' ? <label className="block text-sm">说明文字<input className="mt-1 block w-full rounded border border-line bg-surface p-2" value={note} maxLength={40} placeholder="输入后，点照片上的位置" onChange={e => setNote(e.target.value)} /></label>
        : <p className="text-sm text-muted">{tool === 'ellipse' ? '在照片上拖动，圈出装夹位置。' : '从起点拖到箭头要指向的位置。'}</p>}
      {!!marks.length && <details>
        <summary className="cursor-pointer py-2 text-sm">修改已有标注（{marks.length}）</summary>
        <ul className="space-y-2">{marks.map((m, i) => <li key={i} className="flex items-center gap-2">
          <span className="text-sm">{i + 1}. {tools[m.kind]}</span>
          {m.kind === 'text' && <input className="min-w-0 flex-1 rounded border border-line bg-surface p-2" aria-label={`修改第 ${i + 1} 处说明`} value={m.text} maxLength={40}
            onChange={e => setDraft({ ...draft, marks: marks.map((item, index) => index === i ? { ...item, text: e.target.value } : item) })} />}
          <Button variant="secondary" className="ml-auto" aria-label={`移除第 ${i + 1} 处标注`} onClick={() => setDraft({ ...draft, marks: marks.filter((_, index) => index !== i) })}>移除</Button>
        </li>)}</ul>
      </details>}
      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" disabled={!marks.length} onClick={() => setDraft({ ...draft, marks: marks.slice(0, -1) })}>撤销上一处</Button>
        <Button variant="secondary" onClick={() => { if (discard()) void query.refetch() }}>取消编辑</Button>
        <Button disabled={!dirty || !!pending || !!note.trim() || marks.some(m => m.kind === 'text' && !m.text?.trim())} onClick={save}>保存标注</Button>
      </div>
    </fieldset> : <>
      {!!marks.length && <Button className="mr-2 mt-3" variant="secondary" onClick={() => setOriginal(!original)}>{original ? '显示标注' : '查看原图'}</Button>}
      {editable && <Button className="mr-2 mt-3" variant="secondary" disabled={!query.data || !ratio} onClick={() => { if (query.data) { setDraft(query.data); setOriginal(false) } }}>编辑标注</Button>}
      {children}
    </>}
  </Sheet>
}
