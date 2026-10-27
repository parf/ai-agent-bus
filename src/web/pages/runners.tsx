import { TagBadges } from "../ui/tags.tsx";
// /runners and /runner: the runners the visitor may see, each one's status
// asked of the runner itself, and its agents linked to their own pages
// (Plans/R1.0-Release/runner/web-face.md#goal). The face is a client of the
// runner and nothing more: every observation is the runner's answer, every
// action a call the runner's ACL decides — its controls and the host's files
// (docs/runner/operations.md#controlling-one-agent) — and no record is edited here.
import { h, Fragment } from "../jsx.ts";
import { Ctx, LocalProblem, NotFound, Refusal, Unreachable, type CallResult, type Rec } from "../ctx.ts";
import { respond } from "../ui/frame.tsx";
import { Icon, Help, Name, PageHead, Card, Facts, Pill, Empty, Muted, Button, KindIcon, DocumentationLink, recordHref } from "../ui/kit.tsx";
import { redirect } from "../http.ts";
import { number, relative } from "../format.ts";

/** A runner is `#runner/<instance>@<host>` (Plans/R1.0-Release/DECISIONS.md#recorded-decisions, RD84); a tag never makes one. */
export const isRunner = (r: { name: string; kind: string }) => r.kind === "agent" && /^#runner\/[^/@]+@[^/@]+$/.test(r.name);

/** How long each call waits for the runner, in ms; tests shorten them. */
export const waits = { status: 10_000, chooser: 3_000, reload: 30_000, control: 30_000, file: 10_000 };

/** The calls that take one agent (docs/runner/operations.md#controlling-one-agent). */
const CONTROLS = ["start", "stop", "restart", "enable", "disable"];

type Exit = { code: number; at?: string; reason?: string };
type AgentRow = {
  name: string; enabled: boolean; state: string; reason?: string; processes?: { running: number; min: number; max: number };
  // Since 0.8.203 (Plans/R1.0-Release/DECISIONS.md#recorded-decisions, RD99); each absent when unknown.
  serves?: "runner" | "itself"; sandbox?: "on" | "off"; limits?: { memory_bytes?: number; processes?: number; disk_bytes?: number };
  started_at?: string; last_exit?: Exit;
  // Since 0.8.210: its agents.json key, which names its files and its controls.
  key?: string;
};
type Answer = { runner?: string; version?: string; error?: string; agents?: AgentRow[]; started_at?: string; agent_dir?: string; runner_dir?: string };

const bytes = (n?: number) => n == null ? "" : n >= 1 << 30 ? `${+(n / (1 << 30)).toFixed(1)} GiB` : `${Math.round(n / (1 << 20))} MiB`;

/** The sandbox and the limits it enforces; nothing when the runner does not say. */
function protection(a: AgentRow) {
  if (a.sandbox === "off") return <Pill tone="warn">sandbox off</Pill>;
  if (a.sandbox !== "on") return <Muted>—</Muted>;
  const l = a.limits ?? {};
  const parts = [l.memory_bytes != null ? `${bytes(l.memory_bytes)} memory` : "", l.processes != null ? `${number(l.processes)} processes` : "", l.disk_bytes != null ? `${bytes(l.disk_bytes)} disk` : ""].filter(Boolean);
  // The limits on hover, one per line, so the cell stays one word wide.
  return parts.length ? <span title={parts.map(p => `• ${p}`).join("\n")} data-limits><Pill tone="ok">sandbox</Pill></span> : <Pill tone="ok">sandbox</Pill>;
}

/** The most recent exit, as the runner saw it. */
const exitText = (e: Exit) => `${e.code < 0 ? "killed by a signal" : `exit ${e.code}`}${e.at ? `, ${relative(e.at)}` : ""}`;

/** What one call to a runner observed: its parsed answer, or why there is none. */
type Observed = { answer: Answer; at: Date } | { problem: string; uncertain?: boolean };

async function observe(ctx: Ctx, name: string, verb: string, waitMs: number): Promise<Observed> {
  let r: CallResult;
  try { r = await ctx.call(name, verb, waitMs); }
  catch (e) {
    if (e instanceof Refusal) return { problem: `The bus refused the call: ${e.detail}` };
    // The send itself went unanswered: it may have arrived.
    if (e instanceof Unreachable) return { problem: "The daemon did not answer the call; it may still have been sent.", uncertain: true };
    throw e;
  }
  if (r.kind === "timeout") return { problem: `No answer within ${waitMs / 1000} s. The runner may be stopped or busy; nothing here says its agents are gone.`, uncertain: true };
  if (r.kind === "done") return { problem: "The runner finished and sent no answer.", uncertain: true };
  if (r.kind === "lost") return { problem: `The call was sent, and waiting for its answer failed: ${r.why}.`, uncertain: true };
  try {
    const a = JSON.parse(r.body) as Answer;
    if (a && typeof a === "object" && Array.isArray(a.agents)) return { answer: a, at: new Date() };
  } catch {}
  // A runner's refusal of a verb is a text answer, shown as it said it.
  return { problem: `The runner answered: ${r.body.slice(0, 500)}` };
}

