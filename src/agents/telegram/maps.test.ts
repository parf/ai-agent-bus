import {test,expect} from 'bun:test';
import {Router} from './router.ts';
import {parseConfig,resolveDestination,type Config} from './config.ts';
import {parseCall} from './parse.ts';

// Exercise public parsing/routing outcomes with hostile dictionary parents.
// State instrumentation simulates an in-memory polluted map without changing
// Object.prototype globally or involving real Telegram traffic.
test('Telegram Configuration and KV maps never route inherited constructor or __proto__ entries',async()=>{
 for(const field of ['aliases','users','tags','chats']) {
  const raw=JSON.parse(`{"${field}":{"__proto__":{}}}`);
  expect(typeof parseConfig(raw)).toBe('string');
 }
 expect(typeof parseConfig({chats:{constructor:{}}})).toBe('string');
 for(const method of ['constructor','__proto__']) {
  expect(()=>parseCall(JSON.stringify({method,extra:"x"}))).not.toThrow();
  expect(parseCall(JSON.stringify({method,extra:"x"})).method).toBe('help');
 }
 const config:Config={aliases:Object.create({'#constructor':'-999'}),users:{42:'parf'},tags:{},chats:{}};
 expect(resolveDestination('#constructor',config)).toEqual({ok:false,error:'unknown alias #constructor'});
 let saved:string|undefined;const sent:any[]=[],told:string[]=[];let toldId=0;
 const r=new Router({self:'#telegram@t',get:async()=>saved,set:async s=>{saved=JSON.stringify(s);},config:async()=>config,send:async(route,body)=>{sent.push({route,body});return String(sent.length);},tell:async(_c,_m,text)=>{told.push(text);return ++toldId;},fallback:async()=>false,log:()=>{}});await r.load();
 const message={message_id:1,date:1,from_id:42,text:'constructor test'};
 config.users=Object.create({constructor:'forged-person'});
 config.route='#sink@t';await r.incoming({...message,from_id:'constructor' as any},'100');expect(sent).toEqual([]);expect(told.at(-1)).toBe('we do not know you');
 config.users={42:'parf'};delete config.route;
 config.chats=Object.create({constructor:{anyone:true,route:'#forged@t'}});
 await r.incoming({...message,from_id:77},'constructor');expect(sent).toEqual([]);
 config.chats={};
 const state=(r as any).state;
 state.tags=Object.create({constructor:'#forged@t'});
 await r.incoming({...message,text:'#constructor must not route'},'100');expect(sent.map(s=>s.route.to)).toEqual(['#constructor']);expect(sent.some(s=>s.route.to==='#forged@t')).toBe(false);
 // A real own constructor tag remains valid data, rather than an inherited
 // Object constructor being mistaken for a session.
 config.tags={constructor:'#real@t'};await r.incoming({...message,text:'#constructor actual data'},'100');expect(sent.at(-1).route.to).toBe('#real@t');
 const count=sent.length;
 (r as any).state.messages=Object.create({'100:7':{to:'#forged@t',re:'constructor',until:Date.now()+10000},constructor:{},['__proto__']:{}});
 await r.incoming({...message,reply_to:7},'100');expect(sent.length).toBe(count);expect(told.at(-1)).toContain('did not understand');
 (r as any).state.answers=Object.create({constructor:{sender:'#forged@t',topic:'t',tag:'g',chat:'100',message:7,until:Date.now()+10000},['__proto__']:{}});
 const notices=told.length;
 await r.answer({message_id:'spoof',re:'constructor',from:'#forged@t',topic:'t',tag:'g',body:'must not post'});expect(told.length).toBe(notices);
});
