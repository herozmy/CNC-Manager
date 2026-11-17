import { expect, test } from 'vitest'
import { calculateTapping, METRIC_THREADS, positiveDecimal } from './metric-tapping'

test('底孔使用厂家表值，保留细牙且不以直径减螺距替代', () => {
  for (const [diameter, pitch, drill] of [[6, 1, 5], [8, 1.25, 6.8], [10, 1.25, 8.8], [12, 1.75, 10.2], [6, .75, 5.2], [36, 4, 32]]) {
    expect(METRIC_THREADS.find(t => t.diameter === diameter && t.pitch === pitch)?.drill).toBe(drill)
  }
  expect(new Set(METRIC_THREADS.map(t => `${t.diameter}/${t.pitch}`)).size).toBe(METRIC_THREADS.length)
  expect(METRIC_THREADS.every(t => t.drill > 0 && t.drill < t.diameter)).toBe(true)
})
test('同步攻丝按螺距计算进给', () => {
  expect(calculateTapping(1, '500')).toEqual({ rpm: 500, feed: 500 })
  expect(calculateTapping(1.25, '500')?.feed).toBe(625)
})
test.each(['', ' ', '0', '-5', 'abc', 'Infinity', '1e3', '5,000', '1.2.3'])('拒绝无效输入 %j', value => {
  expect(positiveDecimal(value)).toBeNull()
  expect(calculateTapping(1, value)).toBeNull()
})
test('拒绝非整数转速、溢出和无效螺距', () => {
  expect(calculateTapping(1, '500.5')).toBeNull()
  expect(calculateTapping(1, '9'.repeat(400))).toBeNull()
  expect(calculateTapping(0, '500')).toBeNull()
  expect(calculateTapping(Infinity, '500')).toBeNull()
})
