import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  esbuild: {
    sourcemap: false
  },
  test: {
    include: ['src/**/*.test.{js,ts}'],
    environment: 'happy-dom',
    globals: false
  }
})
