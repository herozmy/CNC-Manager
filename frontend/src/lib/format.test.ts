import { expect, test } from 'vitest'
import { relativeTime } from './format'

const now = Date.parse('2026-06-15T12:00:00Z')
const ago = (sec: number) => new Date(now - sec * 1000).toISOString()

test('相对时间分档', () => {
  expect(relativeTime(ago(5), now)).toBe('刚刚')
  expect(relativeTime(ago(3 * 60), now)).toBe('3 分钟前')
  expect(relativeTime(ago(5 * 3600), now)).toBe('5 小时前')
  expect(relativeTime(ago(2 * 86400), now)).toBe('2 天前')
})

test('超过 30 天显示日期；非法输入返回空串；未来时间不出现负数', () => {
  expect(relativeTime(ago(40 * 86400), now)).toMatch(/2026/)
  expect(relativeTime('not a date', now)).toBe('')
  expect(relativeTime(new Date(now + 60000).toISOString(), now)).toBe('刚刚')
})
