import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  retries: 0,
  use: {
    baseURL: 'http://127.0.0.1:4179',
    launchOptions: { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH },
    screenshot: 'only-on-failure',
  },
  webServer: {
    command: 'npm run preview -- --host 127.0.0.1 --port 4179 --strictPort',
    url: 'http://127.0.0.1:4179',
    reuseExistingServer: false,
  },
})
