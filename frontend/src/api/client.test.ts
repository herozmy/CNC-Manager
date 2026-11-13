import { afterEach, expect, test, vi } from 'vitest'
import { ApiError, UNAUTHORIZED_EVENT, http } from './client'
import { mockApi } from '../test-utils'

afterEach(() => vi.unstubAllGlobals())

test('成功响应解析 JSON，并带上 JSON 请求头与请求体', async () => {
  const calls = mockApi(() => ({ body: { id: 1 } }))
  expect(await http.post('/drawings', { a: 1 })).toEqual({ id: 1 })
  expect(calls[0]).toMatchObject({ method: 'POST', path: '/drawings', body: { a: 1 } })
  const init = (fetch as unknown as ReturnType<typeof vi.fn>).mock.calls[0][1] as RequestInit
  expect(init.headers).toEqual({ 'Content-Type': 'application/json' })
})

test('204 返回 undefined', async () => {
  mockApi(() => ({ status: 204 }))
  expect(await http.del('/drawings/1')).toBeUndefined()
})

test('错误响应取服务端的中文 error 字段', async () => {
  mockApi(() => ({ status: 409, body: { error: '该图纸号已存在' } }))
  await expect(http.post('/drawings')).rejects.toMatchObject({ name: 'ApiError', status: 409, message: '该图纸号已存在' })
})

test('非 JSON 的错误响应给通用提示', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response('<html>bad gateway</html>', { status: 502 })))
  const err = (await http.get('/x').catch((e: unknown) => e)) as ApiError
  expect(err).toBeInstanceOf(ApiError)
  expect(err.message).toBe('请求失败（502）')
})

test('普通接口 401 触发全局登出事件', async () => {
  mockApi(() => ({ status: 401, body: { error: '请先登录' } }))
  const handler = vi.fn()
  window.addEventListener(UNAUTHORIZED_EVENT, handler)
  await http.get('/drawings').catch(() => {})
  window.removeEventListener(UNAUTHORIZED_EVENT, handler)
  expect(handler).toHaveBeenCalledTimes(1)
})

test('登录接口自己的 401（密码错误）不触发全局登出', async () => {
  mockApi(() => ({ status: 401, body: { error: '用户名或密码错误' } }))
  const handler = vi.fn()
  window.addEventListener(UNAUTHORIZED_EVENT, handler)
  await expect(http.post('/auth/login', {})).rejects.toThrow('用户名或密码错误')
  window.removeEventListener(UNAUTHORIZED_EVENT, handler)
  expect(handler).not.toHaveBeenCalled()
})
