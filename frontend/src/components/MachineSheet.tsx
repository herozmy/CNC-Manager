import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { Machine, MachineInput } from '../api/types'
import { useAccess } from '../auth/permissions'
import OwnerSelect from './OwnerSelect'
import { Button } from './ui/Button'
import { Field, TextArea } from './ui/Field'
import { Sheet } from './ui/Sheet'

export default function MachineSheet({ machine, onClose, onSaved }: { machine?: Machine; onClose: () => void; onSaved: (machine: Machine) => void }) {
  const access = useAccess()
  const qc = useQueryClient()
  const [initial] = useState<MachineInput>({ code: machine?.code ?? '', name: machine?.name ?? '', controller: machine?.controller ?? '', remark: machine?.remark ?? '', ownerUserId: machine?.ownerUserId ?? access.user?.id })
  const [form, setForm] = useState(initial)
  const [busy, setBusy] = useState(false)
  const pending = useRef(false)
  const [discard, setDiscard] = useState(false)
  const [error, setError] = useState('')
  const dirty = JSON.stringify(form) !== JSON.stringify(initial)
  const set = (key: 'code' | 'name' | 'controller' | 'remark') => (e: { target: { value: string } }) => setForm({ ...form, [key]: e.target.value })
  const close = () => { if (!pending.current) { if (dirty) setDiscard(true); else onClose() } }
  useEffect(() => {
    if (!dirty) return
    const warn = (e: BeforeUnloadEvent) => { e.preventDefault(); e.returnValue = '' }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])
  async function submit(e: FormEvent) {
    e.preventDefault()
    if (pending.current || discard) return
    setError('')
    const input = { ...form, code: form.code.trim(), name: form.name.trim(), controller: form.controller.trim(), remark: form.remark.trim() }
    if (!input.code || [...input.code].length > 64) return setError('机台编号应为 1~64 个字符')
    if ([...input.name].length > 100 || [...input.controller].length > 100 || [...input.remark].length > 2000) return setError('名称和控制系统最多 100 个字符，备注最多 2000 个字符')
    if (!machine && !input.ownerUserId) return setError('请选择所属账号')
    pending.current = true; setBusy(true)
    try {
      const saved = machine ? await api.updateMachine(machine.id, input) : await api.createMachine(input)
      qc.setQueryData(['machine', saved.id], saved)
      await qc.invalidateQueries({ predicate: (q) => ['machines', 'drawing-machines', 'drawing', 'machine-drawings'].includes(String(q.queryKey[0])) })
      onSaved(saved)
    } catch (err) { setError(err instanceof ApiError ? err.message : '保存失败，请检查网络后重试') }
    finally { pending.current = false; setBusy(false) }
  }
  return <Sheet open title={machine ? '编辑机台' : '新增机台'} onClose={close}>
    <form onSubmit={submit} noValidate className="space-y-4">
      <fieldset disabled={busy || discard} className="space-y-4">
        {machine ? <p className="text-sm text-muted">所属账号：{machine.ownerName}（归属固定）</p> : access.admin && <OwnerSelect required value={form.ownerUserId ?? ''} onChange={(value) => setForm({ ...form, ownerUserId: value === '' ? undefined : value })} />}
        <Field label="机台编号" value={form.code} onChange={set('code')} maxLength={64} placeholder="如 CNC-01" />
        <Field label="机台名称（可选）" value={form.name} onChange={set('name')} maxLength={100} placeholder="如 立式加工中心" />
        <Field label="控制系统（可选）" value={form.controller} onChange={set('controller')} maxLength={100} placeholder="如 FANUC" />
        <TextArea label="备注" value={form.remark} onChange={set('remark')} maxLength={2000} />
      </fieldset>
      {error && <p role="alert" className="rounded-md bg-danger-soft p-3 text-sm text-danger">{error}</p>}
      <div className="grid grid-cols-2 gap-3"><Button variant="secondary" disabled={busy || discard} onClick={close}>取消</Button><Button type="submit" loading={busy} disabled={discard}>{machine ? '保存修改' : '创建机台'}</Button></div>
    </form>
    {discard && <div className="mt-4 space-y-3 rounded-md bg-warning-soft p-3"><p>有未保存的修改，确定放弃吗？</p><div className="flex gap-2"><Button variant="secondary" onClick={() => setDiscard(false)}>继续编辑</Button><Button variant="danger" onClick={onClose}>放弃修改</Button></div></div>}
  </Sheet>
}
