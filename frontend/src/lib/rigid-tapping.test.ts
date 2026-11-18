import { expect, test } from 'vitest'
import { rigidTappingFeed } from './rigid-tapping'
import { buildPolarDrilling, EXAMPLE_POLAR_INPUT } from './polar-drilling'
import { buildArrayDrilling, EXAMPLE_ARRAY_INPUT } from './array-drilling'

const builders = [
  ['极坐标G90', (tapPitch: string, rpm = '500') => buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, cycle: 'G84', tapPitch, rpm, feed: 'bad', peck: 'bad' })],
  ['极坐标G91', (tapPitch: string, rpm = '500') => buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, cycle: 'G84', tapPitch, rpm, feed: 'bad', method: 'repeat' })],
  ['直线G90', (tapPitch: string, rpm = '500') => buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, cycle: 'G84', tapPitch, rpm, feed: 'bad', method: 'absolute' }, 'line')],
  ['直线G91', (tapPitch: string, rpm = '500') => buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, cycle: 'G84', tapPitch, rpm, feed: 'bad' }, 'line')],
  ['矩形', (tapPitch: string, rpm = '500') => buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, cycle: 'G84', tapPitch, rpm, feed: 'bad' }, 'grid')],
] as const

test.each(builders)('%s刚性准备紧邻G84，F由螺距转速计算，不带钻孔Q或M03', (_name, build) => {
  const r = build('1.25')
  expect(r.program).toContain('M29 S500\r\nG98 G84')
  expect(r.program).toContain('Z-10. R3.0 F625.')
  expect(r.program?.match(/M29/g)).toHaveLength(1)
  expect(r.program).not.toMatch(/M03|Q\d|G81|G83|[();]/)
  expect(r.program).toContain('G94')
  expect(r.program).toContain('G80\r\n')
  expect(r.program?.replaceAll('\r\n', '')).not.toMatch(/[\r\n]/)
  expect(build('1.25','600').program).toContain('M29 S600\r\nG98 G84')
  expect(build('1.25','600').program).toContain('F750.')
  for (const pitch of ['', '0', '-1', '1e3', '.0001', '20.001', 'Infinity', '1\nM03']) expect(build(pitch).program).toBeNull()
  expect(build('20','99999').program).toBeNull()
  expect(build('1','0').program).toBeNull()
  expect(build('1','500.5').program).toBeNull()
})
test('进给保持三位小数精度，边界/非法值不显示可导出结果', () => {
  expect(rigidTappingFeed('.333', '1001')).toBe(333.333)
  expect(rigidTappingFeed('.001', '1')).toBe(.001)
  expect(rigidTappingFeed('1', '99999')).toBe(99999)
  expect(rigidTappingFeed('1.001', '99999')).toBeNull()
  expect(rigidTappingFeed('NaN','500')).toBeNull()
  expect(rigidTappingFeed('1','Infinity')).toBeNull()
})
test('G84保留孔位/K/G99及退刀，切回钻孔不带M29或螺距进给', () => {
  const polar = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, cycle: 'G84', tapPitch: '1', rpm: '500', returnLevel: 'G99', method: 'repeat', holeCount: '8', direction: 'cw' })
  expect(polar.program).toContain('G17 G90 G16\r\nM29 S500\r\nG99 G84 X50. Y0. Z-10. R3.0 F500.\r\nG91 Y-45. K7\r\nG80\r\nG90\r\nG15\r\nG00 Z30.')
  const line = buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, cycle: 'G84', tapPitch: '1', rpm: '500', axis: 'Y', sign: 'negative', coolant: false }, 'line')
  expect(line.program).toContain('G91 Y-20. K4\r\nG80\r\nG90\r\nG00 Z30.')
  expect(line.program).not.toContain('M08')
  expect(buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, cycle: 'G84', tapPitch: '1', count: '1' }, 'line').program).not.toMatch(/G91|K\d/)
  const drill = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, tapPitch: 'bad', rpm: '500' })
  expect(drill.program).not.toContain('M29')
  expect(drill.program).toContain('S500 M03')
  expect(drill.program).toContain('F100.')
})
