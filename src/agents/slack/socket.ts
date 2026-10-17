// Slack Socket Mode, ported from /rd/bin/slack-tail's connectOnce: open a
// connection with the app token, ack each envelope at once — before anything
// is forwarded, so a slow bus never delays Slack's ack — reconnect with
// backoff when it drops or Slack says disconnect, and refresh a connection
// that has been silent past the watchdog (README § behaviour pinned by the
// review). Stopping ends the connection and every timer; nothing reconnects.
import { scrub, slackWeb } from "./slackapi.js";

const backoffFirstMs = Number(process.env.SLACK_AGENT_BACKOFF_MS ?? "1000");
const backoffMostMs = 30_000;
const watchdogMs = Number(process.env.SLACK_AGENT_WATCHDOG_MS ?? String(15 * 60 * 1000));

export type SocketEvent = { event: any; eventId: string };

export class SocketMode {
  private ws: WebSocket | undefined;
  private stopped = false;
  private backoff: number;
  private readonly first: number;
  private readonly watchdogMs: number;
  private watchdog: ReturnType<typeof setTimeout> | undefined;
  private retry: ReturnType<typeof setTimeout> | undefined;
  /** How many connections were opened; tests read it. */
  connections = 0;

  constructor(
    private readonly credentials: () => Promise<{ appToken?: string; api?: string }>,
    private readonly onEvent: (e: SocketEvent) => void,
    private readonly log: (line: string) => void,
    timing: { backoffMs?: number; watchdogMs?: number } = {},
  ) {
    this.first = this.backoff = timing.backoffMs ?? backoffFirstMs;
    this.watchdogMs = timing.watchdogMs ?? watchdogMs;
  }

  /** Begin; the promise is the first connection attempt, which never rejects:
   *  every failure is a reconnect (slack-review-S.5). */
  start(): Promise<void> {
    return this.connect();
  }

  /** Close the connection, cancel reconnect and watchdog; nothing runs after. */
  stop(): void {
    this.stopped = true;
    clearTimeout(this.watchdog);
    clearTimeout(this.retry);
    this.watchdog = this.retry = undefined;
    try {
      this.ws?.close();
    } catch { /* already closing */ }
    this.ws = undefined;
  }

  /** Whether any timer of the socket is still armed. */
  get armed(): boolean {
    return this.watchdog !== undefined || this.retry !== undefined;
  }

  private again(why: string): void {
    if (this.stopped) return;
    clearTimeout(this.watchdog);
    this.watchdog = undefined;
    const wait = this.backoff;
    this.backoff = Math.min(this.backoff * 2, backoffMostMs);
    this.log(`socket: ${why}; reconnecting in ${wait} ms`);
    this.retry = setTimeout(() => {
      this.retry = undefined;
      void this.connect();
    }, wait);
  }

  private bump(ws: WebSocket): void {
    clearTimeout(this.watchdog);
    this.watchdog = setTimeout(() => {
      this.watchdog = undefined;
      this.log("socket: watchdog — no traffic, refreshing the connection");
      try {
        ws.close();
      } catch { /* closing */ }
    }, this.watchdogMs);
  }

  private async connect(): Promise<void> {
    if (this.stopped) return;
    let url: string;
    let token: string | undefined;
    // Whatever Slack or a faulty endpoint says goes to the log scrubbed of the
    // token the request carried (slack-review-S.5).
    try {
      const { appToken, api } = await this.credentials();
      token = appToken;
      if (!appToken) return this.again("SLACK_APP_TOKEN is not set");
      const opened = await slackWeb(appToken, api, "write", "apps.connections.open");
      if (opened.ok !== true || typeof opened.url !== "string") return this.again(scrub(`apps.connections.open: ${opened.error ?? "no url"}`, token));
      url = opened.url;
    } catch (e) {
      return this.again(scrub(e instanceof Error ? e.message : "apps.connections.open failed", token));
    }
    if (this.stopped) return;
    let ws: WebSocket;
    try {
      ws = new WebSocket(url);
    } catch {
      // The URL itself may carry a ticket: it is never logged (slack-review-S.5).
      return this.again("Slack answered a socket URL that cannot be opened");
    }
    this.ws = ws;
    this.connections++;
    let closed = false;
    const finish = (why: string) => {
      if (closed) return;
      closed = true;
      if (this.ws === ws) this.ws = undefined;
      this.again(why);
    };
    ws.addEventListener("open", () => {
      this.backoff = this.first;
      this.bump(ws);
      this.log("socket: connected");
    });
    ws.addEventListener("message", ev => {
      this.bump(ws);
      let msg: any;
      try {
        msg = JSON.parse(String(ev.data));
      } catch {
        return;
      }
      // Ack first: Slack decides on redelivery from it, and nothing waits on the bus.
      if (typeof msg?.envelope_id === "string") ws.send(JSON.stringify({ envelope_id: msg.envelope_id }));
      if (msg?.type === "disconnect") {
        try {
          ws.close();
        } catch { /* closing */ }
        return;
      }
      if (msg?.type === "events_api" && msg.payload?.event) {
        this.onEvent({ event: msg.payload.event, eventId: String(msg.payload.event_id ?? msg.envelope_id ?? "") });
      }
    });
    ws.addEventListener("close", () => finish("connection closed"));
    ws.addEventListener("error", () => finish("connection error"));
  }
}
