// Reaching an https:// daemon from TypeScript. Bun's fetch cannot pin by
// fingerprint — it ignores checkServerIdentity — so the pinned certificate is
// fetched once over TLS, checked against the pin, and then trusted as the only
// authority for every call: another certificate on that port is refused.
// That rests on Bun's `tls.ca` replacing the system roots rather than adding
// to them — checked on bun 1.4: with it set, even a publicly trusted site is
// refused — so a relay that passes the first connection and intercepts the
// second with a public certificate gets nothing. pin.test.ts holds the fetch
// leg to that.
// See docs/02-access-remote.md#over-https.
import { createHash } from "node:crypto";
import { isIP } from "node:net";
import tls from "node:tls";

export const PIN_ENV = "AGENT_BUS_TLS_FINGERPRINT";

const norm = (s: string) => s.trim().toLowerCase().replace(/^sha256:/, "").replaceAll(":", "");

/** The same comparison the Go clients make: case, colons and the sha256:
 *  prefix do not matter, and a blank pin matches nothing. */
export function samePin(pin: string, fingerprint: string): boolean {
  return norm(pin) !== "" && norm(pin) === norm(fingerprint);
}

export function fingerprint(der: Uint8Array): string {
  return "sha256:" + createHash("sha256").update(der).digest("hex");
}

export function pemOf(der: Uint8Array): string {
  const b64 = Buffer.from(der).toString("base64").match(/.{1,64}/g)!.join("\n");
  return `-----BEGIN CERTIFICATE-----\n${b64}\n-----END CERTIFICATE-----\n`;
}

/** The daemon's certificate as PEM, once its fingerprint is the pin's. */
export function pinnedCertificate(url: string, pin: string): Promise<string> {
  const u = new URL(url);
  return new Promise((resolve, reject) => {
    // An IP address is not a TLS server name; only a host name is sent.
    const host = u.hostname.replace(/^\[|\]$/g, "");
    const s = tls.connect({ host, port: Number(u.port || 443), ...(isIP(host) ? {} : { servername: host }), rejectUnauthorized: false }, () => {
      const raw: Uint8Array | undefined = (s.getPeerCertificate(true) as any)?.raw;
      s.end();
      if (!raw) return reject(new Error("the daemon presented no certificate"));
      const got = fingerprint(raw);
      if (!samePin(pin, got)) return reject(new Error(`the daemon's certificate is ${got}, not the pinned ${pin} (${PIN_ENV})`));
      resolve(pemOf(raw));
    });
    s.on("error", reject);
  });
}
