// Reference cutting-tap drill sizes (not forming taps), checked 2026-11-17.
// Guhring Japan, ISO metric coarse/fine tables. Store the published drill size;
// do not replace this lookup with D - P or round that estimate to a drill size.
export const DRILL_SOURCE = 'https://www.guhring.co.jp/techinfo/techinfo03/'
export const FORMULA_SOURCE = 'https://www.haascnc.com/content/dam/haascnc/videos/bonus-content/ep25-tap-programming/Tapping_Formulas_Sheet.pdf'

export interface MetricThread {
  diameter: number
  pitch: number
  drill: number
  series: 'coarse' | 'fine'
}

const coarse: [number, number, number][] = [
  [2, .4, 1.6], [2.5, .45, 2.05], [3, .5, 2.5], [3.5, .6, 2.9],
  [4, .7, 3.3], [5, .8, 4.2], [6, 1, 5], [7, 1, 6], [8, 1.25, 6.8],
  [10, 1.5, 8.5], [12, 1.75, 10.2], [14, 2, 12], [16, 2, 14],
  [18, 2.5, 15.5], [20, 2.5, 17.5], [22, 2.5, 19.5], [24, 3, 21],
  [27, 3, 24], [30, 3.5, 26.5], [33, 3.5, 29.5], [36, 4, 32],
]
const fine: [number, number, number][] = [
  [2.5, .35, 2.15], [3, .35, 2.65], [3.5, .35, 3.15], [4, .5, 3.5],
  [5, .5, 4.5], [6, .75, 5.2], [7, .75, 6.2], [8, .75, 7.2], [8, 1, 7],
  [10, .75, 9.2], [10, 1, 9], [10, 1.25, 8.8],
  [12, 1, 11], [12, 1.25, 10.8], [12, 1.5, 10.5],
  [14, 1, 13], [14, 1.25, 12.8], [14, 1.5, 12.5], [16, 1, 15], [16, 1.5, 14.5],
  [18, 1, 17], [18, 1.5, 16.5], [18, 2, 16], [20, 1, 19], [20, 1.5, 18.5], [20, 2, 18],
  [22, 1, 21], [22, 1.5, 20.5], [22, 2, 20], [24, 1, 23], [24, 1.5, 22.5], [24, 2, 22],
  [27, 1, 26], [27, 1.5, 25.5], [27, 2, 25], [30, 1, 29], [30, 1.5, 28.5], [30, 2, 28], [30, 3, 27],
  [33, 1.5, 31.5], [33, 2, 31], [33, 3, 30], [36, 1.5, 34.5],
]
export const METRIC_THREADS: MetricThread[] = [
  ...coarse.map(([diameter, pitch, drill]) => ({ diameter, pitch, drill, series: 'coarse' as const })),
  ...fine.map(([diameter, pitch, drill]) => ({ diameter, pitch, drill, series: 'fine' as const })),
]
export const METRIC_DIAMETERS = coarse.map(([diameter]) => diameter)

export function positiveDecimal(value: string): number | null {
  if (!/^(?:\d+(?:\.\d*)?|\.\d+)$/.test(value.trim())) return null
  const n = Number(value)
  return Number.isFinite(n) && n > 0 ? n : null
}

export function calculateTapping(pitch: number, value: string) {
  const rpm = positiveDecimal(value)
  if (rpm === null || !Number.isSafeInteger(rpm) || !Number.isFinite(pitch) || pitch <= 0) return null
  const feed = rpm * pitch
  if (!Number.isFinite(feed)) return null
  return { rpm, feed }
}

export function tappingNumber(n: number, digits = 3) {
  return n.toLocaleString('zh-CN', { maximumFractionDigits: digits, useGrouping: false })
}
