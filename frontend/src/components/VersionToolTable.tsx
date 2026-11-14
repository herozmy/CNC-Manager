import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { ProgramToolInput, ToolTable, Version } from '../api/types'
import { Button } from './ui/Button'
import { Field } from './ui/Field'
import { ErrorState, Spinner } from './ui/State'

interface Props { version: Version }
type Row = Omit<ProgramToolInput, 'compAmount'> & { key: number; compAmount: string }
const blank: ProgramToolInput = { seq: 1, toolId: null, toolNo: '', offsetNo: '', toolName: '', toolDia: 0, cornerRadius: 0, compAmount: 0, spindleSpeed: 0, speedMode: 0, feed: 0, feedMode: 0, cutDepth: 0, coolant: 0, machiningContent: '', remark: '', customParams: [] }
function draft(item: ProgramToolInput, key: number): Row {
  return { ...item, key, compAmount: String(item.compAmount) }
}
function payload(rows: Row[]): ProgramToolInput[] {
  return rows.map((row, index) => {
    const { key: _key, ...item } = row
    return { ...item, seq: index + 1, compAmount: Number(row.compAmount) }
  })
}

// Keep unsaved versions in this login's query cache, including when navigating away.
// Session changes clear this cache; nothing is written to the server until Save.
interface ToolDraft { table: ToolTable; rows: Row[]; baseline: string; conflict: boolean; dirty: boolean }

export default function VersionToolTable(props: Props) {
  const qc = useQueryClient()
  const cached = qc.getQueryData<ToolDraft>(['version-tools-draft', props.version.id])
  const query = useQuery({ queryKey: ['version-tools', props.version.id], queryFn: () => api.versionTools(props.version.id), retry: false, refetchOnWindowFocus: false, refetchOnMount: 'always' })
  return <section aria-label={`V${props.version.versionNo} 刀具信息`} className="mt-5 space-y-4 rounded-2xl border border-slate-200 bg-white p-4">
    <h2 className="text-lg font-bold">刀具信息 · V{props.version.versionNo}{props.version.isCurrent ? '（当前）' : ''}</h2>
    {cached || (query.data && query.isFetchedAfterMount)
      ? <ToolEditor key={props.version.id} {...props} initial={cached?.table ?? query.data!} />
      : query.isPending ? <Spinner /> : <ErrorState message="刀具表加载失败" onRetry={() => void query.refetch()} />}
  </section>
}

