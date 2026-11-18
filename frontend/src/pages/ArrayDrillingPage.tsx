import { useState } from 'react'
import { rigidTappingFeed, RIGID_TAPPING_MANUAL } from '../lib/rigid-tapping'
import DrillingCycleFields from '../components/DrillingCycleFields'
import DrillingTypeSelector from '../components/DrillingTypeSelector'
import { ArrowLeft, Copy, Download } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Button } from '../components/ui/Button'
import { Field, TextArea } from '../components/ui/Field'
import { buildArrayDrilling, EMPTY_ARRAY_INPUT, EXAMPLE_ARRAY_INPUT } from '../lib/array-drilling'
import type { ArrayDrillingInput, ArrayKind } from '../lib/array-drilling'
import { DRILLING_MANUAL, RETURN_MANUAL, WORK_OFFSETS, polarNumber, PECK_MANUAL } from '../lib/polar-drilling'

const selectClass = 'h-12 w-full rounded-md border border-line-strong bg-surface px-3 text-base text-ink focus:outline-2 focus:outline-accent'
export default function ArrayDrillingPage({ kind }: { kind: ArrayKind }) {
  const [input, setInput] = useState<ArrayDrillingInput>({ ...EMPTY_ARRAY_INPUT })
  const [copying, setCopying] = useState(false)
  const [notice, setNotice] = useState<{ code: string; message: string; error?: boolean } | null>(null)
  const result = buildArrayDrilling(input, kind), { holes, program, filename } = result
  const title = kind === 'line' ? '直线等距钻孔' : '矩形阵列钻孔'
  function update<K extends keyof ArrayDrillingInput>(key: K, value: ArrayDrillingInput[K]) { setInput(prev => ({ ...prev, [key]: value })); setNotice(null) }
  const field = (key: keyof ArrayDrillingInput, label: string, hint?: string, disabled = false) => <Field key={key} label={label} inputMode="decimal" value={String(input[key])} disabled={disabled} onChange={e => update(key, e.target.value as ArrayDrillingInput[typeof key])} error={String(input[key]).trim() ? result.errors[key] : undefined} hint={hint} />
  const select = (key: keyof ArrayDrillingInput, label: string, options: [string, string][]) => <div className="space-y-1.5"><label htmlFor={`array-${key}`} className="block text-sm font-medium text-body">{label}</label><select id={`array-${key}`} className={selectClass} value={String(input[key])} onChange={e => update(key, e.target.value as ArrayDrillingInput[typeof key])}>{options.map(([value, text]) => <option key={value} value={value}>{text}</option>)}</select></div>
  async function copy() {
    if (!program || copying) return
    const code = program; setCopying(true)
    try { await navigator.clipboard.writeText(code); setNotice({ code, message: '已复制程序。' }) }
    catch { setNotice({ code, message: '复制失败，请在代码框选择后手动复制。', error: true }) }
    finally { setCopying(false) }
  }
  function download() {
    if (!program || !filename) return
    try {
      const url = URL.createObjectURL(new Blob([program], { type: 'text/plain;charset=us-ascii' }))
      const a = document.createElement('a'); a.href = url; a.download = filename; document.body.appendChild(a); a.click(); a.remove()
      window.setTimeout(() => URL.revokeObjectURL(url), 60_000)
      setNotice({ code: program, message: `已开始下载 ${filename}，请在浏览器下载记录确认保存。` })
    } catch { setNotice({ code: program, message: '下载未能启动，请复制代码另存为 NC 文件。', error: true }) }
  }
  return <main className="page-width min-w-0 space-y-5 px-4 pt-4 pb-8 md:px-6">
    <header className="flex flex-wrap items-center justify-between gap-3"><div className="flex items-center gap-3"><Link to="/tools/gcode" aria-label="返回 G 代码编程" className="flex size-12 shrink-0 items-center justify-center rounded-lg text-muted hover:bg-hover"><ArrowLeft className="size-5" aria-hidden /></Link><div><p className="text-xs text-muted">钻孔编程</p><h1 className="mt-1 text-xl font-semibold">{title}</h1></div></div><Button variant="secondary" onClick={() => { setInput({ ...EXAMPLE_ARRAY_INPUT }); setNotice(null) }}>载入示例</Button></header>
    <DrillingTypeSelector value={kind === 'line' ? 'line-drilling' : 'grid-drilling'} />
    <p className="rounded-lg border border-line bg-inset p-4 text-sm leading-6 text-muted">G17 XY 平面、G81 普通钻孔、G83 深孔钻或 G84 右旋刚性攻丝；输入所选工件坐标系中的起点，Z0 为工件上表面，程序清零 G52。使用毫米、G94 每分钟进给、G81/G83 使用 M03，G84 使用 FANUC 类 M29 S… 刚性准备；上机前核对坐标偏移、旋转/缩放、刀长、机床指令及夹具净空，并先仿真与空运行。示例仅供核对格式。</p>
    <div className="grid min-w-0 items-start gap-5 xl:grid-cols-2"><div className="min-w-0 space-y-5">
      <DrillingCycleFields cycle={input.cycle} peck={input.peck} tapPitch={input.tapPitch} onPitch={value => update('tapPitch', value)} pitchError={result.errors.tapPitch} onCycle={value => update('cycle', value)} onPeck={value => update('peck', value)} error={result.errors.peck} />
      <section className="panel min-w-0 space-y-4 p-4 md:p-5"><h2 className="font-semibold">布孔参数</h2><div className="grid gap-4 sm:grid-cols-2">
        {field('startX', '起点 X（mm）')}{field('startY', '起点 Y（mm）')}
        {kind === 'line' ? <>
          {field('count', '孔数', '包含首孔，共 1～360 孔。')}{field('pitch', '孔距（mm）', '填写正值，单孔时忽略。', Number(input.count) === 1)}
          {select('axis', '排列轴', [['X', 'X 轴'], ['Y', 'Y 轴']])}{select('sign', '排列方向', [['positive', '正向'], ['negative', '负向']])}
          {select('method', '编程方式', [['repeat', 'G91 · 孔距 K 重复'], ['absolute', 'G90 · 绝对坐标逐孔']])}
        </> : <>
          {field('columns', '列数（X 方向）')}{field('rows', '行数（Y 方向）', '行数 × 列数不超过 360。')}
          {field('pitchX', 'X 孔距（mm）', '正值向 +X，负值向 -X；单列忽略。', Number(input.columns) === 1)}{field('pitchY', 'Y 孔距（mm）', '正值向 +Y，负值向 -Y；单行忽略。', Number(input.rows) === 1)}
          <label className="flex min-h-12 items-center gap-3 text-sm"><input type="checkbox" checked={input.snake} onChange={e => update('snake', e.target.checked)} />蛇形顺序（相邻行反向）</label>
        </>}
      </div><p className="text-sm leading-6 text-muted">{kind === 'line' ? 'G91 方式首孔以 G90 加工，之后沿所选轴移动一个孔距，K 为剩余孔数；仅用于支持 FANUC 类 K 重复格式的机床。单孔不生成重复块。' : '以输入起点为第一行第一列，先沿 X 排列，再沿 Y 换行；采用 G90 绝对坐标逐孔输出。关闭蛇形后，每行均从第一列开始。'}</p></section>
      <section className="panel space-y-4 p-4 md:p-5"><h2 className="font-semibold">程序、刀具与进给</h2><div className="grid gap-4 sm:grid-cols-2">
        {field('programNo', '程序号 O', '1～9999，避开已用或保护的程序号。')}{select('workOffset', '工件坐标系', WORK_OFFSETS.map(w => [w, w]))}
        {field('toolNo', '刀号 T', 'G43 的 H 刀补号自动与 T 刀号一致。')}{field('rpm', '主轴转速 S（r/min）')}{input.cycle === 'G84' ? <Field label="攻丝进给 F（mm/min）" readOnly value={rigidTappingFeed(input.tapPitch, input.rpm) ?? ''} hint="G94：转速 × 螺距，自动计算。" /> : field('feed', '钻孔进给 F（mm/min）')}
      </div><label className="flex min-h-12 items-center gap-3 text-sm"><input type="checkbox" checked={input.coolant} onChange={e => update('coolant', e.target.checked)} />钻孔时开启冷却 M08</label></section>
      <section className="panel space-y-4 p-4 md:p-5"><h2 className="font-semibold">Z 高度与返回方式</h2><p className="text-sm leading-6 text-muted">输入绝对 Z 值，Z0 为工件上表面。</p><div className="grid gap-4 sm:grid-cols-2">
        {field('safeZ', '初始安全高度 Z（mm）', '高于工件表面，且不低于 R 平面。')}{field('rPlane', '循环 R 平面 Z（mm）', '高于工件表面。')}{field('bottomZ', '孔底 Z（mm）', '填写负值孔底位置。')}
        {select('returnLevel', '孔间返回方式', [['G98', 'G98 · 返回初始安全高度'], ['G99', 'G99 · 返回 R 平面']])}
      </div><p className="text-sm text-muted">G99 孔间移动高度为 R 平面，须确认全部移动路径的夹具净空。</p></section>
    </div><section className="panel min-w-0 space-y-4 p-4 md:p-5 xl:sticky xl:top-4"><div className="flex flex-wrap items-center justify-between gap-3"><h2 className="font-semibold">NC 程序预览</h2><div className="flex flex-wrap gap-2"><Button variant="secondary" disabled={!program} loading={copying} onClick={() => void copy()}><Copy className="size-4" aria-hidden />复制程序</Button><Button disabled={!program} onClick={download}><Download className="size-4" aria-hidden />下载 NC</Button></div></div>
      <p className="text-sm leading-6 text-muted">无注释的 ASCII / CRLF 程序；结束取消循环，恢复 G90 后退到初始安全高度。不写入程序台账，也不直接下发机床。</p>
      {program ? <TextArea label="生成的 NC 程序（只读）" readOnly wrap="off" spellCheck={false} rows={28} value={program} className="font-mono text-sm leading-6" /> : <p className="rounded-md border border-dashed border-line p-5 text-sm text-muted">补齐有效参数后生成程序；无效输入时不能复制或下载旧程序。</p>}
      {notice && notice.code === program && <p role={notice.error ? 'alert' : 'status'} className="text-sm text-muted">{notice.message}</p>}
    </section></div>
    {holes && <ArrayPreview holes={holes} />}
    <details className="panel p-4 text-sm leading-6 text-muted"><summary className="cursor-pointer font-medium text-body">格式说明与参考手册</summary><div className="mt-3 space-y-2"><p>支持普通 G81 与固定 Q 的 G83，支持 FANUC 类 M29/G84，不含 G73、左旋攻丝、任意角度直线、机床行程或碰撞验证。尺寸最多三位小数。矩形孔数包含全部行列，直线 K 重复数不包含首孔；K 格式须按目标控制系统确认。</p><a href={DRILLING_MANUAL} target="_blank" rel="noreferrer" className="block py-3 text-accent underline">三菱编程手册：G81</a><a href={RIGID_TAPPING_MANUAL} target="_blank" rel="noreferrer" className="block py-3 text-accent underline">EMCO FANUC 31i：M29 / G84 刚性攻丝</a><a href={PECK_MANUAL} target="_blank" rel="noreferrer" className="block py-3 text-accent underline">三菱编程手册：G83 深孔钻与 Q 增量</a><a href={RETURN_MANUAL} target="_blank" rel="noreferrer" className="block py-3 text-accent underline">三菱编程手册：G98 / G99</a><a href="https://sharp-industries.com/wp-content/uploads/2020/08/Operation-manual-parts-list-SV-SVL-2517-Fanuc-1.pdf" target="_blank" rel="noreferrer" className="block py-3 text-accent underline">SHARP FANUC 操作手册：K 重复格式</a></div></details>
  </main>
}

