import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { api } from '../api'
import type { VersionContent, VersionSaveResult } from '../api/types'
import { Button } from './ui/Button'
import { Field, TextArea } from './ui/Field'
import { Sheet } from './ui/Sheet'
import { ErrorState, Spinner } from './ui/State'

interface Props {
  versionId: number
  drawingId: number
  programId: number
  onClose: () => void
  onSaved: (result: VersionSaveResult) => void
}

export default function NCEditorSheet(props: Props) {
  const query = useQuery({ queryKey: ['version-content', props.versionId], queryFn: () => api.versionContent(props.versionId), retry: false })
  if (query.data) return <Editor {...props} initial={query.data} />
  return <Sheet open title="NC 内容" onClose={props.onClose}>
    {query.isPending ? <Spinner /> : <ErrorState message="NC 内容加载失败" onRetry={() => void query.refetch()} />}
  </Sheet>
}

function Editor({ initial, drawingId, programId, onClose, onSaved }: Props & { initial: VersionContent }) {
  const qc = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [content, setContent] = useState(initial.content)
  const [note, setNote] = useState('')
  const [confirm, setConfirm] = useState<'discard' | 'overwrite' | null>(null)
  // textarea normalizes CRLF; preserve the source convention when saving edited text.
  const dirty = content !== initial.content || note !== ''
  const save = useMutation({
    mutationFn: (overwrite: boolean) => {
      const text = initial.content.includes('\r\n') ? content.replace(/\r?\n/g, '\r\n') : content
      const input = { content: text, encoding: initial.encoding, fileName: initial.fileName, changeNote: note.trim() }
      return overwrite ? api.overwriteContent(initial.versionId, input) : api.saveContentAsNew(programId, input)
    },
    onSuccess: async (result) => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['versions', programId] }),
        qc.invalidateQueries({ queryKey: ['drawing', drawingId] }),
        qc.invalidateQueries({ queryKey: ['version-content', initial.versionId] }),
      ])
      onSaved(result)
    },
    onError: () => setConfirm(null),
  })
  useEffect(() => {
    if (!dirty) return
    const warn = (e: BeforeUnloadEvent) => { e.preventDefault(); e.returnValue = '' }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])
  const close = () => {
    if (save.isPending) return
    if (dirty) setConfirm('discard')
    else onClose()
  }
  return <Sheet open title={`NC 内容 · V${initial.versionNo}`} onClose={close}>
    <div className="space-y-4">
      <p className="break-all text-sm text-slate-600">{initial.fileName} · {initial.encoding.toUpperCase()} · {initial.lineCount} 行</p>
      <TextArea label="NC 程序内容" value={content} onChange={(e) => setContent(e.target.value)} readOnly={!editing || save.isPending} rows={16} wrap="off" spellCheck={false} autoCapitalize="off" autoCorrect="off" className="font-mono text-sm" />
      {!editing ? <Button onClick={() => setEditing(true)}>编辑内容</Button> : <>
        <Field label="变更说明（另存新版本时生效）" value={note} disabled={save.isPending} onChange={(e) => setNote(e.target.value)} />
        <p className="text-sm text-slate-600">另存的新版本会自动设为当前版本。保存时沿用原文件名与编码。</p>
        <div className="grid gap-2 sm:grid-cols-2">
          <Button disabled={!dirty || save.isPending || !!confirm} onClick={() => save.mutate(false)}>另存新版本</Button>
          <Button variant="secondary" disabled={!dirty || save.isPending || !!confirm} onClick={() => setConfirm('overwrite')}>覆盖此版本</Button>
        </div>
      </>}
      {save.isPending && <p role="status">正在保存…</p>}
      {save.isError && <p role="alert" className="text-red-700">保存失败：{save.error.message}</p>}
      {confirm && <div className="space-y-3 rounded-xl bg-amber-50 p-3">
        <p>{confirm === 'discard' ? '有未保存的修改，确定放弃吗？' : `确定覆盖 V${initial.versionNo} 吗？此版本原内容将被替换，且不新增版本。`}</p>
        <div className="flex gap-2">
          <Button variant="secondary" disabled={save.isPending} onClick={() => setConfirm(null)}>继续编辑</Button>
          <Button variant="danger" loading={save.isPending} onClick={() => confirm === 'discard' ? onClose() : save.mutate(true)}>{confirm === 'discard' ? '放弃修改' : '确认覆盖'}</Button>
        </div>
      </div>}
    </div>
  </Sheet>
}
