import { useQueryClient } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { DrawingInput } from '../api/types'
import { Button } from '../components/ui/Button'
import { Field, TextArea } from '../components/ui/Field'

const empty: DrawingInput = { drawingNo: '', name: '', customer: '', material: '', drawingVersion: '', remark: '' }

/** 新建图纸：只有图纸号必填，其余可以之后补。 */
export default function NewDrawingPage() {
  const nav = useNavigate()
  const qc = useQueryClient()
  const [form, setForm] = useState<DrawingInput>(empty)
  const [noError, setNoError] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const set = (key: keyof DrawingInput) => (e: { target: { value: string } }) => setForm((f) => ({ ...f, [key]: e.target.value }))

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setNoError('')
    if (!form.drawingNo.trim()) return setNoError('请输入图纸号')
    setBusy(true)
    try {
      const d = await api.createDrawing(form)
      await qc.invalidateQueries({ queryKey: ['drawings'] })
      nav(`/drawings/${d.id}`, { replace: true })
    } catch (err) {
      const message = err instanceof ApiError ? err.message : '无法连接服务器'
      // 图纸号重复是最常见的错误，直接标在对应输入框上
      if (err instanceof ApiError && err.status === 409) setNoError(message.replace(/^.*?:\s*/, ''))
      else setError(message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="mx-auto max-w-xl px-4 pt-4 pb-8">
      <header className="mb-4 flex items-center gap-2">
        <Link to="/" aria-label="返回总览" className="flex size-12 items-center justify-center rounded-xl hover:bg-slate-200">
          <ArrowLeft className="size-6" aria-hidden />
        </Link>
        <h1 className="text-2xl font-bold">新建图纸</h1>
      </header>
      <form onSubmit={submit} className="space-y-4" noValidate>
        <Field label="图纸号" value={form.drawingNo} onChange={set('drawingNo')} error={noError} autoFocus autoCapitalize="characters" enterKeyHint="next" />
        <Field label="名称" value={form.name} onChange={set('name')} enterKeyHint="next" />
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="客户" value={form.customer} onChange={set('customer')} enterKeyHint="next" />
          <Field label="材料" value={form.material} onChange={set('material')} enterKeyHint="next" />
        </div>
        <Field label="图纸版本" value={form.drawingVersion} onChange={set('drawingVersion')} enterKeyHint="next" />
        <TextArea label="备注" value={form.remark} onChange={set('remark')} />
        {error && (
          <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
            {error}
          </p>
        )}
        <Button type="submit" loading={busy} className="w-full">
          创建图纸
        </Button>
      </form>
    </main>
  )
}
