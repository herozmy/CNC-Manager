import { useState } from 'react'
import { rigidTappingFeed, RIGID_TAPPING_MANUAL } from '../lib/rigid-tapping'
import DrillingCycleFields from '../components/DrillingCycleFields'
import DrillingTypeSelector from '../components/DrillingTypeSelector'
import { ArrowLeft, Copy, Download } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Button } from '../components/ui/Button'
import { Field, TextArea } from '../components/ui/Field'
import PolarHolePreview from '../components/PolarHolePreview'
import { buildPolarDrilling, EMPTY_POLAR_INPUT, EXAMPLE_POLAR_INPUT, WORK_OFFSETS, POLAR_MANUAL, DRILLING_MANUAL, RETURN_MANUAL, PECK_MANUAL } from '../lib/polar-drilling'
import type { PolarDrillingInput } from '../lib/polar-drilling'

const selectClass = 'h-12 w-full rounded-md border border-line-strong bg-surface px-3 text-base text-ink focus:outline-2 focus:outline-accent'

export default function PolarDrillingPage() {
  const [input, setInput] = useState<PolarDrillingInput>({ ...EMPTY_POLAR_INPUT })
  const [example, setExample] = useState(false)
  const [copying, setCopying] = useState(false)
  const [notice, setNotice] = useState<{ code: string; message: string; error: boolean } | null>(null)
  const { errors, geometry, program, filename } = buildPolarDrilling(input)
  function update<K extends keyof PolarDrillingInput>(key: K, value: PolarDrillingInput[K]) {
    setInput(prev => ({ ...prev, [key]: value })); setExample(false); setNotice(null)
  }
  const error = (key: keyof PolarDrillingInput) => String(input[key]).trim() ? errors[key] : undefined
  async function copy() {
    if (!program || copying) return
    const code = program
    setCopying(true)
    try {
      if (!navigator.clipboard?.writeText) throw new Error('Clipboard unavailable')
      await navigator.clipboard.writeText(code)
      setNotice({ code, message: '已复制程序。', error: false })
    } catch { setNotice({ code, message: '复制失败，请在代码框选择后手动复制。', error: true }) }
    finally { setCopying(false) }
  }
  function download() {
    if (!program || !filename) return
    try {
      const url = URL.createObjectURL(new Blob([program], { type: 'text/plain;charset=us-ascii' }))
      const link = document.createElement('a')
      link.href = url; link.download = filename; document.body.appendChild(link); link.click(); link.remove()
      window.setTimeout(() => URL.revokeObjectURL(url), 60_000)
      setNotice({ code: program, message: `已开始下载 ${filename}，请在浏览器下载记录确认保存。`, error: false })
    } catch { setNotice({ code: program, message: '下载未能启动，请复制代码另存为 NC 文件。', error: true }) }
  }
  return <main className="page-width min-w-0 space-y-5 px-4 pt-4 pb-8 md:px-6">
    <header className="flex flex-wrap items-center justify-between gap-3">
      <div className="flex min-w-0 items-center gap-3"><Link to="/tools/gcode" aria-label="返回 G 代码编程" title="返回 G 代码编程" className="flex size-12 shrink-0 items-center justify-center rounded-lg text-muted hover:bg-hover focus-visible:outline-2 focus-visible:outline-accent"><ArrowLeft className="size-5" aria-hidden /></Link><div><p className="text-xs font-medium text-muted">钻孔编程</p><h1 className="mt-1 text-xl font-semibold">极坐标打孔</h1></div></div>
      <Button variant="secondary" onClick={() => { setInput({ ...EXAMPLE_POLAR_INPUT }); setExample(true); setNotice(null) }}>载入示例</Button>
    </header>
    <DrillingTypeSelector value="polar-drilling" />
    <div className="rounded-lg border border-line bg-inset px-4 py-3 text-sm leading-6 text-muted"><p>G16 / G15 + G81 / G83 / G84，面向支持该指令的 FANUC／三菱类加工中心。使用 G17 XY 平面、G90 绝对半径和角度、G21 毫米、G94 每分钟进给；G81/G83 使用 M03，G84 使用 FANUC 类 M29 S… 刚性准备。</p><p className="mt-1">圆心为所选工件坐标系原点，Z0 为工件上表面；程序会将 G52 局部偏置清零。转移上机前须核对坐标、刀长、夹具净空和机床指令，并先仿真与空运行。</p></div>
    {example && <p role="status" className="rounded-md bg-accent-soft p-3 text-sm text-accent">已载入示例，仅供核对格式；请按实际工件修改。</p>}
    <div className="grid min-w-0 items-start gap-5 xl:grid-cols-2">
      <div className="min-w-0 space-y-5">
        <DrillingCycleFields cycle={input.cycle} peck={input.peck} tapPitch={input.tapPitch} onPitch={value => update('tapPitch', value)} pitchError={errors.tapPitch} onCycle={value => update('cycle', value)} onPeck={value => update('peck', value)} error={errors.peck} />
        <section aria-labelledby="polar-layout-heading" className="panel min-w-0 space-y-4 p-4 md:p-5">
          <h2 id="polar-layout-heading" className="text-base font-semibold">布孔参数</h2>
          <div className="space-y-1.5"><label htmlFor="polar-method" className="block text-sm font-medium text-body">编程方式</label><select id="polar-method" className={selectClass} value={input.method} onChange={e => update('method', e.target.value as PolarDrillingInput['method'])}><option value="absolute">G90 · 绝对角度逐孔</option><option value="repeat">G91 · 角度增量 K 重复</option></select>{error('method') && <p role="alert" className="text-danger">{error('method')}</p>}<p className="text-sm leading-6 text-muted">两种方式均使用 G16。G91 方式先以 G90 加工首孔，再用 Y 孔间角、K 剩余孔数重复；仅用于支持该 K 格式的机床。非整除均分会累积角度舍入误差，图表按实际三位小数步距核对。</p></div>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="孔圆半径（mm）" required inputMode="decimal" autoComplete="off" placeholder="例如 30" value={input.radius} onChange={e => update('radius', e.target.value)} error={error('radius')} hint="G16 中 X 为半径，孔圆直径为半径的两倍。" />
            <Field label="孔数" required inputMode="numeric" value={input.holeCount} onChange={e => update('holeCount', e.target.value)} error={error('holeCount')} hint="1～360 个孔。" />
            <div className="space-y-1.5"><label htmlFor="polar-layout" className="block text-sm font-medium text-body">布孔方式</label><select id="polar-layout" className={selectClass} value={input.layout} onChange={e => update('layout', e.target.value as PolarDrillingInput['layout'])}><option value="full">整圆均布</option><option value="step">指定孔间角度</option></select></div>
            <div className="space-y-1.5"><label htmlFor="polar-direction" className="block text-sm font-medium text-body">孔位顺序</label><select id="polar-direction" className={selectClass} value={input.direction} onChange={e => update('direction', e.target.value as PolarDrillingInput['direction'])}><option value="ccw">逆时针</option><option value="cw">顺时针</option></select></div>
            <Field label="起始角度（°）" required inputMode="decimal" value={input.startAngle} onChange={e => update('startAngle', e.target.value)} error={error('startAngle')} hint="+X 为 0°，+Y 为 90°；允许 -360～360°。" />
            {input.layout === 'step' && <Field label="孔间角度（°）" inputMode="decimal" disabled={Number(input.holeCount) === 1} value={input.stepAngle} onChange={e => update('stepAngle', e.target.value)} error={error('stepAngle')} hint="单孔时忽略；首末孔跨度须小于 360°。" />}
          </div>
        </section>
        <section aria-labelledby="polar-tool-heading" className="panel min-w-0 space-y-4 p-4 md:p-5">
          <h2 id="polar-tool-heading" className="text-base font-semibold">程序、刀具与进给</h2>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="程序号 O" required inputMode="numeric" value={input.programNo} onChange={e => update('programNo', e.target.value)} error={error('programNo')} hint="填写 1～9999 的数字，避开机床已用或保护的程序号。" />
            <div className="space-y-1.5"><label htmlFor="polar-work-offset" className="block text-sm font-medium text-body">工件坐标系（圆心）</label><select id="polar-work-offset" className={selectClass} value={input.workOffset} onChange={e => update('workOffset', e.target.value)}>{WORK_OFFSETS.map(w => <option key={w}>{w}</option>)}</select></div>
            <Field label="刀号 T" hint="G43 的 H 刀补号自动与刀号一致，例如 T02 对应 H02。" required inputMode="numeric" value={input.toolNo} onChange={e => update('toolNo', e.target.value)} error={error('toolNo')} />
            <Field label="主轴转速 S（r/min）" required inputMode="numeric" value={input.rpm} onChange={e => update('rpm', e.target.value)} error={error('rpm')} />
            {input.cycle === 'G84' ? <Field label="攻丝进给 F（mm/min）" readOnly value={rigidTappingFeed(input.tapPitch, input.rpm) ?? ''} hint="G94：转速 × 螺距，自动计算。" /> : <Field label="钻孔进给 F（mm/min）" required inputMode="decimal" value={input.feed} onChange={e => update('feed', e.target.value)} error={error('feed')} />}
          </div>
          <label className="flex min-h-12 items-center gap-3 text-sm"><input type="checkbox" checked={input.coolant} onChange={e => update('coolant', e.target.checked)} />钻孔时开启冷却 M08</label>
        </section>
        <section aria-labelledby="polar-height-heading" className="panel min-w-0 space-y-4 p-4 md:p-5">
          <h2 id="polar-height-heading" className="text-base font-semibold">Z 高度与返回方式</h2>
          <p className="text-sm leading-6 text-muted">以下均为工件坐标中的绝对 Z 值；Z0 为工件上表面。</p>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="初始安全高度 Z（mm）" required inputMode="decimal" placeholder="例如 30" value={input.safeZ} onChange={e => update('safeZ', e.target.value)} error={error('safeZ')} hint="须高于工件表面，且不低于 R 平面。" />
            <Field label="循环 R 平面 Z（mm）" required inputMode="decimal" placeholder="例如 3.0" value={input.rPlane} onChange={e => update('rPlane', e.target.value)} error={error('rPlane')} hint="须高于工件表面。" />
            <Field label="孔底 Z（mm）" required inputMode="decimal" placeholder="例如 -10" value={input.bottomZ} onChange={e => update('bottomZ', e.target.value)} error={error('bottomZ')} hint="填写负值；这是孔底位置，不是正数深度。" />
            <div className="space-y-1.5"><label htmlFor="polar-return" className="block text-sm font-medium text-body">孔间返回方式</label><select id="polar-return" className={selectClass} value={input.returnLevel} onChange={e => update('returnLevel', e.target.value as PolarDrillingInput['returnLevel'])}><option value="G98">G98 · 返回初始安全高度</option><option value="G99">G99 · 返回 R 平面</option></select><p className="text-sm leading-6 text-muted">G99 时孔间移动高度为 R 平面，需确认夹具净空。</p></div>
          </div>
        </section>
      </div>
      <section aria-labelledby="polar-code-heading" className="panel min-w-0 space-y-4 p-4 md:p-5 xl:sticky xl:top-4">
        <div className="flex flex-wrap items-center justify-between gap-3"><h2 id="polar-code-heading" className="text-base font-semibold">NC 程序预览</h2><div className="flex flex-wrap gap-2"><Button variant="secondary" disabled={!program} loading={copying} onClick={() => void copy()}><Copy className="size-4" aria-hidden />复制程序</Button><Button disabled={!program} onClick={download}><Download className="size-4" aria-hidden />下载 NC</Button></div></div>
        <p className="text-sm leading-6 text-muted">固定循环 G81 / G83 / G84，支持 G90 逐孔或 G91 角度增量 K 重复；结束取消循环和极坐标，G91 方式恢复 G90 后退到初始安全高度。不直接下发机床，也不写入程序台账。</p>
        {program ? <TextArea label="生成的 NC 程序（只读）" readOnly wrap="off" spellCheck={false} rows={28} value={program} className="font-mono text-sm leading-6" /> : <p className="rounded-md border border-dashed border-line-strong bg-inset p-5 text-sm leading-6 text-muted">补齐布孔、刀具、进给与 Z 高度参数后生成程序；无效输入时不能复制或下载旧程序。</p>}
        {notice && notice.code === program && <p role={notice.error ? 'alert' : 'status'} className={`text-sm leading-6 ${notice.error ? 'text-danger' : 'text-muted'}`}>{notice.message}</p>}
        <p className="text-xs leading-5 text-muted">输出为无注释的 ASCII 程序、CRLF 换行，文件名为程序号 .nc。尺寸与角度最多输出三位小数；下载后仍需在目标系统核验。</p>
      </section>
    </div>
    {geometry && <PolarHolePreview geometry={geometry} />}
    <details className="panel p-4 text-sm leading-6 text-muted"><summary className="cursor-pointer font-medium text-body">格式说明与参考手册</summary><div className="mt-3 space-y-2">
      <p>此工具生成 G17 / G16、G81 / G83 / G84 模板。首孔用 G90 指定 X 半径和从 +X 轴起算的 Y 角度；后续可选 G90 逐孔或 G91 角度增量 K 重复。顺逆时针改变孔位输出顺序。整圆均布角度为 360° ÷ 孔数，不重复 360° 首孔。</p>
      <p>工件坐标原点须在圆心，Z0 须在工件上表面。程序清零 G52 局部偏置；使用前确认机床未遗留 G92 偏移、坐标旋转或缩放等状态。刀号、H 号、M06/M08 和 G16 功能须与实际机床一致。</p>
      <p>支持 G81 普通钻孔与固定 Q 的 G83 深孔钻，支持 FANUC 类 M29/G84 右旋刚性攻丝，不含 G73 断屑钻、左旋攻丝、任意圆心偏移或机床行程/碰撞验证。非整除角度会舍入；G91 按三位小数孔间角重复，误差会累积，孔位表按实际输出步距核对。</p>
      <a href="https://sharp-industries.com/wp-content/uploads/2020/08/Operation-manual-parts-list-SV-SVL-2517-Fanuc-1.pdf" target="_blank" rel="noreferrer" className="flex min-h-12 items-center text-accent underline">SHARP FANUC 操作手册：G91 Y45. K7 重复示例</a>
      <a href={POLAR_MANUAL} target="_blank" rel="noreferrer" className="flex min-h-12 items-center text-accent underline">三菱编程手册：G16 / G15 与坐标原点</a>
      <a href={DRILLING_MANUAL} target="_blank" rel="noreferrer" className="flex min-h-12 items-center text-accent underline">三菱编程手册：G81 钻孔循环</a>
      <a href={RIGID_TAPPING_MANUAL} target="_blank" rel="noreferrer" className="block py-3 text-accent underline">EMCO FANUC 31i：M29 / G84 刚性攻丝</a><a href={PECK_MANUAL} target="_blank" rel="noreferrer" className="block py-3 text-accent underline">三菱编程手册：G83 深孔钻与 Q 增量</a><a href={RETURN_MANUAL} target="_blank" rel="noreferrer" className="flex min-h-12 items-center text-accent underline">三菱编程手册：G98 / G99 返回方式</a>
    </div></details>
  </main>
}
