import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { DrawingDetail, Program, ProgramInput } from '../api/types'
import { Button } from './ui/Button'
import { Field, TextArea } from './ui/Field'
import { Sheet } from './ui/Sheet'
import ProgramMachinePicker from './ProgramMachinePicker'

interface Props {
  drawingId: number
  operationId: number
  ownerUserId?: number
  program?: Program
  onClose: () => void
  onSaved?: () => void
}

const controllers = ['FANUC', 'SIEMENS', '三菱', '广数', '华中']
type TextKey = 'programNo' | 'programName' | 'controller' | 'remark'
const fields: { key: TextKey; label: string; max: number }[] = [
  { key: 'programNo', label: '程序号', max: 100 },
  { key: 'programName', label: '程序名称', max: 100 },
  { key: 'controller', label: '数控系统', max: 100 },
  { key: 'remark', label: '备注', max: 2000 },
]

/** 新增 / 编辑程序基本信息；每次打开回填，保存不改变 NC 版本。 */
export default function ProgramSheet({ drawingId, operationId, ownerUserId, program, onClose, onSaved }: Props) {
  const qc = useQueryClient()
  const nav = useNavigate()
  const [initial, setInitial] = useState<ProgramInput>(() => ({
    programNo: program?.programNo ?? '', programName: program?.programName ?? '',
    controller: program?.controller ?? '', remark: program?.remark ?? '',
    machineId: program?.machineId ?? null,
  }))
  const [form, setForm] = useState(initial)
  const [referenceProgram, setReferenceProgram] = useState(program)
  const [revision, setRevision] = useState(program?.revision ?? 1)
  const [conflict, setConflict] = useState(false)
  const [reloadConfirm, setReloadConfirm] = useState(false)
  const [errors, setErrors] = useState<Partial<Record<keyof ProgramInput, string>>>({})
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [creatingMachine, setCreatingMachine] = useState(false)
  const [addingMachine, setAddingMachine] = useState(false)
  const creating = useRef(false)
  const saving = useRef(false)
  const [discard, setDiscard] = useState(false)
  const dirty = fields.some(({ key }) => form[key] !== initial[key]) || form.machineId !== initial.machineId || addingMachine
  const set = (key: TextKey) => (e: { target: { value: string } }) => setForm({ ...form, [key]: e.target.value })

  useEffect(() => {
    if (!dirty) return
    const warn = (e: BeforeUnloadEvent) => { e.preventDefault(); e.returnValue = '' }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])

  function close() {
    if (saving.current || creating.current) return
    if (dirty) setDiscard(true)
    else onClose()
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (saving.current || creating.current || discard || addingMachine) return
    setError('')
    const input = { ...form, programNo: form.programNo.trim(), programName: form.programName.trim(), controller: form.controller.trim() }
    const invalid: Partial<Record<keyof ProgramInput, string>> = {}
    if (!input.programNo) invalid.programNo = '请输入程序号'
    for (const { key, label, max } of fields) {
      if (new TextEncoder().encode(input[key]).length > max) invalid[key] = `${label}不能超过 ${max} 字节`
    }
    setErrors(invalid)
    if (Object.keys(invalid).length) return
    saving.current = true
    setBusy(true)
    try {
      // Older clients may omit machineId. Avoid overwriting an unchanged association.
      if (program && input.machineId === initial.machineId) delete input.machineId
      const p = program ? await api.updateProgram(program.id, { ...input, expectedRevision: revision }) : await api.createProgram(operationId, input)
      if (program) {
        qc.setQueryData<DrawingDetail>(['drawing', drawingId], (previous) => previous ? {
          ...previous, operations: previous.operations.map((op) => ({
            ...op, programs: op.programs.map((item) => item.id === p.id ? p : item),
          })),
        } : undefined)
      }
      await Promise.all([qc.invalidateQueries({ queryKey: ['drawing', drawingId] }), qc.invalidateQueries({ queryKey: ['drawings'] })])
      void qc.invalidateQueries({ predicate: (q) => ['machines', 'machine', 'machine-drawings'].includes(String(q.queryKey[0])) })
      onClose()
      if (program) onSaved?.()
      else nav(`/drawings/${drawingId}/programs/${p.id}`)
    } catch (err) {
      const message = err instanceof ApiError ? err.message : '无法连接服务器，请重试'
      if (err instanceof ApiError && err.status === 409) { setConflict(true); setError(message) }
      else setError(message)
    } finally {
      saving.current = false
      setBusy(false)
    }
  }

  return (
    <Sheet open title={program ? '编辑程序信息' : '新增程序'} onClose={close}>
      <form onSubmit={submit} className="space-y-4" noValidate>
        <fieldset disabled={busy || creatingMachine || discard} className="space-y-4">
          <Field label="程序号" value={form.programNo} onChange={set('programNo')} error={errors.programNo} autoCapitalize="characters" placeholder="如 O1234" />
          <Field label="程序名称" value={form.programName} onChange={set('programName')} error={errors.programName} />
          <Field label="数控系统" value={form.controller} onChange={set('controller')} error={errors.controller} list="controllers" placeholder="可选，上传文件后会自动识别" />
          <datalist id="controllers">
            {controllers.map((c) => <option key={c} value={c} />)}
          </datalist>
          <ProgramMachinePicker drawingId={drawingId} ownerUserId={ownerUserId} program={referenceProgram} value={form.machineId ?? null} onChange={(machineId) => setForm((old) => ({ ...old, machineId }))} busy={busy || creatingMachine || discard} onAdding={setAddingMachine} onCreating={(value) => { creating.current = value; setCreatingMachine(value) }} />
          <TextArea label="备注" value={form.remark} onChange={set('remark')} error={errors.remark} />
        </fieldset>
        {error && <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>}
        {conflict && <Button variant="secondary" disabled={busy || creatingMachine || addingMachine} onClick={() => setReloadConfirm(true)}>重新载入程序信息</Button>}
        {reloadConfirm && <div className="space-y-3 rounded-xl bg-amber-50 p-3"><p>重新载入将放弃当前未保存的修改，是否继续？</p><Button variant="secondary" onClick={() => setReloadConfirm(false)}>继续编辑</Button><Button disabled={busy || creatingMachine || addingMachine} onClick={async () => {
          if (!program) return
          setBusy(true)
          try { const latest = await api.getProgram(program.id); const values = { programNo: latest.programNo, programName: latest.programName, controller: latest.controller, remark: latest.remark, machineId: latest.machineId ?? null }; setReferenceProgram(latest); setInitial(values); setForm(values); setRevision(latest.revision); setConflict(false); setError(''); setErrors({}); setReloadConfirm(false) } catch (e) { setError(e instanceof Error ? e.message : '载入失败') } finally { setBusy(false) }
        }}>放弃并载入</Button></div>}
        <div className="grid grid-cols-2 gap-3">
          <Button variant="secondary" disabled={busy || creatingMachine || discard} onClick={close}>取消</Button>
          <Button type="submit" loading={busy} disabled={addingMachine || creatingMachine || discard || conflict || reloadConfirm}>{program ? '保存修改' : '创建并上传文件'}</Button>
        </div>
      </form>
      {discard && <div className="mt-4 space-y-3 rounded-xl bg-amber-50 p-3">
        <p>有未保存的修改，确定放弃吗？</p>
        <div className="flex gap-2">
          <Button variant="secondary" onClick={() => setDiscard(false)}>继续编辑</Button>
          <Button variant="danger" onClick={onClose}>放弃修改</Button>
        </div>
      </div>}
    </Sheet>
  )
}
