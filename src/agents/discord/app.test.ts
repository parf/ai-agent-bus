import { expect,test } from 'bun:test';
import { fixture,caller } from './fixture.ts';
const event=(id:string,content='hello',extra={})=>({id,channel_id:'10',guild_id:'1',author:{id:'7',username:'ada'},content,...extra});

test('every method uses the fake API with exact request data and oldest-first history',async()=>{
 const f=fixture();try{await f.app.initialize();
 expect(await f.app.call('whoami',caller)).toEqual({id:'9',username:'fixture-bot',guilds:[{id:'1',name:'allowed'}]});
 expect(await f.app.call('channels alerts',caller)).toEqual([{id:'10',name:'alerts',type:0,guild_id:'1',aliases:['alerts']}]);
 expect(await f.app.call('channel alerts',caller)).toEqual({id:'10',name:'alerts',type:0,topic:'fixture'});
 expect(await f.app.call('read alerts 2',caller)).toEqual([{id:'101',author:'7',text:'old',reply_to:undefined},{id:'102',author:'7',text:'new',reply_to:undefined}]);
 const post:any=await f.app.call('{"method":"post","channel":"alerts","text":"hello","reply_to":"100"}',caller);expect(post).toEqual({channel:'10',id:'1000',ids:['1000']});
 expect(f.requests.find(r=>r.method==='POST')?.body).toEqual({content:'hello\n\n#a',allowed_mentions:{parse:[],replied_user:false},message_reference:{message_id:'100',fail_if_not_exists:true}});
 expect(await f.app.call('react alerts 100 😀',caller)).toEqual({reacted:true});expect(f.requests.find(r=>r.method==='PUT')?.path).toBe('/channels/10/messages/100/reactions/%F0%9F%98%80/@me');
 expect(await f.app.call('edit alerts 100 changed',caller)).toEqual({edited:true});expect(f.requests.find(r=>r.method==='PATCH')?.body).toEqual({content:'changed',allowed_mentions:{parse:[]}});
 expect(await f.app.call('delete alerts 100',caller)).toEqual({deleted:true});expect(await f.app.call('thread alerts 100 answers',caller)).toEqual({id:'50',name:'answers'});
 expect(f.requests.every(r=>r.auth==='Bot fake-secret-do-not-print')).toBe(true);
 }finally{await f.close();}
});
test('an action in a guild outside guilds is refused before any write or forward',async()=>{
 const f=fixture();try{await f.app.initialize();for(const method of ['post','read','channel','react','edit','delete','thread','sub.add']){
 const c:any={method,channel:'20',text:'x',id:'100',emoji:'x',name:'x',target:'#destination@fixture'};
 const keys:any={post:['method','channel','text'],read:['method','channel'],channel:['method','channel'],react:['method','channel','id','emoji'],edit:['method','channel','id','text'],delete:['method','channel','id'],thread:['method','channel','id','name'],'sub.add':['method','channel','target']};
 await expect(f.app.call(JSON.stringify(Object.fromEntries(keys[method].map((k:string)=>[k,c[k]]))),caller)).rejects.toThrow('outside Configuration guilds');}
 await f.app.event('MESSAGE_CREATE',event('1','foreign',{guild_id:'2',channel_id:'20'}));expect(f.requests.filter(r=>r.method!=='GET')).toEqual([]);expect(f.sent).toEqual([]);
 }finally{await f.close();}
});
test('post and read without a configured default are refused and do not choose a channel',async()=>{
 const f=fixture({guilds:['1'],channels:{alerts:{id:'10'}}});try{await f.app.initialize();await expect(f.app.call('post hello',caller)).rejects.toThrow('Configuration default');await expect(f.app.call('read',caller)).rejects.toThrow('Configuration default');expect(f.requests.some(r=>r.path.includes('/channels/'))).toBe(false);}finally{await f.close();}
});
test('another author message cannot be edited and provider write failures stay explicit without a retry loop',async()=>{
 const f=fixture();try{await f.app.initialize();await expect(f.app.call('edit alerts 101 no',caller)).rejects.toThrow('only the bot');expect(f.requests.filter(r=>r.method==='PATCH')).toEqual([]);
 f.state.failPath='/channels/10/messages/100';f.state.failStatus=403;await expect(f.app.call('delete alerts 100',caller)).rejects.toThrow('(403)');await expect(f.app.call('delete alerts 100',caller)).rejects.toThrow('stopped');expect(f.requests.filter(r=>r.method==='DELETE')).toHaveLength(1);
 }finally{await f.close();}
});
test('provider post chunks never exceed 2000 characters or eight parts',async()=>{
 const f=fixture();try{await f.app.initialize();await f.app.call(JSON.stringify({method:'post',text:'x'.repeat(20000)}),caller);const posts=f.requests.filter(r=>r.method==='POST');expect(posts).toHaveLength(8);expect(posts.every(r=>[...r.body.content].length<=2000)).toBe(true);expect(posts.at(-1)!.body.content).toEndWith('… (truncated)');}finally{await f.close();}
});
test('shared subscriptions refuse malformed filters, count refused/unknown copies once and keep next matches active',async()=>{
 const f=fixture();try{await f.app.initialize();await expect(f.app.call('sub add alerts #target@fixture /(/',caller)).rejects.toMatchObject({code:'invalid_filter'});expect([...f.values.keys()].filter(k=>k.includes('sub.'))).toEqual([]);
 const row:any=await f.app.call('sub add alerts #target@fixture',caller);expect(row.id).toMatch(/^[0-9a-f]{12}$/);
 f.state.refuse=true;await f.app.event('MESSAGE_CREATE',event('1'));f.state.refuse=false;await f.app.event('MESSAGE_CREATE',event('2'));
 f.state.unknown=true;await f.app.event('MESSAGE_CREATE',event('3'));f.state.unknown=false;await f.app.event('MESSAGE_CREATE',event('4'));
 expect(f.sent).toHaveLength(2);expect(f.warnings).toEqual([`subscription ${row.id}: send_refused`,`subscription ${row.id}: send_unknown`]);
 expect(f.sent.every(r=>!Object.hasOwn(r.options,'wait'))).toBe(true);expect(await f.app.call('sub list',caller)).toEqual([expect.objectContaining({id:row.id,sent:2,dropped:2})]);
 f.state.admit=false;await f.app.event('MESSAGE_CREATE',event('5'));expect(f.sent).toHaveLength(2);expect(f.warnings.at(-1)).toContain('admission_refused');
 }finally{await f.close();}
});
test('subscription management is author or direct staff, and sub.edit is refused',async()=>{
 const f=fixture();try{await f.app.initialize();const row:any=await f.app.call('sub add alerts #target@fixture',caller);
 const other={...caller,from:'#other@fixture'};expect(await f.app.call('sub list',other)).toEqual([]);await expect(f.app.call('sub delete '+row.id,other)).rejects.toMatchObject({code:'not_author'});
 await expect(f.app.call('sub edit '+row.id,caller)).rejects.toThrow('delete it and add it again');
 await expect(f.app.call('sub delete '+row.id,{...other,roles:['owner'],original_to:'#front@fixture'})).rejects.toMatchObject({code:'not_author'});
 expect(await f.app.call('sub delete '+row.id,{...other,roles:['owner']})).toEqual({deleted:true});expect(await f.app.call('sub list',caller)).toEqual([]);
 }finally{await f.close();}
});
test('own posts, edits and duplicate gateway events never forward subscription copies',async()=>{
 const f=fixture();try{await f.app.initialize();await f.app.call('sub add alerts #target@fixture',caller);
 await f.app.event('MESSAGE_CREATE',event('1','own',{author:{id:'9'}}));await f.app.event('MESSAGE_UPDATE',event('2'));await f.app.event('MESSAGE_CREATE',event('3'));await f.app.event('MESSAGE_CREATE',event('3'));
 expect(f.sent).toHaveLength(1);expect(JSON.parse(f.sent[0].body)).toMatchObject({channel:'10',user:'7',text:'hello',message_id:'3'});
 }finally{await f.close();}
});
test('replies, tags, direct messages and default routes reach the right session; unlisted users never pick a tag',async()=>{
 const f=fixture();try{await f.app.initialize();const posted:any=await f.app.call('post hello',caller);
 await f.app.event('MESSAGE_CREATE',event('1','reply',{message_reference:{message_id:posted.id}}));expect(f.sent.at(-1)).toMatchObject({to:caller.from,re:caller.message_id,topic:'work',tag:'t'});
 await f.app.event('MESSAGE_CREATE',event('2','<@9> #a tagged'));expect(f.sent.at(-1).to).toBe(caller.from);
 f.config.route='#default@fixture';await f.app.event('MESSAGE_CREATE',event('3','a direct message',{channel_id:'30',guild_id:undefined}));expect(f.sent.at(-1).to).toBe('#default@fixture');
 const n=f.sent.length;await f.app.event('MESSAGE_CREATE',event('4','<@9> #a forbidden',{author:{id:'8'}}));expect(f.sent).toHaveLength(n);expect(f.requests.filter(r=>r.method==='POST').at(-1)?.body.content).toBe('we do not know you');
 delete f.config.route;await f.app.event('MESSAGE_CREATE',event('5','<@9> where'));expect(f.requests.filter(r=>r.method==='POST').at(-1)?.body.content).toContain('did not understand');
 }finally{await f.close();}
});
test('an expired caller deadline refuses before any provider action or subscription write',async()=>{
 const f=fixture();try{await f.app.initialize();const n=f.requests.length;await expect(f.app.call('post no',caller,Date.now()-1)).rejects.toThrow('deadline');expect(f.requests).toHaveLength(n);expect(f.sent).toEqual([]);}finally{await f.close();}
});