const href = (name: string) => `/runner?name=${encodeURIComponent(name)}`;

/** What on a runner needs a look, worst first; nothing when all is well. */
function alertsCard(answer: Answer, skew: number, nodeVersion?: string) {
  const agents = answer.agents ?? [];
  const items: { tone: string; text: any }[] = [];
  if (answer.error) items.push({ tone: "bad", text: <>Nothing is loaded: <span data-load-error-alert>{answer.error}</span></> });
  for (const a of agents.filter(a => a.state === "refused" || a.state === "unconfigured"))
    items.push({ tone: a.state === "refused" ? "bad" : "warn", text: <><code>{a.name}</code> is {a.state}{a.reason ? <>: {a.reason}</> : null}</> });
  for (const a of agents.filter(a => a.state === "stopped")) items.push({ tone: "warn", text: <><code>{a.name}</code> is stopped{a.reason ? <>: {a.reason}</> : null}</> });
  for (const a of agents.filter(a => a.sandbox === "off")) items.push({ tone: "warn", text: <><code>{a.name}</code> has its sandbox off: when it runs, it has the runner account's whole reach</> });
  for (const a of agents.filter(a => a.last_exit && a.last_exit.code !== 0 && a.state !== "refused" && a.state !== "unconfigured"))
    items.push({ tone: "warn", text: <><code>{a.name}</code> last exited {exitText(a.last_exit!)}{a.last_exit!.reason ? <>: <code>{a.last_exit!.reason}</code></> : null}</> });
  if (skew < 0) items.push({ tone: "warn", text: <>The runner is older than this node's {nodeVersion}: restart it to update it</> });
  if (agents.length && !agents.some(a => a.key)) items.push({ tone: "warn", text: <>The runner is older than 0.8.210: it has no per-agent control or settings until it is restarted</> });
  if (!items.length) return null;
  return <Card title="Alerts" icon="triangle-alert" id="alerts">
    <ul class="alerts" data-alerts>{items.map(i => <li data-tone={i.tone}><Pill tone={i.tone}>{i.tone === "bad" ? "error" : "warning"}</Pill> {i.text}</li>)}</ul>
  </Card>;
}

const STATE_TONE: Record<string, string> = { idle: "ok", running: "accent", refused: "bad", unconfigured: "warn", disabled: "muted", stopped: "warn" };

async function runnersPage(ctx: Ctx): Promise<Response> {
  const [st, all] = await Promise.all([ctx.status(), ctx.records()]);
  const runners = all.filter(isRunner).sort((a, b) => a.name.localeCompare(b.name));
  // Exactly one: straight to it, from this visitor's own set, never stored
  // (every answer of this face is no-store).
  if (runners.length === 1) return redirect(href(runners[0]!.name), 301);
  const seen = await Promise.all(runners.map(r => r.status === "inactive" ? Promise.resolve(undefined) : observe(ctx, r.name, "status", waits.chooser)));
  const body = <>
    <PageHead icon={<Icon name="server" />} title="Runners"
      help={<Help id="runners-help" label="About Runners" title="Runners" items={[
        "A runner is a person's agent that keeps other agents running on a host.",
        "Each row is asked for its status as you open the page; nothing is remembered between visits.",
      ]} />} />
    {runners.length === 0 ? <Empty icon="server" title="No runners you can see"
        action={<DocumentationLink label="Set up a runner" target="07-setup.md#a-persons-runner" />}>
        A runner appears here once one is set up and its access list admits you.
      </Empty>
      : <div class="table-wrap"><div class="table-scroll"><table class="data stack record-table">
        <thead><tr><th>Runner</th><th>Owner</th><th>Answer</th><th>Agents</th></tr></thead>
        <tbody>{runners.map((r, i) => {
          const o = seen[i];
          return <tr data-runner={r.name}>
            <td data-label="Runner"><div class="rec-cell"><KindIcon kind="agent" /><a href={href(r.name)}><code>{r.name}</code></a></div>{r.descr ? <div class="muted small">{r.descr}</div> : null}</td>
            <td data-label="Owner"><code>{r.owner}</code></td>
            <td data-label="Answer">{!o ? <Pill>inactive</Pill> : "answer" in o ? <Pill tone="ok" dot>answering · v{o.answer.version ?? "?"}</Pill> : <Pill tone="warn" dot>{o.uncertain ? "no answer" : "refused"}</Pill>}</td>
            <td data-label="Agents">{o && "answer" in o ? number(o.answer.agents!.length) : <Muted>—</Muted>}</td>
          </tr>;
        })}</tbody>
      </table></div></div>}
  </>;
  return respond(ctx, { title: "Runners", section: "runners", signedIn: true, you: st.you }, body);
}

async function runnerFor(ctx: Ctx): Promise<Rec> {
  const name = ctx.q("name") || ctx.f("name");
  if (!name) throw new LocalProblem(400, "name a runner: /runner?name=…");
  let rec: Rec;
  try { rec = await ctx.lookup(name); }
  catch (e) { if (e instanceof Refusal && e.status === 404) throw new NotFound(); throw e; }
  if (!isRunner(rec)) throw new NotFound();
  return rec;
}

/** -1, 0 or 1 as release `a` is before, equal to or after `b`, part by part. */
export function versionOrder(a: string, b: string): number {
  const x = a.split(".").map(Number), y = b.split(".").map(Number);
  for (let i = 0; i < Math.max(x.length, y.length); i++) if ((x[i] ?? 0) !== (y[i] ?? 0)) return (x[i] ?? 0) < (y[i] ?? 0) ? -1 : 1;
  return 0;
}

/** A record's own tags, without the listing's marker for none. */


/** The states in the order a reader looks for trouble. */
const STATE_ORDER = ["refused", "unconfigured", "stopped", "running", "idle", "disabled"];

/** One runner's page: what it answered, or why it did not, and what may be asked of it.
 *  `done` is a success the runner answered; `warning` an outcome the visitor must not read as one. */
async function runnerPage(ctx: Ctx, rec: Rec, o: Observed, note?: { done?: string; warning?: string }): Promise<Response> {
  const [st, all, id] = await Promise.all([ctx.status(), ctx.records(), ctx.identity()]);
  const visible = new Map(all.map(r => [r.name, r]));
  const back = href(rec.name);
  const answer = "answer" in o ? o.answer : undefined;
  const agents = answer?.agents ?? [];
  const counts = STATE_ORDER.map(s => [s, agents.filter(a => a.state === s).length] as const).filter(([, n]) => n > 0);
  const other = agents.filter(a => !STATE_ORDER.includes(a.state)).length;
  // A runner older than the node it serves on was not restarted after an
  // update (git-install does not restart runners): worth saying, not hiding.
  const skew = answer?.version && id?.version ? versionOrder(answer.version, id.version) : 0;
  const body = <>
    <PageHead icon={<Icon name="server" />} title={<code>{rec.name}</code>} back={{ href: "/runners", label: "Runners" }}
      sub={<>Owner <code>{rec.owner}</code>{rec.descr ? <> · {rec.descr}</> : null}</>}>
      <a class="btn" href={back} data-refresh><Icon name="rotate-cw" />Ask again</a>
      <a class="btn" href={settingsHref(rec.name)} data-host-settings><Icon name="settings" />Host settings</a>
    </PageHead>
    {note?.warning ? <p class="warn" role="alert" data-runner-warning>{note.warning}</p> : null}
    {answer ? alertsCard(answer, skew, id?.version) : null}
    <Card title="Runner" icon="server">
      {answer ? <>
        <Facts rows={[
          ["Answer", <Pill tone="ok" dot>answering</Pill>],
          ["Version", <>{answer.version ? <code>{answer.version}</code> : <Muted>—</Muted>}{skew ? <> <span data-skew><Pill tone="warn">{skew < 0 ? `older than this node's ${id!.version}: restart the runner to update it` : `newer than this node's ${id!.version}`}</Pill></span></> : null}</>],
          ["Host", <code>{rec.name.slice(rec.name.indexOf("@") + 1)}</code>],
          ["Agents", agents.length === 0 ? <Muted>none</Muted> : <span data-counts>{[...counts.map(([s, n]) => `${number(n)} ${s}`), ...(other ? [`${number(other)} other`] : [])].join(" · ")}</span>],
          ...(answer.started_at ? [["Up since", <span title={answer.started_at}>{relative(answer.started_at)}</span>] as [string, any]] : []),
        ]} />
        {answer.agent_dir || answer.runner_dir ? <div class="runner-paths" data-paths>
          {answer.agent_dir ? <>Agent code: <code>{answer.agent_dir}</code></> : null}
          {answer.agent_dir && answer.runner_dir ? " · " : null}
          {answer.runner_dir ? <>Host settings: <code>{answer.runner_dir}</code></> : null}
          <Help id="runner-path-info" label="About the runner's record"
            tip="The runner's own record holds its description, access list and Maintainers."
            items={[<><a href={recordHref(rec,{return:back})}>The runner's own record</a> holds its description, access list and Maintainers.</>]} />
        </div> : null}
        {answer.error ? <p class="warn" data-load-error>Load error: {answer.error}</p> : null}
      </> : <p class="warn" data-runner-problem>{(o as { problem: string }).problem}</p>}
    </Card>
    <Card title="Agents" icon="kind:agent" id="agents">
      {!answer ? <Muted>Not known: the runner did not answer, so its agents are not listed rather than shown as none.</Muted>
        : agents.length === 0 ? <Muted>The runner serves no agents.</Muted>
        : <div class="table-wrap"><div class="table-scroll"><table class="data stack record-table">
          <thead><tr><th>Agent</th><th>State</th><th>Processes</th><th>Protection</th><th>Inbox</th><th>Last used</th><th>Manage</th></tr></thead>
          <tbody>{agents.map(a => {
            const r = visible.get(a.name);
            return <tr data-agent={a.name}>
              {/* As the agents list draws a record: description over name, then its tags. */}
              <td data-label="Agent"><div class="rec-cell"><KindIcon kind="agent" template={r?.resource?.template} />
                <div class="rec-title">{r ? <a href={recordHref(r, { return: back })}>{r.descr ? <><strong>{r.descr}</strong><code>{a.name}</code></> : <strong><code>{a.name}</code></strong>}</a>
                  : <><strong><code>{a.name}</code></strong><span class="muted small">its record is not visible to you</span></>}{r ? <TagBadges rec={r} /> : null}</div></div></td>
              <td data-label="State"><Pill tone={STATE_TONE[a.state] ?? ""}>{a.state}</Pill>{!a.enabled && a.state !== "disabled" ? <div class="muted small">not enabled</div> : null}
                {a.serves ? <div class="muted small">{a.serves === "itself" ? "serves itself" : "a script per message"}</div> : null}</td>
              <td data-label="Processes">{a.processes ? <span title={`min ${a.processes.min}, max ${a.processes.max}`}>{number(a.processes.running)} of {number(a.processes.max)}</span> : <Muted>—</Muted>}
                {a.started_at ? <div class="muted small" title={a.started_at}>since {relative(a.started_at)}</div> : null}</td>
              <td data-label="Protection" data-protection>{protection(a)}</td>
              <td data-label="Inbox" data-inbox>{r ? <>{r.queued > 0 ? <>{number(r.queued)} queued{r.at_bound ? <> <Pill tone="warn">full</Pill></> : null}<br /></> : null}{number(r.readers)} reading</> : <Muted>—</Muted>}</td>
              <td data-label="Last used">{r?.last_used ? <span title={r.last_used}>{relative(r.last_used)}</span> : <Muted>—</Muted>}</td>
              {/* A runner before 0.8.210 names no key, so nothing to manage by. */}
              <td data-label="Manage">{a.key ? <a href={settingsHref(rec.name, a.key)} data-manage>Manage</a> : <Muted>—</Muted>}</td>
            </tr>;
          })}</tbody>
        </table></div></div>}
      {answer && agents.some(a => !visible.has(a.name)) ? <p class="muted small">An agent whose record you may not see shows only what the runner says about it.</p> : null}
    </Card>
  </>;
  return respond(ctx, { title: rec.name, section: "runners", signedIn: true, you: st.you, flash: note?.done }, body);
}

async function runner(ctx: Ctx): Promise<Response> {
  const rec = await runnerFor(ctx);
  return runnerPage(ctx, rec, await observe(ctx, rec.name, "status", waits.status));
}

/** Reload, and show what the runner answered; an uncertain outcome says so and is not repeated. */
async function postReload(ctx: Ctx): Promise<Response> {
  const rec = await runnerFor(ctx);
  // Back to the page it was asked from: an agent's, or the host's settings.
  const key = keyOf(ctx);
  const o = await observe(ctx, rec.name, "reload", waits.reload);
  if ("answer" in o) return settingsPage(ctx, rec, key, { done: "Reloaded: the runner answered." }, undefined, o);
  if (!o.uncertain) return settingsPage(ctx, rec, key, { warning: `The reload was not carried out. ${o.problem}` });
  const warning = `The reload was sent and its outcome is not known: ${o.problem} It may still take effect; check the status before reloading again.`;
  // The warning outlives a daemon that cannot draw the page around it: the
  // generic problem page would say nothing was changed.
  try { return await settingsPage(ctx, rec, key, { warning }); }
  catch { return respond(ctx, { title: rec.name, section: "runners", signedIn: true }, <p class="warn" role="alert" data-runner-warning>{warning}</p>, 502); }
}

/** A call that changes the runner, and the page after it: the runner's answer,
 *  a refusal, or an outcome not known — never read as success. */
async function act(ctx: Ctx, rec: Rec, body: string, what: string, waitMs: number): Promise<{ o: Observed; note: { done?: string; warning?: string } }> {
  const o = await observe(ctx, rec.name, body, waitMs);
  if ("answer" in o) return { o, note: { done: `${what}: the runner answered.` } };
  if (!o.uncertain) return { o, note: { warning: `${what} was not carried out. ${o.problem}` } };
  return { o: await observe(ctx, rec.name, "status", waits.status).catch(() => o), note: { warning: `${what} was sent and its outcome is not known: ${o.problem} It may still take effect; check the status before asking again.` } };
}

/** start, stop, restart, enable or disable one agent. */
async function postControl(ctx: Ctx): Promise<Response> {
  const rec = await runnerFor(ctx);
  const verb = ctx.f("verb"), agent = ctx.f("agent").trim();
  if (!CONTROLS.includes(verb) || !KEY.test(agent)) throw new LocalProblem(400, "a control is start, stop, restart, enable or disable of one agent");
  const { o, note } = await act(ctx, rec, `${verb} ${agent}`, `${verb[0]!.toUpperCase()}${verb.slice(1)} ${agent}`, waits.control);
  return settingsPage(ctx, rec, agent, note, undefined, "answer" in o ? o : undefined);
}

/** An agent's key, `<agent>` or `<agent>/<instance>`, as the runner makes them. */
const KEY = /^[A-Za-z0-9_][A-Za-z0-9._-]{0,63}(\/[A-Za-z0-9_][A-Za-z0-9._-]{0,63})?$/;

const settingsHref = (name: string, key?: string) => `/runner-settings?name=${encodeURIComponent(name)}${key ? `&key=${encodeURIComponent(key)}` : ""}`;

type FileRead = { file: string; exists: boolean; content: string; sha256?: string; size?: number; names?: string[] } | { problem: string };

/** One of the host's files, as the runner read it. */
async function readFile(ctx: Ctx, runner: string, file: string): Promise<FileRead> {
  let r: CallResult;
  try { r = await ctx.call(runner, `read ${file}`, waits.file); }
  catch (e) {
    if (e instanceof Refusal) return { problem: `The bus refused the call: ${e.detail}` };
    if (e instanceof Unreachable) return { problem: "The daemon did not answer the call." };
    throw e;
  }
  if (r.kind !== "answer") return { problem: r.kind === "timeout" ? `No answer within ${waits.file / 1000} s.` : "No answer came back." };
  try {
    const a = JSON.parse(r.body);
    if (a && typeof a === "object" && typeof a.exists === "boolean") return { file, exists: a.exists, content: String(a.content ?? ""), sha256: /^[a-f0-9]{64}$/.test(a.sha256) ? a.sha256 : undefined, size: typeof a.size === "number" ? a.size : undefined, names: Array.isArray(a.names) ? a.names.filter((name: unknown) => typeof name === "string" && /^[A-Za-z0-9_.-]+$/.test(name)) : [] };
  } catch {}
  return { problem: file.endsWith("/env") && !/^error: read is for the runner's Owner and Maintainers/.test(r.body) ? "The runner could not read this env file. Check its syntax on the host." : `The runner answered: ${r.body.slice(0, 500)}` };
}

/** One agent or instance managed — its controls, stop and disable behind the
 *  danger zone, its files and instances — or with no key the host's settings:
 *  reload and the inventory. The author's files are read only, the host's each
 *  in a form of its own. `o` is a status already in hand, the one a control answered. */
async function settingsPage(ctx: Ctx, rec: Rec, key: string, note?: { done?: string; warning?: string }, kept?: { file: string; content: string }, o?: Observed): Promise<Response> {
  const [st, all, seen] = await Promise.all([ctx.status(), ctx.records(), key && !o ? observe(ctx, rec.name, "status", waits.status) : o]);
  const dir = key.split("/")[0]!, instance = key.includes("/") ? key.slice(key.indexOf("/") + 1) : "";
  const author = key ? [`agent.d/${dir}/agent.json`, `agent.d/${dir}/env.dist`] : [];
  const host = !key ? ["runner/agents.json"]
    : [`runner/${dir}/env`, `runner/${dir}/agent.json`, ...(instance ? [`runner/${key}/env`, `runner/${key}/agent.json`] : [])];
  const read = await Promise.all([...author, ...host].map(f => readFile(ctx, rec.name, f)));
  // The runner's own refusal of a visitor who may see it but not manage it:
  // the page says that once and offers nothing it would refuse.
  const denied = read.some(r => "problem" in r && /^The runner answered: error: \w+ is for the runner's Owner and Maintainers/.test(r.problem));
  const of = (f: string) => read[[...author, ...host].indexOf(f)]!;
  const back = href(rec.name);
  const what: Record<string, string> = {
    "agents.json": "which agents run here: names, tags, sandbox, limits",
    "env": "KEY=value per line; holds secrets",
    "agent.json": "overrides processes, deadline, idle, depends",
  };
  const answer = seen && "answer" in seen ? seen.answer : undefined;
  const row = answer?.agents?.find(a => a.key === key);
  const control = (buttons: any) => <form method="post" action="/runner-control" class="inline-form">
    <input type="hidden" name="name" value={rec.name} /><input type="hidden" name="agent" value={key} />{buttons}</form>;
  // The sample files of every option (docs/runner/configuration.md).
  const base = (f: string) => f.slice(f.lastIndexOf("/") + 1);
  const sample: Record<string, string> = { "agents.json": "agents.json.dist", "agent.json": "agent.json.dist" };
  const agentName = row?.name ?? `#${key}@${rec.name.slice(rec.name.indexOf("@") + 1)}`;
  const record = all.find(r => r.name === agentName);
  // The danger zone: closed, its summary naming what it holds; opened, its
  // buttons run on to the right of it, each saying on hover what it does.
  const stop = !!row && row.state !== "stopped" && row.state !== "disabled", disable = !!row?.enabled;
  const names = [stop ? "Stop" : "", disable ? "Disable" : "", instance ? "Delete" : ""].filter(Boolean);
  const dangerOpen = ctx.q("danger") === "open" || (ctx.method === "POST" && ["stop", "disable", "delete"].includes(ctx.f("verb")));
  const danger = key && (names.length || dangerOpen) ? <details class="danger-inline" data-danger-zone open={dangerOpen}>
    <summary><Icon name="flame" /> Danger zone{names.length ? ` (${names.join(", ")})` : ""}</summary>
    {stop ? <span class="danger-action" data-danger="stop" title="Down until the next reload or the runner's restart: a running script finishes, a kept process is asked to stop, and new messages wait in its inbox.">
      {control(<Button name="verb" value="stop" tone="warn" icon="square">Stop</Button>)}</span> : null}
    {disable ? <span class="danger-action" data-danger="disable" title={`Writes "enabled": false to runner/agents.json: down across reloads and restarts until enabled.`}>
      {control(<Button name="verb" value="disable" tone="danger" icon="power-off">Disable</Button>)}</span> : null}
    {instance ? <span class="danger-action" data-danger="delete" title="Its entry, env and override go; its work, kept token and record stay. The next page asks for the key again.">
      <a class="btn btn-danger" href={`/runner-instance-delete?name=${encodeURIComponent(rec.name)}&key=${encodeURIComponent(key)}`} data-delete><Icon name="trash" />Delete instance…</a></span> : null}
    {!names.length ? <span class="muted">No dangerous actions are available in this state.</span> : null}
  </details> : null;
  // Headed as the agent's own Edit page is: its kind, its name, its Owner.
  const body = <>
    <PageHead back={{ href: back, label: `Back to ${rec.name}` }}
      icon={key ? <Icon name="kind:agent" /> : <Icon name="settings" />}
      title={key ? <>Manage Agent <Name>{agentName}</Name>{record ? <span class="muted small"> (owner: <Icon name="kind:user" /><Name>{record.owner}</Name>)</span> : null}</> : <>Host settings <Name>{rec.name}</Name></>} />
    {note?.warning ? <p class="warn" role="alert" data-runner-warning>{note.warning}</p> : null}
    {denied ? <p class="warn" role="alert" data-denied>Access denied: only the runner's Owner and Maintainers manage it.</p> : <>
    {!key ? <Card title="Runner" icon="server">
      <form method="post" action="/runner-reload" class="inline-form">
        <input type="hidden" name="name" value={rec.name} />
        <Button icon="refresh-cw">Reload</Button>
      </form>
      <p class="muted small">Reload asks the runner to read every installed agent and its settings again, for the whole runner: agents a stop took down are served again. It does not restart the runner itself, so it does not update an older runner.</p>
    </Card> : <Card id="control">
      {!seen || !("answer" in seen) ? <><p class="warn" data-runner-problem>{seen && "problem" in seen ? seen.problem : "Not asked."}</p>{danger}</>
        : !row ? <><Muted>The runner names no agent {key}: it is not served here, or the runner is older than 0.8.210.</Muted>{danger ? <div class="agent-status">{danger}</div> : null}</>
        : <>
          {/* One line: its state, then what may be done to it now. */}
          <div class="agent-status" data-agent={row.name}>
            <span class="muted">Status</span><Pill tone={STATE_TONE[row.state] ?? ""}>{row.state}</Pill>{!row.enabled && row.state !== "disabled" ? <Muted>not enabled</Muted> : null}
            <span class="sep" aria-hidden="true"></span>
            <span data-control>{control(<>
              {row.state === "stopped" ? <Button name="verb" value="start" icon="play">Start</Button> : null}
              {row.enabled ? <Button name="verb" value="restart" icon="rotate-cw">Restart</Button> : <Button name="verb" value="enable" icon="play">Enable</Button>}</>)}</span>
            <form method="post" action="/runner-reload" class="inline-form" data-reload>
              <input type="hidden" name="name" value={rec.name} /><input type="hidden" name="key" value={key} />
              <Button icon="refresh-cw">Reload</Button>
            </form>
            {danger}
          </div>
          {row.reason || row.last_exit ? <dl class="pairs" data-exit>
            {row.reason ? <><dt>Reason</dt><dd data-reason>{row.reason}</dd></> : null}
            {row.last_exit ? <><dt>Last exit</dt><dd data-last-exit>{row.last_exit.at ? <span title={row.last_exit.at}>{exitText(row.last_exit)}</span> : exitText(row.last_exit)}{row.last_exit.reason ? <>: <code>{row.last_exit.reason}</code></> : null}</dd></> : null}
          </dl> : null}
        </>}
    </Card>}
    {key ? <Card title="The author's files" icon="file">
      {author.map(f => { const r = of(f); const gone = !("problem" in r) && !r.exists;
        return <div class="runner-file" data-file={f}>
          <h3 class="file-head"><code>{f}</code> <span class="muted">— {gone ? "absent" : "read only, from the agent's code"}</span></h3>
          {"problem" in r ? <p class="warn">{r.problem}</p> : r.exists ? <pre class="file-view">{r.content}</pre> : null}</div>; })}
    </Card> : null}
    <Card title="The host's files" icon="settings">
      <p class="muted small">{key ? "Restart the agent to apply changes." : "Reload the runner to apply changes."}</p>
      {host.map(f => { const r = of(f); const secret = base(f) === "env"; const content = secret ? "" : kept?.file === f ? kept.content : "problem" in r ? "" : r.content;
        const hostPath = answer?.runner_dir ? answer.runner_dir.replace(/\/$/, "") + "/" + f.slice("runner/".length) : undefined;
        return <form method="post" action="/runner-file" class="runner-file" data-file={f}>
          {/* One line: the file, what it is, absent when it is, its sample. */}
          <h3 class="file-head"><code>{f}</code> <span class="muted">— {what[base(f)]}{"problem" in r || r.exists ? null : " · absent"}</span>
            {sample[base(f)] ? <span data-sample><DocumentationLink label={`Every option: ${sample[base(f)]}`} target={`runner/${sample[base(f)]}`} /></span> : null}</h3>
          {"problem" in r ? <p class="warn">{r.problem}</p> : <>
            <input type="hidden" name="name" value={rec.name} />
            <input type="hidden" name="key" value={key} />
            <input type="hidden" name="file" value={f} />
            {secret ? <>
              <p data-env-stored>{r.exists ? r.sha256 ? `stored: sha256:${r.sha256} (${(r.names ?? []).join(", ")}) · ${r.size ?? "?"} bytes` : "stored; upgrade the runner to show its digest and names" : "not stored"}</p>
              <Help label="About runner secrets" title="Two places for secrets" tip="Values stay on this host; a write in flight passes through the bus. The editor starts empty; saving replaces the whole file." items={[
                <>The record’s <a href={`/agent/edit?name=${encodeURIComponent(agentName)}#f-secret`}>Secret on the bus</a> is separate from the runner’s files on this server. The bus keeps no copy of these files beyond a write in flight.</>,
                "The runner’s secrets take over a Secret key with the same name.",
                hostPath ? <>To see values, use host access: <code>{`sudo cat '${hostPath.replace(/'/g, "'\\''")}'`}</code>.</> : "To see values you need host access; this runner did not report its host path.",
                "Leave this empty until replacing the file. Saving replaces all its values, including when empty.",
              ]} />
            </> : null}
            <textarea name="content" rows="8" spellcheck="false" autocomplete="off">{"\n"}{content}</textarea>
            <Button icon="save">Save</Button></>}
        </form>; })}
    </Card>
    {key ? <Card title="Instances" icon="copy">
      {record ? <p><a href={recordHref(record, { return: settingsHref(rec.name, key) })}>Its record</a> holds the bus secret and configuration.</p> : null}
      <form method="post" action="/runner-instance" class="instance-form">
        <input type="hidden" name="name" value={rec.name} /><input type="hidden" name="verb" value="create" />
        <label><span>New instance of <code>{dir}</code></span><input type="text" name="key" value={`${dir}/`} required /></label>
        <Button icon="plus">Create</Button>
      </form>
    </Card> : null}
    </>}
  </>;
  return respond(ctx, { title: key ? `Manage ${key}` : "Host settings", section: "runners", signedIn: true, you: st.you, flash: note?.done }, body);
}

function keyOf(ctx: Ctx): string {
  const key = (ctx.q("key") || ctx.f("key")).trim();
  if (key && !KEY.test(key)) throw new LocalProblem(400, "a key is <agent> or <agent>/<instance>");
  return key;
}

async function settings(ctx: Ctx): Promise<Response> {
  const rec = await runnerFor(ctx);
  return settingsPage(ctx, rec, keyOf(ctx));
}

/** Write one of the host's files; a refusal keeps what was typed. */
async function postFile(ctx: Ctx): Promise<Response> {
  const rec = await runnerFor(ctx);
  // The browser drops the newline the textarea opens with, so the content is
  // as the visitor sees it; only line endings are made the runner's.
  const key = keyOf(ctx), file = ctx.f("file"), content = ctx.f("content").replace(/\r\n/g, "\n");
  if (!/^(runner\/agents\.json|runner\/[A-Za-z0-9_][A-Za-z0-9._-]*(\/[A-Za-z0-9_][A-Za-z0-9._-]*)?\/(env|agent\.json))$/.test(file)) throw new LocalProblem(400, "not one of the host's files");
  const { o, note } = await act(ctx, rec, `write ${file}\n${content}`, `Saving ${file}`, waits.control);
  return settingsPage(ctx, rec, key, "answer" in o && note.done ? { done: `Saved ${file}: the runner checked and applied it.` } : note, "answer" in o ? undefined : { file, content });
}

/** Create or delete an instance. */
async function postInstance(ctx: Ctx): Promise<Response> {
  const rec = await runnerFor(ctx);
  const verb = ctx.f("verb"), key = keyOf(ctx);
  if ((verb !== "create" && verb !== "delete") || !key.includes("/")) throw new LocalProblem(400, "create or delete <agent>/<instance>");
  if (verb === "delete" && ctx.f("confirm").trim() !== key) return confirmDelete(ctx, rec, key, `Type ${key} to delete it; nothing was deleted.`);
  const { o, note } = await act(ctx, rec, `${verb} ${key}`, `${verb === "create" ? "Creating" : "Deleting"} ${key}`, waits.control);
  if (verb === "delete" && !("answer" in o)) return confirmDelete(ctx, rec, key, note.warning);
  if (verb === "create" && "answer" in o && note.done) return settingsPage(ctx, rec, key, { done: `Created ${key}: configure its env below.` });
  return settingsPage(ctx, rec, key.slice(0, key.indexOf("/")), note, undefined, o);
}

/** What deleting an instance takes and leaves, and the key typed again to do it. */
async function confirmDelete(ctx: Ctx, rec: Rec, key: string, warning?: string): Promise<Response> {
  const st = await ctx.status();
  const body = <>
    <PageHead icon={<Icon name="trash" />} title={<>Delete <code>{key}</code></>} back={{ href: settingsHref(rec.name, key) + "&danger=open", label: key }} sub={<>On <code>{rec.name}</code></>} />
    {warning ? <p class="warn" role="alert" data-runner-warning>{warning}</p> : null}
    <Card title="What goes" icon="trash">
      <ul data-goes><li>its entry in <code>runner/agents.json</code></li><li><code>runner/{key}/env</code>, with the secrets in it</li><li><code>runner/{key}/agent.json</code>, its override</li></ul>
      <p>The runner stops serving it. This cannot be undone here: the env is not kept anywhere else.</p>
    </Card>
    <Card title="What stays" icon="shield">
      <ul data-stays><li>its work directory</li><li>its kept token</li><li>its record on the bus, with its inbox, bus secret and configuration</li></ul>
    </Card>
    <form method="post" action="/runner-instance">
      <input type="hidden" name="name" value={rec.name} /><input type="hidden" name="key" value={key} /><input type="hidden" name="verb" value="delete" />
      <label>Type <code>{key}</code> to confirm: <input type="text" name="confirm" value={ctx.f("confirm")} autocomplete="off" required /></label>
      <Button tone="danger" icon="trash">Delete instance</Button>
    </form>
  </>;
  return respond(ctx, { title: `Delete ${key}`, section: "runners", signedIn: true, you: st.you }, body);
}

async function instanceDelete(ctx: Ctx): Promise<Response> {
  const rec = await runnerFor(ctx);
  const key = keyOf(ctx);
  if (!key.includes("/")) throw new LocalProblem(400, "only an instance, <agent>/<instance>, is deleted");
  return confirmDelete(ctx, rec, key);
}

export const handlers = { runners: runnersPage, runner, postReload, postControl, settings, postFile, postInstance, instanceDelete };
