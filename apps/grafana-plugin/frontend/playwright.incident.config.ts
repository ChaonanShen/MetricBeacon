import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '../../../tests/e2e/incident',
  testMatch: 'browser-e2e.spec.ts',
  timeout: 90_000,
  expect: { timeout: 20_000 },
  retries: 0,
  workers: 1,
  use: {
    baseURL: process.env.GRAFANA_URL ?? 'http://127.0.0.1:3000',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
});
