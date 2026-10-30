import { Api, fixtureURL, DEFAULT_GATEWAY } from './api.ts';
// GUILDS, GUILD_MESSAGES, DIRECT_MESSAGES, MESSAGE_CONTENT; no members intent.
export const INTENTS = 1 | (1 << 9) | (1 << 12) | (1 << 15);
export class Gateway {
  private socket?: WebSocket;
  private heartbeat?: ReturnType<typeof setInterval>;
  private hello?: ReturnType<typeof setTimeout>;
  private retry?: ReturnType<typeof setTimeout>;
  private stopped = false;
  private ack = true;
  private sequence: number | null = null;
  private session?: string;
  private resume?: string;
  private generation = 0;
  private backoff = 1000;
  constructor(private api: Api, private event: (kind: string, data: any) => void, private log: (text: string) => void, private retryMs?: number) {}
  async start() { this.stopped = false; await this.connect(); }
  stop() { this.stopped = true; this.generation++; this.clear(); clearTimeout(this.retry); this.retry = undefined; this.socket?.close(1000); this.socket = undefined; }
  get armed() { return !!(this.heartbeat || this.hello || this.retry); }
  private clear() { clearInterval(this.heartbeat); clearTimeout(this.hello); this.heartbeat = this.hello = undefined; }
  private again() {
    this.clear(); if (this.stopped || this.retry) return;
    const wait = this.retryMs ?? this.backoff; this.backoff = Math.min(this.backoff * 2, 30000);
    this.retry = setTimeout(() => { this.retry = undefined; void this.connect(); }, wait);
  }
  private async connect() {
    const generation = ++this.generation;
    try {
      const { token, gateway } = await this.api.credentials();
      if (this.stopped || generation !== this.generation) return;
      let base = gateway;
      if (this.session && this.resume) {
        const u = new URL(this.resume);
        if (gateway === DEFAULT_GATEWAY) {
          if (u.protocol !== 'wss:' || !(u.hostname === 'gateway.discord.gg' || u.hostname.endsWith('.discord.gg')) || u.username || u.password) throw Error('invalid resume gateway');
          base = this.resume;
        } else base = fixtureURL(this.resume, gateway, true);
      }
      const url = new URL(base); url.searchParams.set('v', '10'); url.searchParams.set('encoding', 'json');
      const ws = new WebSocket(url); this.socket = ws; this.ack = true;
      this.hello = setTimeout(() => ws.close(4000), 10000);
      const send = (op: number, d: unknown) => { if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ op, d })); };
      const beat = () => { if (!this.ack) { ws.close(4000); return; } this.ack = false; send(1, this.sequence); };
      ws.onmessage = e => {
        if (generation !== this.generation || this.stopped) return;
        let frame: any; try { if (typeof e.data !== 'string' || e.data.length > 1024 * 1024) throw Error(); frame = JSON.parse(e.data); } catch { ws.close(4000); return; }
        if (frame.op === 10) {
          this.clear(); const ms = frame.d?.heartbeat_interval;
          if (!Number.isFinite(ms) || ms < 10 || ms > 300000) { ws.close(4000); return; }
          this.heartbeat = setInterval(beat, ms);
          send(this.session ? 6 : 2, this.session ? { token, session_id: this.session, seq: this.sequence } : { token, intents: INTENTS, properties: { os: process.platform, browser: 'agent-bus', device: 'agent-bus' } });
        } else if (frame.op === 11) this.ack = true;
        else if (frame.op === 1) { this.ack = false; send(1, this.sequence); }
        else if (frame.op === 7) ws.close(4000);
        else if (frame.op === 9) { if (frame.d !== true) { this.session = this.resume = undefined; this.sequence = null; } ws.close(4000); }
        else if (frame.op === 0) {
          if (!Number.isSafeInteger(frame.s) || typeof frame.t !== 'string') { ws.close(4000); return; }
          // A resume can replay a last dispatch; it is never served twice.
          if (this.sequence !== null && frame.s <= this.sequence) return;
          this.sequence = frame.s;
          if (frame.t === 'READY') {
            if (typeof frame.d?.session_id !== 'string' || typeof frame.d?.resume_gateway_url !== 'string') { ws.close(4000); return; }
            this.session = frame.d.session_id; this.resume = frame.d.resume_gateway_url; this.backoff = 1000;
          }
          this.event(frame.t, frame.d);
        }
      };
      ws.onerror = () => ws.close(4000);
      ws.onclose = e => {
        if (generation !== this.generation) return;
        this.clear(); this.socket = undefined;
        if ([4004, 4010, 4011, 4012, 4013, 4014].includes(e.code)) { this.stopped = true; this.log(`gateway refused configuration (${e.code}); reload after fixing it`); return; }
        if ([4007, 4009].includes(e.code)) { this.session = this.resume = undefined; this.sequence = null; }
        this.again();
      };
    } catch { this.log('gateway connection unavailable; retrying'); this.again(); }
  }
}
