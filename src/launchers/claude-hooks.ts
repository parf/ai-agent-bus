import { readFileSync } from "node:fs";

// The hook events that move the tab's state marker (state-events.ts).
export const HOOK_EVENTS = ["Notification", "Stop", "UserPromptSubmit", "PreToolUse"] as const;
export const HOOK_TIMEOUT_S = 1;

// Claude takes one --settings. The launcher's hooks join the user's own
// settings (fastMode and the rest kept as given); an unreadable value fails
// the start rather than dropping either. Each hook is an HTTP POST to the
// launcher's control endpoint: no process per tool call, a one-second
// timeout, and an answer that is always empty.
export function withStateHooks(args: string[], control: string, read = (path: string) => readFileSync(path, "utf8")): string[] {
  const rest: string[] = [];
  let user: Record<string, unknown> = {};
  for (let i = 0; i < args.length; i++) {
    const a = args[i]!;
    let value: string | undefined;
    if (a === "--settings") value = args[++i];
    else if (a.startsWith("--settings=")) value = a.slice("--settings=".length);
    else { rest.push(a); continue; }
    if (value === undefined) throw new Error("--settings needs a JSON object or a file");
    let parsed: unknown;
    try { parsed = JSON.parse(value.trimStart().startsWith("{") ? value : read(value)); }
    catch (e) { throw new Error(`--settings is not a JSON object or a readable JSON file: ${e instanceof Error ? e.message : e}`); }
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error("--settings must be a JSON object");
    user = merge(user, parsed as Record<string, unknown>);
  }
  const hook = {
    type: "http", url: `${control}/state`, timeout: HOOK_TIMEOUT_S,
    headers: { Authorization: "Bearer $AGENT_BUS_CONTROL_TOKEN" }, allowedEnvVars: ["AGENT_BUS_CONTROL_TOKEN"],
  };
  const ours = { hooks: Object.fromEntries(HOOK_EVENTS.map(e => [e, [{ hooks: [hook] }]])) };
  return [...rest, "--settings", JSON.stringify(merge(user, ours))];
}

// Later keys win, except hooks: their per-event lists are concatenated.
function merge(a: Record<string, unknown>, b: Record<string, unknown>): Record<string, unknown> {
  const out = { ...a, ...b };
  if (a.hooks && b.hooks && typeof a.hooks === "object" && typeof b.hooks === "object") {
    const hooks: Record<string, unknown[]> = {};
    for (const [event, list] of [...Object.entries(a.hooks), ...Object.entries(b.hooks)]) hooks[event] = [...(hooks[event] ?? []), ...(Array.isArray(list) ? list : [])];
    out.hooks = hooks;
  }
  return out;
}

// A proxy in the environment would carry the hooks off the machine; loopback goes direct.
export function loopbackNoProxy(env: NodeJS.ProcessEnv): Record<string, string> {
  if (!["HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"].some(k => env[k])) return {};
  const list = (env.NO_PROXY || env.no_proxy || "").split(",").map(s => s.trim()).filter(Boolean);
  const value = [...new Set([...list, "127.0.0.1", "localhost"])].join(",");
  return { NO_PROXY: value, no_proxy: value };
}
