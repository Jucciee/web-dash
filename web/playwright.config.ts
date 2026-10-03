import { defineConfig } from '@playwright/test'
import path from 'node:path'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  use: { baseURL: 'http://127.0.0.1:15173', actionTimeout: 10_000, trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  webServer: [
    { command: 'node ../scripts/e2e-server.mjs', url: 'http://127.0.0.1:18080/api/v1/health/live', timeout: 180_000, reuseExistingServer: false },
    { command: 'npm run dev -- --port 15173', url: 'http://127.0.0.1:15173', timeout: 60_000, reuseExistingServer: false, env: { DASHBOARD_API_TARGET: 'http://127.0.0.1:18080', npm_config_cache: path.resolve('../.cache/npm') } },
  ],
})
