import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  timeout: 45_000,
  use: {
    baseURL: process.env.ERP_BASE_URL ?? 'http://127.0.0.1:8080',
    trace: 'retain-on-failure',
    ...(process.env.ERP_BROWSER_EXECUTABLE
      ? { launchOptions: { executablePath: process.env.ERP_BROWSER_EXECUTABLE } }
      : {}),
  },
  reporter: 'list',
});
