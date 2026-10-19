import { expect, test } from "bun:test";
import { Router, type Route } from "./router.ts";
import { parseConfig, type Config } from "./config.ts";

function fixture() {
 let saved: string | undefined;
 let config: Config = { aliases: {}, users: { "42": "parf" }, tags: { b: "#beta@t" } };
 const sent: { route: Partial<Route>; body: string }[] = [], told: string[] = [], logs: string[] = [];
 let matched = false;
 const deps = { self: "#telegram@t", get: async () => saved, set: async (s: unknown) => { saved = JSON.stringify(s); }, config: async () => config,
  send: async (route: Partial<Route>, body: string) => { sent.push({ route, body }); return "bus-"+sent.length; }, tell: async (_c: string, _m: number, text: string) => { told.push(text); return told.length; }, fallback: async () => matched, log: (line: string) => logs.push(line) };
 return { deps, sent, told, logs, config, saved: () => saved, matched: () => { matched = true; } };
}
const message = (id=1) => ({ message_id: id, date: 1, from_id: 42, username: "parf", name: "Parf Person", text: "answer" });

test("session tags survive restarts, reserve pins and cannot be reassigned", async () => {
 const f=fixture(); let r=new Router(f.deps);await r.load();
 expect(await r.tag("#bob@t",f.config)).toBe("b1");
 expect(await r.tag("#beta@t",f.config)).toBe("b");
 r=new Router(f.deps);await r.load();
 expect(await r.tag("#bob@t",f.config)).toBe("b1");
 expect(await r.tag("#bill@t",f.config)).toBe("b2");
 await expect(r.tag("#evil@t",{aliases:{},tags:{b:"#evil@t"}})).rejects.toThrow("permanent");
 expect(JSON.parse(f.saved()!).tags.b).toBe("#beta@t");
});

test("an expired reply is answered rather than routed", async () => {
 const f=fixture(),r=new Router(f.deps);await r.load();
 const now=Date.now;
 try {
  Date.now=()=>1;
  await r.posted("100",7,{to:"#bob@t",topic:"original",tag:"one",re:"m1"});
  Date.now=()=>7*24*60*60*1000+2;
  await r.incoming({...message(),reply_to:7},"100");
  expect(f.sent).toEqual([]);expect(f.told).toEqual(["that thread expired; use #tag"]);
 } finally {Date.now=now;}
});

test("routing precedence is reply, tag, subscriptions, route, then a useful notice", async () => {
 const f=fixture(),r=new Router(f.deps);await r.load();
 await r.posted("100",7,{to:"#alice@t",topic:"old",tag:"tag",re:"m1"});
 f.config.route="#default@t";
 await r.incoming({...message(),reply_to:7,text:"#b answer"},"100");
 expect(f.sent.at(-1)).toEqual({route:{to:"#alice@t",topic:"old",tag:"tag",re:"m1"},body:"[parf (TG)] #b answer"});
 await r.incoming({...message(),text:"#b direct"},"100");
 expect(f.sent.at(-1)?.route.to).toBe("#beta@t");
 await r.incoming({...message(),text:"#full/name@t direct"},"100");
 expect(f.sent.at(-1)?.route.to).toBe("#full/name@t");
 await r.incoming(message(),"100");expect(f.sent.at(-1)?.route.to).toBe("#default@t");
 const count=f.sent.length;f.matched();await r.incoming(message(),"100");expect(f.sent.length).toBe(count);
 const g=fixture(),s=new Router(g.deps);await s.load();await s.incoming(message(),"100");
 expect(g.sent).toEqual([]);expect(g.told[0]).toContain("did not understand");expect(g.told[0]).toContain("#b");
});

