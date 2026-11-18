import type { PolarGeometry } from '../lib/polar-drilling'
import { polarNumber as fmt } from '../lib/polar-drilling'

export default function PolarHolePreview({ geometry }: { geometry: PolarGeometry }) {
  return <section aria-labelledby="hole-preview-heading" className="panel min-w-0 space-y-3 p-4 md:p-5">
    <h2 id="hole-preview-heading" className="text-base font-semibold">孔位核对</h2>
    <div className="grid min-w-0 gap-4 md:grid-cols-[minmax(0,16rem)_minmax(0,1fr)]">
      <figure className="min-w-0"><svg role="img" aria-label={`${geometry.count} 孔极坐标示意图，孔圆半径 ${fmt(geometry.radius)} 毫米`} viewBox="-125 -125 250 250" className="mx-auto w-full max-w-64 text-muted">
        <circle r="90" fill="none" stroke="currentColor" strokeDasharray="4 4" opacity=".5" />
        <path d="M-110 0H110 M0 110V-110" fill="none" stroke="currentColor" opacity=".5" />
        <text x="110" y="-6" textAnchor="end" className="fill-current text-[11px]">+X / 0°</text><text x="5" y="-112" className="fill-current text-[11px]">+Y</text>
        <circle r="3" fill="currentColor" />
        {geometry.holes.map(h => {
          const a = h.angle * Math.PI / 180, x = 90 * Math.cos(a), y = -90 * Math.sin(a)
          return <g key={h.number} className={h.number === 1 ? 'text-accent' : 'text-body'}><title>{`孔 ${h.number}，角度 ${fmt(h.angle)}°`}</title><circle cx={x} cy={y} r={geometry.count > 90 ? 2 : 4} fill="currentColor" />{geometry.count <= 24 && <text x={x * 1.13} y={y * 1.13 + 4} textAnchor="middle" className="fill-current text-[11px]">{h.number}</text>}</g>
        })}
      </svg><figcaption className="text-center text-xs leading-5 text-muted">数字为加工顺序，1 为起始孔。孔位示意，不是刀路仿真。</figcaption></figure>
      <div className="min-w-0 space-y-2"><p className="text-sm leading-6 text-muted">共 {geometry.count} 孔 · 半径 {fmt(geometry.radius)} mm · 直径 {fmt(geometry.radius * 2)} mm</p><p className="text-sm leading-6 text-muted">G16 程序中的 X 是半径、Y 是角度。下表直角坐标仅供核对；角度保留三位小数，均分不能整除时会有舍入误差。</p>
        <div className="max-h-72 min-w-0 overflow-auto rounded-md border border-line"><table className="w-full min-w-[26rem] text-left text-sm tabular-nums"><caption className="sr-only">孔位及参考直角坐标</caption><thead className="sticky top-0 bg-inset text-xs text-muted"><tr><th className="p-3">孔号</th><th className="p-3">角度 °</th><th className="p-3">半径 mm</th><th className="p-3">直角 X mm</th><th className="p-3">直角 Y mm</th></tr></thead><tbody className="divide-y divide-line">{geometry.holes.map(h => <tr key={h.number}><td className="p-3">{h.number}</td><td className="p-3">{fmt(h.angle)}</td><td className="p-3">{fmt(geometry.radius)}</td><td className="p-3">{fmt(h.x)}</td><td className="p-3">{fmt(h.y)}</td></tr>)}</tbody></table></div>
      </div>
    </div>
  </section>
}
