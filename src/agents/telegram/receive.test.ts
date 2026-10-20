import { expect, test } from "bun:test";
import { Receiver } from "./receive.ts";
import { Seen } from "./seen.ts";
import { telegramWeb } from "./telegramapi.ts";

const update = (id: number) => ({ update_id: id, message: { message_id: id, date: 1, text: `message${id}`, from: { id: 42, username: "fixture" }, chat: { id: -100, title: "fixture", type: "supergroup" } } });
const realSleep = Bun.sleep;
const realNow = Date.now;
async function until(done: () => boolean) {
  const end = realNow() + 2000;
  while (!done() && realNow() < end) await realSleep(1);
  expect(done()).toBe(true);
}
async function fast(run: (waits: number[]) => Promise<void>) {
  const waits: number[] = [];
  Bun.sleep = (async (ms: any) => { waits.push(Number(ms)); await realSleep(1); }) as typeof Bun.sleep;
  try { await run(waits); } finally { Bun.sleep = realSleep; }
}

test("a refused history write does not block the offset and stays dirty for retry", () => fast(async () => {
  const seen = new Seen(), durable = new Map<string, unknown>();
  const polls: number[] = []; let writes = 0, done = false, forwards = 0;
  const receiver = new Receiver(seen, async (_, __, params) => {
    const offset = Number(params.offset); polls.push(offset);
    if (offset > 10) { receiver.stop(); done = true; return { ok: true, result: [] }; }
    if (polls.length > 4) { receiver.stop(); done = true; throw Error("batch never became durable"); }
    return { ok: true, result: [update(10)] };
  }, async () => undefined, async (key, value) => {
    if (key === "seen:-100" && ++writes === 1) throw Error("fixture write refused");
    if (key === "offset") expect(durable.has("seen:-100")).toBe(false);
    durable.set(key, structuredClone(value));
  }, () => {});
  receiver.onMessage = () => { forwards++; };
  try {
    await receiver.load(); receiver.start(); await until(() => done); await realSleep(5);
    expect(polls.slice(0, 2)).toEqual([0, 11]);
    await seen.flush(async (key,value) => { if (key === "seen:-100") writes++; durable.set(key, structuredClone(value)); });
    expect(writes).toBe(2); expect(durable.get("offset")).toBe("11");
    expect((durable.get("seen:-100") as any[]).length).toBe(1); expect(forwards).toBe(1);
  } finally { receiver.stop(); }
}));

test("a restart retries one failed KV read, restores history and forwards a replay only once", () => fast(async () => {
  const durable = new Map<string, string>(); let forwards = 0, offsetWrites = 0;
  const write = async (key: string, value: unknown) => { durable.set(key, typeof value === "string" ? value : JSON.stringify(value)); };
  const first = new Receiver(new Seen(), async () => ({ ok: true, result: [update(20)] }), async key => durable.get(key), async (key, value) => {
    if (key === "offset") { if (++offsetWrites === 3) first.stop(); throw Error("offset unavailable"); }
    await write(key, value);
  }, () => {});
  first.onMessage = () => { forwards++; };
  await first.load(); first.start(); await until(() => offsetWrites === 3); await realSleep(5);
  const seen = new Seen(); let reads = 0, done = false;
  const second = new Receiver(seen, async () => ({ ok: true, result: [update(20)] }), async key => {
    if (++reads === 1) throw Error("one startup read failure"); return durable.get(key);
  }, async (key, value) => { await write(key, value); if (key === "offset") { second.stop(); done = true; } }, () => {});
  second.onMessage = () => { forwards++; };
  try {
    await second.load();
    expect(reads).toBeGreaterThan(2);
    expect(seen.read("-100", 200).map(m => m.text)).toEqual(["message20"]);
    expect(seen.chatList().length).toBe(1); expect(seen.userList().length).toBe(1);
    second.start(); await until(() => done); await realSleep(5);
    expect(seen.read("-100", 200).length).toBe(1); expect(forwards).toBe(1);
  } finally { first.stop(); second.stop(); }
}));

