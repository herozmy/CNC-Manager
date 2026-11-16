import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { MachineOption, Program } from '../api/types'
import { useAccess } from '../auth/permissions'
import { Button } from './ui/Button'
import { Field } from './ui/Field'

interface Props {
  drawingId: number
  ownerUserId?: number
  program?: Program
  value: number | null
  onChange: (id: number | null) => void
  busy: boolean
  onCreating: (busy: boolean) => void
  onAdding: (adding: boolean) => void
}

export default function ProgramMachinePicker({ drawingId, ownerUserId, program, value, onChange, busy, onCreating, onAdding }: Props) {
  const qc = useQueryClient()
  const access = useAccess()
  const machines = useQuery({ queryKey: ['drawing-machines', drawingId], queryFn: () => api.drawingMachines(drawingId), retry: false })
  const options = (machines.data ?? []).filter((m) => m.enabled !== false || m.id === program?.machineId)
  const [adding, setAdding] = useState(false)
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const pending = useRef(false)
  async function create() {
    if (pending.current || busy) return
    setError('')
    if (!code.trim() || [...code.trim()].length > 64) return setError('机台编号应为 1~64 个字符')
    if ([...name.trim()].length > 100) return setError('机台名称最多 100 个字符')
    pending.current = true; onCreating(true)
    try {
      const machine = await api.createDrawingMachine(drawingId, { code: code.trim(), name: name.trim() })
      qc.setQueryData<MachineOption[]>(['drawing-machines', drawingId], (old) => [...(old ?? []).filter((m) => m.id !== machine.id), machine])
      void qc.invalidateQueries({ queryKey: ['drawing-machines'] })
      void qc.invalidateQueries({ queryKey: ['machines'] })
      onChange(machine.id); setAdding(false); onAdding(false); setCode(''); setName('')
    } catch (err) { setError(err instanceof ApiError ? err.message : '机台创建失败，请检查网络后重试') }
    finally { pending.current = false; onCreating(false) }
  }
  return <section aria-label="程序机台" className="space-y-2 rounded-md border border-line p-3">
    <label className="block space-y-1.5 text-sm font-medium text-body">
      <span>程序机台（可选）</span>
      <select aria-label="程序机台（可选）" value={value ?? ''} disabled={busy || machines.isPending || adding} onChange={(e) => onChange(e.target.value ? Number(e.target.value) : null)} className="h-12 w-full min-w-0 rounded-md border border-line-strong bg-surface px-3 text-base">
        <option value="">未关联机台</option>
        {value !== null && !options.some((m) => m.id === value) && <option value={value}>{[program?.machineCode, program?.machineName].filter(Boolean).join(' · ') || `机台 #${value}`}（当前选择）</option>}
        {options.map((m) => <option key={m.id} value={m.id}>{m.code}{m.name ? ` · ${m.name}` : ''}{m.enabled === false ? '（已停用，当前关联）' : ''}</option>)}
      </select>
    </label>
    <p className="text-xs text-muted">每个程序独立选择机台。仅列出图纸所属账号的启用机台；已停用的当前关联可保留，选择“未关联机台”可解除关联。</p>
    {machines.isPending && <p role="status" className="text-sm text-muted">正在加载机台…</p>}
    {machines.isError && <p className="text-sm text-danger">机台列表加载失败，当前选择仍保留。</p>}
    <Button variant="ghost" disabled={busy} onClick={() => void machines.refetch()}>{machines.isError ? '重试加载机台' : '刷新机台列表'}</Button>
    {access.can('create') && (ownerUserId === 0 ? <p className="text-sm text-muted">请先分配图纸所属账号，再新增机台。</p> : !adding && <Button variant="secondary" disabled={busy} onClick={() => { setError(''); setAdding(true); onAdding(true) }}>新增机台</Button>)}
    {adding && <fieldset disabled={busy} className="space-y-3">
      <Field label="机台编号" value={code} onChange={(e) => setCode(e.target.value)} maxLength={64} placeholder="如 CNC-01" />
      <Field label="机台名称（可选）" value={name} onChange={(e) => setName(e.target.value)} maxLength={100} />
      <p className="text-xs text-muted">新增后自动选中，也可供同一账号的其他程序复用；程序关联需保存程序信息。</p>
      {error && <p role="alert" className="text-sm text-danger">{error}</p>}
      <div className="flex flex-wrap gap-2"><Button disabled={busy} onClick={() => void create()}>新增并选中</Button><Button variant="ghost" disabled={busy} onClick={() => { setAdding(false); onAdding(false) }}>取消新增机台</Button></div>
    </fieldset>}
  </section>
}
