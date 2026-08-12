import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const backend = resolve(root, "backend");
const binary = resolve(
  root,
  ".cache",
  process.platform === "win32"
    ? "koalaparty-e2e-server.exe"
    : "koalaparty-e2e-server",
);
mkdirSync(dirname(binary), { recursive: true });
execFileSync("go", ["build", "-o", binary, "./cmd/server"], {
  cwd: backend,
  stdio: "inherit",
  env: { ...process.env, GOCACHE: resolve(root, ".cache", "go-build") },
});