test("poll refusals back off and keep their description for whoami", () => fast(async waits => {
  for (const status of [400, 409]) {
    const start = waits.length;
    const receiver = new Receiver(new Seen(), async () => { receiver.stop(); return { ok: false, status, description: `fixture ${status}` }; }, async () => undefined, async () => {}, () => {});
    await receiver.load(); receiver.start(); await until(() => waits.length > start);
    expect(waits[start]).toBe(status === 409 ? 30_000 : 5000);
    expect(receiver.pollError()).toBe(`fixture ${status}`); await realSleep(5);
  }
}));

test("seen directories stay bounded at 500", () => {
  const seen = new Seen();
  for (let i = 0; i < 510; i++) seen.message(String(i), String(i), "supergroup", { message_id: i, date: 1, from_id: i, from: String(i) });
  expect(seen.chatList().length).toBe(500); expect(seen.userList().length).toBe(500);
});

test("only dirty keys are written per batch; a full flush also writes unchanged rings", async () => {
  const seen = new Seen(); const keys: string[] = [];
  const write = async (key: string) => { keys.push(key); };
  seen.message("a", "a", "supergroup", { message_id: 1, date: 1 });
  await seen.flush(write); expect(keys.sort()).toEqual(["chats", "seen:a"]);
  keys.length = 0;
  seen.message("a", "a", "supergroup", { message_id: 2, date: 1 });
  await seen.flush(write); expect(keys).toEqual(["seen:a"]);
  keys.length = 0;
  await seen.flush(write); expect(keys).toEqual([]);
  await seen.flush(write, true); expect(keys.sort()).toEqual(["chats", "seen:a", "users"]);
});

test("the receiver writes the full stored history after a minute of batches", () => fast(async () => {
  let now = realNow(), polls = 0, done = false;
  const oldNow = Date.now; Date.now = () => now;
  const durable = new Map<string, string>([
    ["chats", JSON.stringify([{ id: "old", type: "supergroup" }])],
    ["seen:old", JSON.stringify([{ message_id: 1, date: 1, text: "earlier" }])],
  ]);
  const keys: string[] = [];
  const receiver = new Receiver(new Seen(), async () => {
    if (++polls === 1) { now += 60_001; return { ok: true, result: [update(30)] }; }
    receiver.stop(); done = true; return { ok: true, result: [] };
  }, async key => durable.get(key), async (key, value) => {
    keys.push(key); durable.set(key, typeof value === "string" ? value : JSON.stringify(value));
  }, () => {});
  try {
    await receiver.load(); receiver.start(); await until(() => done); await realSleep(5);
    expect(keys).toContain("seen:old");
    expect(keys.filter(key => key === "seen:-100").length).toBe(2); // dirty batch, then whole state
  } finally { receiver.stop(); Date.now = oldNow; }
}));

test("corrupt cache keys start empty while healthy keys and transient retries survive", () => fast(async () => {
  for (const key of ["chats", "users", "seen:old"]) for (const corrupt of ["not JSON", '{"not":"an array"}']) {
    const durable = new Map<string,string>([
      ["chats", '[{"id":"old","type":"supergroup"}]'],
      ["users", '[{"id":"42","first_name":"Kept"}]'],
      ["seen:old", '[{"message_id":1,"date":1,"text":"kept"}]'],
      [key, corrupt],
    ]);
    const seen = new Seen(); const logs:string[]=[];let reads=0;
    const receiver=new Receiver(seen, async()=>({ok:true,result:[]}),async k=>{
      if (++reads===1) throw Error("transient fixture failure");return durable.get(k);
    },async()=>{},line=>logs.push(line));
    await receiver.load();
    expect(logs.some(line=>line.includes(`stored ${key} is not `))).toBe(true);
    expect(logs.some(line=>line.includes("attempt 1"))).toBe(true);
    expect(seen.chatList().length).toBe(key==="chats"?0:1);
    expect(seen.userList().length).toBe(key==="users"?0:1);
    expect(seen.read("old",10).length).toBe(key==="chats"||key==="seen:old"?0:1);
  }
}));


