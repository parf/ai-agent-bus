import { expect, test } from "bun:test";
import { Router, type Route } from "./router.ts";
import { stateBudget, routeGone, idOrder } from "../shared/router.ts";
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
  expect(f.sent).toEqual([]);expect(f.told).toEqual(["route expired or evicted; use #tag"]);
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

test("valid bus names survive stored routing and both platform configurations",async()=>{
 for(const name of ["ann+ci@parf.us","#codex2","svc/inst@h","#claude/x@my_host"]){
  const f=fixture(),r=new Router(f.deps);await r.load();await r.tag(name,f.config);
  await expect(new Router(f.deps).load()).resolves.toBeUndefined();
  expect(typeof parseConfig({route:name})).not.toBe("string");
 }
});

test("a malformed stored row is reported and skipped without losing valid routes",async()=>{
 const f=fixture();const raw={tags:{a:"ann+ci@parf.us",bad:"bad name",c:"#codex2"},messages:{"100:7":{to:"svc/inst@h",re:"m",until:Date.now()+10000},bad:{to:"bad name",re:"m",until:Date.now()+10000}}};
 const r=new Router({...f.deps,get:async()=>JSON.stringify(raw)});await r.load();
 expect(await r.tag("ann+ci@parf.us",f.config)).toBe("a");expect(await r.tag("#codex2",f.config)).toBe("c");
 expect(r.mapped("100",7)).toBe(true);
 expect(f.logs.filter(s=>s.includes("ignored malformed")).length).toBe(2);
});

test("channel posts stay on their configured route and cannot select sessions",async()=>{
 const f=fixture();f.config.chats={"100":{route:"#announce@t"}};
 const r=new Router(f.deps);await r.load();
 for (const target of ["#b", "#foreign@other"]) {
  await r.incoming({...message(),from_id:undefined,channel_post:true,text:target+" do something"},"100");
  expect(f.sent.at(-1)).toEqual({route:{to:"#announce@t"},body:"[channel 100 (TG)] "+target+" do something"});
 }
});

test("a reply to an ordinary human message follows normal routing",async()=>{
 const f=fixture();f.config.route="#default@t";const r=new Router(f.deps);await r.load();
 await r.incoming({...message(),reply_to:999},"100");
 expect(f.sent.at(-1)?.route.to).toBe("#default@t");expect(f.told).toEqual([]);
});

test("a landed dispatch or post stays successful when route persistence fails",async()=>{
 const f=fixture();let fail=false;
 const r=new Router({...f.deps,set:async state=>{if(fail)throw Error("KV unavailable");await f.deps.set(state);}});await r.load();
 await r.tag("#beta@t",f.config);fail=true;
 await expect(r.incoming({...message(),text:"#b go"},"100")).resolves.toBeUndefined();
 expect(f.sent.length).toBe(1);
 await expect(r.posted("100",99,{to:"#beta@t",re:"answer"})).resolves.toBeUndefined();
 expect(r.mapped("100",99)).toBe(true);expect(f.logs.some(line=>line.includes("persist"))).toBe(true);
});

test("long routed answers split safely into at most eight chunks",async()=>{
 const f=fixture(),r=new Router(f.deps);await r.load();await r.incoming({...message(),text:"#b go"},"100");
 const route=JSON.parse(f.saved()!).answers["bus-1"];
 await r.answer({message_id:"reply",re:"bus-1",from:"#beta@t",topic:route.topic,tag:route.tag,body:"x".repeat(4095)+"😀"+"y".repeat(40000)});
 expect(f.told.length).toBe(8);expect(f.told.every(text=>text.length<=4096)).toBe(true);
 expect(f.told[0]!.endsWith("x")).toBe(true);expect(f.told[1]!.startsWith("😀")).toBe(true);
 expect(f.told[7]!.endsWith("… (truncated)")).toBe(true);
});

test("a final platform posting failure tells the session once on the same correlation",async()=>{
 const f=fixture(),r=new Router({...f.deps,tell:async()=>{throw Error("platform refused");}});await r.load();await r.incoming({...message(),text:"#b go"},"100");
 const route=JSON.parse(f.saved()!).answers["bus-1"];
 await expect(r.answer({message_id:"reply",re:"bus-1",from:"#beta@t",topic:route.topic,tag:route.tag,body:"answer"})).resolves.toBeUndefined();
 expect(f.sent.length).toBe(2);expect(f.sent[1]!.route).toMatchObject({to:"#beta@t",topic:route.topic,tag:route.tag});
 expect(f.sent[1]!.body).toContain("error: could not post");
});


