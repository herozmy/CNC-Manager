import { rigidTappingFeed } from './rigid-tapping'
import type { DrillingCycle } from './rigid-tapping'
export const POLAR_MANUAL = 'https://dl.mitsubishielectric.com/dl/fa/document/manual/cnc/ib1500930(eng)/ib1500930engd.pdf#page=112'
export const DRILLING_MANUAL = 'https://dl.mitsubishielectric.com/dl/fa/document/manual/cnc/ib1500930(eng)/ib1500930engd.pdf#page=327'
export const RETURN_MANUAL = 'https://dl.mitsubishielectric.com/dl/fa/document/manual/cnc/ib1500930(eng)/ib1500930engd.pdf#page=359'
export const WORK_OFFSETS = ['G54', 'G55', 'G56', 'G57', 'G58', 'G59'] as const
export const PECK_MANUAL = 'https://dl.mitsubishielectric.com/dl/fa/document/manual/cnc/ib1500930(eng)/ib1500930engd.pdf#page=329'
export interface PolarDrillingInput {
  cycle: DrillingCycle
  tapPitch: string
  peck: string
  method: 'absolute' | 'repeat'
  programNo: string
  workOffset: string
  radius: string
  holeCount: string
  startAngle: string
  layout: 'full' | 'step'
  stepAngle: string
  direction: 'ccw' | 'cw'
  toolNo: string
  rpm: string
  feed: string
  safeZ: string
  rPlane: string
  bottomZ: string
  returnLevel: 'G98' | 'G99'
  coolant: boolean
}
export const EMPTY_POLAR_INPUT: PolarDrillingInput = {
  cycle: 'G81', tapPitch: '', peck: '', method: 'absolute', programNo: '1000', workOffset: 'G54', radius: '', holeCount: '6', startAngle: '0', layout: 'full',
  stepAngle: '', direction: 'ccw', toolNo: '', rpm: '', feed: '', safeZ: '30', rPlane: '3.0',
  bottomZ: '', returnLevel: 'G98', coolant: true,
}
export const EXAMPLE_POLAR_INPUT: PolarDrillingInput = {
  ...EMPTY_POLAR_INPUT, radius: '50', toolNo: '1', rpm: '1000', feed: '100', bottomZ: '-10',
}
export interface PolarHole { number: number; angle: number; x: number; y: number }
export interface PolarGeometry { radius: number; count: number; step: number; holes: PolarHole[] }
type Errors = Partial<Record<keyof PolarDrillingInput, string>>

const rounded = (n: number) => Number(n.toFixed(3)) || 0
export const polarNumber = (n: number) => String(rounded(n))
// Always include a decimal point for dimensional words, avoiding implied-decimal ambiguity.
export const ncNumber = (n: number) => rounded(n).toFixed(3).replace(/0+$/, '')
const angleAt = (value: number) => rounded(((value % 360) + 360) % 360) % 360

