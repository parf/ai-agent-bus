// A view-triggered ask as the visitor; never a background poll of the registry.
import { h, Fragment, render } from "./jsx.ts";
import type { Ctx } from "./ctx.ts";
import { html } from "./http.ts";
import { Pill } from "./ui/kit.tsx";

const tones: Record<string, string> = {ok:"ok", degraded:"warn", failed:"bad", starting:"neutral", stopping:"neutral", unknown:"muted"};
export function StatusAnswer({body}: {body: string}) {
  if (body.trim() === "unsupported: status") return null;
  const match = /^\s*([^\s:]+)(?::\s*|\s+|$)([\s\S]*)$/.exec(body);
  const word = match && Object.hasOwn(tones, match[1]) ? match[1] : "unknown";
  const detail = match && word === match[1] ? match[2] : body.trim();
  return <><Pill tone={tones[word]}>{word}</Pill>{detail ? <span class="agent-status-text"> · {detail}</span> : null}</>;
}
export async function agentStatus(ctx: Ctx): Promise<Response> {
  const rec = await ctx.lookup(ctx.q("name"));
  if (rec.kind !== "agent" || !(rec.readers || rec.reading)) return html("");
  try {
    const answer = await ctx.call(rec.name, "", 1800, "status");
    // JSX escapes the agent's text. Only this server-rendered fragment is sent.
    return html(answer.kind === "answer" ? render(StatusAnswer({body:answer.body})) : "");
  } catch { return html(""); }
}
