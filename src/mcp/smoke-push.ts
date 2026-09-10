// B.3 acceptance for the Claude push mode: the session stops polling and a
// message reaches it as notifications/claude/channel, with enough metadata to
// answer it. The peer here is the Bus class itself — no second process needed.
//
// The Codex mode cannot be smoked without a live Codex session; that one is
// the by-hand criterion (Plans/PoC/TODO.md: B).

import { Bus } from "./bus.ts";

const me = process.env.PUSH_NAME!;         // the pushed session
const peer = new Bus();                    // AGENT_BUS_NAME is the peer here
const peerName = peer.name;

const proc = Bun.spawn(["bun", "run", "server.ts"], {
  stdin: "pipe", stdout: "pipe", stderr: "pipe",
  env: { ...process.env, AGENT_BUS_NAME: me, AGENT_BUS_PUSH: "claude" },
  cwd: import.meta.dir,
});

// Drained from the start: reading it only in the failure path turns a hang
// into a permanent hang.
let stderr = "";
const draining = (async () => {
  const dec = new TextDecoder();
  for await (const chunk of proc.stderr) stderr += dec.decode(chunk, { stream: true });
})().catch(() => {});

let next = 1;
const pending = new Map<number, { ok: (v: any) => void; fail: (e: Error) => void }>();
const channelled: any[] = [];
let onChannel: (() => void) | undefined;

async function send(msg: unknown) {
  proc.stdin.write(JSON.stringify(msg) + "\n");
  await proc.stdin.flush();
}
function request(method: string, params?: unknown): Promise<any> {
  const id = next++;
  return new Promise<any>((resolve, reject) => {
    const timer = setTimeout(() => {
      pending.delete(id);
      reject(new Error(`timeout on ${method}`));
    }, 20_000);
    pending.set(id, {
      ok: (v) => { clearTimeout(timer); resolve(v); },
      fail: (e) => { clearTimeout(timer); reject(e); },
    });
    send({ jsonrpc: "2.0", id, method, ...(params ? { params } : {}) }).catch((e) => {
      clearTimeout(timer);
      pending.delete(id);
      reject(e);
    });
  });
}
(async () => {
  const dec = new TextDecoder();
  let buf = "";
  for await (const chunk of proc.stdout) {
    buf += dec.decode(chunk, { stream: true });
    let i;
    while ((i = buf.indexOf("\n")) >= 0) {
      const line = buf.slice(0, i).trim();
      buf = buf.slice(i + 1);
      if (!line) continue;
      const msg = JSON.parse(line);
      if (msg.method === "notifications/claude/channel") {
        channelled.push(msg.params);
        onChannel?.();
      } else if (msg.id !== undefined) {
        pending.get(msg.id)?.ok(msg);
        pending.delete(msg.id);
      }
    }
  }
  for (const [id, p] of pending) { p.fail(new Error("server exited")); pending.delete(id); }
})().catch(() => {});

// Waits for the nth notification (1-based), so a check can assert how many
// arrived — a delivery accidentally run twice was invisible without this.
function waitForChannel(n: number, ms: number): Promise<any | undefined> {
  if (channelled.length >= n) return Promise.resolve(channelled[n - 1]);
  return new Promise((resolve) => {
    const t = setTimeout(() => { onChannel = undefined; resolve(undefined); }, ms);
    onChannel = () => {
      if (channelled.length < n) return;
      clearTimeout(t);
      onChannel = undefined;
      resolve(channelled[n - 1]);
    };
  });
}

let pass = 0, fail = 0;
const check = (label: string, cond: boolean, detail = "") => {
  if (cond) { console.log(`  ok   ${label}`); pass++; }
  else { console.log(`  FAIL ${label} ${detail}`); fail++; }
};
const call = async (name: string, args: Record<string, unknown> = {}) => {
  const r = await request("tools/call", { name, arguments: args });
  return { text: r.result?.content?.[0]?.text ?? "", isError: !!r.result?.isError };
};

try {
  const init = await request("initialize", {
    protocolVersion: "2025-06-18", capabilities: {},
    clientInfo: { name: "agent-bus-push-smoke", version: "0.1.0" },
  });
  check("initialize", init.result?.serverInfo?.name === "agent-bus");
  await send({ jsonrpc: "2.0", method: "notifications/initialized" });

  const refused = await call("ab_consume", { wait: "1s" });
  check("an unfiltered ab_consume steps aside for push", refused.isError && refused.text.includes("push is on"), refused.text);

  await peer.register({ name: peerName, kind: "agent" });
  const sent = await peer.send({ to: me, body: "pushed hello", topic: "p1", tag: "q1" });

  const note = await waitForChannel(1, 10_000);
  check("the message arrived as notifications/claude/channel", !!note, "no notification in 10s");
  check("content carries the body and the sender", !!note?.content?.includes("pushed hello") && !!note?.content?.includes(peerName), String(note?.content).slice(0, 120));
  check("meta carries the id, topic and tag", note?.meta?.message_id === sent.message_id && note?.meta?.topic === "p1" && note?.meta?.tag === "q1", JSON.stringify(note?.meta));

  // A pushed message is one the session consumed, so it is replyable by id.
  const replied = await call("ab_reply", { message_id: sent.message_id, text: "pong from push" });
  check("ab_reply answers a pushed message", !replied.isError && replied.text.includes(peerName), replied.text);

  const back = await peer.consume({ wait: "5s" });
  check("the reply reaches the peer with the original topic and tag", back?.body === "pong from push" && back?.topic === "p1" && back?.tag === "q1", JSON.stringify(back));

  // Two distinct messages, in order, once each. Without the count a delivery
  // run twice was completely invisible.
  const second = await peer.send({ to: me, body: "second message", topic: "p2", tag: "q2" });
  const note2 = await waitForChannel(2, 10_000);
  check("a second message arrives as its own notification", note2?.meta?.message_id === second.message_id, JSON.stringify(note2?.meta));
  await Bun.sleep(1500); // long enough for a duplicate to show up
  check(
    "each message is pushed exactly once, in order",
    channelled.length === 2 &&
      channelled[0].meta.message_id === sent.message_id &&
      channelled[1].meta.message_id === second.message_id,
    `${channelled.length} notifications: ${channelled.map((c) => c.meta.message_id).join(", ")}`,
  );
} catch (e) {
  check("no exception", false, String(e));
} finally {
  proc.kill();
  await Promise.race([proc.exited, Bun.sleep(3000)]);
  await Promise.race([draining, Bun.sleep(1000)]);
  if (fail > 0 && stderr.trim()) console.log("stderr:", stderr.slice(0, 800));
}
console.log(`\npush: passed ${pass}, failed ${fail}`);
process.exit(fail === 0 ? 0 : 1);
