import { screen } from '@testing-library/react'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { drawing, mockApi, renderApp } from '../test-utils'
import DrawingPage from './DrawingPage'

afterEach(() => vi.unstubAllGlobals())

const program = (over = {}) => ({ id: 1, operationId: 1, programNo: 'O1234', programName: '粗车', controller: 'FANUC', currentVersionId: 5, currentVersionNo: 3, versionCount: 3, remark: '', ...over })
const op = (over = {}) => ({ id: 1, drawingId: 1, opNo: 10, opName: '粗车', machineName: 'CK6150', fixture: '三爪', zHeight: 25.5, remark: '', programs: [program()], ...over })
const view = () => (
  <Routes>
    <Route path="/drawings/:id" element={<DrawingPage />} />
  </Routes>
)

test('显示图纸信息、工序要点与程序当前版本', async () => {
  mockApi(() => ({ body: { ...drawing({ remark: '注意变形' }), operations: [op(), op({ id: 2, opNo: 20, opName: '精车', zHeight: 0, programs: [program({ id: 2, programNo: 'O2', currentVersionNo: null })] })] } }))
  renderApp(view(), '/drawings/1')
  expect(await screen.findByRole('heading', { name: 'D-1001' })).toBeInTheDocument()
  expect(screen.getByText('注意变形')).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: '10# 粗车' })).toBeInTheDocument()
  expect(screen.getByText('CK6150 · 装夹 三爪 · 垫高 25.5 mm')).toBeInTheDocument()
  expect(screen.getByText('V3')).toBeInTheDocument()
  expect(screen.getByText('无版本')).toBeInTheDocument()
})

test('无工序、无程序的空状态', async () => {
  mockApi(() => ({ body: { ...drawing(), operations: [] } }))
  renderApp(view(), '/drawings/1')
  expect(await screen.findByText('暂无工序')).toBeInTheDocument()
})

test('工序下没有程序时提示', async () => {
  mockApi(() => ({ body: { ...drawing(), operations: [op({ programs: [] })] } }))
  renderApp(view(), '/drawings/1')
  expect(await screen.findByText('暂无程序')).toBeInTheDocument()
})

test('图纸不存在显示提示；非法 ID 不发请求', async () => {
  mockApi(() => ({ status: 404, body: { error: '记录不存在' } }))
  const first = renderApp(view(), '/drawings/999')
  expect(await screen.findByText('图纸不存在或已被删除')).toBeInTheDocument()
  first.unmount()

  const calls = mockApi(() => undefined)
  renderApp(view(), '/drawings/abc')
  expect(await screen.findByText('图纸不存在或已被删除')).toBeInTheDocument()
  expect(calls).toHaveLength(0)
})

test('服务器错误可重试', async () => {
  mockApi(() => ({ status: 500, body: { error: 'x' } }))
  renderApp(view(), '/drawings/1')
  expect(await screen.findByText('图纸加载失败')).toBeInTheDocument()
})