test("drop discards listed and unknown Telegram messages without routing state", async () => {
 const f = fixture(); f.config.chats = {"-100": {handler: "drop"}};
 const r = new Router(f.deps); await r.load(); const saved = f.saved();
 for (const from_id of [42, 43]) expect(await r.incoming({...message(), from_id, text: "#b hello"}, "-100")).toBe(false);
 expect(f.sent).toEqual([]); expect(f.told).toEqual([]); expect(f.logs).toEqual([]);
 expect(r.refused()).toEqual([]); expect(f.saved()).toBe(saved);
 await r.incoming({...message(), text: "#b hello"}, "100"); expect(f.sent).toHaveLength(1);
 await r.incoming({...message(), from_id: 43}, "43"); expect(f.told).toEqual(["we do not know you"]);
});

test("Telegram handler resolves aliases and refuses unknown or combined options", () => {
 const result = parseConfig({aliases: {"#room": "-100"}, chats: {"#room": {handler: "drop"}}});
 expect(typeof result).toBe("object"); expect((result as Config).chats?.["-100"]).toEqual({handler: "drop"});
 for (const handler of ["other", true, null, 1, {}]) expect(parseConfig({chats: {"-100": {handler}}})).toBe("chats: -100.handler must be one of: drop");
 for (const key of ["route", "anyone", "tags"]) expect(parseConfig({chats: {"-100": {handler: "drop", [key]: key === "route" ? "#beta@t" : false}}})).toBe("chats: -100.handler cannot be combined with route, anyone or tags");
});

// L4: the routes are one KV value; past its budget the routes that expire first are evicted.
test("routes beyond the budget are evicted oldest first, the value stays under it, and the newest still answer",async()=>{
 const f=fixture(),r=new Router(f.deps);await r.load();
 const topic="t".repeat(1000),n=Math.ceil(stateBudget/1000)*2;
 for(let i=1;i<=n;i++)await r.posted("100",i,{to:"#alice@t",topic,tag:"x",re:"m"+i});
 const size=Buffer.byteLength(f.saved()!);
 expect(size).toBeLessThanOrEqual(stateBudget);expect(size).toBeGreaterThan(stateBudget/2);
 // The ones evicted are exactly the earliest: what is kept is the newest run, what is gone the oldest.
 const state=JSON.parse(f.saved()!),kept=Object.keys(state.messages).map(k=>Number(k.split(":")[1])).sort((a,b)=>a-b);
 expect(kept).toEqual(Array.from({length:kept.length},(_,i)=>n-kept.length+1+i));
 expect(Object.keys(state.gone).map(k=>Number(k.split(":")[1])).sort((a,b)=>a-b)).toEqual(Array.from({length:n-kept.length},(_,i)=>i+1));
 expect(f.logs.some(line=>line.includes("warning: evicted"))).toBe(true);
 await r.incoming({...message(),reply_to:n},"100");
 expect(f.sent.at(-1)?.route).toEqual({to:"#alice@t",topic,tag:"x",re:"m"+n});
 const sent=f.sent.length;
 await r.incoming({...message(),reply_to:1},"100");
 expect(f.sent.length).toBe(sent);expect(f.told.at(-1)).toBe(routeGone+"; use #tag");
 expect(f.logs.some(line=>line.includes("warning: "+routeGone))).toBe(true);
});

test("an evicted dispatch's session reply gets the named answer, never a chat post or silence",async()=>{
 const f=fixture(),r=new Router(f.deps);await r.load();
 await r.incoming({...message(),text:"#b go"},"100");
 const reply=JSON.parse(f.saved()!).answers["bus-1"];
 const topic="t".repeat(1000);
 for(let i=1;i<=Math.ceil(stateBudget/1000)*2;i++)await r.posted("100",i,{to:"#alice@t",topic,tag:"x",re:"m"+i});
 expect(JSON.parse(f.saved()!).answers?.["bus-1"]).toBeUndefined();
 await r.answer({message_id:"late",re:"bus-1",from:"#beta@t",topic:reply.topic,tag:reply.tag,body:"done"});
 expect(f.told).toEqual([]);
 expect(f.sent.at(-1)).toEqual({route:{to:"#beta@t",topic:reply.topic,tag:reply.tag},body:`error: ${routeGone}; your answer was not posted to the chat`});
 expect(f.logs.some(line=>line.includes(`warning: ${routeGone}: session reply late`))).toBe(true);
});

