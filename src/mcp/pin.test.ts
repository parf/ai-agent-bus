import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { join, resolve } from "node:path";
import { Bus } from "./bus.ts";
import { fingerprint, samePin } from "./pin.ts";

describe("pin comparison", () => {
  test("case, colons and the prefix do not matter; a blank pin matches nothing", () => {
    expect(samePin("SHA256:AB:01", "sha256:ab01")).toBe(true);
    expect(samePin("ab01", "sha256:ab01")).toBe(true);
    expect(samePin("ab02", "sha256:ab01")).toBe(false);
    expect(samePin("", "")).toBe(false);
    expect(samePin("sha256:", "sha256:")).toBe(false);
  });
});

// A real TLS port with a self-signed certificate: a pinned Bus reaches it, a
// wrong pin and no pin are both refused, and nothing retries over plain HTTP.
describe("an https:// daemon", () => {
  let dir = "", server: ReturnType<typeof Bun.serve>, pin = "";
  beforeAll(() => {
    const base = resolve(import.meta.dir, "../../tmp");
    mkdirSync(base, { recursive: true });
    dir = mkdtempSync(join(base, "pin-"));
    const gen = Bun.spawnSync(["openssl", "req", "-x509", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes", "-days", "2",
      "-subj", "/CN=localhost", "-addext", "subjectAltName=IP:127.0.0.1,DNS:localhost", "-keyout", join(dir, "key.pem"), "-out", join(dir, "cert.pem")]);
    if (gen.exitCode !== 0) throw new Error("openssl: " + gen.stderr.toString());
    server = Bun.serve({
      hostname: "127.0.0.1", port: 0,
      tls: { cert: Bun.file(join(dir, "cert.pem")), key: Bun.file(join(dir, "key.pem")) },
      fetch: () => Response.json({ you: "#pinned@test" }),
    });
    const der = Bun.spawnSync(["openssl", "x509", "-in", join(dir, "cert.pem"), "-outform", "DER"]).stdout;
    pin = fingerprint(der);
  });
  afterAll(() => { server?.stop(true); if (dir) rmSync(dir, { recursive: true, force: true }); });
  const bus = (p?: string) => new Bus({ AGENT_BUS_ADDR: `https://127.0.0.1:${server.port}`, AGENT_BUS_TOKEN: "t", AGENT_BUS_NAME: "#pinned@test", ...(p === undefined ? {} : { AGENT_BUS_TLS_FINGERPRINT: p }) });

  test("the right pin reaches it", async () => {
    expect((await bus(pin).status()).you).toBe("#pinned@test");
  });
  test("a wrong pin is refused, naming the pin", async () => {
    await expect(bus("sha256:" + "0".repeat(64)).status()).rejects.toThrow("not the pinned");
  });
  test("no pin trusts nothing self-signed", async () => {
    await expect(bus().status()).rejects.toThrow();
  });
  // The checked certificate, not the first connection, is what every call
  // trusts: once another certificate answers on that port — a relay that
  // passed the check, or a rotation the pin was not told of — calls fail.
  test("a different certificate behind the checked one is refused", async () => {
    const b = bus(pin);
    expect((await b.status()).you).toBe("#pinned@test");
    const port = server.port;
    server.stop(true);
    const gen = Bun.spawnSync(["openssl", "req", "-x509", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes", "-days", "2",
      "-subj", "/CN=localhost", "-addext", "subjectAltName=IP:127.0.0.1,DNS:localhost", "-keyout", join(dir, "key2.pem"), "-out", join(dir, "cert2.pem")]);
    if (gen.exitCode !== 0) throw new Error("openssl: " + gen.stderr.toString());
    server = Bun.serve({
      hostname: "127.0.0.1", port,
      tls: { cert: Bun.file(join(dir, "cert2.pem")), key: Bun.file(join(dir, "key2.pem")) },
      fetch: () => Response.json({ you: "#impostor@test" }),
    });
    await expect(b.status()).rejects.toThrow();
  });
});
