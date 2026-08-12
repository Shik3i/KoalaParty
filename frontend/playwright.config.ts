import { defineConfig, devices } from '@playwright/test';
import { resolve } from 'node:path';

const e2eServer =
  process.platform === 'win32'
    ? resolve(process.cwd(), '..', '.cache', 'koalaparty-e2e-server.exe')
    : resolve(process.cwd(), '..', '.cache', 'koalaparty-e2e-server');

export default defineConfig({
  testDir: './e2e',
  timeout: 45_000,
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  globalTeardown: './e2e/global-teardown.mjs',
  use: { baseURL: 'http://127.0.0.1:4187', trace: 'retain-on-failure' },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: e2eServer,
    url: 'http://127.0.0.1:4187/api/health',
    timeout: 240_000,
    reuseExistingServer: false,
    gracefulShutdown: { signal: 'SIGINT', timeout: 1000 },
    env: {
      KOALAPARTY_ADDR: ':4187',
      KOALAPARTY_DB: '../frontend/e2e.db',
      KOALAPARTY_WEB_ROOT: '../frontend/build',
      KOALAPARTY_TRUSTED_ORIGINS: 'http://127.0.0.1:4187',
      KOALAPARTY_E2E: 'true',
    },
  },
});