export function buildPolarDrilling(input: PolarDrillingInput) {
  const errors: Errors = {}
  function decimal(key: keyof PolarDrillingInput, min: number, max: number) {
    const text = String(input[key]).trim()
    const value = /^[+-]?(?:\d+(?:\.\d{0,3})?|\.\d{1,3})$/.test(text) ? Number(text) : NaN
    if (!Number.isFinite(value) || value < min || value > max) {
      errors[key] = `请输入 ${min}～${max} 范围的数值，最多三位小数。`
      return null
    }
    return value
  }
  function integer(key: keyof PolarDrillingInput, min: number, max: number) {
    const text = String(input[key]).trim()
    const value = /^\d+$/.test(text) ? Number(text) : NaN
    if (!Number.isSafeInteger(value) || value < min || value > max) {
      errors[key] = `请输入 ${min}～${max} 的整数。`
      return null
    }
    return value
  }
  if (!['absolute', 'repeat'].includes(input.method)) errors.method = '请选择 G90 逐孔或 G91 重复。'
  const radius = decimal('radius', .001, 99999.999)
  const count = integer('holeCount', 1, 360)
  const start = decimal('startAngle', -360, 360)
  if (!['full', 'step'].includes(input.layout)) errors.layout = '请选择整圆均布或指定孔间角度。'
  if (!['ccw', 'cw'].includes(input.direction)) errors.direction = '请选择顺时针或逆时针。'
  const step = input.layout === 'full' && count !== null ? 360 / count
    : input.layout === 'step' && count === 1 ? 0 : decimal('stepAngle', .001, 359.999)
  if (input.layout === 'step' && count !== null && step !== null && (count - 1) * step >= 360) {
    errors.stepAngle = '首末孔跨度须小于 360°，避免绕回首孔重复打孔。'
  }
  let geometry: PolarGeometry | null = null
  if (radius !== null && count !== null && start !== null && step !== null && !errors.layout && !errors.direction && !errors.stepAngle) {
    const holes = Array.from({ length: count }, (_, i) => {
      const angle = angleAt(start + (input.direction === 'ccw' ? 1 : -1) * (input.method === 'repeat' ? rounded(step) : step) * i)
      const rad = angle * Math.PI / 180
      return { number: i + 1, angle, x: rounded(radius * Math.cos(rad)), y: rounded(radius * Math.sin(rad)) }
    })
    if (new Set(holes.map(h => h.angle)).size !== count || new Set(holes.map(h => `${h.x},${h.y}`)).size !== count) {
      errors.radius = '当前半径和角度在 0.001 mm 精度下出现重合孔位，请调整参数。'
    } else geometry = { radius, count, step, holes }
  }
  if (!['G81', 'G83', 'G84'].includes(input.cycle)) errors.cycle = '请选择 G81、G83 或 G84。'
  const peck = input.cycle === 'G83' ? decimal('peck', .001, 99999.999) : null
  const programNo = integer('programNo', 1, 9999)
  const toolNo = integer('toolNo', 1, 9999)
  const rpm = integer('rpm', 1, 99999)
  const tapPitch = input.cycle === 'G84' ? decimal('tapPitch', .001, 20) : null
  const feed = input.cycle === 'G84' ? rigidTappingFeed(input.tapPitch, input.rpm) : decimal('feed', .001, 99999.999)
  if (input.cycle === 'G84' && tapPitch !== null && rpm !== null && feed === null) errors.tapPitch = '转速与螺距计算的进给须在 0.001～99999.999 mm/min 内。'
  const safeZ = decimal('safeZ', .001, 99999.999)
  const rPlane = decimal('rPlane', .001, 99999.999)
  const bottomZ = decimal('bottomZ', -99999.999, -.001)
  if (safeZ !== null && rPlane !== null && safeZ < rPlane) errors.safeZ = '初始安全高度须不低于循环 R 平面。'
  if (!WORK_OFFSETS.includes(input.workOffset as typeof WORK_OFFSETS[number])) errors.workOffset = '请选择 G54～G59 工件坐标系。'
  if (!['G98', 'G99'].includes(input.returnLevel)) errors.returnLevel = '请选择 G98 或 G99 返回方式。'
  if (typeof input.coolant !== 'boolean') errors.coolant = '冷却设置无效。'
  if (!geometry || Object.keys(errors).length || programNo === null || toolNo === null || rpm === null || feed === null || safeZ === null || rPlane === null || bottomZ === null) {
    return { errors, geometry, program: null, filename: null }
  }
  const name = `O${String(programNo).padStart(4, '0')}`
  const lines = [
    '%', name,
    'G21 G17 G90 G94 G40 G49 G80 G15', 'M05', 'M09', `T${String(toolNo).padStart(2, '0')} M06`, input.workOffset,
    'G52 X0. Y0. Z0.', ...(input.cycle === 'G84' ? [] : [`S${rpm} M03`]), `G00 G43 H${String(toolNo).padStart(2, '0')} Z${ncNumber(safeZ)}`,
    ...(input.coolant ? ['M08'] : []), 'G17 G90 G16',
    ...(input.cycle === 'G84' ? [`M29 S${rpm}`] : []),
    ...geometry.holes.slice(0, input.method === 'repeat' ? 1 : undefined).map((hole, i) => i === 0
      ? `${input.returnLevel} ${input.cycle} X${ncNumber(geometry.radius)} Y${ncNumber(hole.angle)} Z${ncNumber(bottomZ)} R${ncNumber(rPlane).replace(/\.$/, '.0')}${input.cycle === 'G83' && peck !== null ? ` Q${ncNumber(peck)}` : ''} F${ncNumber(feed)}`
      : `Y${ncNumber(hole.angle)}`),
    ...(input.method === 'repeat' && geometry.count > 1 ? [`G91 Y${ncNumber((input.direction === 'ccw' ? 1 : -1) * geometry.step)} K${geometry.count - 1}`] : []),
    'G80', ...(input.method === 'repeat' ? ['G90'] : []), 'G15', `G00 Z${ncNumber(safeZ)}`, 'M09', 'M05', 'M30', '%',
  ]
  return { errors, geometry, program: lines.join('\r\n') + '\r\n', filename: `${name}.nc` }
}
