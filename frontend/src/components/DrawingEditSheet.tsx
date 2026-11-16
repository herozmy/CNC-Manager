import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { api } from '../api'
import { ApiError } from '../api/client'
import { priceError as validatePrice } from '../price'
import type { Drawing, DrawingDetail, DrawingInput } from '../api/types'
import DrawingFields from './DrawingFields'
import { Button } from './ui/Button'
import { Sheet } from './ui/Sheet'

interface Props {
  drawing: Drawing
  onClose: () => void
  onSaved: () => void
}

export default function DrawingEditSheet({ drawing, onClose, onSaved }: Props) {
  const qc = useQueryClient()
  // 已停用字段沿用原值，编辑其他信息时不会清空历史数据。
  const [initial] = useState<DrawingInput>(() => ({
    drawingNo: drawing.drawingNo, name: drawing.name, material: drawing.material, remark: drawing.remark,
    customer: drawing.customer, drawingVersion: drawing.drawingVersion,
    price: drawing.price ?? '',
  }))
  const [form, setForm] = useState(initial)
  const [noError, setNoError] = useState('')
  const [priceError, setPriceError] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const saving = useRef(false)
  const [discard, setDiscard] = useState(false)
  const dirty = Object.keys(initial).some((key) => form[key as keyof DrawingInput] !== initial[key as keyof DrawingInput])

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
    setNoError('')
    setPriceError('')
    setError('')
    if (!form.drawingNo.trim()) return setNoError('请输入图纸号')
    const invalidPrice = validatePrice(form.price)
    if (invalidPrice) return setPriceError(invalidPrice)
    saving.current = true
    setBusy(true)
    try {
      const updated = await api.updateDrawing(drawing.id, form)
      qc.setQueryData<DrawingDetail>(['drawing', drawing.id], (previous) => previous ? { ...previous, ...updated } : undefined)
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['drawing', drawing.id] }),
        qc.invalidateQueries({ queryKey: ['drawings'] }),
      ])
      onSaved()
    } catch (err) {
      const message = err instanceof ApiError ? err.message : '无法连接服务器，请重试'
      if (err instanceof ApiError && err.status === 409) setNoError(message.replace(/^.*?:\s*/, ''))
      else setError(message)
    } finally {
      saving.current = false
      setBusy(false)
    }
  }

  return <Sheet open title="编辑图纸" onClose={close}>
    <form onSubmit={submit} className="space-y-4" noValidate>
      <fieldset disabled={busy || discard} className="space-y-4">
        <DrawingFields value={form} onChange={setForm} noError={noError} priceError={priceError} />
      </fieldset>
      {error && <p role="alert" className="rounded-lg bg-danger-soft px-3 py-2 text-sm text-danger">{error}</p>}
      <div className="grid grid-cols-2 gap-3">
        <Button variant="secondary" disabled={busy || discard} onClick={close}>取消</Button>
        <Button type="submit" loading={busy} disabled={discard}>保存修改</Button>
      </div>
    </form>
    {discard && <div className="mt-4 space-y-3 rounded-md bg-warning-soft p-3">
      <p>有未保存的修改，确定放弃吗？</p>
      <div className="flex gap-2">
        <Button variant="secondary" onClick={() => setDiscard(false)}>继续编辑</Button>
        <Button variant="danger" onClick={onClose}>放弃修改</Button>
      </div>
    </div>}
  </Sheet>
}
