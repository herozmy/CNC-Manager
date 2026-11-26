import { useEffect, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Camera, Upload } from 'lucide-react'
import { api } from '../api'
import { Button } from './ui/Button'
import { Sheet } from './ui/Sheet'

async function preparePhoto(file: File): Promise<Blob> {
  const url = URL.createObjectURL(file)
  try {
    const image = new Image()
    image.src = url
    await image.decode()
    const scale = Math.min(1, 2048 / Math.max(image.naturalWidth, image.naturalHeight))
    const canvas = document.createElement('canvas')
    canvas.width = Math.max(1, Math.round(image.naturalWidth * scale))
    canvas.height = Math.max(1, Math.round(image.naturalHeight * scale))
    const ctx = canvas.getContext('2d')
    if (!ctx) throw Error('无法处理照片')
    ctx.drawImage(image, 0, 0, canvas.width, canvas.height)
    return await new Promise<Blob>((resolve, reject) => canvas.toBlob(blob => blob ? resolve(blob) : reject(Error('无法保存照片')), 'image/jpeg', 0.85))
  } catch {
    throw Error('无法读取这张照片，请重新拍摄或选择 JPEG、PNG 图片。')
  } finally { URL.revokeObjectURL(url) }
}

const photoInputs = [
  { text: '拍照', label: '拍摄装夹照片', icon: Camera, capture: 'environment' },
  { text: '上传照片', label: '上传装夹照片', icon: Upload, capture: undefined },
] as const

export default function OperationPhotos({ operationId, editable }: { operationId: number; editable: boolean }) {
  const cache = useQueryClient()
  const key = ['operation-photos', operationId]
  const photos = useQuery({ queryKey: key, queryFn: () => api.operationPhotos(operationId) })
  const [selected, setSelected] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [retry, setRetry] = useState<File | null>(null)
  const mounted = useRef(false)
  useEffect(() => { mounted.current = true; return () => { mounted.current = false } }, [])
  const photoUrl = (digest: string) => `/api/operations/${operationId}/photos/${digest}`

  async function save(file: File) {
    setBusy(true); setError(''); setRetry(null)
    try {
      const image = await preparePhoto(file)
      if (!mounted.current) return
      const digest = await api.addOperationPhoto(operationId, image)
      if (!mounted.current) return
      cache.setQueryData<string[]>(key, old => [...new Set([...(old ?? []), digest])])
      await photos.refetch()
    } catch (err) {
      setError(err instanceof Error ? err.message : '照片保存失败')
      setRetry(file)
    } finally { setBusy(false) }
  }

  async function remove() {
    if (!selected || !window.confirm('删除这张装夹照片？')) return
    setBusy(true); setError('')
    try {
      await api.deleteOperationPhoto(operationId, selected)
      setSelected(null)
      await photos.refetch()
    } catch (err) { setError(err instanceof Error ? err.message : '删除失败') }
    finally { setBusy(false) }
  }

  return <section aria-label="装夹照片" className="mt-4 space-y-2">
    <div className="flex flex-wrap items-center justify-between gap-2">
      <h3 className="text-sm font-semibold">装夹照片</h3>
      {editable && <div className="flex flex-wrap gap-2">
        {photoInputs.map(({ text, label, icon: Icon, capture }) => <label key={label} className="relative inline-flex min-h-12 items-center gap-2 rounded-md border border-line-strong px-3 text-sm font-semibold focus-within:outline-2 focus-within:outline-accent">
          <Icon aria-hidden className="size-4" />{text}
          <input className="absolute inset-0 size-full cursor-pointer opacity-0" type="file" accept="image/*" capture={capture} aria-label={label} disabled={busy} onChange={e => {
            const file = e.target.files?.[0]
            e.target.value = ''
            if (file) void save(file)
          }} />
        </label>)}
      </div>}
    </div>
    {busy && !selected && <p role="status" className="text-sm text-muted">正在保存照片…</p>}
    {photos.isError && <button className="text-sm underline" onClick={() => photos.refetch()}>照片列表加载失败，点击重试</button>}
    {error && !selected && <p role="alert" className="text-sm text-danger">{error} {retry && <button className="underline" onClick={() => save(retry)}>重试保存</button>}</p>}
    {!!photos.data?.length && <div className="flex flex-wrap gap-2">
      {photos.data.map((digest, i) => <button key={digest} type="button" onClick={() => { setError(''); setSelected(digest) }} aria-label={`查看装夹照片 ${i + 1}`}>
        <img loading="lazy" className="size-20 rounded-md object-cover" src={photoUrl(digest)} alt={`装夹照片 ${i + 1}`} />
      </button>)}
    </div>}
    {selected && <Sheet open title="装夹照片" onClose={() => { if (!busy) setSelected(null) }}>
      <img className="max-h-[65vh] w-full object-contain" src={photoUrl(selected)} alt="装夹照片大图" />
      {error && <p role="alert">{error}</p>}
      {editable && <Button className="mt-3" variant="danger" disabled={busy} onClick={remove}>删除照片</Button>}
    </Sheet>}
  </section>
}