test("an expired route's allocated tag is freed; a pinned tag and a session with a live route keep theirs",async()=>{
 const f=fixture(),r=new Router(f.deps);await r.load();
 const now=Date.now;
 try {
  Date.now=()=>1;
  expect(await r.tag("#bob@t",f.config)).toBe("b1");expect(await r.tag("#carol@t",f.config)).toBe("c");expect(await r.tag("#beta@t",f.config)).toBe("b");
  await r.posted("100",1,{to:"#bob@t",re:"m1"});await r.posted("100",2,{to:"#beta@t",re:"m2"});await r.posted("100",3,{to:"#carol@t",re:"m3"});
  Date.now=()=>3*24*60*60*1000;
  await r.posted("100",4,{to:"#carol@t",re:"m4"});
  Date.now=()=>7*24*60*60*1000+2;
  await r.posted("100",5,{to:"#dave@t",re:"m5"});
  const state=JSON.parse(f.saved()!);
  expect(state.tags).toEqual({b:"#beta@t",c:"#carol@t"});
  expect(Object.keys(state.messages).sort()).toEqual(["100:4","100:5"]);
  expect(Object.keys(state.gone).sort()).toEqual(["100:1","100:2","100:3"]);
  expect(await r.tag("#bill@t",f.config)).toBe("b1");
 } finally {Date.now=now;}
});

test("past the gone-mark budget a chat keeps a watermark, so a reply to an unmarked evicted route is answered, never routed",async()=>{
 const f=fixture();f.config.route="#default@t";
 const now=Date.now(),messages:Record<string,unknown>={};
 for(let i=1;i<=4000;i++)messages["100:"+i]={to:"#alice@t",topic:"x".repeat(150),re:"m"+i,until:now+1000+i};
 messages["200:5"]={to:"#bob@t",re:"other",until:now+100000};
 const r=new Router({...f.deps,get:async()=>JSON.stringify({tags:{},messages})});await r.load();
 await r.posted("100",4001,{to:"#alice@t",re:"m4001"});
 const state=JSON.parse(f.saved()!),gone=Object.keys(state.gone).map(k=>Number(k.split(":")[1])).sort((a,b)=>a-b);
 expect(Buffer.byteLength(f.saved()!)).toBeLessThanOrEqual(stateBudget);
 expect(state.before["100"].id).toBe(String(gone[0]!-1));expect(state.before["200"]).toBeUndefined();
 expect(gone[0]).toBeGreaterThan(1000);
 for(const old of [1,gone[0]!-1]){
  await r.incoming({...message(),reply_to:old},"100");
  expect(f.sent).toEqual([]);expect(f.told.at(-1)).toBe(routeGone+"; use #tag");
 }
 await r.incoming({...message(),reply_to:4001},"100");expect(f.sent.at(-1)?.route.to).toBe("#alice@t");
 await r.incoming({...message(),reply_to:5},"200");expect(f.sent.at(-1)?.route.to).toBe("#bob@t");
 await r.incoming({...message(),reply_to:7},"300");expect(f.sent.at(-1)?.route.to).toBe("#default@t");
});

test("a flood of unknown senders evicts their notices, never a route",async()=>{
 const f=fixture(),now=Date.now(),notices:Record<string,number>={};
 for(let n=0;n<9000;n++)notices["x".repeat(40)+n]=now+3_600_000;
 const r=new Router({...f.deps,get:async()=>JSON.stringify({tags:{},messages:{},notices})});await r.load();
 await r.posted("100",1,{to:"#alice@t",re:"m1"});
 await r.incoming({...message(),from_id:"newcomer" as any},"100");
 const state=JSON.parse(f.saved()!);
 expect(Buffer.byteLength(f.saved()!)).toBeLessThanOrEqual(stateBudget);
 expect(Object.keys(state.messages)).toEqual(["100:1"]);
 expect(Object.keys(state.notices).length).toBeLessThan(9000);
});

test("message ids order as decimal numbers on every platform",()=>{
 expect(idOrder("9","10")).toBeLessThan(0);expect(idOrder("1740000000.000002","1740000000.000010")).toBeLessThan(0);
 expect(idOrder("1740000000.5","1740000000.50")).toBe(0);expect(idOrder("1234567890123456789","234567890123456789")).toBeGreaterThan(0);
 expect(idOrder("abc","1")).toBeUndefined();
});
