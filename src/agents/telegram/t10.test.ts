import {expect,test} from 'bun:test';
import {Router} from './router.ts';
import {parseConfig,type Config} from './config.ts';
import {Receiver} from './receive.ts';
import {Seen} from './seen.ts';

test('TG.10 per-chat route, disabled tags and anyone are confined to that chat',async()=>{
 const raw={aliases:{'#friends':'-100'},users:{'42':'parf'},tags:{b:'#beta@t'},route:'#global@t',chats:{'#friends':{route:'#friends@t',tags:false,anyone:true},'-200':{route:'#company@t'}}};
 const config=parseConfig(raw) as Config;expect(typeof config).toBe('object');
 let state:string|undefined;const sent:any[]=[],told:string[]=[];
 const r=new Router({self:'#telegram@t',get:async()=>state,set:async v=>{state=JSON.stringify(v);},config:async()=>config,send:async(route,body,reply)=>{sent.push({route,body,reply});return String(sent.length);},tell:async(_c,_m,text)=>{told.push(text);return told.length;},fallback:async()=>false,log:()=>{}});
 await r.load();const m={message_id:1,date:1,from_id:42,text:'ordinary'};
 await r.incoming(m,'-100');expect(sent.at(-1).route.to).toBe('#friends@t');
 await r.incoming({...m,text:'#b ordinary text'},'-100');expect(sent.at(-1).route.to).toBe('#friends@t');expect(sent.at(-1).body).toBe('[parf (TG)] #b ordinary text');
 await r.incoming({...m,from_id:77,text:'group member'},'-100');expect(sent.at(-1).route.to).toBe('#friends@t');expect(told).toEqual([]);
 const count=sent.length;await r.incoming({...m,from_id:77,text:'outside'},'-300');expect(sent.length).toBe(count);expect(told).toEqual(['we do not know you']);
 await r.incoming({...m,text:'#b tag works'},'-300');expect(sent.at(-1).route.to).toBe('#beta@t');
 await r.incoming(m,'-300');expect(sent.at(-1).route.to).toBe('#global@t');
 await r.posted('-100',10,{to:'#beta@t',topic:'original',tag:'tag',re:'bus1'});
 await r.incoming({...m,reply_to:10,text:'Reply still works'},'-100');expect(sent.at(-1).route).toEqual({to:'#beta@t',topic:'original',tag:'tag',re:'bus1'});
 await r.incoming({...m,channel_post:true,from_id:undefined,text:'announcement'},'-200');expect(sent.at(-1).route.to).toBe('#company@t');expect(sent.at(-1).body).toBe('[channel -200 (TG)] announcement');
 await r.posted('-200',11,{to:'#beta@t',re:'own-post'});const before=sent.length;await r.incoming({...m,channel_post:true,message_id:11,from_id:undefined},'-200');expect(sent.length).toBe(before);
});

test('TG.10 chat configuration fails loudly and alias settings follow their current id',()=>{
 for(const chats of [[],{'constructor':{}},{'unknown':{}},{'#room':{bad:true}},{'#room':{tags:'false'}},{'#room':{anyone:1}},{'#room':{route:'bad name'}},{'#room':{},'-100':{}}])expect(typeof parseConfig({aliases:{'#room':'-100'},chats})).toBe('string');
 expect((parseConfig({aliases:{'#room':'-200'},chats:{'#room':{route:'#target@t'}}}) as Config).chats).toEqual({'-200':{route:'#target@t'}});
});

test('TG.10 channel_post enters the receiving and routing path',async()=>{
 const seen=new Seen();let done=false;const messages:any[]=[];
 const receiver=new Receiver(seen,async()=>({ok:true,result:[{update_id:1,channel_post:{message_id:9,date:1,text:'channel announcement',chat:{id:-200,type:'channel',title:'Company'}}}]}),async()=>undefined,async(key)=>{if(key==='offset'){receiver.stop();done=true;}},()=>{});
 receiver.onMessage=async(m,chat,title)=>{messages.push({m,chat,title});};
 try{await receiver.load();receiver.start();const end=Date.now()+2000;while(!done&&Date.now()<end)await Bun.sleep(5);expect(done).toBe(true);expect(messages).toMatchObject([{chat:'-200',title:'Company',m:{message_id:9,channel_post:true,text:'channel announcement'}}]);}finally{receiver.stop();}
});


test('TG.10 anyone-only members cannot select tags, full names, subscriptions or a global route',async()=>{
 const config:Config={aliases:{},tags:{b:'#beta@t'},route:'#global@t',chats:{'-100':{route:'#friends@t',anyone:true},'-200':{anyone:true}}};
 let saved:string|undefined;const sent:any[]=[],told:string[]=[];let subscriptions=0;
 const r=new Router({self:'#telegram@t',get:async()=>saved,set:async v=>{saved=JSON.stringify(v);},config:async()=>config,send:async(route,body)=>{sent.push({route,body});return String(sent.length);},tell:async(_c,_m,text)=>{told.push(text);return told.length;},fallback:async()=>{subscriptions++;return true;},log:()=>{}});await r.load();
 const message={message_id:1,date:1,from_id:77,text:'#b do something'};
 await r.incoming(message,'-100');expect(sent.map(s=>s.route.to)).toEqual(['#friends@t']);expect(sent[0].body).toContain('#b do something');
 await r.incoming({...message,text:'#beta@t full-name request'},'-100');expect(sent.at(-1).route.to).toBe('#friends@t');expect(subscriptions).toBe(0);
 const count=sent.length;await r.incoming(message,'-200');expect(sent.length).toBe(count);expect(told.at(-1)).toContain('did not understand');
 await r.posted('-100',9,{to:'#beta@t',re:'allowed-thread'});await r.incoming({...message,reply_to:9},'-100');expect(sent.at(-1).route.to).toBe('#beta@t');
 await r.incoming({...message,reply_to:9},'-200');expect(sent.length).toBe(count+1);expect(told.at(-1)).toContain('expired');
});