function ToolEditor({ version, initial }: Props & { initial: ToolTable }) {
  const qc = useQueryClient()
  const draftKey = ['version-tools-draft', version.id] as const
  const [restored] = useState(() => qc.getQueryData<ToolDraft>(draftKey))
  // Retain all legacy fields when editing the three visible tool fields.
  const [table, setTable] = useState(restored?.table ?? initial)
  const [rows, setRows] = useState(() => restored?.rows ?? initial.items.map((row, i) => draft(row, i)))
  const seed = useRef(Math.max(-1, ...rows.map((row) => row.key)) + 1)
  const [baseline, setBaseline] = useState(restored?.baseline ?? JSON.stringify(payload(rows)))
  const [confirm, setConfirm] = useState<'discard' | 'reload' | null>(null)
  const [message, setMessage] = useState(restored ? '已恢复此版本未保存的修改' : '')
  const [error, setError] = useState('')
  const [conflict, setConflict] = useState(restored?.conflict ?? false)
  const dirty = JSON.stringify(payload(rows)) !== baseline
  useEffect(() => {
    const key = ['version-tools-draft', version.id]
    if (dirty) {
      qc.setQueryDefaults(key, { gcTime: Infinity })
      qc.setQueryData<ToolDraft>(key, { table, rows, baseline, conflict, dirty })
    } else {
      qc.removeQueries({ queryKey: key, exact: true })
    }
  }, [qc, version.id, table, rows, baseline, conflict, dirty])
  const accept = (data: ToolTable) => {
    const next = data.items.map((row) => draft(row, seed.current++))
    setTable(data); setRows(next); setBaseline(JSON.stringify(payload(next)))
    setConflict(false); setError(''); setConfirm(null)
    qc.setQueryData(['version-tools', version.id], data)
    qc.removeQueries({ queryKey: draftKey, exact: true })
  }
  const save = useMutation({
    mutationFn: () => api.saveVersionTools(version.id, payload(rows), table.revision),
    onSuccess: (data) => { accept(data); setMessage(`已保存 V${version.versionNo} 的刀具信息`) },
    onError: (e) => { setError(`保存失败：${e.message}`); if (e instanceof ApiError && e.status === 409) setConflict(true) },
  })
  const reload = useMutation({
    mutationFn: () => api.versionTools(version.id),
    onSuccess: (data) => { accept(data); setMessage('已载入最新刀具表') },
    onError: (e) => { setConfirm(null); setError(`载入失败，草稿已保留：${e.message}`) },
  })
  const recognize = useMutation({
    mutationFn: () => api.recognizeVersionTools(version.id),
    onSuccess: (parsed) => {
      const pair = (t: { toolNo: string; offsetNo: string }) => `${t.toolNo.trim().toUpperCase()}|${t.offsetNo.trim().toUpperCase()}`
      const seen = new Set(rows.map(pair))
      const added: Row[] = []
      for (const tool of parsed.tools) {
        if (seen.has(pair(tool))) continue
        seen.add(pair(tool))
        added.push(draft({ ...blank, toolNo: tool.toolNo, offsetNo: tool.offsetNo }, seed.current++))
      }
      if (rows.length + added.length > 500) { setError('识别结果超过 500 行，请手动添加需要的刀具'); return }
      setRows([...rows, ...added])
      setMessage(`已补充 ${added.length} 把刀具，已有刀具信息保留。${parsed.warnings.join('；')}${added.length ? '请核对后保存。' : ''}`)
    },
    onError: (e) => setError(`识别失败：${e.message}`),
  })
  const busy = save.isPending || reload.isPending || recognize.isPending
  const update = (key: number, values: Partial<Row>) => setRows((items) => items.map((row) => row.key === key ? { ...row, ...values } : row))
  const move = (index: number, delta: number) => setRows((items) => {
    const next = [...items]; [next[index], next[index + delta]] = [next[index + delta], next[index]]; return next
  })
  return <form className="space-y-4" onInvalid={() => setError('请检查刀具号和刀补范围（小数最多 3 位）')} onSubmit={(e) => {
      e.preventDefault()
      if (!dirty || busy || conflict || confirm) return
      if (rows.some((row) => !Number.isFinite(Number(row.compAmount)))) { setError('刀补应为有效数字'); return }
      setError(''); setMessage(''); save.mutate()
    }}>
      <div className="flex flex-wrap items-center gap-2">
        <Button variant="secondary" disabled={busy || !!confirm} onClick={() => { setError(''); recognize.mutate() }}>从 NC 识别</Button>
        <Button variant="secondary" disabled={busy || !!confirm || rows.length >= 500} onClick={() => { setRows([...rows, draft(blank, seed.current++)]); setMessage('') }}>手动添加</Button>
        <Button type="submit" disabled={!dirty || busy || conflict || !!confirm} loading={save.isPending}>保存刀具表</Button>
        {dirty && <Button variant="ghost" disabled={busy || !!confirm} onClick={() => setConfirm('discard')}>撤销修改</Button>}
        <span className="text-sm text-slate-500">{rows.length} 把刀具</span>
        {dirty && <span className="text-sm font-semibold text-amber-700">未保存</span>}
      </div>
      {message && <p role="status" className="rounded-xl bg-emerald-50 p-3 text-sm text-emerald-800">{message}</p>}
      {error && <p role="alert" className="text-red-700">{error}</p>}
      {conflict && <div className="space-y-2 rounded-xl bg-amber-50 p-3">
        <p>刀具表已被修改，草稿已保留。请核对草稿后载入最新数据。</p>
        <Button variant="secondary" disabled={busy} onClick={() => setConfirm('reload')}>载入最新刀具表</Button>
      </div>}
      {confirm && <div className="space-y-3 rounded-xl bg-amber-50 p-3">
        <p>{confirm === 'reload' ? '确定放弃草稿并载入最新刀具表吗？' : '有未保存的修改，确定放弃吗？'}</p>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" disabled={busy} onClick={() => setConfirm(null)}>继续编辑</Button>
          <Button variant="danger" disabled={busy} onClick={() => confirm === 'reload' ? reload.mutate() : accept(table)}>{confirm === 'reload' ? '放弃并载入' : '放弃修改'}</Button>
        </div>
      </div>}
      {rows.length === 0 && <p className="py-8 text-center text-slate-500">暂无刀具，可从 NC 识别或手动添加。</p>}
      <fieldset disabled={busy || !!confirm} className="min-w-0 space-y-3">
        {rows.map((row, index) => <section key={row.key} aria-label={`第 ${index + 1} 把刀具`} className="min-w-0 space-y-3 rounded-2xl border border-slate-200 bg-slate-50 p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="font-bold">{index + 1}. {row.toolNo || '新刀具'}</h3>
            <div className="flex gap-1">
              <Button variant="ghost" className="px-3" aria-label={`上移第 ${index + 1} 把刀具`} disabled={index === 0} onClick={() => move(index, -1)}>↑</Button>
              <Button variant="ghost" className="px-3" aria-label={`下移第 ${index + 1} 把刀具`} disabled={index === rows.length - 1} onClick={() => move(index, 1)}>↓</Button>
              <Button variant="ghost" className="px-3 text-red-700" aria-label={`删除第 ${index + 1} 把刀具`} onClick={() => setRows(rows.filter((r) => r.key !== row.key))}>删除</Button>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            <Field label="刀具号" required maxLength={100} placeholder="T01" value={row.toolNo} onChange={(e) => update(row.key, { toolNo: e.target.value })} />
            <Field label="刀具名称" maxLength={100} value={row.toolName} onChange={(e) => update(row.key, { toolName: e.target.value })} />
            <Field label="刀补（mm）" type="number" step="0.001" min={-100000} max={100000} value={row.compAmount} onChange={(e) => update(row.key, { compAmount: e.target.value })} />
          </div>
        </section>)}
      </fieldset>
    </form>
}
