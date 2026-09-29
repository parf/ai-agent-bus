import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import { chmodSync, mkdirSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import net from "node:net";
import { join, resolve } from "node:path";
import { dualFront, redirectFor, setSniffMs } from "../listen.ts";
import { tlsDir } from "../config.ts";
import { makeHandler } from "../server.ts";

const openssl = (...args: string[]) => {
  const r = Bun.spawnSync(["openssl", ...args]);
  if (r.exitCode !== 0) throw new Error("openssl " + args[0] + ": " + r.stderr.toString());
  return r.stdout.toString();
};

let dir = "";
beforeAll(() => {
  const base = resolve(import.meta.dir, "../../../tmp");
  mkdirSync(base, { recursive: true });
  dir = mkdtempSync(join(base, "web-listen-"));
  // A CA and a leaf it signs, so a chain can be served and counted.
  openssl("req", "-x509", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes", "-days", "2", "-subj", "/CN=test ca",
    "-keyout", join(dir, "ca.key"), "-out", join(dir, "chain.pem"));
  openssl("req", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes", "-subj", "/CN=localhost",
    "-keyout", join(dir, "key.pem"), "-out", join(dir, "leaf.csr"));
  Bun.write(join(dir, "ext.cnf"), "subjectAltName=IP:127.0.0.1,DNS:localhost\n");
  openssl("x509", "-req", "-in", join(dir, "leaf.csr"), "-CA", join(dir, "chain.pem"), "-CAkey", join(dir, "ca.key"), "-CAcreateserial",
    "-days", "2", "-extfile", join(dir, "ext.cnf"), "-out", join(dir, "cert.pem"));
  chmodSync(join(dir, "key.pem"), 0o600);
});
afterAll(() => { if (dir) rmSync(dir, { recursive: true, force: true }); });

// The inner server and the front, as server.ts wires them.
async function start(handler: (r: Request) => Response | Promise<Response> = (r) => new Response("inner " + new URL(r.url).pathname), extra: object = {}) {
  const { cert, key } = tlsDir(dir);
  const inner = join(dir, `inner-${Math.random().toString(36).slice(2)}.sock`);
  const app = Bun.serve({ unix: inner, tls: { cert, key }, fetch: handler, ...extra } as any);
  const front = await dualFront({ hostname: "127.0.0.1", port: 0, inner });
  const port = (front.address() as net.AddressInfo).port;
  return { port, stop: () => { front.close(); app.stop(true); } };
}
const trust = () => Bun.file(join(dir, "chain.pem")).text();

describe("the web port with TLS on", () => {
  test("answers https:// and redirects http:// on the same port", async () => {
    const s = await start();
    try {
      const r = await fetch(`https://127.0.0.1:${s.port}/agents?x=1`, { tls: { ca: await trust() } } as any);
      expect(await r.text()).toBe("inner /agents");
      const plain = await fetch(`http://127.0.0.1:${s.port}/agents?x=1`, { redirect: "manual" });
      expect(plain.status).toBe(301);
      expect(plain.headers.get("location")).toBe(`https://127.0.0.1:${s.port}/agents?x=1`);
    } finally { s.stop(); }
  });

  test("serves the chain behind the certificate", async () => {
    const s = await start();
    try {
      // Async: the front runs in this process, so a blocking spawn would stall it.
      const p = Bun.spawn(["openssl", "s_client", "-connect", `127.0.0.1:${s.port}`, "-showcerts"], { stdin: "ignore", stdout: "pipe", stderr: "ignore" });
      const out = await new Response(p.stdout).text();
      expect(out.match(/-----BEGIN CERTIFICATE-----/g)?.length).toBe(2);
    } finally { s.stop(); }
  });

  test("a silent client is closed and holds up nobody", async () => {
    setSniffMs(300);
    const s = await start();
    try {
      const silent = net.connect(s.port, "127.0.0.1");
      const closed = new Promise<boolean>((done) => { silent.on("close", () => done(true)); setTimeout(() => done(false), 3000); });
      const t0 = Date.now();
      const r = await fetch(`https://127.0.0.1:${s.port}/`, { tls: { ca: await trust() } } as any);
      expect(r.status).toBe(200);
      expect(Date.now() - t0).toBeLessThan(300);
      expect(await closed).toBe(true);
    } finally { setSniffMs(5000); s.stop(); }
  });

  // The app's own idle limit applies on the unix socket too: a limit of one
  // second cuts a response that takes six (Bun checks it coarsely, within a
  // few seconds), so the face's 150 s is real there.
  test("the inner server's idle timeout is honoured on its unix socket", async () => {
    const s = await start(async () => { await Bun.sleep(6000); return new Response("late"); }, { idleTimeout: 1 });
    try {
      const got = await fetch(`https://127.0.0.1:${s.port}/`, { tls: { ca: await trust() } } as any).then((r) => r.text()).catch(() => "cut");
      expect(got).toBe("cut");
    } finally { s.stop(); }
  }, 15000);
});

describe("the redirect", () => {
  test("keeps the host and path, and never echoes a hostile one", () => {
    expect(redirectFor("GET /a?b=1 HTTP/1.1\r\nHost: bus.example:6780", "127.0.0.1:6780")).toContain("Location: https://bus.example:6780/a?b=1\r\n");
    expect(redirectFor("GET /a HTTP/1.1\r\nHost: evil\r\nX: y", "127.0.0.1:6780")).toContain("Location: https://evil/a\r\n");
    expect(redirectFor("GET /a HTTP/1.1\r\nHost: bad host\r\n", "127.0.0.1:6780")).toContain("Location: https://127.0.0.1:6780/a\r\n");
    expect(redirectFor("GET http://x/y HTTP/1.1\r\nHost: h", "f:1")).toContain("Location: https://h/\r\n");
    expect(redirectFor("", "f:1")).toContain("Location: https://f:1/\r\n");
  });
});

describe("the TLS directory", () => {
  test("a key readable by everyone is refused", () => {
    chmodSync(join(dir, "key.pem"), 0o644);
    try { expect(() => tlsDir(dir)).toThrow("readable by everyone"); } finally { chmodSync(join(dir, "key.pem"), 0o600); }
  });
});

// The real app behind the front: sign-in over TLS sets a Secure session
// cookie, an http:// Origin is refused on the TLS side, and plain /healthz
// answers for the app, 503 once it is gone.
describe("the real app behind the front", () => {
  test("signs in over TLS with a Secure cookie, and health follows the app", async () => {
    const daemonSock = join(dir, "daemon.sock");
    rmSync(daemonSock, { force: true });
    const daemon = Bun.serve({ unix: daemonSock, fetch: (r) => new URL(r.url).pathname === "/session" ? Response.json({ session: "s3ss10n" }) : Response.json({ version: "test" }) });
    const { cert, key } = tlsDir(dir);
    const inner = join(dir, "app.sock");
    rmSync(inner, { force: true });
    const handler = makeHandler({ daemon: daemonSock, listen: { hostname: "127.0.0.1", port: 0 }, dev: false, tls: { cert, key, front: true } });
    const app = Bun.serve({ unix: inner, tls: { cert, key }, fetch: handler } as any);
    const front = await dualFront({ hostname: "127.0.0.1", port: 0, inner });
    const port = (front.address() as net.AddressInfo).port;
    const base = `https://127.0.0.1:${port}`;
    try {
      const signin = (origin: string) => fetch(base + "/signin", { method: "POST", redirect: "manual", tls: { ca: readFileSync(join(dir, "chain.pem"), "utf8") },
        headers: { Origin: origin, "Content-Type": "application/x-www-form-urlencoded" }, body: "token=anything" } as any);
      const ok = await signin(base);
      expect(ok.status).toBe(303);
      const set = ok.headers.get("set-cookie") ?? "";
      expect(set).toContain("agent_bus_session=s3ss10n");
      expect(set).toContain("Secure");
      expect((await signin(`http://127.0.0.1:${port}`)).status).toBe(403);
      const health = await fetch(`http://127.0.0.1:${port}/healthz`, { redirect: "manual" });
      expect(health.status).toBe(200);
      app.stop(true);
      const gone = await fetch(`http://127.0.0.1:${port}/healthz`, { redirect: "manual" });
      expect(gone.status).toBe(503);
    } finally { front.close(); app.stop(true); daemon.stop(true); }
  });
});
