// Session metadata belongs to the transport, not to the live model's turn.
import type { Bus, Envelope, Record_ } from "./bus.ts";
import { dispatchAsk } from "../agents/shared/asks.ts";
import { version } from "./version.ts";

export async function answerSessionAsk(bus: Bus, e: Envelope, env: NodeJS.ProcessEnv = process.env): Promise<boolean> {
  if (!e.ask || e.receipt || e.to !== bus.name) return false;
  let help = "";
  if (e.ask === "help" || e.ask === "help-json") {
    const record = await bus.get(`/lookup?name=${encodeURIComponent(bus.name)}`, AbortSignal.timeout(250)).catch(() => null) as Record_ | null;
    const tags = record?.tags ?? [];
    const runtime = env.AGENT_BUS_RUNTIME || tags.find(tag => ["claude", "codex", "opencode"].includes(tag)) || "unknown";
    const cwd = env.AGENT_BUS_CWD || process.cwd();
    const purpose = record?.descr ?? env.AGENT_BUS_DESCR ?? "";
    const description = ["AI CLI session; talk to it in plain text", purpose].filter(Boolean).join("\n");
    help = e.ask === "help-json"
      ? JSON.stringify({ description, version, tags, runtime, cwd, body_formats: ["text"], methods: {}, config: {}, secrets: [], asks: ["help", "help-config", "help-secret", "help-json", "mcp"] })
      : `${description}\nRuntime: ${runtime}\nCWD: ${cwd}\nTags: ${tags.join(", ") || "none"}`;
  }
  const body = dispatchAsk(e.ask, () => help, {"help-config":()=>"none","help-secret":()=>"none"})!;
  const target = e.reply_to ?? { name: e.from, topic: e.topic, tag: e.tag };
  await bus.send({ to: target.name, topic: target.topic, tag: target.tag, re: e.message_id, body });
  return true;
}
