import { expect, test } from "bun:test";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { claudeHook, CodexState, opencodeEvent, readTail, STOP_MARK, TAIL_BYTES, type TabState } from "./state-events.ts";
import { HOOK_EVENTS, HOOK_TIMEOUT_S, loopbackNoProxy, withStateHooks } from "./claude-hooks.ts";
import { serveControl } from "./control.ts";
import { terminalTitle } from "./terminal.ts";
import fixtures from "./state-events.fixtures.json";

test("the state marker sits between the glyph and the label", () => {
  expect(terminalTitle("claude", "CEO", 1, "ask")).toBe("✴️ ❓ CEO");
  expect(terminalTitle("opencode", "realty", 1, "stop")).toBe("👾 🟥 realty");
  expect(terminalTitle("codex", "W1B", 1, "")).toBe("❇️ W1B");
});

test("Claude hooks: recorded payloads, a question asks, the stop marker stops, input clears", () => {
  const [prompt, failure, end] = fixtures.claude as Record<string, unknown>[];
  expect(prompt!.hook_event_name).toBe("UserPromptSubmit");
  expect(claudeHook(prompt!, "/nowhere")).toBe("");
  // The recorded StopFailure carries last_assistant_message; a Stop has the same fields.
  const stop = { ...failure!, hook_event_name: "Stop" };
  expect(claudeHook(stop, "/nowhere")).toBe("");
  expect(claudeHook({ ...stop, last_assistant_message: `Waiting for you.\n${STOP_MARK}\n` }, "/nowhere")).toBe("stop");
  expect(claudeHook({ ...stop, last_assistant_message: `${STOP_MARK} then more` }, "/nowhere")).toBe("");
  expect(claudeHook(failure!, "/nowhere")).toBeUndefined();
  expect(claudeHook(end!, "/nowhere")).toBeUndefined();
  expect(claudeHook({ hook_event_name: "Notification", notification_type: "permission_prompt" }, "/nowhere")).toBe("ask");
  expect(claudeHook({ hook_event_name: "Notification", notification_type: "elicitation_dialog" }, "/nowhere")).toBe("ask");
  expect(claudeHook({ hook_event_name: "Notification", notification_type: "idle_prompt" }, "/nowhere")).toBeUndefined();
  expect(claudeHook({ hook_event_name: "PreToolUse" }, "/nowhere")).toBe("");
});

test("without last_assistant_message, 🟥 reads only the transcript's tail, and only under this account's projects/", () => {
  const root = resolve(import.meta.dir, "../../tmp/state-tests");
  mkdirSync(root, { recursive: true });
  const dir = mkdtempSync(join(root, "run-"));
  try {
    const projects = join(dir, "projects"), file = join(projects, "-work", "s.jsonl");
    mkdirSync(join(projects, "-work"), { recursive: true });
    const filler = JSON.stringify({ type: "user", message: { content: "x".repeat(1000) } }) + "\n";
    const last = JSON.stringify({ type: "assistant", message: { content: [{ type: "text", text: `Done.\n${STOP_MARK}` }] } }) + "\n";
    writeFileSync(file, filler.repeat(10_000) + last); // about 10 MB
    let bytes = 0;
    const counted = (p: string) => { const t = readTail(p); bytes += Buffer.byteLength(t); return t; };
    const stop = { hook_event_name: "Stop", transcript_path: file };
    expect(claudeHook(stop, projects, counted)).toBe("stop");
    expect(bytes).toBeGreaterThan(0);
    expect(bytes).toBeLessThanOrEqual(TAIL_BYTES);
    // A transcript outside the account's projects/ is never read.
    const outside = join(dir, "other.jsonl");
    writeFileSync(outside, last);
    bytes = 0;
    expect(claudeHook({ ...stop, transcript_path: outside }, projects, counted)).toBe("");
    expect(bytes).toBe(0);
    expect(claudeHook({ ...stop, transcript_path: join(projects, "..", "other.jsonl") }, projects, counted)).toBe("");
    expect(bytes).toBe(0);
  } finally { rmSync(dir, { recursive: true, force: true }); }
});

test("Codex: App Server notifications shaped by the installed 0.166 schema", () => {
  const codex = new CodexState();
  const seq: [string, Record<string, unknown>][] = [
    ["turn/started", { threadId: "t", turn: { id: "1" } }],
    ["thread/status/changed", { threadId: "t", status: { type: "active", activeFlags: ["waitingOnApproval"] } }],
    ["thread/status/changed", { threadId: "t", status: { type: "active", activeFlags: [] } }],
    ["thread/status/changed", { threadId: "t", status: { type: "active", activeFlags: ["waitingOnUserInput"] } }],
    ["item/completed", { threadId: "t", turnId: "1", item: { type: "agentMessage", id: "i", text: `Your call.\n${STOP_MARK}` } }],
    ["turn/completed", { threadId: "t", turn: { id: "1" } }],
    ["thread/status/changed", { threadId: "t", status: { type: "idle" } }],
    ["turn/started", { threadId: "t", turn: { id: "2" } }],
    ["item/completed", { threadId: "t", turnId: "2", item: { type: "agentMessage", id: "j", text: "plain" } }],
    ["turn/completed", { threadId: "t", turn: { id: "2" } }],
  ];
  expect(seq.map(([m, p]) => codex.event(m, p))).toEqual<(TabState | undefined)[]>(["", "ask", "", "ask", undefined, "stop", undefined, "", undefined, ""]);
});

