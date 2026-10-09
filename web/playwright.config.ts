import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  use: { baseURL: 'http://127.0.0.1:18080', actionTimeout: 10_000, trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  webServer: [
    { command: 'node ../scripts/e2e-server.mjs', url: 'http://127.0.0.1:18080/api/v1/health/live', timeout: 180_000, reuseExistingServer: false },
  ],
})
