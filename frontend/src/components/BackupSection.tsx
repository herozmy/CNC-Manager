import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { Archive, Upload } from 'lucide-react'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { BackupPreview, BackupRecovery } from '../api/types'
import { setSession } from '../auth/session'
import { Button } from './ui/Button'
import { Sheet } from './ui/Sheet'

const errorMessage = (e: unknown) => e instanceof ApiError ? e.message : '连接中断，请确认服务器状态后重试'
const date = (value: string) => new Date(value).toLocaleString('zh-CN', { hour12: false })

function saveDownload(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  document.body.appendChild(a)
  a.click()
  a.remove()
  // Mobile browsers may consume the URL after the click handler has finished.
  window.setTimeout(() => URL.revokeObjectURL(url), 60_000)
}

export default function BackupSection() {
  const [open, setOpen] = useState(false)
  const [deleting, setDeleting] = useState<BackupRecovery | null>(null)
  const [deleteError, setDeleteError] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const pending = useRef(false)
  const recoveries = useQuery({ queryKey: ['backup-recoveries'], queryFn: api.backupRecoveries })

  async function create() {
    if (pending.current) return
    pending.current = true; setBusy(true); setError(''); setNotice('')
    try {
      await api.createBackup()
      setNotice('备份已保存到服务器，需要时请在下方列表点击“下载”。')
      await recoveries.refetch()
    } catch (e) { setError(errorMessage(e)) }
    finally { pending.current = false; setBusy(false) }
  }

  async function download(name: string) {
    if (pending.current) return
    pending.current = true
    setBusy(true); setError(''); setNotice('')
    try {
      const blob = await api.downloadRecovery(name)
      saveDownload(blob, name)
      setNotice('备份已准备好，请在浏览器下载记录或手机“文件”中确认保存。')
    } catch (e) { setError(errorMessage(e)) }
    finally { pending.current = false; setBusy(false) }
  }

  async function remove() {
    if (!deleting || pending.current) return
    pending.current = true; setBusy(true); setDeleteError(''); setNotice(''); setError('')
    try {
      await api.deleteRecovery(deleting.name)
      setDeleting(null)
      setNotice('备份已删除。')
      await recoveries.refetch()
    } catch (e) { setDeleteError(errorMessage(e)) }
    finally { pending.current = false; setBusy(false) }
  }

  return <section className="space-y-3 rounded-2xl border border-slate-200 bg-white p-4" aria-label="备份与迁移">
    <h2 className="text-lg font-bold">备份与迁移</h2>
    <p className="text-sm leading-6 text-slate-600">一份备份包含全部账号、图纸、工序、程序版本、刀具表和 NC 文件，可导入另一台服务器。</p>
    <div className="grid grid-cols-2 gap-3">
      <Button variant="secondary" className="px-2" loading={busy} onClick={create}><Archive className="size-4 shrink-0" aria-hidden />创建备份</Button>
      <Button variant="secondary" className="px-2" disabled={busy} onClick={() => setOpen(true)}><Upload className="size-4 shrink-0" aria-hidden />导入备份</Button>
    </div>
    <p className="text-xs leading-5 text-slate-500">仅管理员可操作。备份包含账号及文件解密密钥，请妥善保管；未保存的编辑不会包含在备份中。</p>
    {busy && <p role="status" className="text-sm text-slate-600">正在处理，请保持页面打开。数据较多时需要稍候。</p>}
    {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
    {notice && <p role="status" className="text-sm text-green-800">{notice}</p>}
    {recoveries.isSuccess && !recoveries.data.length && <p className="text-sm text-slate-500">暂无可恢复备份，创建成功后会自动显示在这里。</p>}
    {!!recoveries.data?.length && <details className="border-t border-slate-100 pt-3">
      <summary className="min-h-10 cursor-pointer text-sm font-semibold">可恢复备份（{recoveries.data.length}）</summary>
      <p className="mb-2 text-xs leading-5 text-slate-500">备份成功后保存在服务器。需要恢复时，下载对应备份，再使用“导入备份”检查并确认覆盖。</p>
      <ul className="max-h-64 space-y-3 overflow-y-auto">
        {recoveries.data.map((item) => <li key={item.name} className="flex items-center justify-between gap-2 text-sm">
          <span>{item.name.startsWith('manual-') ? '手动备份' : '导入前备份'} · {date(item.createdAt)}<span className="block text-xs text-slate-500">{(item.sizeBytes / 1048576).toFixed(2)} MB</span></span>
          <div className="flex shrink-0 gap-2"><Button variant="secondary" className="shrink-0 px-3 text-sm" disabled={busy} onClick={() => download(item.name)} aria-label={`下载 ${date(item.createdAt)} 的恢复副本`}>下载</Button>
          <Button variant="danger" className="px-3 text-sm" disabled={busy} onClick={() => { setDeleting(item); setDeleteError('') }} aria-label={`删除 ${date(item.createdAt)} 的备份`}>删除</Button></div>
        </li>)}
      </ul>
    </details>}
    {recoveries.isError && <p className="text-sm text-slate-500">恢复副本列表暂时不可用。<button className="ml-2 underline" onClick={() => recoveries.refetch()}>重试</button></p>}
    {deleting && <Sheet open title="删除备份" onClose={() => { if (!pending.current) setDeleting(null) }}>
      <div className="space-y-4">
        <p className="text-sm leading-6">确定删除 {date(deleting.createdAt)} 的{deleting.name.startsWith('manual-') ? '手动备份' : '导入前备份'}？</p>
        <p className="break-all text-xs text-slate-500">{deleting.name}</p>
        <p className="text-sm leading-6 text-red-700">删除后无法从服务器恢复此备份。当前业务数据和已下载到设备的副本不受影响。</p>
        {deleteError && <p role="alert" className="text-sm text-red-600">{deleteError}</p>}
        <div className="grid grid-cols-2 gap-3">
          <Button variant="secondary" disabled={busy} onClick={() => setDeleting(null)}>取消</Button>
          <Button variant="danger" loading={busy} disabled={busy} onClick={remove}>确认删除</Button>
        </div>
      </div>
    </Sheet>}
    {open && <ImportSheet onClose={() => { setOpen(false); void recoveries.refetch() }} />}
  </section>
}

function ImportSheet({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient()
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<BackupPreview | null>(null)
  const [confirmed, setConfirmed] = useState(false)
  const [busy, setBusy] = useState<'check' | 'import' | null>(null)
  const [error, setError] = useState('')
  const pending = useRef(false)
  const dirty = qc.getQueriesData<{ dirty: boolean }>({ queryKey: ['version-tools-draft'] }).some(([, value]) => value?.dirty)

  useEffect(() => {
    const warn = (e: BeforeUnloadEvent) => { if (pending.current) { e.preventDefault(); e.returnValue = '' } }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [])

  async function check() {
    if (!file || pending.current) return
    setError(''); setPreview(null); setConfirmed(false)
    if (!file.name.toLowerCase().endsWith('.zip') || file.size === 0 || file.size > 512 * 1048576) {
      setError('请选择不超过 512 MB 的备份 ZIP 文件'); return
    }
    pending.current = true; setBusy('check')
    try { setPreview(await api.inspectBackup(file)) }
    catch (e) { setError(errorMessage(e)) }
    finally { pending.current = false; setBusy(null) }
  }

  async function restore() {
    if (!file || !preview || !confirmed || dirty || pending.current) return
    pending.current = true; setBusy('import'); setError('')
    try {
      await api.importBackup(file, preview.sha256)
      await qc.cancelQueries()
      setSession(qc, { state: 'out', notice: '备份导入成功，请使用备份中的账号和密码登录。导入前的数据已自动保留为恢复副本。' })
    } catch (e) { setError(errorMessage(e)) }
    finally { pending.current = false; setBusy(null) }
  }

  return <Sheet open title="导入备份" onClose={() => { if (!pending.current) onClose() }}>
    <div className="space-y-4">
      <p className="text-sm leading-6 text-slate-600">选择本系统导出的 ZIP 文件（最大 512 MB）。先检查备份，确认后再导入。</p>
      <label className="block space-y-2 text-sm font-semibold">备份文件
        <input type="file" accept=".zip,application/zip" disabled={!!busy} className="block w-full min-w-0 rounded-xl border border-slate-300 p-3 text-sm file:mr-3 file:rounded-lg file:border-0 file:bg-slate-100 file:px-3 file:py-2" onChange={(e) => { setFile(e.target.files?.[0] || null); setPreview(null); setConfirmed(false); setError('') }} />
      </label>
      {file && <p className="break-all text-xs text-slate-500">{file.name} · {(file.size / 1048576).toFixed(2)} MB</p>}
      {!preview && <Button className="w-full" disabled={!file || !!busy} loading={busy === 'check'} onClick={check}>检查备份</Button>}
      {preview && <>
        <div className="rounded-xl bg-slate-50 p-3 text-sm leading-6">
          <p className="font-semibold text-green-800">备份检查通过</p>
          <p>备份时间：{date(preview.createdAt)}</p>
          <p className="break-all">程序版本：{preview.version}</p>
          <p>账号 {preview.summary.users} · 图纸 {preview.summary.drawings} · 工序 {preview.summary.operations}</p>
          <p>程序 {preview.summary.programs} · 版本 {preview.summary.versions} · 刀具记录 {preview.summary.tools} · NC 文件 {preview.summary.files}</p>
          <p className="break-all">可登录管理员：{preview.summary.admins.join('、')}</p>
        </div>
        <div className="rounded-xl bg-amber-50 p-3 text-sm leading-6 text-amber-950">
          <p className="font-semibold">将覆盖当前服务器的全部账号和业务数据。</p>
          <p>导入前自动生成恢复副本；导入后所有设备退出登录，需要使用备份中的账号和密码重新登录。导入期间请暂停其他设备上的编辑。</p>
        </div>
        <label className="flex items-start gap-3 text-sm leading-6">
          <input type="checkbox" className="mt-1 size-5 shrink-0 accent-amber-500" checked={confirmed} disabled={!!busy} onChange={(e) => setConfirmed(e.target.checked)} />
          我确认覆盖当前数据，并且知道备份中的管理员账号和密码
        </label>
      </>}
      {dirty && <p role="alert" className="text-sm text-red-600">请先保存或撤销未保存的刀具表修改，再导入备份。</p>}
      {busy && <p role="status" className="text-sm text-slate-600">{busy === 'check' ? '正在上传并检查备份…' : '正在备份当前数据并导入，请勿关闭页面…'}</p>}
      {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
      <div className="grid grid-cols-2 gap-3">
        <Button variant="secondary" disabled={!!busy} onClick={onClose}>取消</Button>
        {preview && <Button variant="danger" disabled={!confirmed || dirty || !!busy} loading={busy === 'import'} onClick={restore}>确认覆盖导入</Button>}
      </div>
    </div>
  </Sheet>
}
