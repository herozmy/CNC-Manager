import { rigidTappingFeed } from './rigid-tapping'
import type { DrillingCycle } from './rigid-tapping'
import { ncNumber, WORK_OFFSETS } from './polar-drilling'

export interface ArrayDrillingInput {
  cycle: DrillingCycle; tapPitch: string; peck: string
  startX: string; startY: string; count: string; pitch: string; axis: 'X' | 'Y'; sign: 'positive' | 'negative'
  columns: string; rows: string; pitchX: string; pitchY: string; snake: boolean; method: 'absolute' | 'repeat'
  programNo: string; workOffset: string; toolNo: string; rpm: string; feed: string
  safeZ: string; rPlane: string; bottomZ: string; returnLevel: 'G98' | 'G99'; coolant: boolean
}
export type ArrayKind = 'line' | 'grid'
export const EMPTY_ARRAY_INPUT: ArrayDrillingInput = {
  cycle: 'G81', tapPitch: '', peck: '', startX: '0', startY: '0', count: '5', pitch: '', axis: 'X', sign: 'positive',
  columns: '3', rows: '2', pitchX: '', pitchY: '', snake: true, method: 'repeat',
  programNo: '1000', workOffset: 'G54', toolNo: '', rpm: '', feed: '', safeZ: '30', rPlane: '3.0', bottomZ: '', returnLevel: 'G98', coolant: true,
}
export const EXAMPLE_ARRAY_INPUT: ArrayDrillingInput = {
  ...EMPTY_ARRAY_INPUT, pitch: '20', pitchX: '30', pitchY: '20', toolNo: '1', rpm: '1000', feed: '100', bottomZ: '-10',
}
export interface ArrayHole { number: number; x: number; y: number }
type Errors = Partial<Record<keyof ArrayDrillingInput, string>>
const round = (n: number) => Number(n.toFixed(3)) || 0

