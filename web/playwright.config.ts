import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  // One workspace, one database: tests share the server the setup starts.
  workers: 1,
  reporter: 'list',
  globalSetup: './e2e/global-setup.ts',
})
