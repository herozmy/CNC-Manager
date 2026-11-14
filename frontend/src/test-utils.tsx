import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import type { ReactElement } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { vi } from 'vitest'

export interface MockReq {
  method: string
  path: string // 含查询串，不含 /api 前缀
  body: unknown
}
export interface MockRes {
  status?: number
  body?: unknown
}

/** 按请求路由伪造 fetch；返回 undefined 视为未处理（测试应失败）。 */
export function mockApi(handler: (req: MockReq) => MockRes | undefined) {
  const calls: MockReq[] = []
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    const req: MockReq = {
      method: init?.method ?? 'GET',
      path: url.replace(/^\/api/, ''),
      // FormData（上传）原样保留，其余按 JSON 解析
      body: init?.body instanceof FormData ? init.body : init?.body ? JSON.parse(String(init.body)) : undefined,
    }
    calls.push(req)
    const res = handler(req)
    if (!res) throw new Error(`未处理的请求 ${req.method} ${req.path}`)
    const status = res.status ?? 200
    if (status === 204) return new Response(null, { status })
    return new Response(JSON.stringify(res.body ?? {}), { status, headers: { 'Content-Type': 'application/json' } })
  })
  vi.stubGlobal('fetch', fn)
  return calls
}

export function renderApp(ui: ReactElement, route = '/') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  const utils = render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[route]}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
  return { ...utils, client }
}

export const user = { id: 1, username: 'admin', displayName: '管理员', role: 'admin', enabled: true }

export function drawing(over: Partial<Record<string, unknown>> = {}) {
  return {
    id: 1, drawingNo: 'D-1001', name: '壳体', customer: '客户A', material: '6061', drawingVersion: 'B',
    remark: '', createdAt: '2026-01-01T00:00:00Z', updatedAt: new Date().toISOString(), operationCount: 2, programCount: 3, ...over,
  }
}

export const program = (over: Partial<Record<string, unknown>> = {}) => ({
  id: 1, operationId: 1, programNo: 'O1234', programName: '粗车', controller: 'FANUC',
  currentVersionId: 5, currentVersionNo: 3, versionCount: 3, remark: '', ...over,
})

export const operation = (over: Partial<Record<string, unknown>> = {}) => ({
  id: 1, drawingId: 1, opNo: 10, opName: '粗车', machineId: null, machineName: '', fixture: '三爪', centeringMethod: '', xCoordinate: 0, yCoordinate: 0, zHeight: 25.5, remark: '',
  programs: [program()], ...over,
})

export const detail = (ops: unknown[], over: Partial<Record<string, unknown>> = {}) => ({ ...drawing(over), operations: ops })

export const version = (over: Partial<Record<string, unknown>> = {}) => ({
  id: 5, programId: 1, versionNo: 3, sha256: 'ab', fileName: 'O1234.nc', fileSize: 2048, encoding: 'utf-8',
  changeNote: '', createdBy: 'alice', createdAt: new Date().toISOString(), isCurrent: true, ...over,
})
