import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  build: {
    // The largest chunk is elkjs (~1.4 MB), loaded only when auto-layout runs.
    // The app is served from localhost, so chunk size is not a concern beyond that.
    chunkSizeWarningLimit: 1500,
  },
  server: {
    port: 5180,
    strictPort: true,
    proxy: { '/api': 'http://127.0.0.1:7070' },
  },
})
