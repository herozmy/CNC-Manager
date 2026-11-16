import { useLayoutEffect, useState } from 'react'
import type { ReactNode } from 'react'

import { ThemeContext } from './theme'
import type { Theme } from './theme'
const storageKey = 'cnc-manager-theme'

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => {
    try { return localStorage.getItem(storageKey) === 'light' ? 'light' : 'dark' } catch { return 'dark' }
  })
  useLayoutEffect(() => {
    document.documentElement.dataset.theme = theme
    try { localStorage.setItem(storageKey, theme) } catch { /* Storage can be unavailable in private browsing. */ }
  }, [theme])
  return <ThemeContext value={{ theme, toggle: () => setTheme(value => value === 'dark' ? 'light' : 'dark') }}>{children}</ThemeContext>
}

