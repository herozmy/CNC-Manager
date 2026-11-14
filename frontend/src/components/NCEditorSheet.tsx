import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { VersionContent, VersionSaveResult } from '../api/types'
import { formatBytes } from '../lib/format'
import type { NCCodeEditorHandle } from './NCCodeEditor'
import { Button } from './ui/Button'
import { Field } from './ui/Field'
import { Sheet } from './ui/Sheet'
import { ErrorState, Spinner } from './ui/State'

const NCCodeEditor = lazy(() => import('./NCCodeEditor'))

interface Props {
  versionId: number
  drawingId: number
  programId: number
  onClose: () => void
  onSaved: (result: VersionSaveResult) => void
}

export default function NCEditorSheet(props: Props) {
  const [epoch, setEpoch] = useState(0)
  const query = useQuery({ queryKey: ['version-content', props.versionId], queryFn: () => api.versionContent(props.versionId), retry: false, refetchOnWindowFocus: false, refetchOnMount: 'always' })
  if (query.data && query.isFetchedAfterMount) return <Editor key={epoch} {...props} initial={query.data} onReload={async () => {
    const latest = await query.refetch()
    if (latest.error) throw latest.error
    setEpoch((n) => n + 1)
  }} />
  return <Sheet open title="NC 内容" onClose={props.onClose}>
    {query.isPending ? <Spinner /> : <ErrorState message="NC 内容加载失败" onRetry={() => void query.refetch()} />}
  </Sheet>
}

function Editor({ initial: loaded, drawingId, programId, onClose, onSaved, onReload }: Props & { initial: VersionContent; onReload: () => Promise<void> }) {
  // 固定本次打开的内容和校验值，后台刷新不能改变草稿的覆盖依据。
  const [initial] = useState(loaded)
  const qc = useQueryClient()
  const editor = useRef<NCCodeEditorHandle>(null)
  const [editing, setEditing] = useState(false)
  const [contentDirty, setContentDirty] = useState(false)
  const [note, setNote] = useState('')
  const [conflict, setConflict] = useState(false)
  const [confirm, setConfirm] = useState<'discard' | 'overwrite' | 'reload' | null>(null)
  const dirty = contentDirty || note !== ''
  const reload = useMutation({ mutationFn: onReload, onError: () => setConfirm(null) })
  const save = useMutation({
    mutationFn: (overwrite: boolean) => {
      const content = editor.current?.getValue() ?? initial.content
      const text = !contentDirty ? initial.content : initial.content.includes('\r\n') ? content.replace(/\r?\n/g, '\r\n') : content
      const input = { content: text, encoding: initial.encoding, fileName: initial.fileName, changeNote: note.trim() }
      return overwrite ? api.overwriteContent(initial.versionId, { ...input, expectedSHA256: initial.sha256 }) : api.saveContentAsNew(programId, input)
    },
    onSuccess: async (result) => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['versions', programId] }),
        qc.invalidateQueries({ queryKey: ['drawing', drawingId] }),
        qc.invalidateQueries({ queryKey: ['version-content', initial.versionId] }),
        qc.invalidateQueries({ queryKey: ['version-diff'] }),
      ])
      onSaved(result)
    },
    onError: (error) => {
      if (error instanceof ApiError && error.status === 409) setConflict(true)
      setConfirm(null)
    },
  })
  useEffect(() => {
    if (!dirty) return
    const warn = (e: BeforeUnloadEvent) => { e.preventDefault(); e.returnValue = '' }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])
  const close = () => {
    if (save.isPending || reload.isPending) return
    if (dirty) setConfirm('discard')
    else onClose()
  }
  return <Sheet open wide title={`NC 内容 · V${initial.versionNo}`} onClose={close}>
    <div className="space-y-4">
      <p className="break-all text-sm text-slate-600">{initial.fileName} · {initial.encoding.toUpperCase()} · {formatBytes(initial.sizeBytes)}</p>
      <Suspense fallback={<Spinner />}><NCCodeEditor ref={editor} initialValue={initial.content} readOnly={!editing || save.isPending || reload.isPending} onDirtyChange={setContentDirty} /></Suspense>
      {!editing ? <Button onClick={() => setEditing(true)}>编辑内容</Button> : <>
        <Field label="变更说明（另存新版本时生效）" value={note} disabled={save.isPending || reload.isPending} onChange={(e) => setNote(e.target.value)} />
        <p className="text-sm text-slate-600">另存的新版本会自动设为当前版本。保存时沿用原文件名与编码。</p>
        <div className="grid gap-2 sm:grid-cols-2">
          <Button disabled={!dirty || save.isPending || reload.isPending || !!confirm} onClick={() => save.mutate(false)}>另存新版本</Button>
          <Button variant="secondary" disabled={!dirty || save.isPending || reload.isPending || !!confirm || conflict} onClick={() => setConfirm('overwrite')}>覆盖此版本</Button>
        </div>
      </>}
      {save.isPending && <p role="status">正在保存…</p>}
      {save.isError && <p role="alert" className="text-red-700">保存失败：{save.error.message}</p>}
      {conflict && <div className="space-y-2 rounded-xl bg-amber-50 p-3 text-sm">
        <p>此版本已被修改，草稿已保留。可以另存新版本，或放弃草稿并载入最新内容。</p>
        <Button variant="secondary" disabled={save.isPending || reload.isPending} onClick={() => setConfirm('reload')}>载入最新内容</Button>
      </div>}
      {reload.isError && <p role="alert" className="text-red-700">载入失败，草稿已保留：{reload.error.message}</p>}
      {confirm && <div className="space-y-3 rounded-xl bg-amber-50 p-3">
        <p>{confirm === 'discard' ? '有未保存的修改，确定放弃吗？' : confirm === 'reload' ? '确定放弃当前草稿并载入最新内容吗？' : `确定覆盖 V${initial.versionNo} 吗？此版本原内容将被替换，且不新增版本。`}</p>
        <div className="flex gap-2">
          <Button variant="secondary" disabled={save.isPending || reload.isPending} onClick={() => setConfirm(null)}>继续编辑</Button>
          <Button variant="danger" loading={save.isPending || reload.isPending} onClick={() => confirm === 'discard' ? onClose() : confirm === 'reload' ? reload.mutate() : save.mutate(true)}>{confirm === 'discard' ? '放弃修改' : confirm === 'reload' ? '放弃并载入' : '确认覆盖'}</Button>
        </div>
      </div>}
    </div>
  </Sheet>
}
