import { expect, test } from 'vitest'
import { buildPolarDrilling, EXAMPLE_POLAR_INPUT } from './polar-drilling'
import { buildArrayDrilling, EXAMPLE_ARRAY_INPUT } from './array-drilling'

const builders = [
  ['极坐标G90', (peck: string) => buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, cycle: 'G83', peck })],
  ['极坐标G91', (peck: string) => buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, cycle: 'G83', peck, method: 'repeat' })],
  ['直线G90', (peck: string) => buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, cycle: 'G83', peck, method: 'absolute' }, 'line')],
  ['直线G91', (peck: string) => buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, cycle: 'G83', peck }, 'line')],
  ['矩形', (peck: string) => buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, cycle: 'G83', peck }, 'grid')],
] as const

test.each(builders)('%s输出G83及正值Q，保留Z/R及结束取消指令', (_name, build) => {
  const r = build('2.5')
  expect(r.program).toContain('G98 G83')
  expect(r.program).toContain('Z-10. R3.0 Q2.5 F100.')
  expect(r.program).not.toContain('G81')
  expect(r.program?.match(/Q2\.5/g)).toHaveLength(1)
  expect(r.program).toContain('G80\r\n')
  expect(r.program).not.toMatch(/[();]/)
  expect(r.program?.replaceAll('\r\n', '')).not.toMatch(/[\r\n]/)
  for (const peck of ['', '0', '-1', 'NaN', 'Infinity', '1e3', '.0001', '1\nM03']) {
    expect(build(peck).program).toBeNull()
    expect(build(peck).errors.peck).toBeDefined()
  }
})
test('G83保持极坐标角度和直线位移K语义，G99及退刀G90不受Q影响', () => {
  const polar = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, cycle: 'G83', peck: '3', method: 'repeat', holeCount: '8', direction: 'cw', returnLevel: 'G99' })
  expect(polar.program).toContain('G99 G83 X50. Y0. Z-10. R3.0 Q3. F100.\r\nG91 Y-45. K7\r\nG80\r\nG90\r\nG15\r\nG00 Z30.')
  const line = buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, cycle: 'G83', peck: '3', axis: 'Y', sign: 'negative' }, 'line')
  expect(line.program).toContain('G91 Y-20. K4\r\nG80\r\nG90\r\nG00 Z30.')
  expect(buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, cycle: 'G83', peck: '.001', count: '1' }, 'line').program).not.toMatch(/K\d|G91/)
})
test('G81忽略旧Q、非法循环类型不可注入，G83不更改孔位', () => {
  expect(buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, peck: 'bad' }).program).not.toContain('Q')
  expect(buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, peck: 'bad' }, 'grid').program).not.toContain('Q')
  expect(buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, cycle: 'G83', peck: '2' }).geometry).toEqual(buildPolarDrilling(EXAMPLE_POLAR_INPUT).geometry)
  expect(buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, cycle: 'G83', peck: '2' }, 'grid').holes).toEqual(buildArrayDrilling(EXAMPLE_ARRAY_INPUT, 'grid').holes)
  expect(buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, cycle: 'G83\nM03' as 'G83' }).program).toBeNull()
  expect(buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, cycle: 'G83\nM03' as 'G83' }, 'line').program).toBeNull()
})
