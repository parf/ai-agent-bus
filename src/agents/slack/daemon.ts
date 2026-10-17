// A real daemon for the agent's tests: one fixture per test file, started
// from the built tree when smoke.sh provides it (SMOKE_BIN) or from a local
// build the first time `bun test` runs here. Identities and their tokens are
// minted over the account socket exactly as smoke.sh's fixture does.
import { spawn, spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync } from "node:fs";
import { join, resolve } from "node:path";

const repo = resolve(import.meta.dir, "../../..");

function daemonBinary(): string {
  if (process.env.SMOKE_BIN) return join(process.env.SMOKE_BIN, "agent-busd");
  // Always (re)built: `go build` is incremental, and a stale scratch binary
  // must never stand in for the current source (slack-review-S.1).
  const local = join(repo, "tmp/agent-tests/agent-busd");
  mkdirSync(join(repo, "tmp/agent-tests"), { recursive: true });
  const built = spawnSync("go", ["build", "-o", local, "./cmd/agent-busd"], { cwd: join(repo, "src"), encoding: "utf8" });
  if (built.status !== 0) throw new Error(`building the fixture daemon failed: ${built.stderr}`);
  return local;
}

export type Fixture = {
  dir: string;
  socket: string;
  owner: string;
  ownerToken: string;
  /** One authenticated API call over the fixture's socket. */
  call: (token: string, method: string, path: string, body?: unknown) => Promise<any>;
  stop: () => Promise<void>;
};

/** Call the fixture daemon over its unix socket with a token. */
export async function call(socket: string, token: string, method: string, path: string, body?: unknown): Promise<any> {
  const response = await fetch(`http://bus${path}`, {
    method,
    unix: socket,
    headers: { ...(token ? { "x-agent-bus-token": token } : {}), ...(body === undefined ? {} : { "content-type": "application/json" }) },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await response.text();
  const parsed = text ? JSON.parse(text) : {};
  if (response.status >= 400) throw new Error(`${method} ${path} refused ${response.status}: ${text.slice(0, 200)}`);
  return parsed;
}

export async function startFixture(account = process.env.USER || "tester"): Promise<Fixture> {
  const dir = mkdtempSync(join(repo, "tmp", `slack-agent-`));
  const owner = `owner-${process.pid}@t${(Math.random() * 1e6) | 0}`;
  const socket = join(dir, "bus.sock");
  const port = 20000 + ((Math.random() * 20000) | 0);
  const daemon = spawn(daemonBinary(), ["-addr", `127.0.0.1:${port}`, "-socket", socket, "-owner", owner, "-db", join(dir, "bus.db"), "-create", "-flush-every", "0"], { stdio: ["ignore", "ignore", "pipe"] });
  const errors: string[] = [];
  daemon.stderr.on("data", d => errors.push(String(d)));
  const userSocket = join(dir, `user-${account}.sock`);
  let lastError = "never reached";
  for (let i = 0; i < 100; i++) {
    if (existsSync(socket) && existsSync(userSocket)) {
      try {
        await call(socket, "", "GET", "/status");
        break;
      } catch (e) { lastError = e instanceof Error ? e.message : String(e); /* the socket exists but the bus behind it is still coming up */ }
    }
    if (daemon.exitCode !== null) throw new Error(`the fixture daemon exited (last status error: ${lastError}): ${errors.join(" ")}`);
    await Bun.sleep(50);
  }
  const { token } = await call(userSocket, "", "POST", "/token", { name: owner });
  return {
    dir, socket, owner, ownerToken: token,
    call: (t, method, path, body) => call(socket, t, method, path, body),
    stop: async () => {
      daemon.kill("SIGTERM");
      if (daemon.exitCode === null) await new Promise<void>(resolve => daemon.on("exit", () => resolve()));
    },
  };
}

/** Register an agent name under the fixture's owner (open ACL: the fixture's
 *  callers must be answerable by the agents under test) and mint its token. */
export async function identity(fx: Fixture, name: string): Promise<string> {
  await fx.call(fx.ownerToken, "POST", "/register", { name, kind: "agent", allow: ["*"] });
  const { token } = await fx.call(fx.ownerToken, "POST", "/token", { name });
  return token;
}
