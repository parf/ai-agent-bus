import { expect, test } from 'bun:test';
import { Api } from './api.ts';
import { Gateway, INTENTS } from './gateway.ts';
const until = async (f:()=>boolean) => {const end=Date.now()+1000;while(!f()&&Date.now()<end)await Bun.sleep(5);expect(f()).toBe(true);};
function fixture(ack=true,close?:number) {
  const frames:any[]=[],sockets:any[]=[];
  const server=Bun.serve({hostname:'127.0.0.1',port:0,fetch(r,s){return s.upgrade(r)?undefined:new Response('no',{status:400});},websocket:{
    open(ws){sockets.push(ws);if(close){ws.close(close);return;}ws.send(JSON.stringify({op:10,d:{heartbeat_interval:20}}));},
    message(ws,data){const f=JSON.parse(String(data));frames.push(f);if(f.op===1&&ack)ws.send(JSON.stringify({op:11,d:null}));
      if(f.op===2)ws.send(JSON.stringify({op:0,s:1,t:'READY',d:{user:{id:'9'},session_id:'session-1',resume_gateway_url:`ws://127.0.0.1:${server.port}`}}));
      if(f.op===6){ws.send(JSON.stringify({op:0,s:2,t:'MESSAGE_CREATE',d:{id:'10'}}));ws.send(JSON.stringify({op:0,s:3,t:'RESUMED',d:{}}));}
    },
  }});
  const api=new Api(async()=>({DISCORD_BOT_TOKEN:'fake-token',DISCORD_GATEWAY_URL:`ws://127.0.0.1:${server.port}`}));
  return {api,frames,sockets,stop:()=>{for(const s of sockets)s.close();server.stop(true);}};
}
test('gateway identifies with content intent, heartbeats, resumes and discards replayed sequences',async()=>{
  const f=fixture(),events:any[]=[],logs:string[]=[];
  const g=new Gateway(f.api,(kind,data)=>events.push([kind,data]),s=>logs.push(s),5);
  try {
    await g.start();await until(()=>events.some(e=>e[0]==='READY'));
    expect(f.frames.find(x=>x.op===2).d).toMatchObject({token:'fake-token',intents:INTENTS});
    expect(INTENTS & (1<<15)).not.toBe(0);
    await until(()=>f.frames.some(x=>x.op===1));
    f.sockets[0].send(JSON.stringify({op:0,s:2,t:'MESSAGE_CREATE',d:{id:'10'}}));
    await until(()=>events.some(e=>e[0]==='MESSAGE_CREATE'));
    f.sockets[0].send(JSON.stringify({op:7,d:null}));
    await until(()=>events.some(e=>e[0]==='RESUMED'));
    expect(f.frames.filter(x=>x.op===6)).toEqual([{op:6,d:{token:'fake-token',session_id:'session-1',seq:2}}]);
    expect(events.filter(e=>e[0]==='MESSAGE_CREATE')).toHaveLength(1);
  }finally{g.stop();f.stop();}
  expect(g.armed).toBe(false);expect(logs.join(' ')).not.toContain('fake-token');
});
test('a missing heartbeat ACK reconnects; stop cancels all timers and future connections',async()=>{
  const f=fixture(false),g=new Gateway(f.api,()=>{},()=>{},5);
  try {await g.start();await until(()=>f.sockets.length>=2);g.stop();const n=f.sockets.length;await Bun.sleep(60);expect(f.sockets).toHaveLength(n);expect(g.armed).toBe(false);}finally{g.stop();f.stop();}
});
test('fatal authentication or intent refusal never creates an invalid-request reconnect loop',async()=>{
  for(const code of [4004,4014]){const f=fixture(true,code),logs:string[]=[],g=new Gateway(f.api,()=>{},s=>logs.push(s),5);
    try{await g.start();await until(()=>logs.some(s=>s.includes(String(code))));await Bun.sleep(30);expect(f.sockets).toHaveLength(1);expect(g.armed).toBe(false);}finally{g.stop();f.stop();}}
});
