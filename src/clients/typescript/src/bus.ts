import { randomUUID } from 'node:crypto';
import { Transport, type Operation, budget, remaining, duration, discoverSocket, IdentityMismatch, BusTimeout, BusClosed, ReaderInUse, DrainTimeout, abortError } from './transport.js';
import { Message, bindings, type Envelope } from './message.js';
import { RecordHandle } from './record.js';
export interface ConnectOptions { addr?: string | null; token?: string; expectName?: string; timeout?: number; fingerprint?: string }
export interface SendOptions extends Operation { topic?: string; tag?: string; ttl?: string | number; wait?: string | number; reply_to?: { name?: string; topic?: string; tag?: string } }
export interface CallOptions extends Omit<SendOptions,'tag'|'wait'> { timeout: number; onAck?: (message: Message) => void }
export interface ConsumeOptions extends Operation { inbox?: string; topic?: string; tag?: string; wait?: string | number; share?: boolean }
export interface ServeOptions { inbox?: string; topic?: string; share?: boolean; workers?: number; signal?: AbortSignal; drainTimeout?: number; onError?: (id: string, errorType: string) => void }
export type CallOutcome = { kind: 'answer'; message: Message } | { kind: 'done' } | { kind: 'timeout'; acked: boolean };
type Pending = { inbox: string; topic: string; tag: string; sent?: string; early: Message[]; acked: boolean; finish: (outcome: CallOutcome) => void; fail: (error: unknown) => void; onAck?: (message: Message) => void; deadline: number };
type Serving = { handler: (message: Message) => string | null | Promise<string | null>; options: ServeOptions; inbox: string; workers: number; active: Set<Promise<void>>; accepting: boolean; polling: number; error?: unknown };
const pause = (ms = 10) => new Promise<void>(resolve => setTimeout(resolve, ms));
export class Bus {
  private readonly transport: Transport;
  private readonly timeout: number;
  private readonly expected?: string;
  private name?: string;
  private identityFlight?: Promise<void>;
  private closed = false;
  private raw = false;
  private readonly pending = new Map<string,Pending>();
  private serving?: Serving;
  private dispatching = false;
  private cursor = 0;
  private readonly prefix = `call-${randomUUID()}-`;
  private readonly handles = new Map<string,RecordHandle>();
  static connect(options: ConnectOptions = {}): Bus { return new Bus(options); }
  constructor(options: ConnectOptions = {}) {
    const token = options.token ?? process.env.AGENT_BUS_TOKEN ?? '';
    this.timeout = options.timeout ?? 30_000; budget(this.timeout);
    this.expected = (options.expectName ?? process.env.AGENT_BUS_NAME)?.trim().toLowerCase();
    this.transport = new Transport(options.addr ?? process.env.AGENT_BUS_ADDR ?? discoverSocket(token), token, options.fingerprint ?? process.env.AGENT_BUS_TLS_FINGERPRINT);
  }
  check(): void { if (this.closed) throw new BusClosed(); }
  private async ensureIdentity(deadline: number, signal?: AbortSignal): Promise<void> {
    this.check(); signal?.throwIfAborted(); if (this.name) return;
    if (!this.identityFlight) {
      const flight = this.transport.request('GET','/status',undefined,{ timeout: remaining(deadline) }).then(result => {
        if (typeof result?.you !== 'string') throw new TypeError('status has no caller identity');
        if (this.expected && result.you !== this.expected) throw new IdentityMismatch(result.you,this.expected);
        this.name = result.you;
      });
      this.identityFlight = flight;
      flight.finally(() => { if (this.identityFlight === flight) this.identityFlight = undefined; }).catch(() => {});
    }
    await this.bounded(this.identityFlight, deadline, signal);
  }
  private async bounded<T>(promise: Promise<T>, deadline: number, signal?: AbortSignal): Promise<T> {
    signal?.throwIfAborted(); const left = remaining(deadline);
    return new Promise((resolve,reject) => {
      const finish = (error: unknown, value?: T) => { clearTimeout(timer); signal?.removeEventListener('abort',cancelled); error ? reject(error) : resolve(value as T); };
      const cancelled = () => finish(abortError());
      const timer = setTimeout(() => finish(new BusTimeout()),left);
      signal?.addEventListener('abort',cancelled,{ once:true }); promise.then(value => finish(undefined,value),finish);
    });
  }
  async request(method: string, path: string, body?: unknown, options: Operation & { query?: {[key:string]: unknown}; raw?: boolean; headers?: {[key:string]:string}; destructive?: boolean } = {}): Promise<any> {
    this.check(); const deadline = budget(options.timeout ?? this.timeout);
    if (path !== '/identity') await this.ensureIdentity(deadline,options.signal);
    this.check();
    if (options.query) { const query = new URLSearchParams(); for (const [key,value] of Object.entries(options.query)) if (value !== undefined && value !== null) query.set(key,String(value)); path += `?${query}`; }
    return this.transport.request(method,path,body,{ ...options,timeout: remaining(deadline) });
  }
  async whoami(options: Operation = {}): Promise<{you:string}> { const deadline = budget(options.timeout ?? this.timeout); await this.ensureIdentity(deadline,options.signal); return { you:this.name! }; }
  status(options: Operation = {}): Promise<any> { return this.request('GET','/status',undefined,options); }
  identity(options: Operation = {}): Promise<any> { return this.request('GET','/identity',undefined,options); }
  records(filters: {kind?:string} = {},options:Operation = {}): Promise<any> { return this.request('GET','/ls',undefined,{ ...options,query:filters }); }
  record(name: string): RecordHandle { this.check(); if (!name.trim()) throw new TypeError('record name required'); const normalized = name.trim().toLowerCase(); if (!this.handles.has(normalized)) this.handles.set(normalized,new RecordHandle(this,normalized)); return this.handles.get(normalized)!; }
  register(record: {name:string;[key:string]:unknown}, options: Operation & {createOnly?: boolean} = {}): Promise<any> { return this.request('POST','/register',record,{ ...options,headers:options.createOnly === false ? {} : {'If-None-Match':'*'} }); }
  private payload(to:string,body:string,options:SendOptions): {[key:string]:unknown} {
    if (typeof body !== 'string' || !to) throw new TypeError('send needs destination and text body');
    const allowed = new Set(['topic','tag','ttl','wait','reply_to','signal','timeout','onAck']);
    if (Object.keys(options).some(key => !allowed.has(key))) throw new TypeError('unsupported send option');
    return { to,body,topic:options.topic,tag:options.tag,ttl:options.ttl === undefined ? undefined : duration(options.ttl),wait:options.wait === undefined ? undefined : duration(options.wait),reply_to:options.reply_to };
  }
  async send(to: string, body: string, options: SendOptions = {}): Promise<string> { return (await this.request('POST','/send',this.payload(to,body,options),options)).message_id; }
  publish(channel:string,body:string,options:SendOptions = {}): Promise<string> { return this.send(channel,body,options); }
  unsubscribe(channel:string,options:Operation = {}): Promise<any> { return this.request('POST','/subscribe',{channel,off:true},options); }
  private async respond(message:Message,body:string,receipt?:string,options:Operation = {}): Promise<string> {
    const binding = bindings.get(message);
    if (!binding || binding.bus !== this || message.receipt) throw new TypeError('response needs a non-receipt received by this Bus');
    if (typeof body !== 'string') throw new TypeError('reply body must be text');
    if (receipt !== 'ack') { if (binding.terminal) throw new TypeError('message already has a terminal response'); binding.terminal = true; }
    const route = message.reply_to;
    return (await this.request('POST','/send',{to:route?.name ?? message.from,topic:route?.topic ?? message.topic,tag:route?.tag ?? message.tag,body,re:message.message_id,receipt},options)).message_id;
  }
  reply(message:Message,body:string,options:Operation = {}): Promise<string> { return this.respond(message,body,undefined,options); }
  ack(message:Message,options:Operation = {}): Promise<string> { return this.respond(message,'','ack',options); }
  done(message:Message,options:Operation = {}): Promise<string> { return this.respond(message,'','done',options); }
  private key(inbox:string,topic:string,tag:string): string { return JSON.stringify([inbox,topic,tag]); }
  private async read(options:ConsumeOptions): Promise<Message|null> {
    const result = await this.request('GET','/consume',undefined,{ signal:options.signal,timeout:options.timeout,destructive:true,query:{inbox:options.inbox,topic:options.topic,tag:options.tag,wait:duration(options.wait ?? 50),share:options.share ? '1' : undefined} });
    return result === null ? null : new Message(result as Envelope,this);
  }
  async consume(options:ConsumeOptions = {}): Promise<Message|null> {
    this.check(); if (this.raw || this.serving || this.pending.size) throw new ReaderInUse(); this.raw = true;
    try { const deadline = budget(options.timeout ?? this.timeout); await this.ensureIdentity(deadline,options.signal); return await this.read({ ...options,inbox:options.inbox ?? this.name,wait:options.wait ?? '30s',timeout:remaining(deadline) }); } finally { this.raw = false; }
  }
  async call(to:string,body:string,options:CallOptions): Promise<CallOutcome> {
    const deadline = budget(options.timeout);
    try { await this.ensureIdentity(deadline,options.signal); } catch (error) { if (error instanceof BusTimeout) return {kind:'timeout',acked:false}; throw error; }
    if (this.raw) throw new ReaderInUse();
    const tag = this.prefix + randomUUID(); const route = options.reply_to;
    const inbox = route?.name ?? this.name!; const topic = route?.topic ?? options.topic ?? ''; const key = this.key(inbox,topic,tag);
    let timer: ReturnType<typeof setTimeout> | undefined;
    let cancelled: (() => void) | undefined;
    let entry!: Pending;
    const result = new Promise<CallOutcome>((resolve,reject) => {
      entry = {inbox,topic,tag,early:[],acked:false,deadline,onAck:options.onAck,finish:resolve,fail:reject};
      timer = setTimeout(() => resolve({kind:'timeout',acked:entry.acked}),remaining(deadline));
      cancelled = () => reject(abortError()); options.signal?.addEventListener('abort',cancelled,{once:true});
    });
    // Avoid an unhandled early timeout/abort while the POST is in flight.
    result.catch(() => {});
    this.pending.set(key,entry); this.startDispatcher();
    try {
      const payload = this.payload(to,body,{...options,tag,wait:remaining(deadline),reply_to:route ? {...route,tag} : undefined});
      const sent = await this.request('POST','/send',payload,{timeout:remaining(deadline),signal:options.signal});
      entry.sent = sent.message_id;
      for (const message of entry.early) this.deliver(entry,message); entry.early = [];
      return await result;
    } catch (error) { if (error instanceof BusTimeout) return {kind:'timeout',acked:entry.acked}; throw error; }
    finally { if (timer) clearTimeout(timer); if (cancelled) options.signal?.removeEventListener('abort',cancelled); this.pending.delete(key); }
  }
  private deliver(entry:Pending,message:Message): void {
    if (performance.now() >= entry.deadline) { entry.finish({kind:'timeout',acked:entry.acked}); return; }
    if (!entry.sent) { entry.early.push(message); return; }
    if (message.receipt && message.re !== entry.sent) return;
    if (message.receipt === 'ack') { entry.acked = true; try { entry.onAck?.(message); } catch (error) { entry.fail(error); } }
    else if (message.receipt === 'done') entry.finish({kind:'done'});
    else if (!message.receipt) entry.finish({kind:'answer',message});
  }
  private startDispatcher(): void { if (!this.dispatching) { this.dispatching = true; void this.dispatch().finally(() => { this.dispatching = false; if (!this.closed && (this.pending.size || this.serving)) this.startDispatcher(); }); } }
  private async dispatch(): Promise<void> {
    while (!this.closed && (this.pending.size || this.serving)) {
      const server = this.serving;
      const choices = [...this.pending.values()].filter(entry => entry.sent && entry.deadline > performance.now());
      const work = server && server.accepting && server.active.size + server.polling < server.workers;
      if (!choices.length && !work) { await pause(); continue; }
      this.cursor %= choices.length + (work ? 1 : 0); const entry = choices[this.cursor++];
      const workPoll = !entry; if (workPoll) server!.polling++;
      const inbox = entry?.inbox ?? server!.inbox;
      try {
        const message = await this.read({inbox,topic:entry?.topic ?? server?.options.topic,tag:entry?.tag,share:!entry && server?.options.share,wait:50,timeout:entry ? Math.min(1000,remaining(entry.deadline)) : 1000});
        if (!message) continue;
        const routed = this.pending.get(this.key(inbox,message.topic ?? '',message.tag ?? ''));
        if (routed) this.deliver(routed,message);
        else if (workPoll && server && !message.receipt && !message.re && !message.tag?.startsWith(this.prefix)) {
          const task = this.handle(server!,message); server!.active.add(task); void task.finally(() => server!.active.delete(task)).catch(() => {});
        }
      } catch (error) { if (entry) { if (error instanceof BusTimeout || performance.now() >= entry.deadline) entry.finish({kind:'timeout',acked:entry.acked}); else entry.fail(error); this.pending.delete(this.key(entry.inbox,entry.topic,entry.tag)); } else if (server) { server.error = error; server.accepting = false; } }
      finally { if (workPoll) server!.polling--; }
    }
  }
  private expired(message:Message): boolean {
    const value = message.deadline;
    if (value === undefined || value === null || value === '0001-01-01T00:00:00Z') return false;
    if (typeof value !== 'string') throw new TypeError('invalid message deadline');
    const at = Date.parse(value);
    if (!Number.isFinite(at)) throw new TypeError('invalid message deadline');
    return Date.now() >= at;
  }
  private async handle(server:Serving,message:Message): Promise<void> {
    try { if (this.expired(message)) return; await this.ack(message); if (this.expired(message)) return; const answer = await server.handler(message); if (!bindings.get(message)!.terminal && !this.closed) { if (answer === null) await this.done(message); else await this.reply(message,answer); } }
    catch (error) { const type = error instanceof Error ? error.name : 'Error'; if (server.options.onError) { try { server.options.onError(message.message_id,type); } catch {} } else console.error(`agent-bus handler failed for message ${message.message_id} (${type})`); }
  }
  async serve(handler:Serving['handler'],options:ServeOptions = {}): Promise<void> {
    this.check(); const workers = options.workers ?? 1; const drain = options.drainTimeout ?? 30_000;
    if (!Number.isInteger(workers) || workers < 1 || !Number.isFinite(drain) || drain < 0) throw new TypeError('positive workers and nonnegative drainTimeout required');
    if (this.serving || this.raw) throw new ReaderInUse();
    await this.ensureIdentity(budget(this.timeout),options.signal); if (this.serving || this.raw) throw new ReaderInUse();
    const server:Serving = {handler,options,inbox:options.inbox ?? this.name!,workers,active:new Set(),accepting:true,polling:0}; this.serving = server;
    const stop = () => { server.accepting = false; }; options.signal?.addEventListener('abort',stop,{once:true}); if (options.signal?.aborted) stop(); this.startDispatcher();
    try { while (server.accepting && !this.closed) await pause(); const end = performance.now() + drain;
      while (server.active.size || server.polling) { if (performance.now() >= end) throw new DrainTimeout(server.active.size + server.polling); await pause(); }
      if (server.error) throw server.error;
    } finally { server.accepting = false; options.signal?.removeEventListener('abort',stop); if (this.serving === server) this.serving = undefined; }
  }
  stop(): void { this.check(); if (this.serving) this.serving.accepting = false; }
  async close(): Promise<void> { if (this.closed) return; this.closed = true; if (this.serving) this.serving.accepting = false; for (const entry of this.pending.values()) entry.fail(new BusClosed()); this.pending.clear(); this.transport.close(); }
}
