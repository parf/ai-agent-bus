// B.1/B.2 acceptance: drive the MCP server over stdio exactly as a client
// does — initialize, tools/list, tools/call — and check the five tools work
// against a real daemon. Run by src/smoke.sh, which starts that daemon.
//
// The peer on the other side is the Bus class in this process, not a
// backgrounded shell: no start-order race, and the reply can be checked field
// by field. Asserting only that ab_reply *said* it replied passed happily
// with the routing mutated to nonsense.

import { Bus } from "./bus.ts";
import { version } from "./version.ts";

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
    clientInfo: { name: "agent-bus-smoke", version },
  });
  check("initialize", init.result?.serverInfo?.name === "agent-bus", JSON.stringify(init).slice(0, 120));
  check("MCP advertises the shared version", init.result?.serverInfo?.version === version, JSON.stringify(init.result?.serverInfo));
  await send({ jsonrpc: "2.0", method: "notifications/initialized" });

  const list = await request("tools/list");
  const names = (list.result?.tools ?? []).map((t: any) => t.name).sort();
  const expected = ["ab_consume", "ab_ls", "ab_receipt", "ab_rename", "ab_reply", "ab_send"];
  check("exactly the ab_ tools", JSON.stringify(names) === JSON.stringify(expected), names.join(","));

  const me = process.env.AGENT_BUS_NAME!;
  const ls = await call("ab_ls");
  check("registered itself on start", ls.text.includes(me), ls.text);

  const peerName = process.env.SMOKE_PEER!;
  // The harness mints the credentials it needs as the daemon's owner: a name
  // is bound to its token now, so a second principal is a second token.
  const owner = new Bus({
    ...process.env,
    AGENT_BUS_NAME: process.env.AGENT_BUS_OWNER,
    AGENT_BUS_TOKEN: process.env.AGENT_BUS_OWNER_TOKEN,
  });
  const peer = await owner.as(peerName);
  await peer.register({ name: peerName, kind: "agent" });

  // The catalog is per caller, because the daemon is what filters and the
  // face only asks. Two principals, one registry, different answers — and
  // each one matching what that principal may actually call.
  // See docs/05-discovery.md#audience.
  const hidden = "peers-only@srv1";
  await peer.register({ name: hidden, kind: "generic", descr: "for the peer alone", allow: [peerName] });
  const mine = await call("ab_ls");
  check("the catalog leaves out what its caller may not use", !mine.text.includes(hidden), mine.text);
  const theirs = await peer.ls();
  check("while the principal it is for sees it", theirs.some((r) => r.name === hidden), JSON.stringify(theirs.map((r) => r.name)));
  const refused = await call("ab_send", { to: hidden, text: "not for me" });
  check("and the catalog matches what a send would do", refused.isError, refused.text);

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

  // A face that can only answer cannot say "finished, nothing to send back".
  // Checking what the tool *said* is not enough: the peer is what proves the
  // receipt was routed and carried the right kind.
  await peer.send({ to: me, body: "work with no answer", topic: "t-done", tag: "g3" });
  const chore = await call("ab_consume", { topic: "t-done", tag: "g3", wait: "5s" });
  const choreId = chore.text.match(/id ([0-9a-f]+)/)?.[1] ?? "";
  const said = await call("ab_receipt", { message_id: choreId, kind: "done" });
  check("ab_receipt reports the done it sent", !said.isError && said.text.includes("done"), said.text);
  const receipt = await peer.consume({ topic: "t-done", tag: "g3", wait: "5s" });
  check(
    "the peer gets a done, not an answer",
    receipt?.receipt === "done" && receipt?.re === choreId && receipt?.from === me,
    JSON.stringify(receipt),
  );
  // The face is a client like any other: when a request names a third party,
  // ab_reply must answer there and not to whoever asked.
  {
    const third = `${peerName.split("@")[0]}.third@${peerName.split("@")[1]}`;
    const bystander = await owner.as(third);
    await bystander.register({ name: third, kind: "agent" });
    await peer.send({ to: me, body: "answer elsewhere", topic: "t-rt", tag: "g5", reply_to: { service: third, topic: "t-rt", tag: "g5" } } as any);
    const chore = await call("ab_consume", { topic: "t-rt", tag: "g5", wait: "5s" });
    const rtId = chore.text.match(/id ([0-9a-f]+)/)?.[1] ?? "";
    await call("ab_reply", { message_id: rtId, text: "sent to the third party" });
    const atThird = await bystander.consume({ topic: "t-rt", tag: "g5", wait: "5s" });
    check("ab_reply answers the third party the request named", atThird?.body === "sent to the third party", JSON.stringify(atThird));
    const atAsker = await peer.consume({ topic: "t-rt", tag: "g5", wait: "1s" });
    check("and not the asker", atAsker === null, JSON.stringify(atAsker));
  }

  // Same for the face: a filtered wait must end on the done, not run out its
  // deadline. The tool description promises exactly this.
  {
    setTimeout(() => {
      void peer.send({ to: me, body: "", topic: "t-fin", tag: "g4", receipt: "done", re: "whatever" } as any);
    }, 400);
    const t0 = Date.now();
    const fin = await call("ab_consume", { topic: "t-fin", tag: "g4", wait: "8s" });
    const took = Date.now() - t0;
    check("a filtered wait ends on done, not at the deadline", took < 4000, `waited ${took} ms of 8s`);
    check(
      "and does not tell the model an answer is still coming",
      fin.text.includes("sent no answer") && !fin.text.includes("still to come"),
      fin.text.slice(0, 200),
    );
  }

  // A refusal is never reported as silence: the daemon owns the closed set,
  // and the face's job is to hand its words to the model rather than swallow
  // them into a cheerful success.
  const notAReceipt = await call("ab_receipt", { message_id: choreId, kind: "maybe" });
  check(
    "a refused receipt reaches the model in the daemon's words",
    // the daemon's words arrive JSON-escaped, so match the sentence, not the quoting
    notAReceipt.isError && notAReceipt.text.includes("a receipt is") && notAReceipt.text.includes("maybe"),
    notAReceipt.text,
  );

  // A receipt is not an answer. The CLI has always known this; the face did
  // not, and handed the model an ack with an empty body while the answer
  // stayed queued (docs/04-messaging.md#receipts).
  await peer.send({ to: me, body: "", topic: "t-receipt", tag: "g1", receipt: "ack", re: "deadbeef" });
  await peer.send({ to: me, body: "the actual answer", topic: "t-receipt", tag: "g1" });
  const answered = await call("ab_consume", { topic: "t-receipt", tag: "g1", wait: "5s" });
  check("a filtered ab_consume returns the answer, not the ack", answered.text.includes("the actual answer"), answered.text.slice(0, 160));

  // Asking again after a receipt must use what is LEFT of the deadline. It
  // asked for the whole wait again, so a caller who said 2s waited 3.5s;
  // the margin here is wide enough not to be a timing test.
  {
    setTimeout(() => { void peer.send({ to: me, body: "", topic: "t-late", tag: "g2", receipt: "ack", re: "x" } as any); }, 800);
    const t0 = Date.now();
    await call("ab_consume", { topic: "t-late", tag: "g2", wait: "2s" });
    const took = Date.now() - t0;
    check("a receipt does not extend the caller's deadline", took < 2600, `asked 2s, returned after ${took} ms`);
  }

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

  // ab_rename. Address changes are register-then-unregister, so the failure
  // that matters is the one where the old address still holds something.
  {
    // Nothing to rename from: no session file here, so the face must say how
    // to get a title rather than invent one or fail silently.
    const nothing = await call("ab_rename");
    check("ab_rename with nothing to rename says how to get a title",
      nothing.isError && nothing.text.includes("/rename"), nothing.text.slice(0, 160));

    // An unread message at the old address. Releasing it now would lose that
    // message, so the rename must keep it and say so.
    await peer.send({ to: me, body: "left at the old address" });
    const renamed = await call("ab_rename", { name: "Smoke renamed session" });
    check("ab_rename registers the new address",
      !renamed.isError && renamed.text.includes("smoke-renamed-session"), renamed.text.slice(0, 200));
    check("ab_rename keeps a busy old address instead of losing its queue",
      renamed.text.includes(`kept ${me}`), renamed.text.slice(0, 200));

    const moved = await call("ab_ls");
    check("the new address is registered", moved.text.includes("smoke-renamed-session"), moved.text.slice(0, 200));
    check("and the busy old one is still there", moved.text.includes(me), moved.text.slice(0, 200));

    // The message the old address was holding is still readable there.
    const old = await owner.as(me);
    check("the queued message survived the rename",
      (await old.consume({ wait: "2s" }))?.body === "left at the old address", "old inbox");

    // Renaming to what it already is must not unregister the session out from
    // under itself; it is a no-op with instructions.
    const again = await call("ab_rename", { name: "Smoke renamed session" });
    check("renaming to the current address is refused, not re-registered",
      again.isError && again.text.includes("already registered"), again.text.slice(0, 160));

    // Sending still works from the new address, and arrives from it.
    await call("ab_send", { to: peerName, text: "after the rename", topic: "t-rn", tag: "g9" });
    const fromNew = await peer.consume({ topic: "t-rn", tag: "g9", wait: "5s" });
    check("the session sends from its new address",
      fromNew?.body === "after the rename" && fromNew?.from.includes("smoke-renamed-session"),
      JSON.stringify({ from: fromNew?.from, body: fromNew?.body }));
  }
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
