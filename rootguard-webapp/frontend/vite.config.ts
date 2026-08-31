/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        // Found in review: this was a fixed developer machine's own LAN
        // IP - `npm run dev` only ever worked out of the box on that one
        // machine, silently proxying nowhere (ECONNREFUSED) for anyone
        // else. Defaults to loopback, where WebApp listens locally by
        // default; override per-machine via .env.local if the backend
        // runs elsewhere (see vite's own env file loading).
        target: process.env.VITE_API_PROXY_TARGET ?? 'http://127.0.0.1:8080',
        changeOrigin: true,
      }
    }
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/setupTests.ts'],
  },
})