export function buildArrayDrilling(input: ArrayDrillingInput, kind: ArrayKind) {
  const errors: Errors = {}
  const number = (key: keyof ArrayDrillingInput, min: number, max: number, integer = false) => {
    const raw = String(input[key]).trim()
    const n = (integer ? /^\d+$/ : /^[+-]?(?:\d+(?:\.\d{0,3})?|\.\d{1,3})$/).test(raw) ? Number(raw) : NaN
    if (!Number.isFinite(n) || n < min || n > max || (integer && !Number.isSafeInteger(n))) {
      errors[key] = `请输入 ${min}～${max} 的${integer ? '整数' : '数值，最多三位小数'}。`
      return null
    }
    return n
  }
  const startX = number('startX', -99999.999, 99999.999), startY = number('startY', -99999.999, 99999.999)
  let holes: ArrayHole[] | null = null
  let linePitch = 0
  if (kind === 'line') {
    const count = number('count', 1, 360, true)
    const pitch = count === 1 ? 0 : number('pitch', .001, 99999.999)
    if (!['X', 'Y'].includes(input.axis)) errors.axis = '请选择 X 或 Y 方向。'
    if (!['positive', 'negative'].includes(input.sign)) errors.sign = '请选择正向或负向。'
    if (!['absolute', 'repeat'].includes(input.method)) errors.method = '请选择编程方式。'
    if (startX !== null && startY !== null && count !== null && pitch !== null && !errors.axis && !errors.sign && !errors.method) {
      linePitch = pitch * (input.sign === 'positive' ? 1 : -1)
      holes = Array.from({ length: count }, (_, i) => ({ number: i + 1, x: round(startX + (input.axis === 'X' ? linePitch * i : 0)), y: round(startY + (input.axis === 'Y' ? linePitch * i : 0)) }))
    }
  } else if (kind === 'grid') {
    const columns = number('columns', 1, 360, true), rows = number('rows', 1, 360, true)
    const px = columns === 1 ? 0 : number('pitchX', -99999.999, 99999.999)
    const py = rows === 1 ? 0 : number('pitchY', -99999.999, 99999.999)
    if (columns !== null && columns > 1 && px === 0) errors.pitchX = '多列时 X 孔距不能为 0。'
    if (rows !== null && rows > 1 && py === 0) errors.pitchY = '多行时 Y 孔距不能为 0。'
    if (typeof input.snake !== 'boolean') errors.snake = '请选择走孔顺序。'
    if (columns !== null && rows !== null && columns * rows > 360) errors.rows = '总孔数不能超过 360。'
    if (startX !== null && startY !== null && columns !== null && rows !== null && px !== null && py !== null && !errors.rows && !errors.pitchX && !errors.pitchY && !errors.snake) {
      holes = Array.from({ length: columns * rows }, (_, i) => {
        const row = Math.floor(i / columns), col = input.snake && row % 2 ? columns - 1 - i % columns : i % columns
        return { number: i + 1, x: round(startX + col * px), y: round(startY + row * py) }
      })
    }
  } else errors.count = '无效的钻孔类型。'
  if (holes?.some(h => Math.abs(h.x) > 99999.999 || Math.abs(h.y) > 99999.999)) {
    errors.startX = '生成孔位超出 ±99999.999 mm，请调整起点、孔距或孔数。'; holes = null
  }
  if (!['G81', 'G83', 'G84'].includes(input.cycle)) errors.cycle = '请选择 G81、G83 或 G84。'
  const peck = input.cycle === 'G83' ? number('peck', .001, 99999.999) : null
  const programNo = number('programNo', 1, 9999, true), tool = number('toolNo', 1, 9999, true)
  const rpm = number('rpm', 1, 99999, true)
  const tapPitch = input.cycle === 'G84' ? number('tapPitch', .001, 20) : null
  const feed = input.cycle === 'G84' ? rigidTappingFeed(input.tapPitch, input.rpm) : number('feed', .001, 99999.999)
  if (input.cycle === 'G84' && tapPitch !== null && rpm !== null && feed === null) errors.tapPitch = '转速与螺距计算的进给须在 0.001～99999.999 mm/min 内。'
  const safe = number('safeZ', .001, 99999.999), r = number('rPlane', .001, 99999.999), bottom = number('bottomZ', -99999.999, -.001)
  if (safe !== null && r !== null && safe < r) errors.safeZ = '初始安全高度须不低于循环 R 平面。'
  if (!WORK_OFFSETS.includes(input.workOffset as typeof WORK_OFFSETS[number])) errors.workOffset = '请选择 G54～G59。'
  if (!['G98', 'G99'].includes(input.returnLevel)) errors.returnLevel = '请选择 G98 或 G99。'
  if (typeof input.coolant !== 'boolean') errors.coolant = '冷却设置无效。'
  if (!holes || Object.keys(errors).length || programNo === null || tool === null || rpm === null || feed === null || safe === null || r === null || bottom === null) return { errors, holes, program: null, filename: null }
  const repeat = kind === 'line' && input.method === 'repeat'
  const name = `O${String(programNo).padStart(4, '0')}`
  const lines = ['%', name, 'G21 G17 G90 G94 G40 G49 G80 G15', 'M05', 'M09', `T${String(tool).padStart(2, '0')} M06`, input.workOffset,
    'G52 X0. Y0. Z0.', ...(input.cycle === 'G84' ? [] : [`S${rpm} M03`]), `G00 G43 H${String(tool).padStart(2, '0')} Z${ncNumber(safe)}`, ...(input.coolant ? ['M08'] : []),
    ...(input.cycle === 'G84' ? [`M29 S${rpm}`] : []),
    ...holes.slice(0, repeat ? 1 : undefined).map((h, i) => i === 0
      ? `${input.returnLevel} ${input.cycle} X${ncNumber(h.x)} Y${ncNumber(h.y)} Z${ncNumber(bottom)} R${ncNumber(r).replace(/\.$/, '.0')}${input.cycle === 'G83' && peck !== null ? ` Q${ncNumber(peck)}` : ''} F${ncNumber(feed)}`
      : `X${ncNumber(h.x)} Y${ncNumber(h.y)}`),
    ...(repeat && holes.length > 1 ? [`G91 ${input.axis}${ncNumber(linePitch)} K${holes.length - 1}`] : []),
    'G80', ...(repeat ? ['G90'] : []), `G00 Z${ncNumber(safe)}`, 'M09', 'M05', 'M30', '%']
  return { errors, holes, program: lines.join('\r\n') + '\r\n', filename: `${name}.nc` }
}
