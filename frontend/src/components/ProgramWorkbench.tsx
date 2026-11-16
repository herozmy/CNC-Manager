import { useQuery } from '@tanstack/react-query'
import { Check, Code2, Copy, GitCompare, Wrench } from 'lucide-react'
import { lazy, Suspense, useState } from 'react'
import { api } from '../api'
import { useAccess } from '../auth/permissions'
import type { Version } from '../api/types'
import VersionToolTable from './VersionToolTable'
import { VersionDiffPanel } from './VersionDiffSheet'
import { Button } from './ui/Button'
import { ErrorState, Spinner } from './ui/State'

const NCCodeEditor = lazy(() => import('./NCCodeEditor'))
const ignoreDirtyChange = () => {}

function CodePreview({ version, onEdit }: { version: Version; onEdit: () => void }) {
  const access = useAccess()
  const [loaded, setLoaded] = useState(false)
  const [copyMessage, setCopyMessage] = useState('')
  const content = useQuery({ queryKey: ['version-content', version.id], queryFn: () => api.versionContent(version.id), enabled: loaded, retry: false, refetchOnWindowFocus: false })
  async function copy() {
    try {
      if (!content.data || !navigator.clipboard?.writeText) throw new Error('Clipboard unavailable')
      await navigator.clipboard.writeText(content.data.content)
      setCopyMessage('已复制程序')
    } catch { setCopyMessage('复制失败，请选择代码后手动复制') }
  }
  return <div className="min-w-0 space-y-3 p-4">
    <div className="flex flex-wrap items-center justify-between gap-2">
      <p className="font-mono text-xs text-muted">{version.fileName} · {version.encoding.toUpperCase()} · V{version.versionNo}</p>
      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" onClick={onEdit}>打开 NC 编辑器</Button>
        {loaded && content.data && access.can('download') && <Button variant="secondary" onClick={() => void copy()}>{copyMessage === '已复制程序' ? <Check className="size-4" aria-hidden /> : <Copy className="size-4" aria-hidden />}复制程序</Button>}
      </div>
    </div>
    {copyMessage && <p role="status" className="text-sm text-muted">{copyMessage}</p>}
    {!loaded ? <div className="rounded-md border border-dashed border-line-strong bg-canvas px-4 py-6 text-center">
      <p className="mb-3 text-sm text-muted">加载代码后，可在此核对程序、查找内容和跳转行号。</p>
      <Button variant="secondary" onClick={() => setLoaded(true)}>加载 NC 预览</Button>
    </div> : content.isPending ? <Spinner label="正在加载 NC 代码…" /> : content.isError ? <ErrorState message="NC 预览加载失败" onRetry={() => void content.refetch()} /> : content.data && <Suspense fallback={<Spinner />}>
      <NCCodeEditor label="NC 程序预览" initialValue={content.data.content} readOnly onDirtyChange={ignoreDirtyChange} />
    </Suspense>}
  </div>
}

export default function ProgramWorkbench({ version, programId, canCompare, onEdit }: { version: Version; programId: number; canCompare: boolean; onEdit: () => void }) {
  const [tab, setTab] = useState<'editor' | 'tools' | 'diff'>('editor')
  const tabs = [
    { id: 'editor', label: 'NC 代码', icon: Code2 },
    { id: 'tools', label: '版本刀具表', icon: Wrench },
    { id: 'diff', label: '版本差异', icon: GitCompare },
  ] as const
  return <section aria-label="程序工作台" className="panel min-w-0 overflow-hidden">
    <div role="tablist" aria-label="程序工作台视图" className="flex min-w-0 flex-wrap items-center gap-1 border-b border-line p-2">
      {tabs.map(({ id, label, icon: Icon }) => <button type="button" role="tab" id={`workbench-tab-${id}`} aria-controls={`workbench-panel-${id}`} aria-selected={tab === id} disabled={id === 'diff' && !canCompare} key={id} onClick={() => setTab(id)} onKeyDown={event => {
        if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
        event.preventDefault()
        const available = tabs.filter(item => item.id !== 'diff' || canCompare)
        const index = available.findIndex(item => item.id === id)
        const next = event.key === 'Home' ? available[0] : event.key === 'End' ? available[available.length - 1] : available[(index + (event.key === 'ArrowRight' ? 1 : -1) + available.length) % available.length]
        setTab(next.id)
        document.getElementById(`workbench-tab-${next.id}`)?.focus()
      }} className={`flex min-h-12 items-center gap-1.5 rounded-md px-3 text-sm font-medium disabled:opacity-40 focus-visible:outline-2 focus-visible:outline-accent ${tab === id ? 'bg-accent-soft text-accent' : 'text-muted hover:bg-hover'}`}><Icon className="size-4 shrink-0" aria-hidden />{label}</button>)}
    </div>
    <p className={`border-b border-line px-4 py-2 text-xs ${version.isCurrent ? 'bg-success-soft text-success' : 'bg-warning-soft text-warning'}`}>{version.isCurrent ? '当前有效版本' : '正在浏览历史版本'} · V{version.versionNo} · {version.encoding.toUpperCase()}</p>
    <div id="workbench-panel-editor" role="tabpanel" aria-labelledby="workbench-tab-editor" hidden={tab !== 'editor'}>
      <CodePreview key={version.id} version={version} onEdit={onEdit} />
    </div>
    {/* Keep the tool form mounted so tab changes retain unsaved rows and focus state. */}
    <div id="workbench-panel-tools" role={tab === 'tools' ? 'tabpanel' : undefined} aria-labelledby="workbench-tab-tools" hidden={tab === 'diff'} className="border-t border-line p-4 [&>section]:mt-0 [&>section]:border-0 [&>section]:p-0">
      <VersionToolTable version={version} />
    </div>
    {tab === 'diff' && canCompare && <div id="workbench-panel-diff" role="tabpanel" aria-labelledby="workbench-tab-diff" className="min-w-0 p-4"><VersionDiffPanel key={version.id} programId={programId} versionId={version.id} /></div>}
  </section>
}
