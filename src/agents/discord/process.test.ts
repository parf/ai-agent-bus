import { expect,test } from 'bun:test';
import { spawn } from 'bun';
import { join } from 'node:path';
import { readFileSync,statSync,existsSync } from 'node:fs';
import { Bus } from '../../clients/typescript/src/index.ts';
import { startFixture,identity } from '../slack/daemon.ts';
import { fixture } from './fixture.ts';

test('packaged self-serving agent reads its own record Secret and Configuration and answers real read-only bus calls',async()=>{
 const fx=await startFixture(),f=fixture();let child:ReturnType<typeof spawn>|undefined,client:Bus|undefined;
 const frames:any[]=[];let socket:any;
 const gateway=Bun.serve({hostname:'127.0.0.1',port:0,fetch(r,s){return s.upgrade(r)?undefined:new Response(null,{status:400});},websocket:{open(ws){socket=ws;ws.send(JSON.stringify({op:10,d:{heartbeat_interval:100}}));},message(ws,data){const m=JSON.parse(String(data));frames.push(m);if(m.op===1)ws.send(JSON.stringify({op:11,d:null}));if(m.op===2)ws.send(JSON.stringify({op:0,s:1,t:'READY',d:{session_id:'private-test',resume_gateway_url:`ws://127.0.0.1:${gateway.port}`,user:{id:'9'}}}));}}});
 try{
 const name='#discord/fixture@'+fx.owner.split('@')[1],token=await identity(fx,name);
 await fx.call(fx.ownerToken,'POST','/configure',{name,config:f.config});
 await fx.call(fx.ownerToken,'POST','/secret',{name,secret:`DISCORD_BOT_TOKEN=fake-secret-do-not-print\nDISCORD_API_URL=http://127.0.0.1:${f.server.port}\nDISCORD_GATEWAY_URL=ws://127.0.0.1:${gateway.port}\n`});
 const dest=join(fx.dir,'discord');const pkg=Bun.spawnSync([join(import.meta.dir,'package.sh'),dest]);expect(pkg.exitCode).toBe(0);
 const manifest=JSON.parse(readFileSync(join(dest,'agent.json'),'utf8'));expect(manifest.command).toBe('./discord.js');expect(manifest.serves).toBe('itself');
 expect(existsSync(join(dest,'LICENSE.md'))).toBe(true);expect(existsSync(join(dest,'licenses/NOTICE.md'))).toBe(true);
 const bundled=readFileSync(join(dest,'discord.js'),'utf8');expect(bundled).toContain('class Subscriptions');expect(bundled).not.toContain('internal/version/VERSION');expect(bundled).not.toMatch(/from\s+["']\.\.\//);expect(readFileSync(join(dest,'licenses/NOTICE.md'),'utf8')).toContain('Notices');expect(statSync(join(dest,'licenses')).mode&0o777).toBe(0o755);expect(readFileSync(join(dest,'LICENSE.md'),'utf8')).toContain('PolyForm');
 child=spawn(['bun',join(dest,'discord.js')],{cwd:fx.dir,env:{...process.env,AGENT_BUS_NAME:name,AGENT_BUS_ADDR:fx.socket,AGENT_BUS_TOKEN:token,DISCORD_BOT_TOKEN:'wrong-env'},stdout:'pipe',stderr:'pipe'});
 const end=Date.now()+5000;while(!frames.some(m=>m.op===2)&&Date.now()<end)await Bun.sleep(10);
 if(!frames.some(m=>m.op===2)){child.kill();await child.exited;throw Error('agent did not connect: '+await new Response(child.stderr).text());}
 expect(frames.find(m=>m.op===2).d.token).toBe('fake-secret-do-not-print');
 client=Bus.connect({addr:fx.socket,token:fx.ownerToken});
 const who=await client.call(name,'whoami',{timeout:3000});expect(who.kind).toBe('answer');if(who.kind!=='answer')throw Error('missing whoami');expect(JSON.parse(who.message.body)).toEqual({ok:true,result:{id:'9',username:'fixture-bot',guilds:[{id:'1',name:'allowed'}]}});
 const bin=process.env.SMOKE_BIN??fx.dir;
 if(!existsSync(join(bin,'agent-bus'))){const build=spawn(['go','build','-o',join(bin,'agent-bus'),'./cmd/agent-bus'],{cwd:join(import.meta.dir,'../..'),stdout:'ignore',stderr:'pipe'});const [exit,errors]=await Promise.all([build.exited,new Response(build.stderr).text()]);expect(exit,errors).toBe(0);}
 const cli=spawn(['bash','-o','pipefail','-c',`agent-bus call "$DISCORD_AGENT" --wait 3s whoami | jq '.body | fromjson | .result'`],{env:{...process.env,PATH:bin+':'+process.env.PATH,DISCORD_AGENT:name,AGENT_BUS_ADDR:fx.socket,AGENT_BUS_TOKEN:fx.ownerToken,AGENT_BUS_NAME:fx.owner},stdout:'pipe',stderr:'pipe'});
 const cliOut=await new Response(cli.stdout).text(),cliErr=await new Response(cli.stderr).text();expect(await cli.exited,cliErr).toBe(0);expect(JSON.parse(cliOut)).toEqual({id:'9',username:'fixture-bot',guilds:[{id:'1',name:'allowed'}]});
 const help=await client.call(name,'',{ask:'help-json',timeout:3000});expect(help.kind).toBe('answer');if(help.kind!=='answer')throw Error('missing help');const data=JSON.parse(help.message.body);expect(data.methods['sub.edit']).toBeUndefined();expect(data.methods['sub.add']).toBeDefined();expect(JSON.stringify(data)).not.toContain('fake-secret');
 const edit=await client.call(name,'sub edit abc',{timeout:3000});expect(edit.kind).toBe('answer');if(edit.kind==='answer')expect(edit.message.body).toContain('delete it and add it again');
 // Exercise an actual provider refusal after packaging, not merely help or validation.
 f.state.failPath='/channels/10/messages';f.state.failMethod='GET';f.state.failStatus=403;
 const beforeRefusal=f.requests.length;
 const refused=await client.call(name,'read alerts',{timeout:3000});
 expect(refused.kind).toBe('answer');if(refused.kind!=='answer')throw Error('missing refused read reply');
 expect(refused.message.body).toContain('error: Discord refused the request (403)');
 expect(refused.message.body).not.toContain('fake-secret-do-not-print');
 expect(f.requests.slice(beforeRefusal).some(r=>r.path.startsWith('/channels/10/messages')&&r.method==='GET')).toBe(true);
 // A gateway action takes the asynchronous stderr error path in the installed bundle.
 f.state.failPath='/channels/11';
 socket.send(JSON.stringify({op:0,s:2,t:'MESSAGE_CREATE',d:{id:'200',channel_id:'11',guild_id:'1',author:{id:'7'},content:'fixture'}}));
 const gatewayEnd=Date.now()+3000;
 while(!f.requests.some(r=>r.path==='/channels/11')&&Date.now()<gatewayEnd)await Bun.sleep(10);
 expect(f.requests.some(r=>r.path==='/channels/11')).toBe(true);
 const barrier=await client.call(name,'whoami',{timeout:3000});expect(barrier.kind).toBe('answer');
 child.kill('SIGTERM');expect(await Promise.race([child.exited,Bun.sleep(7000).then(()=>-999)])).toBe(0);
 const stderr=await new Response(child.stderr).text();
 expect(stderr).not.toContain('fake-secret-do-not-print');
 expect(stderr).toBe('discord: gateway event failed; it is not replayed\n');expect(f.requests.every(r=>r.auth==='Bot fake-secret-do-not-print')).toBe(true);
 }finally{child?.kill('SIGKILL');await child?.exited;await client?.close();gateway.stop(true);await f.close();await fx.stop();}
},20000);

test('fake gateway matches cross the real bus once: a refused copy drops and the next match works; revoked authors send nothing',async()=>{
 const fx=await startFixture(),f=fixture(),sockets:any[]=[];let child:ReturnType<typeof spawn>|undefined;const clients:Bus[]=[];
 const gateway=Bun.serve({hostname:'127.0.0.1',port:0,fetch(r,s){return s.upgrade(r)?undefined:new Response(null,{status:400});},websocket:{open(ws){sockets.push(ws);ws.send(JSON.stringify({op:10,d:{heartbeat_interval:100}}));},message(ws,data){const m=JSON.parse(String(data));if(m.op===1)ws.send(JSON.stringify({op:11,d:null}));if(m.op===2)ws.send(JSON.stringify({op:0,s:1,t:'READY',d:{session_id:'delivery',resume_gateway_url:`ws://127.0.0.1:${gateway.port}`,user:{id:'9',username:'fixture-bot'}}}));}}});
 try{
 const realm=fx.owner.split('@')[1],name='#discord/delivery@'+realm,author='#author@'+realm,target='#sink@'+realm;
 const token=await identity(fx,name),authorToken=await identity(fx,author);
 await fx.call(fx.ownerToken,'POST','/register',{name:target,kind:'agent',bound:1,allow:[name,author]});
 const targetToken=(await fx.call(fx.ownerToken,'POST','/token',{name:target})).token;
 await fx.call(fx.ownerToken,'POST','/manage',{name,allow:[author]});
 await fx.call(fx.ownerToken,'POST','/configure',{name,config:f.config});
 await fx.call(fx.ownerToken,'POST','/secret',{name,secret:`DISCORD_BOT_TOKEN=fake-secret-do-not-print\nDISCORD_API_URL=http://127.0.0.1:${f.server.port}\nDISCORD_GATEWAY_URL=ws://127.0.0.1:${gateway.port}\n`});
 child=spawn(['bun',join(import.meta.dir,'discord.ts')],{env:{...process.env,AGENT_BUS_NAME:name,AGENT_BUS_ADDR:fx.socket,AGENT_BUS_TOKEN:token},stdout:'pipe',stderr:'pipe'});
 const owner=Bus.connect({addr:fx.socket,token:fx.ownerToken}),writer=Bus.connect({addr:fx.socket,token:authorToken}),sink=Bus.connect({addr:fx.socket,token:targetToken});clients.push(owner,writer,sink);
 const added=await writer.call(name,`sub add alerts ${target}`,{timeout:5000});expect(added.kind).toBe('answer');if(added.kind!=='answer')throw Error('no add reply');const row=JSON.parse(added.message.body).result;expect(row.id).toMatch(/^[0-9a-f]{12}$/);
 let sequence=1;const emit=()=>{const s=++sequence;sockets.at(-1).send(JSON.stringify({op:0,s,t:'MESSAGE_CREATE',d:{id:String(100+s),channel_id:'10',guild_id:'1',author:{id:'7',username:'ada'},content:'same match'}}));};
 const counts=async(sent:number,dropped:number)=>{const end=Date.now()+3000;let r:any;do{r=(await owner.record(name).kv.get('json','sub.'+row.id)).value;if(r.sent===sent&&r.dropped===dropped)break;await Bun.sleep(10);}while(Date.now()<end);expect(r).toMatchObject({sent,dropped});};
 emit();await counts(1,0);emit();await counts(1,1);expect((await sink.consume({wait:100}))?.body).toContain('same match');
 emit();await counts(2,1);expect((await sink.consume({wait:1000}))?.body).toContain('same match');expect(await sink.consume({wait:50})).toBeNull();
 await fx.call(fx.ownerToken,'POST','/manage',{name:target,allow:[name]});emit();await counts(2,2);expect(await sink.consume({wait:50})).toBeNull();
 await fx.call(fx.ownerToken,'POST','/manage',{name:target,allow:[name,author]});emit();await counts(3,2);expect((await sink.consume({wait:1000}))?.body).toContain('same match');
 await fx.call(fx.ownerToken,'POST','/manage',{name,allow:[]});emit();await counts(3,3);expect(await sink.consume({wait:50})).toBeNull();
 child.kill('SIGTERM');expect(await Promise.race([child.exited,Bun.sleep(7000).then(()=>-999)])).toBe(0);
 const stderr=await new Response(child.stderr).text();expect(stderr.split('\n').filter(x=>x.includes('send_refused'))).toHaveLength(1);expect(stderr.split('\n').filter(x=>x.includes('admission_refused'))).toHaveLength(2);expect(stderr).not.toContain('fake-secret-do-not-print');
 }finally{child?.kill('SIGKILL');await child?.exited;for(const c of clients)await c.close();gateway.stop(true);await f.close();await fx.stop();}
},20000);
