import { spawn } from "node:child_process";
import { createServer } from "node:net";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const temporaryDirectory = await mkdtemp(join(tmpdir(), "finapp-e2e-"));
const portServer = createServer();
let port;

try {
  await new Promise((resolveListen, rejectListen) => {
    portServer.once("error", rejectListen);
    portServer.listen(0, "127.0.0.1", resolveListen);
  });
  port = portServer.address().port;
  await new Promise((resolveClose, rejectClose) => {
    portServer.close((error) => error ? rejectClose(error) : resolveClose());
  });

  const cli = resolve("node_modules/@playwright/test/cli.js");
  const child = spawn(process.execPath, [cli, "test", ...process.argv.slice(2)], {
    stdio: "inherit",
    env: {
      ...process.env,
      FINAPP_E2E_PORT: String(port),
      FINAPP_E2E_DB_PATH: join(temporaryDirectory, "finapp.db"),
    },
  });
  const exitCode = await new Promise((resolveExit, rejectExit) => {
    child.once("error", rejectExit);
    child.once("exit", (code, signal) => {
      if (signal) {
        rejectExit(new Error(`Playwright exited after ${signal}`));
      } else {
        resolveExit(code ?? 1);
      }
    });
  });
  process.exitCode = exitCode;
} finally {
  await rm(temporaryDirectory, { recursive: true, force: true });
}