test("unlisted senders never route and their bounded audit persists", async () => {
 const f=fixture(),r=new Router(f.deps);await r.load();
 for(let n=0;n<22;n++)await r.incoming({...message(n),from_id:100+n,text:"#b "+"x".repeat(100)},"100","Private room");
 await r.incoming({...message(23),from_id:121},"101","Other room");
 expect(f.sent).toEqual([]);expect(f.told.every(text=>text==="we do not know you")).toBe(true);
 expect(f.logs[0]).toContain('"user":"100"');expect(f.logs[0]).toContain('@parf');expect(f.logs[0]).toContain('Parf Person');
 expect(f.logs[0]).toContain('Private room');expect(f.logs[0]).toContain('not a listed user');expect(f.logs[0]).not.toContain("x".repeat(41));
 const previous=f.told.length;await r.incoming({...message(24),from_id:100},"100");expect(f.told.length).toBe(previous);
 const restored=new Router(f.deps);await restored.load();
 expect(restored.refused().length).toBe(20);expect(restored.refused().find(s=>s.id==="121")?.count).toBe(2);expect(restored.refused().find(s=>s.id==="121")?.chat).toBe("101");
});

test("routing Configuration refuses malformed tags, users and route", () => {
 for(const bad of [{tags:[]},{tags:{b:"not an agent"}},{tags:{b:"#b@t",c:"#b@t"}},{users:{no:"parf"}},{users:{42:"[spoof]"}},{route:"bad name"}])expect(typeof parseConfig(bad)).toBe("string");
 expect(parseConfig({tags:{b:"#beta@t"},users:{42:"parf"},route:"#sink@t"})).toEqual({aliases:{},tags:{b:"#beta@t"},users:{42:"parf"},route:"#sink@t"});
});

test("session answers require the mapped sender, correlation and unexpired reverse route",async()=>{
 const f=fixture(),r=new Router(f.deps);await r.load();await r.incoming({...message(),text:'#b go'},'100');
 const route=JSON.parse(f.saved()!).answers['bus-1'];
 const answer={message_id:'answer-1',re:'bus-1',from:'#beta@t',topic:route.topic,tag:route.tag,body:'answered'};
 await r.answer({...answer,from:'#spoof@t'});await r.answer({...answer,tag:'wrong'});
 expect(f.told).toEqual([]);expect(f.logs.filter(s=>s.includes('unmatched')).length).toBe(2);
 await r.answer(answer);expect(f.told).toEqual(['answered\n\n#b']);
 const restore=new Router(f.deps);await restore.load();
 const now=Date.now;try {Date.now=()=>route.until+1;await restore.answer(answer);}finally{Date.now=now;}
 expect(f.told.length).toBe(1);
});


test("unknown-user notices are limited to once per sender per hour, audit counts every refusal",async()=>{
 const f=fixture(),r=new Router(f.deps);await r.load();
 const now=Date.now;let clock=now();Date.now=()=>clock;
 try {
  await r.incoming({...message(),from_id:43},"100");
  await r.incoming({...message(2),from_id:43},"101");
  expect(f.told.length).toBe(1);expect(f.logs.length).toBe(2);expect(r.refused()[0]?.count).toBe(2);
  const restarted=new Router(f.deps);await restarted.load();await restarted.incoming({...message(3),from_id:43},"100");expect(f.told.length).toBe(1);
  clock+=3600000;await restarted.incoming({...message(4),from_id:43},"100");expect(f.told.length).toBe(2);expect(restarted.refused()[0]?.count).toBe(4);
 } finally {Date.now=now;}
});


test("many distinct refusal-notice entries expire on the next state write",async()=>{
 const f=fixture(),r=new Router(f.deps);await r.load();
 const now=Date.now;let clock=now();Date.now=()=>clock;
 try {
  for(let n=100;n<200;n++)await r.incoming({...message(n),from_id:n},"100");
  expect(Object.keys(JSON.parse(f.saved()!).notices).length).toBe(100);expect(r.refused().length).toBe(20);
  clock+=3600000;
  await r.tag('#new@t',f.config);
  expect(Object.keys(JSON.parse(f.saved()!).notices).length).toBe(0);
  await r.incoming({...message(201),from_id:201},"100");
  expect(Object.keys(JSON.parse(f.saved()!).notices)).toEqual(['201']);
 }finally{Date.now=now;}
});
