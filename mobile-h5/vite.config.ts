import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backendTarget = process.env.DEV_PROXY_TARGET || 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    port: 5173,
    proxy: {
      '/api': {
        target: backendTarget,
        changeOrigin: true,
      },
      '/ws': {
        target: backendTarget,
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