function ArrayPreview({ holes }: { holes: { number: number; x: number; y: number }[] }) {
  const minX = Math.min(...holes.map(h => h.x)), maxX = Math.max(...holes.map(h => h.x)), minY = Math.min(...holes.map(h => h.y)), maxY = Math.max(...holes.map(h => h.y))
  const span = Math.max(maxX - minX, maxY - minY, .001), cx = (maxX + minX) / 2, cy = (maxY + minY) / 2
  return <section className="panel min-w-0 space-y-4 p-4 md:p-5"><h2 className="font-semibold">孔位核对 · 共 {holes.length} 孔</h2><div className="grid min-w-0 gap-4 md:grid-cols-[16rem_minmax(0,1fr)]"><figure className="min-w-0"><svg role="img" aria-label={`${holes.length} 孔钻孔顺序示意图`} viewBox="-125 -125 250 250" className="mx-auto w-full max-w-64 text-muted"><text x="-115" y="-110" className="fill-current text-[11px]">+X 向右 / +Y 向上</text>{holes.map(h => { const x = (h.x - cx) / span * 180, y = -(h.y - cy) / span * 180; return <g key={h.number} className={h.number === 1 ? 'text-accent' : 'text-body'}><title>{`孔 ${h.number}：X${polarNumber(h.x)} Y${polarNumber(h.y)}`}</title><circle cx={x} cy={y} r={holes.length > 90 ? 2 : 4} fill="currentColor" />{holes.length <= 24 && <text x={x + 7} y={y - 7} className="fill-current text-[11px]">{h.number}</text>}</g> })}</svg><figcaption className="text-xs text-muted">数字为加工顺序，1 为首孔；孔位示意，不是刀路仿真。</figcaption></figure><div className="max-h-72 min-w-0 overflow-auto rounded-md border border-line"><table className="w-full min-w-[16rem] text-left text-sm tabular-nums"><caption className="sr-only">钻孔绝对坐标核对表</caption><thead className="sticky top-0 bg-inset"><tr><th className="p-3">孔号</th><th className="p-3">X mm</th><th className="p-3">Y mm</th></tr></thead><tbody className="divide-y divide-line">{holes.map(h => <tr key={h.number}><td className="p-3">{h.number}</td><td className="p-3">{polarNumber(h.x)}</td><td className="p-3">{polarNumber(h.y)}</td></tr>)}</tbody></table></div></div></section>
}
