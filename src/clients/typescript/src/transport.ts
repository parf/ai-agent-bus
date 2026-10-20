import http from 'node:http';
import https from 'node:https';
import tls from 'node:tls';
import { createHash, timingSafeEqual } from 'node:crypto';
import { existsSync } from 'node:fs';
import { userInfo } from 'node:os';
import { parse, stringify } from 'lossless-json';

export class BusError extends Error { constructor(public readonly status: number, message: string) { super(message); this.name = 'BusError'; } }
export class BusUnreachable extends Error { constructor(message: string, options?: ErrorOptions) { super(message, options); this.name = 'BusUnreachable'; } }
export class UncertainOutcome extends BusUnreachable { constructor(message: string, options?: ErrorOptions) { super(message, options); this.name = 'UncertainOutcome'; } }
export class BusTimeout extends Error { constructor() { super('operation timed out'); this.name = 'BusTimeout'; } }
export class BusClosed extends Error { constructor() { super('client is closed'); this.name = 'BusClosed'; } }
export class IdentityMismatch extends Error { constructor(public readonly actual: string, public readonly expected?: string) { super(`credential acts as ${actual}, expected ${expected}`); this.name = 'IdentityMismatch'; } }
export class ReaderInUse extends Error { constructor() { super('consume conflicts with an active reader'); this.name = 'ReaderInUse'; } }
export class DrainTimeout extends Error { constructor(public readonly unfinished: number) { super(`drain deadline reached with ${unfinished} unfinished handlers`); this.name = 'DrainTimeout'; } }
export function abortError(): DOMException { return new DOMException('operation aborted', 'AbortError'); }
export interface Operation { signal?: AbortSignal; timeout?: number }
export function budget(timeout: number): number { if (!Number.isFinite(timeout) || timeout <= 0) throw new TypeError('timeout must be positive milliseconds'); return performance.now() + timeout; }
export function remaining(deadline: number): number { const n = deadline - performance.now(); if (n <= 0) throw new BusTimeout(); return n; }
export function duration(value: string | number): string { if (typeof value === 'string') return value; if (!Number.isFinite(value) || value < 0) throw new TypeError('duration must be nonnegative milliseconds'); return `${value}ms`; }
export function decodeJson(text: string): unknown { return parse(text, null, (n: string) => { const numeric = Number(n); return /^-?\d+$/.test(n) && !Number.isSafeInteger(numeric) ? BigInt(n) : numeric; }); }
export function discoverSocket(token: string): string {
  const account = userInfo().username;
  for (const root of [process.env.XDG_RUNTIME_DIR && `${process.env.XDG_RUNTIME_DIR}/agent-bus`, '/run/agent-bus']) {
    if (!root) continue;
    for (const socket of token ? [`${root}/bus.sock`, `${root}/user-${account}.sock`] : [`${root}/user-${account}.sock`]) if (existsSync(socket)) return socket;
  }
  return `/tmp/agent-bus-${process.getuid?.() ?? 0}.sock`; // CLI development fallback; no file is created.
}
export class Transport {
  private readonly sockets = new Set<import('node:net').Socket>();
  private closed = false;
  readonly addr: string;
  private readonly pin?: Buffer;
  #token: string;
  toJSON(): {addr:string;token:string} { return {addr:this.addr,token:'[redacted]'}; }
  [Symbol.for('nodejs.util.inspect.custom')](): {addr:string;token:string} { return this.toJSON(); }
  constructor(addr: string, token: string, fingerprint?: string) {
    this.#token = token;
    this.addr = addr;
    if (fingerprint) {
      const text = fingerprint.replace(/^sha256:/i, '').replace(/:/g, '').trim();
      if (!/^[a-fA-F0-9]{64}$/.test(text)) throw new TypeError('fingerprint must be SHA256 certificate hex');
      this.pin = Buffer.from(text, 'hex');
      if (!addr.startsWith('https://')) throw new TypeError('a certificate pin requires HTTPS');
    }
  }
  check(): void { if (this.closed) throw new BusClosed(); }
  close(): void { this.closed = true; for (const socket of this.sockets) socket.destroy(); this.sockets.clear(); }
  async request(method: string, path: string, body: unknown, options: Operation & { raw?: boolean; headers?: Record<string,string>; destructive?: boolean } = {}): Promise<any> {
    this.check(); options.signal?.throwIfAborted();
    const deadline = budget(options.timeout ?? 30_000);
    const timeout = remaining(deadline);
    const isUnix = !/^https?:\/\//.test(this.addr);
    const url = new URL(path, isUnix ? 'http://localhost' : this.addr.endsWith('/') ? this.addr : this.addr + '/');
    const payload = body === undefined ? undefined : stringify(body);
    const write = method !== 'GET' || !!options.destructive;
    return new Promise((resolve, reject) => {
      let req: http.ClientRequest | undefined;
      let socket: import('node:net').Socket | undefined;
      let left = false, settled = false;
      const finish = (error?: unknown, value?: unknown) => {
        if (settled) return; settled = true; clearTimeout(timer); options.signal?.removeEventListener('abort', cancelled);
        if (socket) { this.sockets.delete(socket); socket.destroy(); }
        if (error) reject(error); else resolve(value);
      };
      const fail = (error: unknown) => finish(this.closed ? new BusClosed() : options.signal?.aborted ? abortError() : write && left ? new UncertainOutcome('request outcome is unknown', { cause: error }) : new BusUnreachable('request failed', { cause: error }));
      const cancelled = () => { req?.destroy(); socket?.destroy(); finish(abortError()); };
      const timer = setTimeout(() => { req?.destroy(); socket?.destroy(); finish(write && left ? new UncertainOutcome('request exceeded deadline') : new BusTimeout()); }, timeout);
      options.signal?.addEventListener('abort', cancelled, { once: true });
      const start = (verifiedSocket?: tls.TLSSocket) => {
        if (settled) return;
        const client = url.protocol === 'https:' ? https : http;
        const headers: Record<string,string> = { ...options.headers };
        if (this.#token) headers['X-Agent-Bus-Token'] = this.#token;
        if (payload !== undefined) { headers['Content-Type'] = 'application/json'; headers['Content-Length'] = String(Buffer.byteLength(payload)); }
        const opts: https.RequestOptions = { method, headers, agent: false, socketPath: isUnix ? this.addr : undefined };
        // Supplying a preverified TLS socket prevents HTTP headers from preceding pin verification.
        if (verifiedSocket) { const agent = new https.Agent({ keepAlive: false }); agent.createConnection = () => verifiedSocket; opts.agent = agent; }
        req = client.request(url, opts, response => {
          const chunks: Buffer[] = []; let size = 0;
          response.on('data', chunk => { size += chunk.length; if (size > 16 * 1024 * 1024) { req?.destroy(); fail(new Error('response too large')); } else chunks.push(chunk); });
          response.on('error', fail);
          response.on('end', () => {
            const bytes = Buffer.concat(chunks); const status = response.statusCode ?? 0;
            if (performance.now() >= deadline) { finish(write && left ? new UncertainOutcome('request exceeded deadline') : new BusTimeout()); return; }
            if (status < 200 || status >= 300) { let text = bytes.toString('utf8').trim(); try { const parsed = decodeJson(text) as {error?:unknown}; if (typeof parsed.error === 'string') text = parsed.error; } catch {} finish(new BusError(status, text)); return; }
            try { finish(undefined, status === 204 ? null : options.raw ? bytes : decodeJson(bytes.toString('utf8'))); } catch (e) { finish(write && left ? new UncertainOutcome('operation returned an invalid response', { cause: e }) : new BusUnreachable('invalid daemon response', { cause: e })); }
          });
        });
        req.on('socket', s => { socket = s; this.sockets.add(s); const connected = () => { left = true; }; if (s.connecting) s.once('connect', connected); else connected(); });
        req.on('error', fail);
        req.end(payload);
      };
      if (url.protocol === 'https:' && this.pin) {
        socket = tls.connect({ host: url.hostname, port: Number(url.port || 443), servername: /^\d+\.\d+\.\d+\.\d+$/.test(url.hostname) ? undefined : url.hostname, rejectUnauthorized: false });
        this.sockets.add(socket!); socket!.once('error', fail);
        (socket as tls.TLSSocket).once('secureConnect', () => {
          try { const raw = (socket as tls.TLSSocket).getPeerCertificate().raw; const hash = createHash('sha256').update(raw).digest(); if (!timingSafeEqual(hash, this.pin!)) throw new Error('certificate fingerprint mismatch'); start(socket as tls.TLSSocket); } catch (e) { fail(e); }
        });
      } else start();
    });
  }
}
