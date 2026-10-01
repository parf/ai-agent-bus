// The web face's port when TLS is on: TLS and plain HTTP on one port. Each
// connection is sniffed. A first byte of 0x16, a TLS handshake, is piped to the
// face's own Bun.serve, which holds the certificate and listens on a unix
// socket. Anything else is answered here with a redirect to https:// and never
// reaches the app, so no page is served in the clear.
// Bun cannot hand an accepted socket to Bun.serve, hence the pipe.
// See docs/08-processes.md#the-web-face.
import net from "node:net";

export let SNIFF_MS = 5000; // to see the first byte, and for a plain request's head
export const HEAD_MAX = 8192;
export function setSniffMs(ms: number) { SNIFF_MS = ms; }

// A host a browser might send, and nothing that could end a header line.
const HOST = /^[A-Za-z0-9.-]+(:\d{1,5})?$|^\[[0-9A-Fa-f:.]+\](:\d{1,5})?$/;

/** The redirect for a plain request head: to https:// on the same host and
 *  path, or to fallback when the head names no usable host. */
export function redirectFor(head: string, fallback: string): string {
  const [requestLine = "", ...lines] = head.split("\r\n");
  const target = requestLine.split(" ")[1] ?? "/";
  const path = target.startsWith("/") && !/[\s\x00-\x1f]/.test(target) ? target : "/";
  let host = fallback;
  for (const l of lines) {
    const m = /^host:\s*(.+?)\s*$/i.exec(l);
    if (m && HOST.test(m[1]!)) { host = m[1]!; break; }
  }
  const location = `https://${host}${path}`;
  return `HTTP/1.1 301 Moved Permanently\r\nLocation: ${location}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n`;
}

const HEALTHY = "HTTP/1.1 200 OK\r\nContent-Length: 0\r\nConnection: close\r\n\r\n";
const UNHEALTHY = "HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n";

/** Listen on hostname:port for both; TLS is piped to the unix socket inner. */
export function dualFront(opts: { hostname: string; port: number; inner: string }): Promise<net.Server> {
  const fallback = `${opts.hostname.includes(":") ? `[${opts.hostname}]` : opts.hostname}:${opts.port}`;
  const server = net.createServer((c) => {
    // Nothing said within the deadline, or no full head within it: closed.
    const timer = setTimeout(() => c.destroy(), SNIFF_MS);
    c.on("error", () => { clearTimeout(timer); c.destroy(); });
    let head = Buffer.alloc(0);
    const onData = (chunk: Buffer) => {
      if (head.length === 0 && chunk[0] === 0x16) {
        clearTimeout(timer);
        c.off("data", onData);
        c.pause();
        const u = net.connect(opts.inner);
        u.on("error", () => { c.destroy(); u.destroy(); });
        c.on("close", () => u.destroy());
        u.on("close", () => c.destroy());
        u.on("connect", () => {
          u.write(chunk);
          c.pipe(u);
          u.pipe(c);
          c.resume();
        });
        return;
      }
      head = Buffer.concat([head, chunk]);
      const text = head.toString("latin1");
      const end = text.indexOf("\r\n\r\n");
      if (end < 0 && head.length < HEAD_MAX) return;
      clearTimeout(timer);
      c.off("data", onData);
      // A head past the cap is read as far as the cap: a Host beyond it is not
      // seen, and the redirect falls back to the port's own address.
      const request = text.slice(0, end < 0 ? HEAD_MAX : end);
      // Monitors probe plain /healthz; it answers for the app behind the
      // front — 200 only when the app's socket takes a connection.
      if (/^(GET|HEAD) \/healthz(\?\S*)? HTTP\/1\.[01]$/.test(request.split("\r\n")[0] ?? "")) {
        const probe = net.connect(opts.inner);
        const answer = (ok: boolean) => { probe.destroy(); c.end(ok ? HEALTHY : UNHEALTHY); };
        probe.once("connect", () => answer(true));
        probe.once("error", () => answer(false));
        return;
      }
      c.end(redirectFor(request, fallback));
    };
    c.on("data", onData);
  });
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(opts.port, opts.hostname, () => { server.off("error", reject); resolve(server); });
  });
}
