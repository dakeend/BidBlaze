import { readFile } from 'node:fs/promises'
import { resolve } from 'node:path'

const root = resolve('E:/code/ai_zijie/auction-system')

async function read(relPath) {
  return readFile(resolve(root, relPath), 'utf8')
}

function assertMatch(source, pattern, message) {
  if (!pattern.test(source)) {
    throw new Error(message)
  }
}

function assertNoMatch(source, pattern, message) {
  if (pattern.test(source)) {
    throw new Error(message)
  }
}

const [adminVite, mobileVite, adminApi, mobileApi, serveAdmin, serveMobile] = await Promise.all([
  read('admin-web/vite.config.ts'),
  read('mobile-h5/vite.config.ts'),
  read('admin-web/src/lib/api-client.ts'),
  read('mobile-h5/src/lib/api-client.ts'),
  read('scripts/serve-admin-dev.sh'),
  read('scripts/serve-mobile-dev.sh'),
])

assertMatch(adminVite, /proxy:\s*\{[\s\S]*['"]\/api['"]:/, 'admin-web vite config must proxy /api')
assertMatch(adminVite, /proxy:\s*\{[\s\S]*['"]\/ws['"]:/, 'admin-web vite config must proxy /ws')
assertMatch(mobileVite, /proxy:\s*\{[\s\S]*['"]\/api['"]:/, 'mobile-h5 vite config must proxy /api')
assertMatch(mobileVite, /proxy:\s*\{[\s\S]*['"]\/ws['"]:/, 'mobile-h5 vite config must proxy /ws')

assertNoMatch(
  adminApi,
  /baseURL:\s*import\.meta\.env\.VITE_API_BASE\s*\|\|\s*['"]http:\/\/localhost:8080['"]/,
  'admin-web api client must not default to localhost:8080 in dev',
)
assertNoMatch(
  mobileApi,
  /const apiBase = import\.meta\.env\.VITE_API_BASE\s*\|\|\s*['"]http:\/\/localhost:8080['"]/,
  'mobile-h5 api client must not default to localhost:8080 in dev',
)

assertNoMatch(
  serveAdmin,
  /export VITE_API_BASE=|export VITE_WS_BASE=/,
  'serve-admin-dev.sh should rely on dev proxy instead of VITE_API_BASE/VITE_WS_BASE',
)
assertNoMatch(
  serveMobile,
  /export VITE_API_BASE=|export VITE_WS_BASE=/,
  'serve-mobile-dev.sh should rely on dev proxy instead of VITE_API_BASE/VITE_WS_BASE',
)

console.log('dev proxy checks passed')