test("an idle long poll returns an empty answer without a poll error", async () => {
  const logs: string[] = []; let completed = false, emptyAnswers = 0;
  const api = Bun.serve({ hostname: "127.0.0.1", port: 0, async fetch(req) {
    const params = await req.json() as { timeout: number };
    // Telegram holds an idle request for its requested timeout. Include a
    // little response overhead so a matching HTTP timeout also fails.
    await realSleep(params.timeout * 1000 + 100);
    return Response.json({ ok: true, result: [] });
  } });
  const receiver = new Receiver(new Seen(), async (kind, method, params) => {
    try {
      const answer = await telegramWeb("fixture", `http://127.0.0.1:${api.port}`, kind, method, params);
      if (answer.ok && Array.isArray(answer.result) && answer.result.length === 0) emptyAnswers++;
      return answer;
    } finally { receiver.stop(); completed = true; }
  }, async () => undefined, async () => {}, line => logs.push(line));
  try {
    await receiver.load(); receiver.start();
    const end = realNow() + 42_000;
    while (!completed && realNow() < end) await realSleep(20);
    await realSleep(10); // allow the receive loop to record its outcome
    expect(completed).toBe(true);
    expect(emptyAnswers).toBe(1);
    expect(receiver.pollError()).toBe("");
    expect(logs).toEqual([]);
  } finally { receiver.stop(); api.stop(true); }
}, 45_000);

test("a pre-send failure retries before seen and offset, poison messages stop at three attempts", () => fast(async () => {
  for (const failures of [1, Infinity, 403]) {
    let attempts=0, delivered=0, done=false;const polls:number[]=[],logs:string[]=[],durable=new Map<string,unknown>(),seen=new Seen();
    const receiver=new Receiver(seen,async(_k,_m,p)=>{
      if(attempts>=4){receiver.stop();done=true;return {ok:true,result:[]};}
      polls.push(Number(p.offset));
      if(Number(p.offset)>10){receiver.stop();done=true;return {ok:true,result:[]};}
      return {ok:true,result:[update(10)]};
    },async()=>undefined,async(key,value)=>{durable.set(key,value);},line=>logs.push(line));
    receiver.onMessage=async()=>{attempts++;if(failures===403)throw Object.assign(Error("definitive refusal"),{status:403});if(attempts<=failures)throw Error("bus unavailable before send");delivered++;};
    try {
      await receiver.load();receiver.start();await until(()=>done);
      expect(durable.get("offset")).toBe("11");expect(seen.read("-100",200).length).toBe(1);
      expect(attempts).toBe(failures===1?2:failures===403?1:3);expect(delivered).toBe(failures===1?1:0);
      expect(polls.filter(n=>n===0).length).toBe(attempts);
      if(failures!==1)expect(logs.join()).toContain("message -100:10:");
    }finally{receiver.stop();await realSleep(5);}
  }
}));

test("seen history stays below its UTF-8 byte cap",()=>{
 const seen=new Seen();
 for(let n=0;n<200;n++)seen.message("a","A","supergroup",{message_id:n,date:1,from_id:42,from:"first",username:"person",text:"x".repeat(3000)});
 expect(Buffer.byteLength(JSON.stringify(seen.flushAll()["seen:a"]))).toBeLessThanOrEqual(65536);
});

test("sender ids and usernames survive a restart",()=>{
 const seen=new Seen();seen.message("a","A","supergroup",{message_id:1,date:1,from_id:42,from:"first",username:"person"});
 const restored=new Seen({kv:seen.flushAll()});
 restored.message("a","A","supergroup",{message_id:201,date:1,from_id:42,from:"first",username:"person"});
 expect(restored.userList()).toEqual([{id:"42",first_name:"first",username:"person"}]);
});
