import { useAccess } from '../auth/permissions'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Download, Pencil, Upload } from 'lucide-react'
import { useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { UploadResult, Version } from '../api/types'
import DeleteSheet from '../components/DeleteSheet'
import NCEditorSheet from '../components/NCEditorSheet'
import UploadSheet from '../components/UploadSheet'
import ProgramSheet from '../components/ProgramSheet'
import VersionToolTable from '../components/VersionToolTable'
import VersionDiffSheet from '../components/VersionDiffSheet'
import { Button } from '../components/ui/Button'
import { ConfirmSheet } from '../components/ui/Sheet'
import { EmptyState, ErrorState, Spinner } from '../components/ui/State'
import { formatBytes, relativeTime } from '../lib/format'
import { VersionBadge } from './DrawingPage'

function VersionDetails({ v, onActivate, onView, onCompare }: { v: Version; onActivate: () => void; onView: () => void; onCompare?: () => void }) {
  const access = useAccess()
  return (
    <section aria-label={`V${v.versionNo} 版本详情`} className={`rounded-2xl border p-4 ${v.isCurrent ? 'border-amber-400 bg-amber-50' : 'border-slate-200 bg-white'}`}>
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="text-lg font-bold">V{v.versionNo}</span>
          {v.isCurrent && <span className="rounded-full bg-amber-500 px-2.5 py-0.5 text-xs font-semibold text-slate-900">当前</span>}
        </div>
        <span className="text-xs text-slate-500">{relativeTime(v.createdAt)}</span>
      </div>
      <p className="mt-1 break-all text-slate-800">{v.fileName}</p>
      <p className="text-sm text-slate-500">
        {formatBytes(v.fileSize)} · {v.encoding === 'gbk' ? 'GBK' : 'UTF-8'}
        {v.createdBy && ` · ${v.createdBy}`}
      </p>
      {v.changeNote && <p className="mt-1 text-sm text-slate-700">{v.changeNote}</p>}
      <Button variant="secondary" className="mt-3 w-full" onClick={onView} aria-label={`查看 V${v.versionNo} 内容`}>查看 / 编辑 NC</Button>
      {onCompare && <Button variant="secondary" className="mt-2 w-full" onClick={onCompare}>版本对比</Button>}
      <div className="mt-3 grid grid-cols-2 gap-2">
        {access.can('download') && <a
          href={api.downloadUrl(v.id)}
          download
          aria-label={`下载 V${v.versionNo}`}
          className="inline-flex min-h-12 items-center justify-center gap-2 rounded-xl border border-slate-300 bg-white font-semibold active:bg-slate-200"
        >
          <Download className="size-5" aria-hidden />
          下载
        </a>}
        {v.isCurrent || !access.can('edit') ? (
          <span />
        ) : (
          <Button variant="secondary" onClick={onActivate} aria-label={`将 V${v.versionNo} 设为当前`}>
            设为当前
          </Button>
        )}
      </div>
    </section>
  )
}

/** 程序页：当前版本一键下载、上传新版本、版本历史与回滚。 */
export default function ProgramPage() {
  const access = useAccess()
  const navigate = useNavigate()
  const [deleting, setDeleting] = useState(false)
  const [editing, setEditing] = useState(false)
  const params = useParams()
  const drawingId = Number(params.id)
  const programId = Number(params.programId)
  const valid = [drawingId, programId].every((n) => Number.isInteger(n) && n > 0)
  const qc = useQueryClient()

  const drawing = useQuery({ queryKey: ['drawing', drawingId], queryFn: () => api.drawingDetail(drawingId), enabled: valid && access.can('read'), retry: false })
  const versions = useQuery({ queryKey: ['versions', programId], queryFn: () => api.listVersions(programId), enabled: valid && access.can('read'), retry: false })

  const fileInput = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [uploaded, setUploaded] = useState<UploadResult | null>(null)
  const [viewVersion, setViewVersion] = useState<Version | null>(null)
  const [compareVersion, setCompareVersion] = useState<Version | null>(null)
  const [saved, setSaved] = useState<string | null>(null)
  const [activate, setActivate] = useState<Version | null>(null)
  const [selection, setSelection] = useState<{ programId: number; versionId: number } | null>(null)

  const switchVersion = useMutation({
    mutationFn: (v: Version) => api.setCurrentVersion(programId, v.id),
    onSuccess: async (_, v) => {
      await Promise.all([qc.invalidateQueries({ queryKey: ['versions', programId] }), qc.invalidateQueries({ queryKey: ['drawing', drawingId] })])
      setSelection({ programId, versionId: v.id })
      setActivate(null)
    },
  })

  const op = drawing.data?.operations.find((o) => o.programs.some((p) => p.id === programId))
  const program = op?.programs.find((p) => p.id === programId)
  const current = versions.data?.find((v) => v.isCurrent)
  const selected = selection?.programId === programId ? versions.data?.find((v) => v.id === selection.versionId) : undefined
  const notFound = [drawing.error, versions.error].some((e) => e instanceof ApiError && e.status === 404) || (drawing.isSuccess && !program)
  if (!access.can('read')) return <main className="p-6"><p>账号不可用，请联系管理员。</p><Link to="/" className="mt-4 inline-block">返回总览</Link></main>

  return (
    <main className="mx-auto max-w-3xl px-4 pt-4 pb-8">
      <Link to={`/drawings/${drawingId}`} aria-label="返回图纸" className="mb-2 inline-flex size-12 items-center justify-center rounded-xl hover:bg-slate-200">
        <ArrowLeft className="size-6" aria-hidden />
      </Link>
      {valid && (drawing.isPending || versions.isPending) && !notFound && <Spinner />}
      {(!valid || notFound) && <EmptyState title="程序不存在或已被删除" />}
      {!notFound && (drawing.isError || versions.isError) && <ErrorState message="程序加载失败" onRetry={() => { void drawing.refetch(); void versions.refetch() }} />}

      {program && op && versions.data && (
        <>
          <header className="mb-4">
            <p className="text-sm text-slate-500">
              {drawing.data?.drawingNo} · {op.opNo}# {op.opName}
            </p>
            <div className="flex items-center justify-between gap-3">
              <h1 className="break-words text-2xl font-bold">{program.programNo}</h1>
              <VersionBadge no={current?.versionNo ?? null} />
            </div>
            {(program.programName || program.controller) && <p className="text-slate-600">{[program.programName, program.controller].filter(Boolean).join(' · ')}</p>}
            {program.remark && <p className="mt-2 rounded-xl bg-amber-50 p-3 text-sm text-slate-700">{program.remark}</p>}
            <div className="mt-3 flex flex-wrap gap-2">
              {access.can('edit') && <Button variant="secondary" onClick={() => setEditing(true)}><Pencil className="size-5" aria-hidden />编辑程序信息</Button>}
              {access.can('delete') && <Button variant="ghost" className="text-red-600" onClick={() => setDeleting(true)}>删除程序</Button>}
            </div>
          </header>

          {saved && <p role="status" className="mb-4 rounded-xl bg-green-50 p-3">{saved}</p>}
          {uploaded && (
            <div role="status" className="mb-4 rounded-xl bg-green-50 p-3 text-sm text-green-900">
              <p className="font-semibold">已上传 V{uploaded.versionNo}</p>
              {uploaded.parse && (
                <p>
                  {uploaded.parse.tools.length > 0
                    ? `识别到 ${uploaded.parse.tools.length} 把刀：${uploaded.parse.tools.map((t) => (t.offsetNo ? `${t.toolNo}/${t.offsetNo}` : t.toolNo)).join('、')}`
                    : '未识别到刀具调用'}
                </p>
              )}
              {uploaded.parse?.warnings.map((w) => (
                <p key={w} className="text-amber-800">
                  {w}
                </p>
              ))}
            </div>
          )}

          <div className={`mb-6 grid gap-3 ${current ? 'grid-cols-2 sm:grid-cols-3' : 'sm:grid-cols-2'}`}>
            {current ? (
              <>
                {access.can('download') && <a href={api.downloadUrl(current.id)} download aria-label="下载当前版本" className="inline-flex min-h-14 items-center justify-center gap-2 rounded-xl bg-amber-500 font-semibold text-slate-900 active:bg-amber-600">
                  <Download className="size-5" aria-hidden />
                  下载当前 V{current.versionNo}
                </a>}
                <Button variant="secondary" className="min-h-14" aria-label="查看 / 编辑当前版本" onClick={() => setViewVersion(current)}>
                  查看 / 编辑 V{current.versionNo}
                </Button>
              </>
            ) : (
              <p className="flex min-h-14 items-center justify-center rounded-xl bg-slate-100 text-slate-500">还没有 NC 文件</p>
            )}
            {access.can('edit') && <Button variant="secondary" className={`min-h-14 ${current ? 'col-span-2 sm:col-span-1' : ''}`} onClick={() => fileInput.current?.click()}>
              <Upload className="size-5" aria-hidden />
              上传新版本
            </Button>}
            <input
              ref={fileInput}
              type="file"
              hidden
              aria-label="选择 NC 文件"
              onChange={(e) => {
                const f = e.target.files?.[0]
                if (f) setFile(f)
                e.target.value = '' // 允许再次选择同一个文件
              }}
            />
          </div>

          {versions.data.length === 0 ? (
            <EmptyState title="还没有版本">
              <p className="text-sm text-slate-500">点击「上传新版本」选择 NC 文件</p>
            </EmptyState>
          ) : (
            <div className="space-y-3">
              <label htmlFor="nc-version" className="block text-lg font-bold">版本历史</label>
              <select
                id="nc-version"
                aria-label="选择 NC 程序版本"
                className="h-12 w-full min-w-0 rounded-xl border border-slate-300 bg-white px-3 text-base text-slate-900 focus:outline-2 focus:outline-amber-500"
                value={selected?.id ?? ''}
                onChange={(e) => setSelection(e.target.value ? { programId, versionId: Number(e.target.value) } : null)}
              >
                <option value="">选择版本查看详情（共 {versions.data.length} 个）</option>
                {versions.data.map((v) => (
                  <option key={v.id} value={v.id}>V{v.versionNo}{v.isCurrent ? '（当前）' : ''} · {v.fileName}</option>
                ))}
              </select>
              {selected && <VersionDetails v={selected} onView={() => setViewVersion(selected)} onCompare={versions.data.length > 1 ? () => setCompareVersion(selected) : undefined} onActivate={() => { switchVersion.reset(); setActivate(selected) }} />}
            </div>
          )}
          {(selected ?? current) && <VersionToolTable version={(selected ?? current)!} />}
        </>
      )}

      {editing && program && <ProgramSheet key={program.id} drawingId={drawingId} operationId={program.operationId} program={program} onClose={() => setEditing(false)} onSaved={() => setSaved('程序信息已保存')} />}
      {deleting && program && <DeleteSheet title="删除程序" message={`将删除程序「${program.programNo}」及其全部 ${versions.data?.length ?? 0} 个版本，无法恢复。`} remove={() => api.deleteProgram(programId)} onDeleted={() => {
        for (const v of versions.data ?? []) qc.removeQueries({ queryKey: ['version-tools-draft', v.id], exact: true })
        navigate(`/drawings/${drawingId}`, { replace: true })
      }} onClose={() => setDeleting(false)} />}
      {compareVersion && <VersionDiffSheet programId={programId} versionId={compareVersion.id} onClose={() => setCompareVersion(null)} />}
      {viewVersion && <NCEditorSheet key={viewVersion.id} versionId={viewVersion.id} drawingId={drawingId} programId={programId} onClose={() => setViewVersion(null)} onSaved={(result) => {
        setSaved(`已保存 V${result.versionNo}${result.warning ? `：${result.warning}` : ''}`)
        setSelection({ programId, versionId: result.id })
        setViewVersion(null)
      }} />}
      {file && (
        <UploadSheet
          drawingId={drawingId}
          programId={programId}
          file={file}
          onClose={() => setFile(null)}
          onDone={(r) => {
            setFile(null)
            setUploaded(r)
            setSelection({ programId, versionId: r.id })
          }}
        />
      )}
      <ConfirmSheet
        open={!!activate}
        title="切换当前版本"
        message={`确定将 V${activate?.versionNo} 设为当前版本吗？机床取用的将是这一版。${switchVersion.isError ? '\n切换失败，请重试。' : ''}`}
        confirmLabel="确认切换"
        busy={switchVersion.isPending}
        onConfirm={() => activate && switchVersion.mutate(activate)}
        onClose={() => setActivate(null)}
      />
    </main>
  )
}
