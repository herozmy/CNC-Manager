import { createContext, useContext } from 'react'

export type Theme = 'dark' | 'light'
export const ThemeContext = createContext<{ theme: Theme; toggle: () => void }>({ theme: 'light', toggle: () => {} })
export function useTheme() { return useContext(ThemeContext) }
