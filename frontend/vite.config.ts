import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// 构建产物输出到后端 embed 目录，由 Go 二进制内嵌。
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: { outDir: '../backend/internal/web/dist', emptyOutDir: true },
  server: { proxy: { '/api': 'http://127.0.0.1:8080' } },
  test: { environment: 'jsdom', setupFiles: ['./src/test-setup.ts'], globals: true },
})
