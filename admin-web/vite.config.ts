import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    port: 5174, // 见 docs/dev-setup.md §2
    fs: {
      // 允许 import 仓库根 fixtures/（与 mobile-h5 共用）
      allow: ['..'],
    },
  },
})
