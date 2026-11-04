// Everything the face is told, from its environment and nothing else.
import { existsSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

export type Config = {
  daemon: string;        // a Unix socket path, or http://host:port
  listen: { hostname: string; port: number };
  // PEM text; TLS and redirected plain HTTP share one public port.
  tls?: { cert: string; key: string };
  inner?: string;        // where the TLS server listens behind the front
  dev: boolean;          // serves /_styleguide
};

/** A TLS directory as setup writes it: cert.pem, the chain appended when
 *  chain.pem is there, and key.pem, refused when anybody may read it. */
export function tlsDir(dir: string): { cert: string; key: string } {
  const file = (f: string) => join(dir, f);
  if (!existsSync(file("cert.pem")) || !existsSync(file("key.pem"))) throw new Error(`TLS directory ${dir} lacks cert.pem or key.pem`);
  const mode = statSync(file("key.pem")).mode & 0o777;
  if (mode & 0o007) throw new Error(`${file("key.pem")} is readable by everyone (mode ${mode.toString(8).padStart(4, "0")}); it must be 0640 or tighter`);
  let cert = readFileSync(file("cert.pem"), "utf8");
  if (existsSync(file("chain.pem"))) cert = cert.replace(/\s*$/, "\n") + readFileSync(file("chain.pem"), "utf8");
  return { cert, key: readFileSync(file("key.pem"), "utf8") };
}

export function listenAddr(v: string): { hostname: string; port: number } {
  const m = /^(.*):(\d+)$/.exec(v);
  if (!m) throw new Error(`AGENT_BUS_WEB_ADDR must be host:port, not ${v}`);
  return { hostname: m[1]!.replace(/^\[|\]$/g, "") || "127.0.0.1", port: Number(m[2]) };
}

export function defaultSocket(env: NodeJS.ProcessEnv): string {
  if (env.XDG_RUNTIME_DIR) return `${env.XDG_RUNTIME_DIR}/agent-bus/bus.sock`;
  return "/run/agent-bus/bus.sock";
}

export function load(env: NodeJS.ProcessEnv = process.env): Config {
  for (const name of ["AGENT_BUS_WEB_CERT", "AGENT_BUS_WEB_KEY", "AGENT_BUS_WEB_RUNTIME"]) {
    if (env[name] !== undefined) throw new Error(`${name} is not supported; use AGENT_BUS_WEB_TLS_DIR and RUNTIME_DIRECTORY`);
  }
  const tls = env.AGENT_BUS_WEB_TLS_DIR ? tlsDir(env.AGENT_BUS_WEB_TLS_DIR) : undefined;
  return {
    daemon: env.AGENT_BUS_ADDR || defaultSocket(env),
    listen: listenAddr(env.AGENT_BUS_WEB_ADDR || "127.0.0.1:6780"),
    tls,
    // systemd's RuntimeDirectory, or the caller's own runtime directory.
    inner: join(env.RUNTIME_DIRECTORY || env.XDG_RUNTIME_DIR || "/tmp", env.RUNTIME_DIRECTORY ? "https.sock" : `agent-bus-web-${process.pid}.sock`),
    dev: env.AGENT_BUS_WEB_DEV === "1",
  };
}
