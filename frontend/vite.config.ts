/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 3000,
    allowedHosts: true,
    proxy: {
      // /api/sse must be listed before /api to take precedence.
      // The 'compress: false' disables response compression so SSE chunks
      // are forwarded to the browser immediately without buffering.
      '/api/sse': {
        target: 'http://backend:8080',
        changeOrigin: true,
      },
      '/api': {
        target: 'http://backend:8080',
        changeOrigin: true,
      },
      '/v1': {
        target: 'http://backend:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
    exclude: ['**/node_modules/**', '**/dist/**', 'e2e/**'],
  },
})
