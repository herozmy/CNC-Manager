import { useQuery } from '@tanstack/react-query'
import { Fragment, useState } from 'react'
import { api } from '../api'
import { Button } from './ui/Button'
import { Sheet } from './ui/Sheet'
import { ErrorState, Spinner } from './ui/State'

function Row({ label, no, text, same, added }: { label: string; no: string | number; text: string; same?: boolean; added?: boolean }) {
  return <tr className={same ? 'bg-surface text-muted' : added ? 'bg-success-soft text-success' : 'bg-danger-soft text-danger'}>
    <td className="p-2 align-top">{label}</td><td className="p-2 align-top">{no}</td>
    <td className="p-2"><pre className="whitespace-pre-wrap break-all font-mono">{text || ' '}</pre></td>
  </tr>
}

export function VersionDiffPanel({ programId, versionId }: { programId: number; versionId: number }) {
  const versions = useQuery({ queryKey: ['versions', programId], queryFn: () => api.listVersions(programId), retry: false })
  const [leftId, setLeftId] = useState<number | null>(null)
  const [rightId, setRightId] = useState(versionId)
  const [page, setPage] = useState(0)
  const [changesOnly, setChangesOnly] = useState(true)
  const right = versions.data?.find((v) => v.id === rightId)
  const left = versions.data?.find((v) => v.id === leftId) ?? versions.data?.find((v) => v.versionNo < (right?.versionNo ?? 0)) ?? versions.data?.find((v) => v.id !== rightId)
  const enabled = !!left && !!right && left.id !== right.id
  const diff = useQuery({ queryKey: ['version-diff', left?.id, rightId, page, changesOnly], queryFn: () => api.versionDiff(left!.id, rightId, page * 200, changesOnly), enabled, retry: false })
  const selectClass = 'h-12 w-full min-w-0 rounded-md border border-line-strong bg-surface px-3'
  return <div className="space-y-4">
      {versions.isPending && <Spinner />}
      {versions.isError && <ErrorState message="版本列表加载失败" onRetry={() => void versions.refetch()} />}
      {versions.data && <>
        <div className="grid grid-cols-2 gap-3">
          <label className="min-w-0 space-y-1 text-sm">对照版本
            <select aria-label="对照版本" className={selectClass} value={left?.id ?? ''} onChange={(e) => { setLeftId(Number(e.target.value)); setPage(0) }}>
              {versions.data.map((v) => <option key={v.id} value={v.id}>V{v.versionNo}{v.isCurrent ? '（当前）' : ''}</option>)}
            </select>
          </label>
          <label className="min-w-0 space-y-1 text-sm">目标版本
            <select aria-label="目标版本" className={selectClass} value={rightId} onChange={(e) => { setRightId(Number(e.target.value)); setPage(0) }}>
              {versions.data.map((v) => <option key={v.id} value={v.id}>V{v.versionNo}{v.isCurrent ? '（当前）' : ''}</option>)}
            </select>
          </label>
        </div>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <label className="flex min-h-12 items-center gap-2 text-sm"><input type="checkbox" checked={changesOnly} onChange={(e) => { setChangesOnly(e.target.checked); setPage(0) }} />仅显示差异</label>
          <Button variant="secondary" disabled={!enabled} onClick={() => { setLeftId(rightId); setRightId(left!.id); setPage(0) }}>交换方向</Button>
        </div>
        <p className="text-xs text-muted">比较已保存的内容；忽略换行符形式及末尾换行差异。− 为对照版内容，+ 为目标版内容。</p>
        {!enabled && <p className="text-sm text-muted">请选择两个不同的版本</p>}
        {enabled && diff.isPending && <Spinner />}
        {enabled && diff.isError && <ErrorState message="版本对比失败" onRetry={() => void diff.refetch()} />}
        {enabled && diff.data && <>
          <p role="status" className="text-sm font-semibold">{diff.data.identical ? '两版内容相同' : `差异行：${diff.data.totalChanges}`}</p>
          {diff.data.truncated && <p className="rounded-md bg-warning-soft p-3 text-sm">改动范围较大，部分内容按整块替换显示。</p>}
          {diff.data.lines.length > 0 && <div className="max-h-[48dvh] overflow-auto rounded-md border border-line">
            <table className="w-full table-fixed text-left text-sm" aria-label="版本差异">
              <thead className="sticky top-0 bg-inset"><tr><th className="w-16 p-2">版本</th><th className="w-20 p-2">行号</th><th className="p-2">内容</th></tr></thead>
              <tbody>{diff.data.lines.map((line, index) => <Fragment key={`${page}-${index}`}>
                {line.leftNo !== null && <Row label={line.type === 'same' ? '两版' : `− V${diff.data.leftVersionNo}`} no={line.type === 'same' ? `${line.leftNo}/${line.rightNo}` : line.leftNo} text={line.leftText} same={line.type === 'same'} />}
                {line.rightNo !== null && line.type !== 'same' && <Row label={`+ V${diff.data.rightVersionNo}`} no={line.rightNo} text={line.rightText} added />}
              </Fragment>)}</tbody>
            </table>
          </div>}
          {diff.data.totalLines > 200 && <div className="flex items-center justify-between gap-2">
            <Button variant="secondary" disabled={page === 0} onClick={() => setPage((n) => n - 1)}>上一页</Button>
            <span className="text-sm">{page + 1} / {Math.ceil(diff.data.totalLines / 200)}</span>
            <Button variant="secondary" disabled={(page + 1) * 200 >= diff.data.totalLines} onClick={() => setPage((n) => n + 1)}>下一页</Button>
          </div>}
        </>}
      </>}
    </div>
}

export default function VersionDiffSheet({ programId, versionId, onClose }: { programId: number; versionId: number; onClose: () => void }) {
  return <Sheet open wide title="版本对比" onClose={onClose}><VersionDiffPanel programId={programId} versionId={versionId} /></Sheet>
}
