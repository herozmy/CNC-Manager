import { QueryClient } from '@tanstack/react-query'
import { expect, test } from 'vitest'
import { SESSION_KEY, setSession } from './session'

test('切换登录状态时丢弃业务缓存，但保留会话查询本身', () => {
  const qc = new QueryClient()
  qc.setQueryData(['drawings', ''], { secret: 'A 用户的数据' })
  qc.setQueryData(['drawing', 1], { secret: 'A 用户的数据' })
  const sessionQuery = qc.getQueryCache().build(qc, { queryKey: SESSION_KEY })

  setSession(qc, { state: 'out' })

  expect(qc.getQueryData(['drawings', ''])).toBeUndefined()
  expect(qc.getQueryData(['drawing', 1])).toBeUndefined()
  expect(qc.getQueryData(SESSION_KEY)).toEqual({ state: 'out' })
  // 关键：会话查询对象没有被换掉，已挂载的观察者才能收到更新
  expect(qc.getQueryCache().find({ queryKey: SESSION_KEY })).toBe(sessionQuery)
})
