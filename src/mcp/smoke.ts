// B.1/B.2 acceptance: drive the MCP server over stdio exactly as a client
// does — initialize, tools/list, tools/call — and check the four tools work
// against a real daemon. Run by src/smoke.sh, which starts that daemon.

const proc = Bun.spawn(["bun", "run", "server.ts"], {
  stdin: "pipe", stdout: "pipe", stderr: "pipe",
  env: { ...process.env },
  cwd: import.meta.dir,
});

let next = 1;
const pending = new Map<number, (v: any) => void>();
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
      if (msg.id !== undefined && pending.has(msg.id)) {
        pending.get(msg.id)!(msg);
        pending.delete(msg.id);
      }
    }
  }
})();

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

  const peer = process.env.SMOKE_PEER!;
  const sent = await call("ab_send", { to: peer, text: "ping from mcp", topic: "t1", tag: "g1" });
  check("ab_send", !sent.isError && sent.text.startsWith("sent "), sent.text);

  const got = await call("ab_consume", { wait: "5s" });
  check("ab_consume reads its own inbox", got.text.includes("from " + peer), got.text.slice(0, 120));

  const id = got.text.match(/id ([0-9a-f]+)/)?.[1] ?? "";
  const replied = await call("ab_reply", { message_id: id, text: "pong from mcp" });
  check("ab_reply answers by id", !replied.isError && replied.text.includes(peer), replied.text);

  const bogus = await call("ab_reply", { message_id: "deadbeef", text: "x" });
  check("ab_reply refuses an id it did not consume", bogus.isError, bogus.text);

  const badSend = await call("ab_send", { to: "ghost@nowhere", text: "x" });
  check("a send to nobody is an error, not a lie", badSend.isError && badSend.text.includes("404"), badSend.text);
} catch (e) {
  check("no exception", false, String(e));
  console.log("stderr:", (await new Response(proc.stderr).text()).slice(0, 500));
} finally {
  proc.kill();
}
console.log(`\nmcp: passed ${pass}, failed ${fail}`);
process.exit(fail === 0 ? 0 : 1);
