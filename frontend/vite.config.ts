import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

/**
 * Vite 配置
 * - 开发端口固定 5173 / 127.0.0.1
 * - /api 反向代理到本机后端 http://127.0.0.1:8080，前端代码里统一写相对路径 /api/**
 * - 生产构建输出到 dist，文件名保留 hash（Vite 默认行为，便于后续前端热更新时做缓存失效）
 */
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    port: 5173,
    host: '127.0.0.1',
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true
        // 后端接口本身带 /api 前缀，因此不做 rewrite
      }
    }
  },
  build: {
    outDir: 'dist',
    // 不做额外压缩配置，保持 Vite 默认的 [name]-[hash].js / [name]-[hash].css
    assetsDir: 'assets',
    chunkSizeWarningLimit: 1500
  }
})
