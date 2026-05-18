import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

const backendUrl = process.env.VITE_BACKEND_URL || 'http://localhost:8080'
const projectRoot = fileURLToPath(new URL('.', import.meta.url))

// https://vitejs.dev/config/
export default defineConfig({
  root: projectRoot,
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  define: {
    __VUE_I18N_FULL_INSTALL__: true,
    __VUE_I18N_LEGACY_API__: false,
    __INTLIFY_PROD_DEVTOOLS__: false,
  },
  build: {
    outDir: fileURLToPath(new URL('./dist', import.meta.url))
  },
  server: {
    proxy: {
      // 代理 API 请求
      '/api': {
        target: backendUrl,
        changeOrigin: true,
      },
      // 代理 WebSocket
      '/ws': {
        target: backendUrl,
        ws: true,
        changeOrigin: true
      },
      // 代理音频流
      '/proxy': {
        target: backendUrl,
        changeOrigin: true
      },
      '/media': {
        target: backendUrl,
        changeOrigin: true
      }
    }
  }
})
