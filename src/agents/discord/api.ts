// Discord REST, bounded by the caller's deadline. Tokens never come from env.
export type Secret = { DISCORD_BOT_TOKEN?: string; DISCORD_API_URL?: string; DISCORD_GATEWAY_URL?: string };
export const DEFAULT_API = 'https://discord.com/api/v10';
export const DEFAULT_GATEWAY = 'wss://gateway.discord.gg';
export function fixtureURL(raw: string | undefined, fallback: string, socket = false): string {
  if (!raw) return fallback;
  const u = new URL(raw);
  if (!['127.0.0.1', 'localhost', '[::1]'].includes(u.hostname) || u.username || u.password || u.hash ||
      !(socket ? ['ws:', 'wss:'] : ['http:', 'https:']).includes(u.protocol)) throw Error('fixture URL must be loopback');
  return u.toString().replace(/\/$/, '');
}
export class ProviderError extends Error { constructor(readonly status: number) { super(`Discord refused the request (${status})`); } }
export class Api {
  private token = '';
  private invalid = 0;
  private invalidSince = 0;
  private blocked = new Map<string, number>();
  private next = 0;
  constructor(readonly secret: () => Promise<Secret>) {}
  async credentials() {
    const s = await this.secret();
    if (!s.DISCORD_BOT_TOKEN?.trim()) throw Error('Secret DISCORD_BOT_TOKEN is required');
    if (s.DISCORD_BOT_TOKEN !== this.token) { this.token = s.DISCORD_BOT_TOKEN; this.invalid = 0; this.blocked.clear(); }
    return { token: this.token, api: fixtureURL(s.DISCORD_API_URL, DEFAULT_API), gateway: fixtureURL(s.DISCORD_GATEWAY_URL, DEFAULT_GATEWAY, true) };
  }
  async call(method: string, path: string, body?: unknown, deadline = Date.now() + 15000): Promise<any> {
    const { token, api } = await this.credentials();
    if (!path.startsWith('/') || path.includes('://')) throw Error('invalid Discord API path');
    const route = method + path.split('?')[0];
    if (Date.now() - this.invalidSince > 600000) { this.invalid = 0; this.invalidSince = Date.now(); }
    if (this.invalid >= 20 || (this.blocked.get('*') ?? 0) > Date.now() || (this.blocked.get(route) ?? 0) > Date.now()) throw Error('Discord requests stopped after an invalid request; fix access or the token');
    for (let attempt = 0; attempt < 2; attempt++) {
      const wait = Math.max(0, this.next - Date.now());
      if (Date.now() + wait >= deadline) throw Error('the caller deadline passed');
      if (wait) await Bun.sleep(wait);
      // One bot: stay below the global 50/s even before rate-limit headers arrive.
      this.next = Date.now() + 25;
      const left = deadline - Date.now();
      if (left <= 0) throw Error('the caller deadline passed');
      let response: Response;
      try {
        response = await fetch(api + path, { method, headers: { authorization: `Bot ${token}`, 'content-type': 'application/json' },
          ...(body === undefined ? {} : { body: JSON.stringify(body) }), redirect: 'error', signal: AbortSignal.timeout(Math.min(left, 15000)) });
      } catch { throw Error('Discord did not answer; this request was not retried'); }
      const reader = response.body?.getReader(); let size = 0; const chunks: Uint8Array[] = [];
      if (reader) try { for (;;) { const { done, value } = await reader.read(); if (done) break; size += value.length; if (size > 1024 * 1024) throw Error('Discord response is too large'); chunks.push(value); } } finally { await reader.cancel().catch(() => {}); }
      let value: any;
      try { value = size ? JSON.parse(Buffer.concat(chunks).toString()) : undefined; } catch { throw Error('Discord returned a malformed response'); }
      if ([401, 403, 429].includes(response.status)) this.invalid++;
      if (response.status === 401) this.blocked.set('*', Date.now() + 600000);
      if (response.status === 403) this.blocked.set(route, Date.now() + 600000);
      if (response.status === 429) {
        const seconds = Number(value?.retry_after ?? response.headers.get('retry-after'));
        if (!Number.isFinite(seconds) || seconds < 0 || attempt || Date.now() + seconds * 1000 + 1 >= deadline) throw new ProviderError(429);
        this.next = Math.max(this.next, Date.now() + seconds * 1000);
        continue;
      }
      if (!response.ok) throw new ProviderError(response.status);
      return value;
    }
  }
}
export function chunks(text: string): string[] {
  if (typeof text !== 'string' || !text.trim() || !text.isWellFormed()) throw Error('text must be nonempty Unicode');
  const chars = [...text], out: string[] = [], marker = '… (truncated)';
  for (let i = 0; i < chars.length && out.length < 8;) {
    const truncated = out.length === 7 && chars.length - i > 2000;
    const n = 2000 - (truncated ? [...marker].length : 0);
    out.push(chars.slice(i, i + n).join('') + (truncated ? marker : '')); i += n;
  }
  return out;
}
