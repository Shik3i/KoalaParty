import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 45_000,
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  use: { baseURL: 'http://127.0.0.1:4173', trace: 'retain-on-failure' },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'npm run build && cd ../backend && go run ./cmd/server',
    url: 'http://127.0.0.1:4173/api/health',
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
    env: {
      KOALAPARTY_ADDR: ':4173',
      KOALAPARTY_DB: '../frontend/e2e.db',
      KOALAPARTY_WEB_ROOT: '../frontend/build',
      KOALAPARTY_TRUSTED_ORIGINS: 'http://127.0.0.1:4173',
    },
  },
});
