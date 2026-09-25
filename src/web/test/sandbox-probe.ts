// Run by src/web/probe-unit.sh inside the web unit's own sandbox; each line says whether a wall held.
import { readdirSync, writeFileSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
const out: string[] = [];
const expectFail = (label: string, f: () => unknown) => { try { f(); out.push(`FAIL ${label}: allowed`); } catch (e: any) { out.push(`ok   ${label}: ${e.code ?? e.message}`); } };
expectFail("read daemon state", () => readdirSync("/var/lib/agent-bus/daemon"));
expectFail("read runner state", () => readdirSync("/var/lib/agent-bus/runner"));
expectFail("read .git", () => readdirSync("/usr/local/src/ai-agent-bus/.git"));
expectFail("write the checkout", () => writeFileSync("/usr/local/src/ai-agent-bus/src/web/probe.local", "x"));
expectFail("read /etc/ssh", () => readdirSync("/etc/ssh"));
expectFail("read /root", () => readdirSync("/root"));
const ex = spawnSync("/usr/local/lib/agent-bus/current/agent-busd", ["--version"]);
out.push(ex.error || ex.status !== 0 ? `ok   exec agent-busd: ${(ex.error as any)?.code ?? ex.status}` : "FAIL exec agent-busd: ran");
const ex2 = spawnSync("/usr/lib/systemd/systemd-executor", ["--version"]);
out.push(ex2.error || ex2.status !== 0 ? `ok   exec a program under /usr/lib: ${(ex2.error as any)?.code ?? ex2.status}` : "FAIL exec a program under /usr/lib: ran");
const sh = spawnSync("/bin/sh", ["-c", "echo hi"]);
out.push(sh.error || sh.status !== 0 ? `ok   exec /bin/sh: ${(sh.error as any)?.code ?? sh.status}` : "FAIL exec /bin/sh: ran");
try { await fetch("http://1.1.1.1/", { signal: AbortSignal.timeout(3000) }); out.push("FAIL connect out: allowed"); } catch (e: any) { out.push(`ok   connect out: ${e.name}`); }
try { const r = await fetch("http://unix/identity", { unix: "/run/agent-bus/bus.sock" } as any); out.push(r.ok ? "ok   shared socket answers" : `FAIL shared socket ${r.status}`); } catch (e: any) { out.push(`FAIL shared socket: ${e.message}`); }
out.push(`uid ${process.getuid?.()} caps ${readFileSync("/proc/self/status", "utf8").match(/CapEff:\s*(\S+)/)?.[1]}`);
console.log(out.join("\n"));
