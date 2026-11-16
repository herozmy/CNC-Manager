import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'
import { ThemeProvider } from './ThemeProvider'
import { useTheme } from './theme'

function Control() {
  const { theme, toggle } = useTheme()
  return <button onClick={toggle}>{theme}</button>
}
afterEach(() => { cleanup(); localStorage.clear(); delete document.documentElement.dataset.theme; vi.restoreAllMocks() })

test('主题切换不卸载内容，刷新后恢复已选主题', async () => {
  localStorage.clear()
  const { unmount } = render(<ThemeProvider><Control /><input aria-label="草稿" defaultValue="未保存内容" /></ThemeProvider>)
  const input = screen.getByLabelText('草稿')
  expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
  await userEvent.setup().click(screen.getByRole('button', { name: 'dark' }))
  expect(document.documentElement).toHaveAttribute('data-theme', 'light')
  expect(screen.getByLabelText('草稿')).toBe(input)
  expect(input).toHaveValue('未保存内容')
  unmount()
  render(<ThemeProvider><Control /></ThemeProvider>)
  expect(screen.getByRole('button', { name: 'light' })).toBeInTheDocument()
})

test('浏览器禁用本地存储时仍能切换主题', async () => {
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked') })
  render(<ThemeProvider><Control /></ThemeProvider>)
  await userEvent.setup().click(screen.getByRole('button', { name: 'dark' }))
  expect(document.documentElement).toHaveAttribute('data-theme', 'light')
})
