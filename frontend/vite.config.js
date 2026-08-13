import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    // В dev-режиме запросы к /api идут через vite-прокси на Go-сервер,
    // поэтому и в dev, и в проде фронтенд работает «через сервер на Go».
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
