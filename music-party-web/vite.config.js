import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

const backendUrl = process.env.VITE_BACKEND_URL || 'http://localhost:8080'
const projectRoot = fileURLToPath(new URL('.', import.meta.url))
const vendorChunks = [
  { name: 'vendor-vue', packages: ['vue', 'pinia', 'vue-i18n', '@vueuse/core'] },
  { name: 'vendor-ui', packages: ['reka-ui', 'lucide-vue-next'] },
  { name: 'vendor-network', packages: ['axios', '@stomp/stompjs'] },
  { name: 'vendor-dnd', packages: ['sortablejs'] },
  { name: 'vendor-utils', packages: ['dayjs', 'clsx', 'tailwind-merge', 'class-variance-authority'] }
]

const normalizeModuleId = (id) => id.replaceAll('\\', '/')
const packagePattern = (pkg) => `/node_modules/${pkg}/`
const resolveVendorChunk = (id) => {
  const normalized = normalizeModuleId(id)
  if (!normalized.includes('/node_modules/')) return undefined

  return vendorChunks.find(({ packages }) =>
    packages.some((pkg) => normalized.includes(packagePattern(pkg)))
  )?.name
}

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
    outDir: fileURLToPath(new URL('./dist', import.meta.url)),
    rollupOptions: {
      output: {
        manualChunks: resolveVendorChunk
      }
    }
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
