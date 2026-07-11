import { defineConfig } from "@playwright/test";

const port = process.env.E2E_PORT ?? "8091";

export default defineConfig({
  testDir: "./tests",
  timeout: 30_000,
  fullyParallel: false,
  workers: 1,
  reporter: "list",
  use: {
    baseURL: `http://localhost:${port}`,
    trace: "retain-on-failure",
  },
  webServer: {
    command: "./setup-server.sh",
    url: `http://localhost:${port}/healthz`,
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
