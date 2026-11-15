import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { UploadResult } from '../api/types'
import { formatBytes } from '../lib/format'
import { Button } from './ui/Button'
import { Field } from './ui/Field'
import { Sheet } from './ui/Sheet'

interface Props {
  drawingId: number
  programId: number
  file: File
  onClose: () => void
  onDone: (result: UploadResult) => void
}

/** 确认上传：显示文件名与大小，可填变更说明。 */
export default function UploadSheet({ drawingId, programId, file, onClose, onDone }: Props) {
  const qc = useQueryClient()
  const [note, setNote] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const pending = useRef(false)

  useEffect(() => {
    const warn = (e: BeforeUnloadEvent) => {
      if (!pending.current) return
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [])

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (pending.current) return
    pending.current = true
    setError('')
    setBusy(true)
    try {
      const result = await api.uploadVersion(programId, file, note)
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['versions', programId] }),
        qc.invalidateQueries({ queryKey: ['drawing', drawingId] }),
        qc.invalidateQueries({ queryKey: ['drawings'] }),
      ])
      onDone(result)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : '上传失败，请检查网络后重试')
    } finally {
      pending.current = false
      setBusy(false)
    }
  }

  return (
    <Sheet open title="上传新版本" onClose={() => { if (!pending.current) onClose() }}>
      <form onSubmit={submit} className="space-y-4" noValidate>
        <div className="rounded-xl bg-slate-50 p-3">
          <p className="break-all font-semibold">{file.name}</p>
          <p className="text-sm text-slate-500">{formatBytes(file.size)}</p>
        </div>
        <fieldset disabled={busy} className="space-y-4">
          <Field label="变更说明（可选）" value={note} onChange={(e) => setNote(e.target.value)} placeholder="如 调整精车进给" maxLength={500} />
        </fieldset>
        {busy && <p role="status" className="text-sm text-slate-600">正在上传，请保持页面打开…</p>}
        {error && (
          <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
            {error}
          </p>
        )}
        <Button type="submit" loading={busy} className="w-full">
          上传
        </Button>
      </form>
    </Sheet>
  )
}
