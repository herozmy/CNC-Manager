import { useQueryClient } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import { ApiError } from '../api/client'
import { Button } from './ui/Button'
import { Sheet } from './ui/Sheet'

interface Props {
  title: string
  message: string
  remove: () => Promise<void>
  onDeleted: () => void
  onClose: () => void
}

/** All three catalog levels share confirmation, retry and in-flight protection. */
export default function DeleteSheet({ title, message, remove, onDeleted, onClose }: Props) {
  const qc = useQueryClient()
  const pending = useRef(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const close = () => { if (!pending.current) onClose() }
  async function confirm() {
    if (pending.current) return
    pending.current = true
    setBusy(true)
    setError('')
    try {
      try { await remove() } catch (err) {
        // Another client may already have deleted this object.
        if (!(err instanceof ApiError && err.status === 404)) throw err
      }
      // Mark descendant caches stale too, so browser Back cannot revive deleted records.
      await qc.invalidateQueries({ predicate: (q) => ['drawings', 'drawing', 'versions', 'version-content'].includes(String(q.queryKey[0])), refetchType: 'none' })
      onDeleted()
      void qc.refetchQueries({ predicate: (q) => ['drawings', 'drawing'].includes(String(q.queryKey[0])), type: 'active' })
    } catch (err) {
      setError(err instanceof Error ? err.message : '无法连接服务器')
    } finally {
      pending.current = false
      setBusy(false)
    }
  }
  return <Sheet open title={title} onClose={close}>
    <p className="mb-5 text-slate-700">{message}</p>
    {error && <p role="alert" className="mb-4 rounded-xl bg-red-50 p-3 text-red-700">删除失败：{error}</p>}
    <div className="grid grid-cols-2 gap-3">
      <Button variant="secondary" disabled={busy} onClick={close}>取消</Button>
      <Button variant="danger" loading={busy} onClick={() => void confirm()}>确认删除</Button>
    </div>
  </Sheet>
}
