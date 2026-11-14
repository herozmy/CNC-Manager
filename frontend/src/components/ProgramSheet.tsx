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

interface Props {
  drawingId: number
  operationId: number
  program?: Program
  onClose: () => void
  onSaved?: () => void
}

const controllers = ['FANUC', 'SIEMENS', '三菱', '广数', '华中']
const fields: { key: keyof ProgramInput; label: string; max: number }[] = [
  { key: 'programNo', label: '程序号', max: 100 },
  { key: 'programName', label: '程序名称', max: 100 },
  { key: 'controller', label: '数控系统', max: 100 },
  { key: 'remark', label: '备注', max: 2000 },
]

/** 新增 / 编辑程序基本信息；每次打开回填，保存不改变 NC 版本。 */
export default function ProgramSheet({ drawingId, operationId, program, onClose, onSaved }: Props) {
  const qc = useQueryClient()
  const nav = useNavigate()
  const [initial] = useState<ProgramInput>(() => ({
    programNo: program?.programNo ?? '', programName: program?.programName ?? '',
    controller: program?.controller ?? '', remark: program?.remark ?? '',
  }))
  const [form, setForm] = useState(initial)
  const [errors, setErrors] = useState<Partial<Record<keyof ProgramInput, string>>>({})
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const saving = useRef(false)
  const [discard, setDiscard] = useState(false)
  const dirty = fields.some(({ key }) => form[key] !== initial[key])
  const set = (key: keyof ProgramInput) => (e: { target: { value: string } }) => setForm({ ...form, [key]: e.target.value })

  useEffect(() => {
    if (!dirty) return
    const warn = (e: BeforeUnloadEvent) => { e.preventDefault(); e.returnValue = '' }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])

  function close() {
    if (saving.current) return
    if (dirty) setDiscard(true)
    else onClose()
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (saving.current || discard) return
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
      const p = program ? await api.updateProgram(program.id, input) : await api.createProgram(operationId, input)
      if (program) {
        qc.setQueryData<DrawingDetail>(['drawing', drawingId], (previous) => previous ? {
          ...previous, operations: previous.operations.map((op) => ({
            ...op, programs: op.programs.map((item) => item.id === p.id ? p : item),
          })),
        } : undefined)
      }
      await Promise.all([qc.invalidateQueries({ queryKey: ['drawing', drawingId] }), qc.invalidateQueries({ queryKey: ['drawings'] })])
      onClose()
      if (program) onSaved?.()
      else nav(`/drawings/${drawingId}/programs/${p.id}`)
    } catch (err) {
      const message = err instanceof ApiError ? err.message : '无法连接服务器，请重试'
      if (err instanceof ApiError && err.status === 409) setErrors({ programNo: message.replace(/^.*?:\s*/, '') })
      else setError(message)
    } finally {
      saving.current = false
      setBusy(false)
    }
  }

  return (
    <Sheet open title={program ? '编辑程序信息' : '新增程序'} onClose={close}>
      <form onSubmit={submit} className="space-y-4" noValidate>
        <fieldset disabled={busy || discard} className="space-y-4">
          <Field label="程序号" value={form.programNo} onChange={set('programNo')} error={errors.programNo} autoCapitalize="characters" placeholder="如 O1234" />
          <Field label="程序名称" value={form.programName} onChange={set('programName')} error={errors.programName} />
          <Field label="数控系统" value={form.controller} onChange={set('controller')} error={errors.controller} list="controllers" placeholder="可选，上传文件后会自动识别" />
          <datalist id="controllers">
            {controllers.map((c) => <option key={c} value={c} />)}
          </datalist>
          <TextArea label="备注" value={form.remark} onChange={set('remark')} error={errors.remark} />
        </fieldset>
        {error && <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>}
        <div className="grid grid-cols-2 gap-3">
          <Button variant="secondary" disabled={busy || discard} onClick={close}>取消</Button>
          <Button type="submit" loading={busy} disabled={discard}>{program ? '保存修改' : '创建并上传文件'}</Button>
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
