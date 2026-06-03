import { setupWorker } from 'msw/browser'
import { handlers } from './handlers'

export const worker = setupWorker(...handlers)

/** 由 main.tsx 在启动前调用；VITE_USE_MSW=false 时跳过。 */
export async function startMockWorker(): Promise<void> {
  if (import.meta.env.VITE_USE_MSW === 'false') return
  await worker.start({
    onUnhandledRequest: 'bypass', // 静态资源 / 未 mock 的真接口直接放行
    quiet: false,
  })
  console.info('[MSW] mock server started (admin-web)')
}
