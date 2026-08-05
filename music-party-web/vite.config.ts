import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'
import { resolveVendorChunk } from './config/vendorChunks.js'

const backendUrl = process.env.VITE_BACKEND_URL || 'http://localhost:8080'
const projectRoot = fileURLToPath(new URL('.', import.meta.url))

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
    __INTLIFY_PROD_DEVTOOLS__: false
  },
  build: {
    outDir: fileURLToPath(new URL('./dist', import.meta.url)),
    rollupOptions: {
      output: {
        manualChunks: resolveVendorChunk
      }
    }
  },
  server: {
    proxy: {
      '/api': { target: backendUrl, changeOrigin: true },
      '/ws': { target: backendUrl, ws: true, changeOrigin: true },
      '/proxy': { target: backendUrl, changeOrigin: true },
      '/media': { target: backendUrl, changeOrigin: true }
    }
  }
})
