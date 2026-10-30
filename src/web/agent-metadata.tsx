import { Json } from "./ui/json.tsx";
// Agent-owned descriptions, queried as the visitor. Never read /secret.
import type { Ctx, Rec } from "./ctx.ts";
import { h, Fragment } from "./jsx.ts";
import { Card, Facts, Icon } from "./ui/kit.tsx";

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
    const answer = await ctx.call(rec.name, "", Math.max(1, until - Date.now()), "help-json").catch(() => ({kind: "timeout" as const}));
    if (Date.now() >= until || answer.kind === "timeout") return timeout;
    if (answer.kind !== "answer") return { unavailable: "description unavailable" };
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
const argsOf = (method: Fields) => (Array.isArray(method.args) ? method.args : []).filter(object);
const rolesOf = (method: Fields) => Array.isArray(method.role) ? strings(method.role) : text(method.role) ? [text(method.role)] : [];
const quote = (value: string) => "'" + value.replaceAll("'", "'\\''") + "'";
function signature(name: string, method: Fields): string {
  return [name, ...argsOf(method).filter(arg => arg.json_only !== true).map(arg => {
    const label = text(arg.name) + (arg.rest === true ? "…" : "");
    return arg.optional === true ? `[${label}]` : `<${label}>`;
  })].join(" ");
}
function sample(arg: Fields): unknown {
  if (Object.hasOwn(arg, "default")) return arg.default;
  if (Array.isArray(arg.type)) return strings(arg.type)[0] ?? `<${text(arg.name)}>`;
  switch (arg.type) {
    case "int": return 1;
    case "bool": return true;
    case "list": return [`<${text(arg.name)}>`];
    case "json": return {};
    default: return `<${text(arg.name)}>`;
  }
}
export function methodExamples(name: string, method: Fields, agent: string, forms: string[]) {
  const args = argsOf(method);
  const firstOptional = args.find(a => a.optional === true && a.json_only !== true);
  const body = text(method.text) || [name.replaceAll(".", " "), ...args.filter(a => a.json_only !== true && (a.optional !== true || a === firstOptional)).map(a => String(sample(a)))].join(" ");
  const cli = forms.includes("text") ? text(method.example) || `agent-bus call ${quote(agent)} ${quote(body)}` : undefined;
  const json = forms.includes("json") ? JSON.stringify(Object.fromEntries([["method", name], ...args.filter(a => a.optional !== true || a.json_only === true || a === firstOptional).map(a => [text(a.name), sample(a)])]), null, 2) : undefined;
  return { cli, json };
}
function CopyExample({ label, value }: { label: string; value: string }) {
  return <div class="method-example"><div class="method-example-head"><strong>{label}</strong><button type="button" class="copy-btn" data-copy={value} aria-label={`Copy ${label}`}><Icon name="copy" /></button></div><pre><code><Json value={value} /></code></pre></div>;
}
function Arguments({ value }: { value: Fields[] }) {
  return <dl class="method-arguments">{value.map(arg => <div><dt><code>{text(arg.name)}</code> <span class="muted">{Array.isArray(arg.type) ? "enum" : typeName(arg.type)} · {arg.optional === true ? "optional" : "required"}{arg.json_only === true ? " · JSON only" : ""}</span></dt><dd>{text(arg.help)}{Array.isArray(arg.type) ? <span class="enum-chips">{strings(arg.type).map(choice => <code class="pill">{choice}</code>)}</span> : null}</dd></div>)}</dl>;
}
export function AgentDescription({ help, name = "#agent@node.example", filter = "", path = "/agent", returnTo = "" }: { help: AgentHelp; name?: string; filter?: string; path?: string; returnTo?: string }) {
  const d = help.data;
  if (!d && help.unavailable) return <Card title="About this agent" icon="info"><p class="muted">{help.unavailable}</p></Card>;
  if (!d) return help.text ? <Card title="About this agent" icon="info"><pre><Json value={help.text} /></pre></Card> : null;
  const homepage = text(d.homepage), docs = text(d.docs), methods = entries(d.methods).filter(([method]) => method !== "help");
  const needle = filter.trim().toLowerCase();
  const shown = methods.filter(([method, value]) => `${method} ${text(value.help)}`.toLowerCase().includes(needle));
  const forms = Array.isArray(d.body_formats) ? strings(d.body_formats) : ["text"];
  const group = (method: Fields) => method.destructive === true ? "destructive" : rolesOf(method).length ? "act" : "read";
  return <>
    <Card title="About this agent" icon="info"><p class="agent-description">{text(d.description)}</p>
      <dl class="agent-about-facts">
        <div><dt>Version</dt><dd class="no-wrap">{text(d.version) || "—"}</dd></div>
        <div><dt>License</dt><dd class="no-wrap">{text(d.license) || "—"}</dd></div>
        <div><dt>Author</dt><dd class="no-wrap">{text(d.author) || "—"}</dd></div>
        <div><dt>Homepage</dt><dd class="no-wrap">{/^https?:\/\//i.test(homepage) ? <a href={homepage} rel="noreferrer">{homepage}</a> : homepage || "—"}</dd></div>
        {/^https:\/\//i.test(docs) ? <div><dt>Docs</dt><dd class="no-wrap"><a href={docs} rel="noreferrer">{docs.replace(/^.*\/blob\/[^/]+\//, "")}</a></dd></div> : null}
      </dl>
    </Card>
    <details class="card agent-methods" id="methods"><summary class="card-head"><h2><Icon name="terminal" />Methods ({methods.length})</h2></summary><div class="card-body">
      <form method="get" action={`${path}#methods`} class="method-filter"><input type="hidden" name="name" value={name} />{returnTo ? <input type="hidden" name="return" value={returnTo} /> : null}<label for="method-filter">Find a method</label><input id="method-filter" name="m" value={filter} type="search" placeholder="Name or description" /><button class="btn btn-sm" type="submit">Filter</button>{needle ? <a href={`${path}?name=${encodeURIComponent(name)}${returnTo ? `&return=${encodeURIComponent(returnTo)}` : ""}#methods`}>Clear</a> : null}</form>
      {needle ? <p class="muted small">{shown.length} of {methods.length} methods match.</p> : null}
      {!shown.length ? <p class="muted">{methods.length ? "No methods match this filter." : "No methods are available to you."}</p> : null}
      {(["read", "act", "destructive"] as const).map(kind => {
        const rows = shown.filter(([, method]) => group(method) === kind);
        return rows.length ? <div class="method-group"><h3>{kind === "read" ? "Read" : kind === "act" ? "Act" : "Destructive"} <span class="muted small">({rows.length})</span></h3>{rows.map(([methodName, method]) => {
          const examples = methodExamples(methodName, method, name, forms), roles = rolesOf(method);
          return <details class="method-row" id={`method-${encodeURIComponent(methodName)}`}><summary><span class="method-heading"><strong>{methodName}</strong>{argsOf(method).some(arg => arg.json_only !== true) ? <code class="method-signature">{signature(methodName, method).slice(methodName.length).trim()}</code> : null}{roles.length ? <span class="pill method-role">{roles.join(" or ")}</span> : null}{method.destructive === true ? <span class="pill tone-bad">⚠ destructive</span> : null}</span><span class="method-help">{text(method.help)}</span></summary><div class="method-body">
            {argsOf(method).length ? <Arguments value={argsOf(method)} /> : null}
            {text(method.returns) ? <p class="method-returns"><strong>Returns:</strong> {text(method.returns)}</p> : null}
            {examples.cli ? <CopyExample label="CLI" value={examples.cli} /> : null}
            {examples.json ? <CopyExample label="JSON body" value={examples.json} /> : null}
            {(examples.cli?.includes("<") || examples.json?.includes("<")) ? <p class="muted small">Replace values in &lt;angle brackets&gt; before calling.</p> : null}
          </div></details>;
        })}</div> : null;
      })}
      {object(d.hidden_methods) && Number.isSafeInteger(d.hidden_methods.count) && Number(d.hidden_methods.count) > 0 ? <p class="method-access muted small">{Number(d.hidden_methods.count)} more {Number(d.hidden_methods.count) === 1 ? "method is" : "methods are"} hidden by your roles.{strings(d.hidden_methods.roles).length ? <> Required roles include: <strong>{strings(d.hidden_methods.roles).join(", ")}</strong>.</> : null} <a href="#roles">See roles</a>.</p> : null}
      <noscript><p class="muted small">Expand a method and select its example text to copy it.</p></noscript>
    </div></details>
  </>;
}
/** A key name that may wrap only after an underscore, never inside a word. */
function KeyName({ name }: { name: string }) {
  return <>{name.split("_").map((part, i, all) => <>{part}{i < all.length - 1 ? <>_<wbr /></> : null}</>)}</>;
}
export function ConfigurationHelp({ help, value }: { help: AgentHelp; value?: string }) {
  if (!help.data) return null;
  const keys = entries(help.data.config);
  let unknown: string[] = [];
  try { const config: unknown = JSON.parse(value || "null"); if (object(config)) unknown = Object.keys(config).filter(k => !keys.some(([name]) => name === k)); } catch {}
  return <div class="metadata-config"><h3>Configuration keys</h3>
    {keys.length ? <dl class="metadata-keys">{keys.map(([name, field]) => <div><dt><code><KeyName name={name} /></code><span class="muted">{typeName(field.type)}</span></dt><dd>{text(field.help)}{field.example !== undefined || field.default !== undefined ? <details><summary>Example / default</summary>{field.example !== undefined ? <div>Example: <code><Json value={JSON.stringify(field.example)} /></code></div> : null}{field.default !== undefined ? <div>Default: <code><Json value={JSON.stringify(field.default)} /></code></div> : null}</details> : null}</dd></div>)}</dl> : <p>No Configuration keys declared.</p>}
    {unknown.length ? <p class="warn">Undocumented Configuration keys: {unknown.join(", ")}. They are allowed.</p> : null}
  </div>;
}
export function SecretHelp({ help }: { help: AgentHelp }) {
  if (!help.data) return null;
  const declarations = (Array.isArray(help.data.secrets) ? help.data.secrets : []).filter(object);
  return <div class="metadata-secret"><h3>Expected Secret keys</h3><p>Names and descriptions only; stored values are never shown.</p>
    {declarations.length ? <dl class="metadata-keys">{declarations.map(key => <div><dt><code><KeyName name={text(key.name)} /></code>{key.required === true ? <span class="muted">required</span> : null}</dt><dd>{text(key.help)}</dd></div>)}</dl> : <p>No Secret keys declared.</p>}
  </div>;
}
