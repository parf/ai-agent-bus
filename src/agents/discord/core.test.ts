import { expect, test } from 'bun:test';
import { configuration } from './config.ts';
import { parse } from './parse.ts';
import { chunks, fixtureURL, Api } from './api.ts';

test('configuration requires explicit guilds and rejects unknown keys and malformed routes', () => {
  for (const raw of [null, {}, {guilds:['1','1']}, {guilds:['x']}, {guilds:[], nope:true}, {guilds:[],users:{x:'person@t'}}, {guilds:[],route:'bad name'}]) expect(() => configuration(raw)).toThrow();
  expect(configuration({guilds:['1'],users:{'2':'Ada@t'},tags:{a:'#ai@t'},route:'#fallback@t'})).toMatchObject({guilds:['1'],users:{'2':'ada@t'},tags:{a:'#ai@t'},route:'#fallback@t'});
});
test('channel aliases need ids and duplicates name both keys before serving', () => {
  expect(() => configuration({guilds:['1'],channels:{alerts:{}}})).toThrow('alerts needs id');
  expect(() => configuration({guilds:['1'],channels:{alerts:{id:'2'},'#alerts':{id:'2'}}})).toThrow('alerts and #alerts');
  expect(() => configuration({guilds:['1'],channels:{'2':{id:'3'}}})).toThrow('disagrees');
  expect(() => configuration({guilds:['1'],channels:{'2':{id:null}}})).toThrow('needs id');
  expect(() => configuration({guilds:['1'],channels:{'2':{anyone:'yes'}}})).toThrow('anyone');
  expect(configuration({guilds:['1'],channels:{'2':{route:'#a@t',tags:false,anyone:true}}}).chats).toEqual({'2':{route:'#a@t',tags:false,anyone:true}});
});
test('text and JSON calls enforce fields, ids, bounds and delete/add instead of editing a subscription', () => {
  expect(parse('post alerts hello team',['alerts'])).toEqual({method:'post',channel:'alerts',text:'hello team'});
  expect(parse('post hello team')).toEqual({method:'post',text:'hello team'});
  expect(parse('read #alerts 100')).toEqual({method:'read',channel:'#alerts',limit:100});
  expect(parse('read 10')).toEqual({method:'read',limit:10});
  expect(parse('sub add #alerts #ai@t /error/i')).toEqual({method:'sub.add',channel:'#alerts',target:'#ai@t',filter:'/error/i'});
  for (const body of ['sub edit abc','{"method":"sub.edit"}','read 101 101','{"method":"read","limit":0}','{"method":"post","text":"ok","token":"no"}','react #alerts x hi','thread #alerts 1 '+ 'x'.repeat(101), 'whoami extra']) expect(() => parse(body)).toThrow();
});
test('posts are split at 2000 Unicode characters and capped at eight parts with a truncation marker', () => {
  const parts = chunks('😀'.repeat(20001)); expect(parts).toHaveLength(8);
  expect(parts.every(p => [...p].length <= 2000 && p.isWellFormed())).toBe(true);
  expect(parts.at(-1)).toEndWith('… (truncated)'); expect(chunks('hello')).toEqual(['hello']);
  expect(() => chunks('')).toThrow();
});
test('provider overrides are loopback fixtures only, never arbitrary token recipients', () => {
  expect(fixtureURL(undefined,'https://discord.com/api/v10')).toBe('https://discord.com/api/v10');
  expect(fixtureURL('http://127.0.0.1:1234/','x')).toBe('http://127.0.0.1:1234');
  for (const url of ['https://evil.example','http://user@127.0.0.1:1','file:///etc/passwd']) expect(() => fixtureURL(url,'x')).toThrow();
});

function rest(handler: (request: Request) => Response | Promise<Response>) {
  const requests: Request[] = [];
  const server = Bun.serve({hostname:'127.0.0.1',port:0,fetch:r=>{requests.push(r.clone());return handler(r);}});
  const api = new Api(async()=>({DISCORD_BOT_TOKEN:'fixture-record-token',DISCORD_API_URL:`http://127.0.0.1:${server.port}`}));
  return {api,requests,stop:()=>server.stop(true)};
}
test('REST token comes from the record secret, not the process environment; replies are bounded', async () => {
  const before=process.env.DISCORD_BOT_TOKEN;process.env.DISCORD_BOT_TOKEN='wrong-environment-token';
  const f=rest(r=>Response.json({header:r.headers.get('authorization')}));
  try {expect(await f.api.call('GET','/users/@me')).toEqual({header:'Bot fixture-record-token'});}
  finally {if(before===undefined)delete process.env.DISCORD_BOT_TOKEN;else process.env.DISCORD_BOT_TOKEN=before;f.stop();}
  await expect(new Api(async()=>({})).call('GET','/users/@me')).rejects.toThrow('DISCORD_BOT_TOKEN');
});
test('429 retries once after retry_after; an invalid token stops subsequent requests without leaking the response', async () => {
  let calls=0;const times:number[]=[];const f=rest(()=>{times.push(Date.now());return ++calls===1?Response.json({retry_after:.04},{status:429}):Response.json({id:'1'});});
  const sleeps:number[]=[];const sleeping=(f.api as any).sleep.bind(f.api);(f.api as any).sleep=async(ms:number)=>{sleeps.push(ms);return sleeping(ms);};
  try {expect(await f.api.call('GET','/users/@me')).toEqual({id:'1'});expect(calls).toBe(2);expect(sleeps[0]).toBeGreaterThanOrEqual(35);expect(times[1]!-times[0]!).toBeGreaterThanOrEqual(35);}finally{f.stop();}
  const bad=rest(()=>Response.json({message:'fixture-record-token'},{status:401}));
  try {await expect(bad.api.call('GET','/users/@me')).rejects.toThrow('(401)');await expect(bad.api.call('GET','/users/@me')).rejects.toThrow('stopped');expect(bad.requests).toHaveLength(1);bad.api.reload();await expect(bad.api.call('GET','/users/@me')).rejects.toThrow('(401)');expect(bad.requests).toHaveLength(2);}finally{bad.stop();}
});
test('REST failures never retry an uncertain write and never follow redirects with the token', async () => {
  for (const response of [()=>new Response('',{status:500}),()=>new Response('',{status:302,headers:{location:'https://example.invalid'}})]) {
    const f=rest(response);try{await expect(f.api.call('POST','/channels/1/messages',{content:'hello'})).rejects.toThrow();expect(f.requests).toHaveLength(1);}finally{f.stop();}
  }
  const f=rest(()=>Response.json({retry_after:10},{status:429}));try{await expect(f.api.call('POST','/channels/1/messages',{},Date.now()+100)).rejects.toThrow('(429)');expect(f.requests).toHaveLength(1);}finally{f.stop();}
});
test('provider response bytes are capped before decoding and a burst of invalid requests stops at twenty',async()=>{
 const huge=rest(()=>new Response('x'.repeat(1024*1024+1)));try{await expect(huge.api.call('GET','/large')).rejects.toThrow('too large');}finally{huge.stop();}
 const bad=rest(()=>Response.json({},{status:403}));try{for(let n=0;n<20;n++)await expect(bad.api.call('GET','/bad/'+n)).rejects.toThrow('(403)');await expect(bad.api.call('GET','/another')).rejects.toThrow('stopped');expect(bad.requests).toHaveLength(20);}finally{bad.stop();}
});
