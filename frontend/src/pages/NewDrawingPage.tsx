import { useQueryClient } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../api'
import { ApiError } from '../api/client'
import { priceError as validatePrice } from '../price'
import type { DrawingInput } from '../api/types'
import DrawingFields from '../components/DrawingFields'
import { useAccess } from '../auth/permissions'
import OwnerSelect from '../components/OwnerSelect'
import { Button } from '../components/ui/Button'

const empty: DrawingInput = { drawingNo: '', name: '', customer: '', material: '', price: '', drawingVersion: '', remark: '' }

/** 新建图纸：只有图纸号必填，其余可以之后补。 */
export default function NewDrawingPage() {
  const access = useAccess()
  const nav = useNavigate()
  const qc = useQueryClient()
  const [form, setForm] = useState<DrawingInput>({ ...empty, ...(access.admin ? { ownerUserId: access.user?.id } : {}) })
  const [noError, setNoError] = useState('')
  const [priceError, setPriceError] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (busy || !access.can('create')) return
    setError('')
    setNoError('')
    setPriceError('')
    if (access.admin && !form.ownerUserId) return setError('请选择所属账号')
    if (!form.drawingNo.trim()) return setNoError('请输入图纸号')
    const invalidPrice = validatePrice(form.price)
    if (invalidPrice) return setPriceError(invalidPrice)
    setBusy(true)
    try {
      const d = await api.createDrawing(form)
      await qc.invalidateQueries({ queryKey: ['drawings'] })
      nav(access.can('read') ? `/drawings/${d.id}` : '/', { replace: true })
    } catch (err) {
      const message = err instanceof ApiError ? err.message : '无法连接服务器'
      // 图纸号重复是最常见的错误，直接标在对应输入框上
      if (err instanceof ApiError && err.status === 409) setNoError(message.replace(/^.*?:\s*/, ''))
      else setError(message)
    } finally {
      setBusy(false)
    }
  }

  if (!access.can('create')) return <main className="p-6">没有新建权限，请联系管理员。</main>

  return (
    <main className="page-width px-4 pt-4 pb-8 md:px-6">
      <header className="mb-4 flex items-center gap-2">
        <Link to="/" aria-label="返回总览" className="flex size-12 items-center justify-center rounded-md hover:bg-hover">
          <ArrowLeft className="size-6" aria-hidden />
        </Link>
        <h1 className="text-2xl font-semibold tracking-tight">新建图纸</h1>
      </header>
      <form onSubmit={submit} className="panel max-w-4xl space-y-5 overflow-hidden" noValidate>
        <fieldset disabled={busy} className="min-w-0 space-y-5 px-4 pt-5 sm:px-7 sm:pt-6">
          <legend className="float-left mb-4 w-full border-b border-line pb-3 text-sm font-semibold text-accent">图纸资料</legend>
          {access.admin && <OwnerSelect value={form.ownerUserId ?? ''} onChange={(ownerUserId) => setForm({ ...form, ownerUserId: ownerUserId || undefined })} required />}
          <DrawingFields value={form} onChange={setForm} noError={noError} priceError={priceError} />
        </fieldset>
        {error && (
          <p role="alert" className="rounded-lg bg-danger-soft px-3 py-2 text-sm text-danger">
            {error}
          </p>
        )}
        <div className="flex justify-end border-t border-line bg-inset px-4 py-3 sm:px-7"><Button type="submit" loading={busy} className="w-full sm:w-auto">
          创建图纸
        </Button></div>
      </form>
    </main>
  )
}
