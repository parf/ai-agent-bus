// B.1/B.2 acceptance: drive the MCP server over stdio exactly as a client
// does — initialize, tools/list, tools/call — and check the four tools work
// against a real daemon. Run by src/smoke.sh, which starts that daemon.
//
// The peer on the other side is the Bus class in this process, not a
// backgrounded shell: no start-order race, and the reply can be checked field
// by field. Asserting only that ab_reply *said* it replied passed happily
// with the routing mutated to nonsense.

import { Bus } from "./bus.ts";

import { Pending, drain, lines } from "./rpc.ts";
const proc = Bun.spawn(["bun", "run", "server.ts"], {
  stdin: "pipe", stdout: "pipe", stderr: "pipe",
  env: { ...process.env },
  cwd: import.meta.dir,
});

// stderr is drained from the start: a harness that only reads it in the
// failure path waits forever on a server that has not exited.
let stderr = "";
const draining = drain(proc.stderr, (s) => { stderr += s; }).catch(() => "");

const pending = new Pending();
async function send(msg: unknown) {
  proc.stdin.write(JSON.stringify(msg) + "\n");
  await proc.stdin.flush();
}
function request(method: string, params?: unknown): Promise<any> {
  return pending.request(
    (id) => send({ jsonrpc: "2.0", id, method, ...(params ? { params } : {}) }),
    20_000,
    () => new Error(`timeout on ${method}`),
  );
}
const reading = (async () => {
  for await (const line of lines(proc.stdout)) {
    const msg = JSON.parse(line);
    if (msg.id !== undefined) pending.settle(msg.id, null, msg);
  }
  // The server is gone; nothing in flight will be answered.
  pending.failAll("server exited");
})().catch(() => {});

let pass = 0, fail = 0;
const check = (label: string, cond: boolean, detail = "") => {
  if (cond) { console.log(`  ok   ${label}`); pass++; }
  else { console.log(`  FAIL ${label} ${detail}`); fail++; }
};
const call = async (name: string, args: Record<string, unknown> = {}) => {
  const r = await request("tools/call", { name, arguments: args });
  // A JSON-RPC error is not an empty successful result; saying so hid a
  // failing tool behind "".
  if (r.error) return { text: `jsonrpc error ${r.error.code}: ${r.error.message}`, isError: true };
  const body = r.result?.content?.[0]?.text;
  if (typeof body !== "string") return { text: `malformed result: ${JSON.stringify(r.result)}`, isError: true };
  return { text: body, isError: !!r.result?.isError };
};

try {
  const init = await request("initialize", {
    protocolVersion: "2025-06-18",
    capabilities: {},
    clientInfo: { name: "agent-bus-smoke", version: "0.1.0" },
  });
  check("initialize", init.result?.serverInfo?.name === "agent-bus", JSON.stringify(init).slice(0, 120));
  await send({ jsonrpc: "2.0", method: "notifications/initialized" });

  const list = await request("tools/list");
  const names = (list.result?.tools ?? []).map((t: any) => t.name).sort();
  check("four ab_ tools", JSON.stringify(names) === JSON.stringify(["ab_consume", "ab_ls", "ab_reply", "ab_send"]), names.join(","));

  const me = process.env.AGENT_BUS_NAME!;
  const ls = await call("ab_ls");
  check("registered itself on start", ls.text.includes(me), ls.text);

  const peerName = process.env.SMOKE_PEER!;
  const peer = new Bus({ ...process.env, AGENT_BUS_NAME: peerName });
  await peer.register({ name: peerName, kind: "agent" });

  const sent = await call("ab_send", { to: peerName, text: "ping from mcp", topic: "t1", tag: "g1" });
  check("ab_send", !sent.isError && sent.text.includes(peerName), sent.text);

  const atPeer = await peer.consume({ wait: "5s" });
  check(
    "the peer gets the body and routing it was sent",
    atPeer?.body === "ping from mcp" && atPeer?.from === me && atPeer?.topic === "t1" && atPeer?.tag === "g1",
    JSON.stringify(atPeer),
  );

  await peer.send({ to: me, body: "pong from the peer", topic: "t1", tag: "g1" });
  const got = await call("ab_consume", { wait: "5s" });
  check("ab_consume reads its own inbox", got.text.includes("from " + peerName), got.text.slice(0, 120));

  const id = got.text.match(/id ([0-9a-f]+)/)?.[1] ?? "";
  const replied = await call("ab_reply", { message_id: id, text: "answer from mcp" });
  check("ab_reply answers by id", !replied.isError && replied.text.includes(peerName), replied.text);

  // The claim above is only worth what the peer actually receives.
  const back = await peer.consume({ wait: "5s" });
  check(
    "the reply reaches the peer with the original sender, topic and tag",
    back?.body === "answer from mcp" && back?.from === me && back?.topic === "t1" && back?.tag === "g1",
    JSON.stringify(back),
  );

  const missing = await call("ab_send", { to: peer });
  check("ab_send without a body is refused, not sent empty", missing.isError, missing.text);

  const bogus = await call("ab_reply", { message_id: "deadbeef", text: "x" });
  check("ab_reply refuses an id it did not consume", bogus.isError, bogus.text);

  // A receipt is not an answer. The CLI has always known this; the face did
  // not, and handed the model an ack with an empty body while the answer
  // stayed queued (docs/04-messaging.md#receipts).
  await peer.send({ to: me, body: "", topic: "t-receipt", tag: "g1", receipt: "ack", re: "deadbeef" });
  await peer.send({ to: me, body: "the actual answer", topic: "t-receipt", tag: "g1" });
  const answered = await call("ab_consume", { topic: "t-receipt", tag: "g1", wait: "5s" });
  check("a filtered ab_consume returns the answer, not the ack", answered.text.includes("the actual answer"), answered.text.slice(0, 160));

  const badSend = await call("ab_send", { to: "ghost@nowhere", text: "x" });
  check("a send to nobody is an error, not a lie", badSend.isError && badSend.text.includes("404"), badSend.text);

  // A cancelled long poll must let go of the inbox. Without the abort signal
  // the wait ran on, took the next message and threw the answer away.
  const cancelled = pending.reserve();
  await send({ jsonrpc: "2.0", id: cancelled, method: "tools/call", params: { name: "ab_consume", arguments: { wait: "20s" } } });
  await Bun.sleep(400);
  await send({ jsonrpc: "2.0", method: "notifications/cancelled", params: { requestId: cancelled, reason: "smoke" } });
  await Bun.sleep(400);
  await peer.send({ to: me, body: "survives a cancelled read" });
  await Bun.sleep(600);
  const after = await call("ab_consume", { wait: "5s" });
  check("a cancelled ab_consume does not eat the next message", after.text.includes("survives a cancelled read"), after.text.slice(0, 120));
} catch (e) {
  check("no exception", false, String(e));
} finally {
  proc.kill();
  await Promise.race([proc.exited, Bun.sleep(3000)]);
  await Promise.race([draining, Bun.sleep(1000)]);
  if (fail > 0 && stderr.trim()) console.log("stderr:", stderr.slice(0, 800));
}
console.log(`\nmcp: passed ${pass}, failed ${fail}`);
process.exit(fail === 0 ? 0 : 1);
