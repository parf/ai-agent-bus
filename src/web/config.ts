// Everything the face is told, from its environment and nothing else.
import { existsSync } from "node:fs";

export type Config = {
  daemon: string;        // a Unix socket path, or http://host:port
  listen: { hostname: string; port: number };
  tls?: { cert: string; key: string };
  dev: boolean;          // serves /_styleguide
};

export function listenAddr(v: string): { hostname: string; port: number } {
  const m = /^(.*):(\d+)$/.exec(v);
  if (!m) throw new Error(`AGENT_BUS_WEB_ADDR must be host:port, not ${v}`);
  return { hostname: m[1]!.replace(/^\[|\]$/g, "") || "127.0.0.1", port: Number(m[2]) };
}

export function defaultSocket(env: NodeJS.ProcessEnv): string {
  if (env.XDG_RUNTIME_DIR) return `${env.XDG_RUNTIME_DIR}/agent-bus/bus.sock`;
  return `/tmp/agent-bus-${process.getuid?.() ?? 0}.sock`;
}

export function load(env: NodeJS.ProcessEnv = process.env): Config {
  const cert = env.AGENT_BUS_WEB_CERT ?? "", key = env.AGENT_BUS_WEB_KEY ?? "";
  let tls: Config["tls"];
  if (cert || key) {
    // Asking for TLS and missing half of it is a refusal to start, never plain HTTP.
    if (!cert || !key) throw new Error("TLS needs both AGENT_BUS_WEB_CERT and AGENT_BUS_WEB_KEY");
    for (const f of [cert, key]) if (!existsSync(f)) throw new Error(`TLS file ${f} does not exist`);
    tls = { cert, key };
  }
  return {
    daemon: env.AGENT_BUS_ADDR || defaultSocket(env),
    listen: listenAddr(env.AGENT_BUS_WEB_ADDR || "127.0.0.1:6781"),
    tls,
    dev: env.AGENT_BUS_WEB_DEV === "1",
  };
}
