// Runtime events → the tab's state marker (docs/06-launchers.md#terminal-appearance).
// ❓ a question for the operator; 🟥 the last answer ended with the stop
// marker; both clear on new input or a new turn. There is no busy marker.
import { closeSync, fstatSync, openSync, readSync, realpathSync } from "node:fs";
import { sep } from "node:path";

export type TabState = "" | "ask" | "stop";
export const STOP_MARK = "🟥🟥🟥";
export const TAIL_BYTES = 64 * 1024;
type Event = Record<string, unknown>;

export const endsWithStop = (text: unknown) => typeof text === "string" && text.trimEnd().endsWith(STOP_MARK);

// The last assistant text in a Claude transcript, from its last TAIL_BYTES
// only, and only for a transcript under this account's projects/.
export function transcriptTail(path: unknown, projects: string, read = readTail): string | undefined {
  if (typeof path !== "string") return undefined;
  let real: string, root: string;
  try { real = realpathSync(path); root = realpathSync(projects); } catch { return undefined; }
  if (!real.startsWith(root + sep)) return undefined;
  const lines = read(real).split("\n").slice(1).reverse(); // the first line may be cut
  for (const line of lines) {
    try {
      const entry = JSON.parse(line) as { type?: string; message?: { content?: unknown } };
      if (entry.type !== "assistant") continue;
      const content = entry.message?.content;
      return typeof content === "string" ? content
        : Array.isArray(content) ? content.filter(c => c?.type === "text").map(c => c.text).join("") : "";
    } catch { /* a partial or foreign line */ }
  }
  return undefined;
}

export function readTail(path: string, bytes = TAIL_BYTES): string {
  const fd = openSync(path, "r");
  try {
    const size = fstatSync(fd).size, length = Math.min(size, bytes);
    const buf = Buffer.alloc(length);
    readSync(fd, buf, 0, length, size - length);
    return buf.toString("utf8");
  } finally { closeSync(fd); }
}

// A Claude hook payload; undefined leaves the state as it is.
export function claudeHook(event: Event, projects: string, read = readTail): TabState | undefined {
  switch (event.hook_event_name) {
    case "Notification":
      return event.notification_type === "permission_prompt" || event.notification_type === "elicitation_dialog" ? "ask" : undefined;
    case "Stop": {
      const text = typeof event.last_assistant_message === "string" ? event.last_assistant_message
        : transcriptTail(event.transcript_path, projects, read);
      return endsWithStop(text) ? "stop" : "";
    }
    case "UserPromptSubmit":
    case "PreToolUse":
      return "";
  }
  return undefined;
}

// Codex App Server notifications for the bound thread. The last agent message
// of a turn decides 🟥 when the turn completes.
export class CodexState {
  private last = "";
  event(method: string, params: Event): TabState | undefined {
    if (method === "turn/started") { this.last = ""; return ""; }
    if (method === "item/completed") {
      const item = params.item as { type?: string; text?: unknown } | undefined;
      if (item?.type === "agentMessage" && typeof item.text === "string") this.last = item.text;
      return undefined;
    }
    if (method === "turn/completed") return endsWithStop(this.last) ? "stop" : "";
    if (method === "thread/status/changed") {
      const status = params.status as { type?: string; activeFlags?: unknown } | undefined;
      const flags = Array.isArray(status?.activeFlags) ? status!.activeFlags : [];
      if (flags.includes("waitingOnApproval") || flags.includes("waitingOnUserInput")) return "ask";
      return status?.type === "active" ? "" : undefined;
    }
    return undefined;
  }
}

// OpenCode server events for the bound session: a permission request or a
// form asks; a reply, a cancel or a new execution clears. No turn-end event was
// observable without a model turn, so OpenCode has no 🟥.
export function opencodeEvent(event: Event, session: string): TabState | undefined {
  const data = (event.data ?? {}) as { sessionID?: unknown; form?: { sessionID?: unknown } };
  if ((data.sessionID ?? data.form?.sessionID) !== session) return undefined;
  switch (event.type) {
    case "permission.asked":
    case "form.created":
      return "ask";
    case "permission.replied":
    case "form.replied":
    case "form.cancelled":
    case "session.execution.started":
      return "";
  }
  return undefined;
}