test("OpenCode: recorded server events of the bound session ask and clear; another session's are ignored", () => {
  const events = fixtures.opencode as Record<string, unknown>[];
  const session = (events.find(e => e.type === "permission.asked")!.data as { sessionID: string }).sessionID;
  const states = events.map(e => [e.type, opencodeEvent(e, session)]).filter(([, s]) => s !== undefined);
  expect(states).toEqual([["permission.asked", "ask"], ["permission.replied", ""], ["permission.asked", "ask"], ["permission.replied", ""], ["form.created", "ask"], ["form.replied", ""]]);
  expect(events.every(e => opencodeEvent(e, "ses_other") === undefined)).toBe(true);
  expect(opencodeEvent({ type: "session.execution.started", data: { sessionID: session } }, session)).toBe("");
});

test("the hook settings keep the user's own, add one bounded HTTP hook per event, and refuse an unreadable value", () => {
  const control = "http://127.0.0.1:4242";
  const parse = (args: string[]) => {
    expect(args.filter(a => a === "--settings" || a.startsWith("--settings=")).length).toBe(1);
    return JSON.parse(args[args.indexOf("--settings") + 1]!);
  };
  const user = { fastMode: true, hooks: { Stop: [{ hooks: [{ type: "command", command: "mine" }] }] } };
  const args = withStateHooks(["--model", "m", "--settings", JSON.stringify(user), "a prompt"], control);
  expect(args.slice(0, 3)).toEqual(["--model", "m", "a prompt"]);
  const merged = parse(args);
  expect(merged.fastMode).toBe(true);
  expect(merged.hooks.Stop[0].hooks[0].command).toBe("mine");
  for (const event of HOOK_EVENTS) {
    const ours = merged.hooks[event].at(-1).hooks[0];
    expect(ours).toEqual({ type: "http", url: `${control}/state`, timeout: HOOK_TIMEOUT_S,
      headers: { Authorization: "Bearer $AGENT_BUS_CONTROL_TOKEN" }, allowedEnvVars: ["AGENT_BUS_CONTROL_TOKEN"] });
    expect(ours.timeout).toBeLessThanOrEqual(1);
  }
  expect(parse(withStateHooks(["--settings=/some/file.json"], control, () => '{"fastMode":true}')).fastMode).toBe(true);
  expect(() => withStateHooks(["--settings", "{not json"], control)).toThrow("--settings");
  expect(() => withStateHooks(["--settings", "/missing/file.json"], control, () => { throw new Error("ENOENT"); })).toThrow("--settings");
  expect(() => withStateHooks(["--settings", "[1]"], control)).toThrow("JSON object");
  expect(parse(withStateHooks([], control)).hooks.Notification).toHaveLength(1);
  expect(loopbackNoProxy({})).toEqual({});
  expect(loopbackNoProxy({ HTTPS_PROXY: "http://p:3128", NO_PROXY: "corp" }).NO_PROXY).toBe("corp,127.0.0.1,localhost");
});

test("/state needs the bearer token, always answers 204 and empty, and reaches neither the label, the rename, the binding nor the bus name", async () => {
  const bus = { name: "#claude/home@t", binding: "home" };
  const calls = { rename: 0, fork: 0, label: 0 };
  const seen: Record<string, unknown>[] = [];
  const control = serveControl(async () => { calls.rename++; bus.name = "#claude/moved@t"; bus.binding = "moved"; return "renamed"; },
    async () => { calls.fork++; return "#claude/fork@t"; },
    () => { calls.label++; return { current: "x", recommended: "x" }; },
    event => { seen.push(event); if (event.explode) throw new Error("boom"); });
  try {
    const at = (path: string, body: string, auth = true) => fetch(`${control.env.AGENT_BUS_CONTROL_ADDR}${path}`, {
      method: "POST", body, headers: auth ? { authorization: `Bearer ${control.env.AGENT_BUS_CONTROL_TOKEN}` } : {},
    });
    expect((await at("/state", JSON.stringify({ hook_event_name: "Stop" }), false)).status).toBe(401);
    expect(seen).toEqual([]);
    for (const body of [JSON.stringify({ hook_event_name: "Stop", label: "x", name: "#claude/other@t" }), "{not json", "[1]", JSON.stringify({ explode: true })]) {
      const response = await at("/state", body);
      expect(response.status).toBe(204);
      expect(await response.text()).toBe("");
    }
    expect(seen.length).toBe(2);
    // A Stop hook carries the whole last answer; the other endpoints keep their 4 KiB bound.
    const big = JSON.stringify({ hook_event_name: "Stop", last_assistant_message: "a".repeat(200_000) });
    expect((await at("/state", big)).status).toBe(204);
    expect(seen.length).toBe(3);
    expect((await at("/label", JSON.stringify({ label: "a".repeat(5000) }))).status).toBe(400);
    expect(calls).toEqual({ rename: 0, fork: 0, label: 0 });
    expect(bus).toEqual({ name: "#claude/home@t", binding: "home" });
  } finally { control.stop(); }
});
