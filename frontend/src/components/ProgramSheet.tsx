import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import { ApiError } from '../api/client'
import { Button } from './ui/Button'
import { Field, TextArea } from './ui/Field'
import { Sheet } from './ui/Sheet'

interface Props {
  drawingId: number
  operationId: number
  onClose: () => void
}

const controllers = ['FANUC', 'SIEMENS', '三菱', '广数', '华中']

/** 在工序下新增程序，成功后直接进入程序页去上传文件。 */
export default function ProgramSheet({ drawingId, operationId, onClose }: Props) {
  const qc = useQueryClient()
  const nav = useNavigate()
  const [programNo, setProgramNo] = useState('')
  const [programName, setProgramName] = useState('')
  const [controller, setController] = useState('')
  const [remark, setRemark] = useState('')
  const [noError, setNoError] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setNoError('')
    setError('')
    if (!programNo.trim()) return setNoError('请输入程序号')
    setBusy(true)
    try {
      const p = await api.createProgram(operationId, { programNo, programName, controller, remark })
      await Promise.all([qc.invalidateQueries({ queryKey: ['drawing', drawingId] }), qc.invalidateQueries({ queryKey: ['drawings'] })])
      onClose()
      nav(`/drawings/${drawingId}/programs/${p.id}`)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '无法连接服务器')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Sheet open title="新增程序" onClose={onClose}>
      <form onSubmit={submit} className="space-y-4" noValidate>
        <Field label="程序号" value={programNo} onChange={(e) => setProgramNo(e.target.value)} error={noError} autoCapitalize="characters" placeholder="如 O1234" />
        <Field label="程序名称" value={programName} onChange={(e) => setProgramName(e.target.value)} />
        <Field label="数控系统" value={controller} onChange={(e) => setController(e.target.value)} list="controllers" placeholder="可选，上传文件后会自动识别" />
        <datalist id="controllers">
          {controllers.map((c) => (
            <option key={c} value={c} />
          ))}
        </datalist>
        <TextArea label="备注" value={remark} onChange={(e) => setRemark(e.target.value)} />
        {error && (
          <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
            {error}
          </p>
        )}
        <Button type="submit" loading={busy} className="w-full">
          创建并上传文件
        </Button>
      </form>
    </Sheet>
  )
}
