import { inspect } from 'node:util';
import type { Bus } from './bus.js';
export type Json = null | boolean | number | bigint | string | Json[] | { [key: string]: Json };
export type Envelope = { message_id: string; from: string; to?: string; body?: string; topic?: string; tag?: string; receipt?: string; re?: string; ask?: string; reply_to?: { name?: string; topic?: string; tag?: string }; roles?: readonly string[]; [key: string]: unknown };
export function freeze<T>(value: T): Readonly<T> { if (value && typeof value === 'object') { for (const v of Object.values(value)) freeze(v); Object.freeze(value); } return value; }
export class Secret {
  #bytes: Uint8Array;
  constructor(bytes: Uint8Array) { this.#bytes = Uint8Array.from(bytes); }
  reveal(): Uint8Array { return Uint8Array.from(this.#bytes); }
  toString(): string { return 'Secret[redacted]'; }
  toJSON(): string { return this.toString(); }
  [inspect.custom](): string { return this.toString(); }
}
export class Message {
  readonly envelope: Readonly<Envelope>;
  constructor(envelope: Envelope, bus: Bus) { this.envelope = freeze(envelope); bindings.set(this, { bus, terminal: false }); Object.freeze(this); }
  get message_id(): string { return this.envelope.message_id; }
  get body(): string { return this.envelope.body ?? ''; }
  get from(): string { return this.envelope.from; }
  get topic(): string | undefined { return this.envelope.topic; }
  get tag(): string | undefined { return this.envelope.tag; }
  get receipt(): string | undefined { return this.envelope.receipt; }
  get re(): string | undefined { return this.envelope.re; }
  /** What the sender asks about the receiver itself, `help`, `help-config`, `help-secret`, `help-json`, `mcp` or `status`, with no body; undefined for ordinary work. */
  get ask(): string | undefined { return this.envelope.ask; }
  get roles(): readonly string[] { return (this.envelope.roles ?? []) as readonly string[]; }
  get reply_to(): Readonly<Envelope['reply_to']> { return this.envelope.reply_to; }
  get original_to(): unknown { return this.envelope.original_to; }
  get deadline(): unknown { return this.envelope.deadline; }
  get forwards(): unknown { return this.envelope.forwards; }
  get expires(): unknown { return this.envelope.expires; }
  get at(): unknown { return this.envelope.at; }
  reply(body: string): Promise<string> { return bindings.get(this)!.bus.reply(this, body); }
  ack(): Promise<string> { return bindings.get(this)!.bus.ack(this); }
  done(): Promise<string> { return bindings.get(this)!.bus.done(this); }
}
export const bindings = new WeakMap<Message, { bus: Bus; terminal: boolean }>();
