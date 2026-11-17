import { useState } from 'react'
import { ArrowLeft } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Field } from '../components/ui/Field'
import { calculateTapping, DRILL_SOURCE, FORMULA_SOURCE, METRIC_DIAMETERS, METRIC_THREADS, tappingNumber } from '../lib/metric-tapping'

const selectClass = 'h-12 w-full rounded-md border border-line-strong bg-surface px-3 text-base text-ink focus:outline-2 focus:outline-accent'
const fmt = tappingNumber

export default function TappingPage() {
  const [diameter, setDiameter] = useState(6)
  const [pitch, setPitch] = useState(1)
  const [rpmInput, setRpmInput] = useState('')
  const pitches = METRIC_THREADS.filter(t => t.diameter === diameter)
  const thread = pitches.find(t => t.pitch === pitch)!
  const result = calculateTapping(pitch, rpmInput)
  const error = rpmInput.trim() && !result ? '请输入大于 0 的有效整数转速。' : undefined

  return <main className="page-width min-w-0 space-y-5 px-4 pt-4 pb-8 md:px-6">
    <header className="flex items-center gap-3">
      <Link to="/tools" aria-label="返回加工助手" title="返回加工助手" className="flex size-12 shrink-0 items-center justify-center rounded-lg text-muted hover:bg-hover focus-visible:outline-2 focus-visible:outline-accent"><ArrowLeft className="size-5" aria-hidden /></Link>
      <div><p className="text-xs font-medium text-muted">加工助手</p><h1 className="mt-1 text-xl font-semibold">公制攻丝</h1></div>
    </header>
    <p className="text-sm leading-6 text-muted">查询常用 M 螺纹底孔与螺距，输入转速计算同步攻丝进给。</p>
    <div className="grid min-w-0 gap-5 lg:grid-cols-2">
      <section aria-labelledby="thread-heading" className="panel min-w-0 space-y-5 p-4 md:p-5">
        <h2 id="thread-heading" className="text-base font-semibold">1. 选择螺纹</h2>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-1.5"><label htmlFor="tapping-diameter" className="block text-sm font-medium text-body">公称直径</label>
            <select id="tapping-diameter" className={selectClass} value={diameter} onChange={e => {
              const next = Number(e.target.value)
              setDiameter(next)
              setPitch(METRIC_THREADS.find(t => t.diameter === next && t.series === 'coarse')!.pitch)
            }}>{METRIC_DIAMETERS.map(d => <option key={d} value={d}>M{d}</option>)}</select>
          </div>
          <div className="space-y-1.5"><label htmlFor="tapping-pitch" className="block text-sm font-medium text-body">螺距（mm）</label>
            <select id="tapping-pitch" className={selectClass} value={pitch} onChange={e => setPitch(Number(e.target.value))}>{pitches.map(t => <option key={t.pitch} value={t.pitch}>{t.pitch} · {t.series === 'coarse' ? '粗牙' : '细牙'}</option>)}</select>
          </div>
        </div>
        <div className="rounded-lg border border-line bg-inset p-4">
          <p className="text-sm text-muted">所选规格 · {thread.series === 'coarse' ? '粗牙' : '细牙'}</p>
          <p className="mt-1 text-2xl font-semibold tabular-nums">M{diameter} × {pitch}</p>
          <dl className="mt-5 grid grid-cols-2 gap-4">
            <div><dt className="text-sm text-muted">底孔参考直径</dt><dd className="mt-1 text-xl font-semibold text-accent tabular-nums" aria-label="底孔参考直径">{fmt(thread.drill)} <span className="text-sm font-normal">mm</span></dd></div>
            <div><dt className="text-sm text-muted">螺距</dt><dd className="mt-1 text-xl font-semibold tabular-nums" aria-label="所选螺距">{fmt(pitch)} <span className="text-sm font-normal">mm</span></dd></div>
          </dl>
        </div>
        <p className="text-sm leading-6 text-muted">底孔为普通切削丝锥参考值，不适用于挤压丝锥。实际按材料、螺纹公差及丝锥厂家的要求确认。</p>
        <a href={DRILL_SOURCE} target="_blank" rel="noreferrer" className="inline-flex min-h-12 items-center text-sm text-accent underline">底孔数据来源：Guhring 技术表</a>
      </section>
      <section aria-labelledby="calculation-heading" className="panel min-w-0 space-y-5 p-4 md:p-5">
        <h2 id="calculation-heading" className="text-base font-semibold">2. 输入转速计算进给</h2>
        <Field label="主轴转速（r/min）" inputMode="numeric" autoComplete="off" value={rpmInput} onChange={e => setRpmInput(e.target.value)} error={error || undefined} placeholder="输入转速" hint="手动设置，不代表推荐转速。" />
        <div aria-live="polite" aria-atomic="true" className="rounded-lg border border-line bg-inset p-4">
          {result ? <dl className="grid gap-4 sm:grid-cols-2">
            <div><dt className="text-sm text-muted">每分钟进给</dt><dd aria-label="每分钟进给" className="mt-1 text-xl font-semibold text-accent tabular-nums">{fmt(result.feed)} <span className="text-sm font-normal">mm/min</span></dd></div>
            <div><dt className="text-sm text-muted">每转进给</dt><dd aria-label="每转进给" className="mt-1 text-xl font-semibold tabular-nums">{fmt(pitch)} <span className="text-sm font-normal">mm/rev</span></dd></div>
          </dl> : <p className="py-4 text-sm text-muted">{rpmInput.trim() ? '请修正输入后查看计算结果。' : '输入转速后显示进给结果。'}</p>}
        </div>
        <p className="text-sm leading-6 text-muted">适用于普通单线螺纹的同步攻丝。每分钟进给 = 转速 × 螺距；每转进给 = 螺距。请按机床实际进给模式选用对应数值。</p>
      </section>
    </div>
    <details className="panel p-4 text-sm leading-6 text-muted">
      <summary className="cursor-pointer font-medium text-body">计算公式与参考来源</summary>
      <div className="mt-3 space-y-2"><p>每分钟进给 F = n × P，n 为主轴转速（r/min），P 为螺距（mm）。显示结果最多保留三位小数，进给按显示的整数转速计算。</p><a className="inline-flex min-h-12 items-center text-accent underline" href={FORMULA_SOURCE} target="_blank" rel="noreferrer">Haas 攻丝进给公式</a><p>当前收录 M2～M36 的部分常用粗牙、细牙规格；未列出的规格不作估算。</p></div>
    </details>
  </main>
}
