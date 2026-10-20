// Agent-owned descriptions, queried as the visitor. Never read /secret.
import type { Ctx, Rec } from "./ctx.ts";
import { h, Fragment } from "./jsx.ts";
import { Card, Facts } from "./ui/kit.tsx";

type Fields = Record<string, unknown>;
export type AgentHelp = { data?: Fields; text?: string; unavailable?: string };
const object = (v: unknown): v is Fields => !!v && typeof v === "object" && !Array.isArray(v);
const text = (v: unknown): string => typeof v === "string" ? v : "";
const entries = (v: unknown): [string, Fields][] => object(v) ? Object.entries(v).filter((row): row is [string, Fields] => object(row[1])) : [];
const strings = (v: unknown) => Array.isArray(v) ? v.filter((s): s is string => typeof s === "string") : [];
export const metadataWait = { ms: 2000 };
// Session-scoped: method visibility depends on the caller's delivered roles.
const cache = new Map<string, { until: number; answer: AgentHelp }>();
export function clearMetadataCache() { cache.clear(); }
export async function agentHelp(ctx: Ctx, rec: Rec): Promise<AgentHelp> {
  if (rec.kind !== "agent") return {};
  const key = JSON.stringify([ctx.session, rec.name, rec.at]);
  const cached = cache.get(key);
  if (cached && cached.until > Date.now()) return cached.answer;
  if (!rec.readers) return { unavailable: "description unavailable (offline)" };
  const until = Date.now() + metadataWait.ms;
  const timeout: AgentHelp = { unavailable: "description unavailable (timed out)" };
  const remember = (answer: AgentHelp) => {
    if (cache.size >= 256) cache.delete(cache.keys().next().value!);
    cache.set(key, { until: Date.now() + 60_000, answer });
    return answer;
  };
  try {
    const answer = await ctx.call(rec.name, "", Math.max(1, Math.floor(metadataWait.ms / 2)), "help-json").catch(() => ({kind: "timeout" as const}));
    if (Date.now() >= until) return timeout;
    if (answer.kind === "answer") {
      try {
        const data: unknown = JSON.parse(answer.body);
        if (object(data) && typeof data.description === "string" && object(data.methods)) return remember({ data });
      } catch {}
    }
    const fallback = await ctx.call(rec.name, "", Math.max(1, until - Date.now()), "help");
    if (fallback.kind === "timeout" || Date.now() >= until) return timeout;
    return fallback.kind === "answer" ? remember({ text: fallback.body }) : { unavailable: "description unavailable" };
  } catch { return Date.now() >= until ? timeout : { unavailable: "description unavailable" }; }
}

function typeName(v: unknown): string { return Array.isArray(v) ? strings(v).join(" | ") : text(v) || "string"; }
function Arguments({ value }: { value: unknown }) {
  return <>{(Array.isArray(value) ? value : []).filter(object).map(arg => <div><code>{text(arg.name)}{arg.rest === true ? "…" : ""}</code> ({typeName(arg.type)}{arg.optional === true ? ", optional" : ", required"}) — {text(arg.help)}</div>)}</>;
}
export function AgentDescription({ help }: { help: AgentHelp }) {
  const d = help.data;
  if (!d && help.unavailable) return <Card title="About this agent" icon="info"><p class="muted">{help.unavailable}</p></Card>;
  if (!d) return help.text ? <Card title="About this agent" icon="info"><pre>{help.text}</pre></Card> : null;
  const homepage = text(d.homepage);
  return <>
    <Card title="About this agent" icon="info"><p>{text(d.description)}</p>
      <Facts rows={[
        ["Version", text(d.version) || "—"], ["License", text(d.license) || "—"], ["Author", text(d.author) || "—"],
        ["Homepage", /^https?:\/\//i.test(homepage) ? <a href={homepage} rel="noreferrer">{homepage}</a> : homepage || "—"],
      ]} />
    </Card>
    <Card title="Methods" icon="terminal"><table><thead><tr><th>Method</th><th>Arguments</th><th>Type</th><th>Returns</th><th>Role</th><th>Destructive</th></tr></thead>
      <tbody>{entries(d.methods).map(([name, method]) => <tr><td><code>{name}</code><p>{text(method.help)}</p></td><td><Arguments value={method.args} /></td><td>{typeName(method.type)}</td><td>{text(method.returns) || "—"}</td><td>{Array.isArray(method.role) ? strings(method.role).join(" or ") : text(method.role) || "none"}</td><td>{method.destructive === true ? "yes" : "no"}</td></tr>)}</tbody></table>
    </Card>
  </>;
}
export function ConfigurationHelp({ help, value }: { help: AgentHelp; value?: string }) {
  if (!help.data) return null;
  const keys = entries(help.data.config);
  let unknown: string[] = [];
  try { const config: unknown = JSON.parse(value || "null"); if (object(config)) unknown = Object.keys(config).filter(k => !keys.some(([name]) => name === k)); } catch {}
  return <div class="metadata-config"><h3>Configuration keys</h3>
    {keys.length ? <dl>{keys.map(([name, field]) => <><dt><code>{name}</code> ({typeName(field.type)})</dt><dd>{text(field.help)}{field.example !== undefined ? <div>Example: <code>{JSON.stringify(field.example)}</code></div> : null}{field.default !== undefined ? <div>Default: <code>{JSON.stringify(field.default)}</code></div> : null}</dd></>)}</dl> : <p>No Configuration keys declared.</p>}
    {unknown.length ? <p class="warn">Undocumented Configuration keys: {unknown.join(", ")}. They are allowed.</p> : null}
  </div>;
}
export function SecretHelp({ help }: { help: AgentHelp }) {
  if (!help.data) return null;
  const declarations = (Array.isArray(help.data.secrets) ? help.data.secrets : []).filter(object);
  return <div class="metadata-secret"><h3>Expected Secret keys</h3><p>Names and descriptions only; stored values are never shown.</p>
    {declarations.length ? <dl>{declarations.map(key => <><dt><code>{text(key.name)}</code> ({key.required === true ? "required" : "optional"})</dt><dd>{text(key.help)}</dd></>)}</dl> : <p>No Secret keys declared.</p>}
  </div>;
}
