import { constants } from 'node:fs';
import { open, realpath } from 'node:fs/promises';
import { isAbsolute, relative, sep } from 'node:path';
import { createHash } from 'node:crypto';
import { Bus, type Message } from '../../typescript/src/index.ts';

const name = process.env.AGENT_BUS_NAME ?? '#stats-ts@localhost';
const configuredRoot = process.env.STATS_ROOT;
if (!configuredRoot) throw new Error('STATS_ROOT is required');
const root = await realpath(configuredRoot);
const bus = Bus.connect({ expectName:name });
const stop = new AbortController();
for (const signal of ['SIGTERM','SIGINT'] as const) process.once(signal,()=>stop.abort());

// One answer for a missing path and an outside one: a caller learns nothing beyond the root.
const unavailable = 'path is unavailable or outside STATS_ROOT';
function confined(path:string): void {
  const part = relative(root,path);
  if (part === '..' || part.startsWith(`..${sep}`) || isAbsolute(part)) throw new Error(unavailable);
}
async function file(path:unknown): Promise<{path:string;bytes:Buffer}> {
  if (typeof path !== 'string' || !isAbsolute(path)) throw new Error('path must be absolute');
  const canonical = await realpath(path).catch(() => { throw new Error(unavailable); }); confined(canonical);
  const handle = await open(canonical,constants.O_RDONLY | constants.O_NOFOLLOW);
  try {
    // Check the opened descriptor as well: a parent symlink can change after realpath.
    const opened = await realpath(`/proc/self/fd/${handle.fd}`); confined(opened);
    if (!(await handle.stat()).isFile()) throw new Error('path must name a file');
    return {path:opened,bytes:await handle.readFile()};
  } finally { await handle.close(); }
}
function statistics(path:string,bytes:Buffer) {
  let lines=0,todos=0;
  for(const byte of bytes) if(byte===10) lines++;
  if(bytes.length && bytes.at(-1)!==10) lines++;
  for(let at=0; (at=bytes.indexOf('TODO',at))!==-1; at+=4) todos++;
  return {worker:'ts',path,bytes:bytes.length,lines,todos,sha256:createHash('sha256').update(bytes).digest('hex')};
}
function parseBody(message:Message): {[key:string]:unknown} {
  const parsed:unknown = JSON.parse(message.body);
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('request must be a JSON object');
  return parsed as {[key:string]:unknown};
}
async function handler(message:Message): Promise<string> {
  try {
    const input=parseBody(message);
    if(message.topic==='relay') {
      if(!Array.isArray(input.route) || !input.route.every(n=>typeof n==='string') || !Array.isArray(input.hops) || !input.hops.every(n=>typeof n==='string')) throw new Error('relay requires route and hops string arrays');
      const route=[...input.route] as string[],hops=[...input.hops,'ts'];
      const next=route.shift();
      if(!next)return JSON.stringify({route,hops});
      const outcome=await bus.call(next,JSON.stringify({route,hops}),{topic:'relay',timeout:20_000});
      if(outcome.kind!=='answer')throw new Error(`relay ended with ${outcome.kind}`);
      return outcome.message.body;
    }
    if(message.topic!=='validate-json' && message.topic!=='stats')throw new Error('supported topics: validate-json, stats, relay');
    const read=await file(input.path);
    await bus.record(name).kv.increment('files');
    if(message.topic==='stats')return JSON.stringify(statistics(read.path,read.bytes));
    let validation:{[key:string]:unknown};
    try {
      const parsed:unknown=JSON.parse(new TextDecoder('utf-8',{fatal:true,ignoreBOM:true}).decode(read.bytes));
      const type=parsed===null?'null':Array.isArray(parsed)?'array':typeof parsed;
      validation={valid:true,type,keys:parsed!==null && typeof parsed==='object' && !Array.isArray(parsed)?Object.keys(parsed):[]};
    } catch(error) { validation={valid:false,error:error instanceof Error?error.message:'invalid JSON'}; }
    let audit:{[key:string]:unknown};
    try {
      const result=await bus.call('#stats-python@localhost',JSON.stringify({path:read.path}),{topic:'audit',timeout:20_000});
      audit=result.kind==='answer'?{audit:JSON.parse(result.message.body)}:{audit_error:`audit ended with ${result.kind}`};
    } catch(error) { audit={audit_error:error instanceof Error?error.message:'audit failed'}; }
    return JSON.stringify({worker:'ts',path:read.path,...validation,...audit});
  } catch(error) { return JSON.stringify({worker:'ts',error:error instanceof Error?error.message:'request failed'}); }
}
try { await bus.serve(handler,{workers:2,signal:stop.signal,drainTimeout:25_000}); }
catch(error) { if(!stop.signal.aborted) { console.error(`ts worker stopped (${error instanceof Error?error.name:'Error'})`);process.exitCode=1; } }
finally { await bus.close(); }
