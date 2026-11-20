import { defineConfig } from "@playwright/test";

const port = process.env.FINAPP_E2E_PORT;
const databasePath = process.env.FINAPP_E2E_DB_PATH;

if (!port || !databasePath) {
  throw new Error("Run browser tests through `make test-browser`.");
}

export default defineConfig({
  testDir: "./tests/browser",
  fullyParallel: false,
  workers: 1,
  reporter: "list",
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    browserName: "chromium",
    trace: "retain-on-failure",
  },
  webServer: {
    command: "go run ./cmd/finapp",
    url: `http://127.0.0.1:${port}/healthz`,
    env: {
      ...process.env,
      FINAPP_LISTEN_ADDR: `127.0.0.1:${port}`,
      FINAPP_DB_PATH: databasePath,
    },
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
