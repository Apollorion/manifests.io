import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  forbidOnly: !!process.env.CI,
  fullyParallel: true,
  workers: 2,
  retries: 0,
  timeout: 30_000,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080',
    browserName: 'chromium',
    viewport: { width: 1280, height: 900 },
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
});
