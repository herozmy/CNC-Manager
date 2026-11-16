import { Cpu, Wrench } from 'lucide-react'
import { Link } from 'react-router-dom'
import type { Operation } from '../api/types'

function Facts({ operation: op, material }: { operation: Operation; material: string }) {
  return <div className="space-y-2 text-sm">
    {material && <p><span className="text-muted">材料：</span>{material}</p>}
    <p><span className="text-muted">装夹：</span>{op.fixture || '未填写'}</p>
    <p><span className="text-muted">分中：</span><span className="text-warning">{op.centeringMethod || '未填写'}</span></p>
    <dl className="grid grid-cols-3 gap-2 border-t border-line pt-3 font-mono tabular-nums">{([['X', op.xCoordinate ?? 0], ['Y', op.yCoordinate ?? 0], ['Z', op.zHeight]] as const).map(([axis, value]) => <div key={axis} className="min-w-0"><dt className="text-xs text-muted">{axis} (mm)</dt><dd className="mt-1 break-all font-semibold">{value}</dd></div>)}</dl>
  </div>
}

export default function ProgramContext({ operation: op, material, drawingId, programId }: { operation: Operation; material: string; drawingId: number; programId: number }) {
  return <>
    <details className="panel p-3 lg:hidden"><summary className="cursor-pointer py-1 text-sm font-medium">装夹与对刀基准</summary><div className="mt-3"><Facts operation={op} material={material} /></div></details>
    <section aria-label="装夹与对刀基准" className="panel hidden space-y-3 p-4 lg:block"><h2 className="flex items-center gap-2 text-sm font-semibold"><Wrench className="size-4 text-accent" aria-hidden />工序 {op.opNo} · 装夹与对刀</h2><Facts operation={op} material={material} /></section>
    <nav aria-label="同工序程序" className="panel hidden space-y-2 p-4 lg:block">
      <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold"><Cpu className="size-4 text-accent" aria-hidden />当前工序程序 ({op.programs.length})</h2>
      {op.programs.map(p => <Link key={p.id} to={`/drawings/${drawingId}/programs/${p.id}`} aria-label={`切换到程序 ${p.programNo}`} aria-current={p.id === programId ? 'page' : undefined} className={`flex min-h-12 min-w-0 items-center justify-between gap-2 rounded-md border px-3 py-2 ${p.id === programId ? 'border-accent bg-accent-soft' : 'border-line hover:bg-hover'}`}>
        <span className="min-w-0"><span className="block truncate font-mono text-sm font-semibold text-accent">{p.programNo}</span>{p.programName && <span className="mt-1 block truncate text-xs text-muted">{p.programName}</span>}</span>
        {p.currentVersionNo && <span className="rounded bg-inset px-1.5 py-0.5 font-mono text-xs">V{p.currentVersionNo}</span>}
      </Link>)}
    </nav>
  </>
}
