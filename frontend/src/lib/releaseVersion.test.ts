import { afterEach, expect, test, vi } from 'vitest'
import { checkRelease } from './releaseVersion'

afterEach(() => vi.unstubAllGlobals())

test.each([
  ['2.5.9', '发现新版本 v2.5.10'],
  ['v2.5.10', '已是最新正式版'],
  ['2.6.0', '当前版本高于最新正式版 v2.5.10'],
  ['dev', '当前为非正式版本，最新正式版为 v2.5.10'],
])('%s 与正式版按版本号比较', async (current, expected) => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json({ tag_name: 'v2.5.10', draft: false, prerelease: false })))
  expect((await checkRelease(current)).message).toBe(expected)
})

test('限流时提示检查失败', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 403 })))
  await expect(checkRelease('2.5.3')).rejects.toThrow()
})
