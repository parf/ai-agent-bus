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

let next = 1;
const pending = new Map<number, (v: any) => void>();
const channelled: any[] = [];
let onChannel: (() => void) | undefined;

async function send(msg: unknown) {
  proc.stdin.write(JSON.stringify(msg) + "\n");
  await proc.stdin.flush();
}
function request(method: string, params?: unknown): Promise<any> {
  return new Promise(async (resolve, reject) => {
    const id = next++;
    pending.set(id, resolve);
    await send({ jsonrpc: "2.0", id, method, ...(params ? { params } : {}) });
    setTimeout(() => reject(new Error(`timeout on ${method}`)), 20_000);
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
      } else if (msg.id !== undefined && pending.has(msg.id)) {
        pending.get(msg.id)!(msg);
        pending.delete(msg.id);
      }
    }
  }
})();

function waitForChannel(ms: number): Promise<any | undefined> {
  if (channelled.length) return Promise.resolve(channelled[0]);
  return new Promise((resolve) => {
    const t = setTimeout(() => resolve(undefined), ms);
    onChannel = () => { clearTimeout(t); resolve(channelled[0]); };
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

  const note = await waitForChannel(10_000);
  check("the message arrived as notifications/claude/channel", !!note, "no notification in 10s");
  check("content carries the body and the sender", !!note?.content?.includes("pushed hello") && !!note?.content?.includes(peerName), String(note?.content).slice(0, 120));
  check("meta carries the id, topic and tag", note?.meta?.message_id === sent.message_id && note?.meta?.topic === "p1" && note?.meta?.tag === "q1", JSON.stringify(note?.meta));

  // A pushed message is one the session consumed, so it is replyable by id.
  const replied = await call("ab_reply", { message_id: sent.message_id, text: "pong from push" });
  check("ab_reply answers a pushed message", !replied.isError && replied.text.includes(peerName), replied.text);

  const back = await peer.consume({ wait: "5s" });
  check("the reply reaches the peer with the original topic and tag", back?.body === "pong from push" && back?.topic === "p1" && back?.tag === "q1", JSON.stringify(back));
} catch (e) {
  check("no exception", false, String(e));
  console.log("stderr:", (await new Response(proc.stderr).text()).slice(0, 800));
} finally {
  proc.kill();
}
console.log(`\npush: passed ${pass}, failed ${fail}`);
process.exit(fail === 0 ? 0 : 1);
