import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { Bus, defaultName } from "../mcp/bus.ts";
import { hash } from "./sessions.ts";

const root = resolve(import.meta.dir, "../../tmp/rename-smoke");
mkdirSync(root, { recursive: true });
const dir = mkdtempSync(join(root, "run-"));
const owner = new Bus();
const me = (await owner.status()).you;
const peerName = `rename-peer-${process.pid}@srv1`;
await owner.register({ name: peerName, kind: "generic", allow: ["*"] });
const peer = await owner.as(peerName);
const abort = new AbortController();
const serve = (async () => {
  while (!abort.signal.aborted) {
    const e = await peer.consume({ wait: "1s" }, abort.signal);
    if (e) await peer.send({ to: e.from, body: "launcher-answer", topic: e.topic, tag: e.tag });
  }
})().catch(e => { if (!abort.signal.aborted) throw e; });
let failures = 0;
const check = (label: string, pass: boolean) => { console.log(`${pass ? "ok" : "FAIL"} ${label}`); if (!pass) failures++; };
try {
  for (const runtime of ["codex", "opencode"]) {
    const cwd = join(dir, runtime);
    mkdirSync(cwd);
    const executable = join(dir, runtime + "-fixture");
    writeFileSync(executable, `#!/bin/sh\nexec '${process.execPath}' '${resolve(import.meta.dir, "../mcp/launcher-runtime-fixture.ts")}' "$@"\n`, { mode: 0o700 });
    const events = join(dir, runtime + ".events");
    const state = join(dir, "state");
    const id = `${runtime}-session`;
    const title = `Renamed ${runtime} ${process.pid}`;
    const env = { ...process.env, AGENT_BUS_NAME: "", TEST_RUNTIME: runtime, TEST_PEER: peerName,
      TEST_EVENTS: events, TEST_TITLE: `Before ${runtime} ${process.pid}`, TEST_RENAME: title,
      TEST_SESSION_ID: id, TEST_TITLE_FILE: join(dir, runtime + ".title"), XDG_STATE_HOME: state, CODEX_BIN: executable, OPENCODE_BIN: executable,
    };
    const launcher = resolve(process.env.LAUNCHER_BUILD || import.meta.dir, `ab-${runtime}`);
    const proc = Bun.spawn([launcher], { cwd, env, stdout: "pipe", stderr: "pipe" });
    const err = new Response(proc.stderr).text();
    const code = await proc.exited;
    const diagnostic = await err;
    const rows = readFileSync(events, "utf8").trim().split("\n").map(s => JSON.parse(s));
    const before = rows.find(r => r.kind === "identity").data.name;
    const after = rows.find(r => r.kind === "after-rename").data.name;
    const renamed = rows.find(r => r.kind === "renamed").data;
    if (code || renamed.isError) console.log(diagnostic, JSON.stringify(renamed));
    const expected = defaultName({ ...env, AGENT_BUS_RUNTIME: runtime, AGENT_BUS_CWD: title }, "/");
    check(`${runtime} concurrent rename claims one address`, rows.filter(r => r.kind === "renamed").length === 2 && rows.filter(r => r.kind === "renamed").every(r => !r.data.isError) && after === expected);
    check(`${runtime} reads the runtime title immediately on a no-argument repeat`, rows.some(r => r.kind === "rename-repeat" && !r.data.isError && JSON.stringify(r.data).includes(after)));
    check(`${runtime} refuses unauthenticated launcher control`, rows.some(r => r.kind === "control-denied" && r.data === 401));
    check(`${runtime} renames the actual runtime session`, rows.some(r => r.kind === "runtime-renamed" && r.data === title));
    check(`${runtime} rename updates the shared identity file`, after === expected && after !== before);
    check(`${runtime} receives replies on the renamed inbox`, after === expected && code === 0 && rows.some(r => r.kind === "rename-delivered" && r.data.includes("launcher-answer")));
    const records = await owner.ls();
    check(`${runtime} rename removes its idle old address`, !records.some(r => r.name === before));
    check(`${runtime} renamed address remains owned by the launching account`, after === expected && records.some(r => r.name === expected && r.owner === me));
    const binding = JSON.parse(readFileSync(join(state, "agent-bus/sessions", `${hash(`${runtime}:${id}`)}.json`), "utf8"));
    check(`${runtime} rename saves the resumed session binding`, binding.name === after && binding.base === expected && after !== before);
    for (const suffix of ["", ".delivered", ".tui-ready"]) rmSync(events + suffix, { force: true });
    const resumed = Bun.spawn([launcher], { cwd, env: { ...env, TEST_RENAME: "" }, stdout: "pipe", stderr: "pipe" });
    const resumedErr = new Response(resumed.stderr).text();
    const resumedCode = await resumed.exited;
    const diagnostic2 = await resumedErr;
    const resumedRows = readFileSync(events, "utf8").trim().split("\n").map(s => JSON.parse(s));
    if (resumedCode) console.log(diagnostic2);
    check(`${runtime} resumes the renamed address without creating a duplicate`, resumedCode === 0 && resumedRows.some(r => r.kind === "identity" && r.data.name === after && r.data.label === title));
  }
} finally {
  abort.abort(); await serve;
  rmSync(dir, { recursive: true, force: true });
}
process.exit(failures ? 1 : 0);
