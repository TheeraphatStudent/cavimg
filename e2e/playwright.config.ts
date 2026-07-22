import { defineConfig } from '@playwright/test';

const url = process.env.CAVIMG_E2E_URL ?? 'http://localhost:5173';
const app = process.env.CAVIMG_E2E_APP ?? 'vite';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  workers: 1,
  reporter: [['list']],
  use: { baseURL: url, headless: true },
  projects: [{ name: app, use: { browserName: 'chromium' } }],
  webServer: {
    command: process.env.CAVIMG_E2E_CMD ?? 'pnpm --filter @cavimg/example-vite dev',
    cwd: process.env.CAVIMG_E2E_CWD || undefined,
    url,
    reuseExistingServer: false,
    timeout: 180_000,
  },
});
