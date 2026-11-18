import { expect, test } from 'vitest'
import { buildArrayDrilling, EMPTY_ARRAY_INPUT, EXAMPLE_ARRAY_INPUT } from './array-drilling'
import type { ArrayDrillingInput } from './array-drilling'

test('直线首孔+K剩余孔数，坐标不受G91高度影响，退刀前恢复G90', () => {
  const r = buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, startX: '10', startY: '-5', toolNo: '12' }, 'line')
  expect(r.holes?.map(h => [h.x, h.y])).toEqual([[10,-5],[30,-5],[50,-5],[70,-5],[90,-5]])
  expect(r.program).toContain('G00 G43 H12 Z30.')
  expect(r.program).toContain('G98 G81 X10. Y-5. Z-10. R3.0 F100.\r\nG91 X20. K4\r\nG80\r\nG90\r\nG00 Z30.')
  expect(r.program).not.toMatch(/[();]/)
  expect(r.program?.replaceAll('\r\n', '')).not.toMatch(/[\r\n]/)
})
test('直线Y负向、绝对方式、单孔无K，毫厘孔距不累积浮点误差', () => {
  const r = buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, axis: 'Y', sign: 'negative', pitch: '.001', count: '4' }, 'line')
  expect(r.holes?.map(h => h.y)).toEqual([0,-.001,-.002,-.003])
  expect(r.program).toContain('G91 Y-0.001 K3')
  const absolute = buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, method: 'absolute', axis: 'Y', sign: 'negative', returnLevel: 'G99', coolant: false }, 'line')
  expect(absolute.program).toContain('G99 G81')
  expect(absolute.program).toContain('X0. Y-80.')
  expect(absolute.program).not.toMatch(/G91|M08/)
  expect(buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, count: '1', pitch: '' }, 'line').program).not.toMatch(/G91|K\d/)
})
test('矩形蛇形与逐行顺序、负向孔距，与输出坐标一致', () => {
  const r = buildArrayDrilling(EXAMPLE_ARRAY_INPUT, 'grid')
  expect(r.holes?.map(h => [h.x,h.y])).toEqual([[0,0],[30,0],[60,0],[60,20],[30,20],[0,20]])
  expect(r.program).toContain('X60. Y0.\r\nX60. Y20.\r\nX30. Y20.')
  expect(r.program).not.toContain('G91')
  const plain = buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, snake: false, pitchX: '-30', pitchY: '-20' }, 'grid')
  expect(plain.holes?.map(h => [h.x,h.y])).toEqual([[0,0],[-30,0],[-60,0],[0,-20],[-30,-20],[-60,-20]])
  expect(buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, rows: '1', columns: '1', pitchX: '', pitchY: '' }, 'grid').holes).toHaveLength(1)
})
test('总孔数、终点范围与加工参数缺失均禁止导出', () => {
  expect(buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, rows: '20', columns: '18' }, 'grid').holes).toHaveLength(360)
  expect(buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, rows: '20', columns: '19' }, 'grid').program).toBeNull()
  expect(buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, startX: '99999', pitch: '1' }, 'line').holes).toBeNull()
  expect(buildArrayDrilling({ ...EMPTY_ARRAY_INPUT, pitch: '20' }, 'line').holes).toHaveLength(5)
  expect(buildArrayDrilling({ ...EMPTY_ARRAY_INPUT, pitch: '20' }, 'line').program).toBeNull()
})
test.each([{safeZ:'2'},{rPlane:'0'},{bottomZ:'0'},{feed:'1e3'},{rpm:'0'},{toolNo:'1\nM03'},{programNo:'10000'},{startX:'NaN'},{count:'2.5'},{pitch:'0'},{axis:'Z'},{sign:'bad'},{method:'bad'},{workOffset:'G92'},{returnLevel:'G00'}])('拒绝直线无效参数 %j', patch => {
  expect(buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, ...patch } as ArrayDrillingInput, 'line').program).toBeNull()
})
test.each([{pitchX:'0'},{pitchY:'0'},{rows:'0'},{columns:'2.5'},{startY:'-99999',pitchY:'-2'},{snake:'false'}])('拒绝阵列无效参数 %j', patch => {
  expect(buildArrayDrilling({ ...EXAMPLE_ARRAY_INPUT, ...patch } as ArrayDrillingInput, 'grid').program).toBeNull()
})
