import { randomUUID } from "node:crypto";
import { writeFileSync } from "node:fs";
import { join } from "node:path";

// Called inside the launcher's private per-run directory. Native App Server
// authentication precedes WebSocket upgrade, including on loopback. Unsupported
// runtimes must fail startup, never fall back to an unprotected listener.
export function codexAuth(runDir: string) {
  const token = randomUUID();
  const file = join(runDir, "app-server.token");
  writeFileSync(file, token, { mode: 0o600 });
  return { token, file, args: ["--ws-auth", "capability-token", "--ws-token-file", file] };
}
