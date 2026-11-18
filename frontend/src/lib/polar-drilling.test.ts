import { expect, test } from 'vitest'
import { buildPolarDrilling, EMPTY_POLAR_INPUT, EXAMPLE_POLAR_INPUT, ncNumber } from './polar-drilling'
import type { PolarDrillingInput } from './polar-drilling'

test('整圆六孔只输出六个不同孔位，G16 X为半径Y为绝对角度', () => {
  const r = buildPolarDrilling(EXAMPLE_POLAR_INPUT)
  expect(r.geometry?.holes.map(h => h.angle)).toEqual([0, 60, 120, 180, 240, 300])
  expect(r.geometry?.holes[0]).toMatchObject({ x: 50, y: 0 })
  expect(r.geometry?.holes[3]).toMatchObject({ x: -50, y: 0 })
  expect(r.program).toContain('G98 G81 X50. Y0. Z-10. R3.0 F100.')
  expect(r.program).toContain('Y300.')
  expect(r.program).not.toContain('Y360.')
  expect(r.program?.split('\r\n').filter(line => /^(?:G98 G81 |Y)/.test(line))).toHaveLength(6)
  expect(r.program).not.toMatch(/[();]/)
  expect(r.program?.match(/X50\./g)).toHaveLength(1)
  expect(r.program?.split('\r\n').filter(line => /^Y/.test(line))).toEqual(['Y60.', 'Y120.', 'Y180.', 'Y240.', 'Y300.'])
  expect(r.filename).toBe('O1000.nc')
})

test('负起始角、顺时针与跨零点按最终输出角度计算孔位', () => {
  const cw = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, holeCount: '3', startAngle: '30', direction: 'cw' })
  expect(cw.geometry?.holes.map(h => h.angle)).toEqual([30, 270, 150])
  expect(cw.geometry?.holes[1]).toMatchObject({ x: 0, y: -50 })
  const crossing = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, layout: 'step', stepAngle: '10', holeCount: '4', startAngle: '-10' })
  expect(crossing.geometry?.holes.map(h => h.angle)).toEqual([350, 0, 10, 20])
  expect(crossing.program).toContain('Y0.')
})

test('七等分的坐标与NC使用同一舍入角度，显示精度内不重合', () => {
  const r = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, holeCount: '7' })
  expect(r.geometry?.holes[1].angle).toBe(51.429)
  expect(r.program).toContain('Y51.429')
  const hole = r.geometry!.holes[1]
  expect(hole.x).toBe(Number((50 * Math.cos(hole.angle * Math.PI / 180)).toFixed(3)))
  expect(r.geometry?.holes.map(h => h.angle)).not.toContain(360)
  expect(buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, radius: '.001', holeCount: '360' }).geometry).toBeNull()
})

test('单孔无需孔间角，支持360孔上限；超限不生成代码', () => {
  const one = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, layout: 'step', stepAngle: '', holeCount: '1' })
  expect(one.geometry?.holes).toHaveLength(1)
  expect(one.program).not.toBeNull()
  expect(buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, holeCount: '360' }).geometry?.holes).toHaveLength(360)
  expect(buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, holeCount: '361' }).program).toBeNull()
})

test('G98/G99、刀长与固定循环的顺序明确，最后取消循环和极坐标并退回Z', () => {
  const r = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, returnLevel: 'G99', workOffset: 'G55', toolNo: '2', coolant: false })
  const code = r.program!
  expect(code).toContain('T02 M06\r\nG55\r\nG52 X0. Y0. Z0.')
  expect(code).toContain('G00 G43 H02 Z30.')
  expect(code.indexOf('G00 G43')).toBeLessThan(code.indexOf('G17 G90 G16'))
  expect(code).toContain('G99 G81')
  expect(code).not.toContain('M08')
  expect(code).toContain('G80\r\nG15\r\nG00 Z30.\r\nM09\r\nM05\r\nM30\r\n%\r\n')
  expect(code).not.toMatch(/G91|G28|G53|#[0-9]/)
  expect([...code].every(c => c.charCodeAt(0) <= 127)).toBe(true)
  expect(code.replaceAll('\r\n', '')).not.toMatch(/[\r\n]/)
})

test('几何参数完成后可核对孔位，但不能在加工参数缺失时导出', () => {
  const r = buildPolarDrilling({ ...EMPTY_POLAR_INPUT, radius: '50' })
  expect(r.geometry?.holes).toHaveLength(6)
  expect(r.program).toBeNull()
  expect(r.filename).toBeNull()
})

test.each([
  { safeZ: '2' }, { rPlane: '0' }, { bottomZ: '10' }, { bottomZ: '0' }, { feed: '0' }, { rpm: '0' },
  { radius: '0' }, { holeCount: '2.5' }, { radius: '50.0001' }, { feed: '1e3' },
  { toolNo: '1\nM03' }, { programNo: '1000\nM30' }, { programNo: '10000' },
  { workOffset: 'G54\nG92 X0' }, { returnLevel: 'G00' as PolarDrillingInput['returnLevel'] },
  { layout: 'other' as PolarDrillingInput['layout'] }, { direction: 'other' as PolarDrillingInput['direction'] },
  { radius: '9'.repeat(400) }, { rpm: '500.5' },
])('拒绝无效或不符合高度/格式条件的输入 %j', patch => {
  const r = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, ...patch })
  expect(r.program).toBeNull()
  expect(Object.keys(r.errors).length).toBeGreaterThan(0)
})

test('指定间角禁止绕回首孔；安全高度等于R平面可用', () => {
  expect(buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, layout: 'step', stepAngle: '90', holeCount: '5' }).geometry).toBeNull()
  expect(buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, layout: 'step', stepAngle: '90', holeCount: '4' }).program).not.toBeNull()
  expect(buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, safeZ: '5', rPlane: '5' }).program).not.toBeNull()
})

test('尺寸字始终输出小数点，不产生隐含小数或负零', () => {
  expect(ncNumber(50)).toBe('50.')
  expect(ncNumber(-10.25)).toBe('-10.25')
  expect(ncNumber(-0)).toBe('0.')
})

test('G16内G91只增量角度，K为首孔后的剩余孔数，结束恢复绝对高度', () => {
  const r = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, method: 'repeat', holeCount: '8' })
  expect(r.program).toContain('G98 G81 X50. Y0. Z-10. R3.0 F100.\r\nG91 Y45. K7\r\nG80\r\nG90\r\nG15\r\nG00 Z30.')
  expect(r.program?.match(/X50\./g)).toHaveLength(1)
  expect(r.program).not.toMatch(/[();]/)
  const cw = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, method: 'repeat', layout: 'step', stepAngle: '45', holeCount: '3', startAngle: '10', direction: 'cw', returnLevel: 'G99' })
  expect(cw.program).toContain('G99 G81 X50. Y10.')
  expect(cw.program).toContain('G91 Y-45. K2')
  expect(cw.geometry?.holes.map(h => h.angle)).toEqual([10, 325, 280])
  const one = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, method: 'repeat', holeCount: '1' })
  expect(one.program).not.toContain('G91')
  expect(one.program).not.toMatch(/K\d/)
  const seven = buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, method: 'repeat', holeCount: '7' })
  expect(seven.program).toContain('G91 Y51.429 K6')
  expect(seven.geometry?.holes[6].angle).toBe(308.574)
  expect(buildPolarDrilling({ ...EXAMPLE_POLAR_INPUT, method: 'bad' as PolarDrillingInput['method'] }).program).toBeNull()
})
