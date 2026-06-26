import { execSync } from 'node:child_process'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

function resolveBackendTarget(): string {
  if (process.env.DEV_PROXY_TARGET) return process.env.DEV_PROXY_TARGET
  try {
    const ip = execSync('wsl -d Ubuntu -- hostname -I', { timeout: 3000, encoding: 'utf8' })
      .split(' ')[0].trim()
    if (/^\d+\.\d+\.\d+\.\d+$/.test(ip)) {
      console.log(`[proxy] WSL2 backend → http://${ip}:8080`)
      return `http://${ip}:8080`
    }
  } catch {}
  return 'http://127.0.0.1:8080'
}

const backendTarget = resolveBackendTarget()

export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    port: 5174,
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
    fs: {
      allow: ['..'],
    },
  },
})
