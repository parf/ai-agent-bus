// /runners and /runner: the runners the visitor may see, each one's status
// asked of the runner itself, and its agents linked to their own pages
// (Plans/R1.0-Release/runner/web-face.md#goal). The face is a client of the
// runner and nothing more: every observation is the runner's answer, every
// action a call the runner's ACL decides — its controls and the host's files
// (docs/runner/operations.md#controlling-one-agent) — and no record is edited here.
import { h, Fragment } from "../jsx.ts";
import { Ctx, LocalProblem, NotFound, Refusal, Unreachable, type CallResult, type Rec } from "../ctx.ts";
import { respond } from "../ui/frame.tsx";
import { Icon, Help, PageHead, Card, Facts, Pill, Empty, Muted, Button, KindIcon, DocumentationLink, recordHref } from "../ui/kit.tsx";
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
  return <><Pill tone="ok">sandbox</Pill>{parts.length ? <div class="muted small">{parts.join(" · ")}</div> : null}</>;
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
        "Who may see and call a runner is its own access list.",
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
const tagsOf = (r?: Rec) => (r?.tags ?? []).filter(t => t !== "no-tags");

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
    </PageHead>
    {note?.warning ? <p class="warn" role="alert" data-runner-warning>{note.warning}</p> : null}
    <Card title="Runner" icon="server">
      {answer ? <Facts rows={[
        ["Answer", <Pill tone="ok" dot>answering</Pill>],
        ["Host", <code>{rec.name.slice(rec.name.indexOf("@") + 1)}</code>],
        ["Version", <>{answer.version ? <code>{answer.version}</code> : <Muted>—</Muted>}{skew ? <> <span data-skew><Pill tone="warn">{skew < 0 ? `older than this node's ${id!.version}: restart the runner to update it` : `newer than this node's ${id!.version}`}</Pill></span></> : null}</>],
        ["Agents", agents.length === 0 ? <Muted>none</Muted> : <span data-counts>{[...counts.map(([s, n]) => `${number(n)} ${s}`), ...(other ? [`${number(other)} other`] : [])].join(" · ")}</span>],
        ...(answer.started_at ? [["Up since", <span title={answer.started_at}>{relative(answer.started_at)}</span>] as [string, any]] : []),
        ...(answer.agent_dir ? [["Agent code", <code>{answer.agent_dir}</code>] as [string, any]] : []),
        ...(answer.runner_dir ? [["Host settings", <code>{answer.runner_dir}</code>] as [string, any]] : []),
        ["Asked", (o as { at: Date }).at.toISOString().replace("T", " ").slice(0, 19) + " UTC"],
        ...(answer.error ? [["Load error", <span class="warn" data-load-error>{answer.error}</span>] as [string, any]] : []),
      ]} /> : <p class="warn" data-runner-problem>{(o as { problem: string }).problem}</p>}
      <p><a href={recordHref(rec, { return: back })}>The runner's own record</a> holds its description, access list and Maintainers.</p>
    </Card>
    <Card title="Agents" icon="kind:agent" id="agents">
      {!answer ? <Muted>Not known: the runner did not answer, so its agents are not listed rather than shown as none.</Muted>
        : agents.length === 0 ? <Muted>The runner serves no agents.</Muted>
        : <div class="table-wrap"><div class="table-scroll"><table class="data stack record-table">
          <thead><tr><th>Agent</th><th>State</th><th>Processes</th><th>Protection</th><th>Inbox</th><th>Last used</th><th>Reason and last exit</th><th>Control</th></tr></thead>
          <tbody>{agents.map(a => {
            const r = visible.get(a.name);
            return <tr data-agent={a.name}>
              <td data-label="Agent"><div class="rec-cell"><KindIcon kind="agent" />{r ? <a href={recordHref(r, { return: back })}><code>{a.name}</code></a> : <code>{a.name}</code>}</div>
                {r?.descr ? <div class="muted small">{r.descr}</div> : !r ? <div class="muted small">its record is not visible to you</div> : null}
                {tagsOf(r).length ? <div class="small">{tagsOf(r).map(t => <><Pill>{t}</Pill> </>)}</div> : null}</td>
              <td data-label="State"><Pill tone={STATE_TONE[a.state] ?? ""}>{a.state}</Pill>{!a.enabled && a.state !== "disabled" ? <div class="muted small">not enabled</div> : null}
                {a.serves ? <div class="muted small">{a.serves === "itself" ? "serves itself" : "a script per message"}</div> : null}</td>
              <td data-label="Processes">{a.processes ? <span title={`min ${a.processes.min}, max ${a.processes.max}`}>{number(a.processes.running)} of {number(a.processes.max)}</span> : <Muted>—</Muted>}
                {a.started_at ? <div class="muted small" title={a.started_at}>since {relative(a.started_at)}</div> : null}</td>
              <td data-label="Protection" data-protection>{protection(a)}</td>
              <td data-label="Inbox" data-inbox>{r ? <>{number(r.queued)} queued · {number(r.readers)} reading{r.at_bound ? <> <Pill tone="warn">full</Pill></> : null}</> : <Muted>—</Muted>}</td>
              <td data-label="Last used">{r?.last_used ? <span title={r.last_used}>{relative(r.last_used)}</span> : <Muted>—</Muted>}</td>
              <td data-label="Reason and last exit">{a.reason ? <span data-reason>{a.reason}</span> : null}
                {a.last_exit ? <div class="small" data-last-exit>{exitText(a.last_exit)}{a.last_exit.reason ? <>: <code>{a.last_exit.reason}</code></> : null}</div> : null}
                {!a.reason && !a.last_exit ? <Muted>—</Muted> : null}</td>
              <td data-label="Control" data-control>{a.key ? <>
                <form method="post" action="/runner-control" class="inline-form">
                  <input type="hidden" name="name" value={rec.name} />
                  <input type="hidden" name="agent" value={a.key} />
                  {a.state === "stopped" ? <Button name="verb" value="start" icon="play">Start</Button> : null}
                  {a.state !== "stopped" && a.state !== "disabled" ? <Button name="verb" value="stop" icon="square">Stop</Button> : null}
                  {a.enabled ? <Button name="verb" value="restart" icon="rotate-cw">Restart</Button> : null}
                  {a.enabled ? <Button name="verb" value="disable" icon="pause">Disable</Button> : <Button name="verb" value="enable" icon="play">Enable</Button>}
                </form>
                <a href={settingsHref(rec.name, a.key)}>Settings</a></> : <Muted>—</Muted>}</td>
            </tr>;
          })}</tbody>
        </table></div></div>}
      {answer && agents.some(a => !visible.has(a.name)) ? <p class="muted small">An agent whose record you may not see shows only what the runner says about it.</p> : null}
    </Card>
    <Card title="Actions" icon="play">
      <form method="post" action="/runner-reload" class="inline-form">
        <input type="hidden" name="name" value={rec.name} />
        <Button icon="refresh-cw">Reload</Button>
      </form>
      <p class="muted small">Reload asks the runner to read every installed agent and its settings again, for the whole runner: a saved change takes effect only then. What the page then shows is the runner's own answer. It does not restart the runner itself, so it does not update an older runner.</p>
      <p class="muted small">Start, Stop and Restart last until the next reload or the runner's restart; Enable and Disable are written to the host's <code>runner/agents.json</code> and last. Stopping lets a script already running finish; messages sent meanwhile wait in the agent's inbox. Controls and settings are for the runner's Owner and Maintainers: the runner refuses anyone else.</p>
      {answer && agents.length && !agents.some(a => a.key) ? <p class="muted small" data-unavailable>This runner names no agent's key, so it is older than 0.8.210 and has no per-agent control or settings: restart it to update it.</p> : null}
      <p><a href={settingsHref(rec.name)}>The host's inventory</a>, <code>runner/agents.json</code>: which agents and instances run here, their names, sandbox and limits.</p>
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
  const o = await observe(ctx, rec.name, "reload", waits.reload);
  if ("answer" in o) return runnerPage(ctx, rec, o, { done: "Reloaded: the runner answered with the status below." });
  if (!o.uncertain) return runnerPage(ctx, rec, o, { warning: "The reload was not carried out." });
  const warning = `The reload was sent and its outcome is not known: ${o.problem} It may still take effect; check the status before reloading again.`;
  // The warning outlives a daemon that cannot draw the page around it: the
  // generic problem page would say nothing was changed.
  try { return await runnerPage(ctx, rec, await observe(ctx, rec.name, "status", waits.status), { warning }); }
  catch { return respond(ctx, { title: rec.name, section: "runners", signedIn: true }, <p class="warn" role="alert" data-runner-warning>{warning}</p>, 502); }
}

/** A call that changes the runner, and the page after it: the runner's answer,
 *  a refusal, or an outcome not known — never read as success. */
async function act(ctx: Ctx, rec: Rec, body: string, what: string, waitMs: number): Promise<{ o: Observed; note: { done?: string; warning?: string } }> {
  const o = await observe(ctx, rec.name, body, waitMs);
  if ("answer" in o) return { o, note: { done: `${what}: the runner answered with the status below.` } };
  if (!o.uncertain) return { o, note: { warning: `${what} was not carried out. ${o.problem}` } };
  return { o: await observe(ctx, rec.name, "status", waits.status).catch(() => o), note: { warning: `${what} was sent and its outcome is not known: ${o.problem} It may still take effect; check the status before asking again.` } };
}

/** start, stop, restart, enable or disable one agent. */
async function postControl(ctx: Ctx): Promise<Response> {
  const rec = await runnerFor(ctx);
  const verb = ctx.f("verb"), agent = ctx.f("agent").trim();
  if (!CONTROLS.includes(verb) || !KEY.test(agent)) throw new LocalProblem(400, "a control is start, stop, restart, enable or disable of one agent");
  const { o, note } = await act(ctx, rec, `${verb} ${agent}`, `${verb[0]!.toUpperCase()}${verb.slice(1)} ${agent}`, waits.control);
  return runnerPage(ctx, rec, o, note);
}

/** An agent's key, `<agent>` or `<agent>/<instance>`, as the runner makes them. */
const KEY = /^[A-Za-z0-9_][A-Za-z0-9._-]{0,63}(\/[A-Za-z0-9_][A-Za-z0-9._-]{0,63})?$/;

const settingsHref = (name: string, key?: string) => `/runner-settings?name=${encodeURIComponent(name)}${key ? `&key=${encodeURIComponent(key)}` : ""}`;

type FileRead = { file: string; exists: boolean; content: string } | { problem: string };

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
    if (a && typeof a === "object" && typeof a.exists === "boolean") return { file, exists: a.exists, content: String(a.content ?? "") };
  } catch {}
  return { problem: `The runner answered: ${r.body.slice(0, 500)}` };
}

/** The files of one agent or instance, or with no key the host's inventory:
 *  the author's read only, the host's each in a form of its own. */
async function settingsPage(ctx: Ctx, rec: Rec, key: string, note?: { done?: string; warning?: string }, kept?: { file: string; content: string }): Promise<Response> {
  const [st, all] = await Promise.all([ctx.status(), ctx.records()]);
  const dir = key.split("/")[0]!, instance = key.includes("/") ? key.slice(key.indexOf("/") + 1) : "";
  const author = key ? [`agent.d/${dir}/agent.json`, `agent.d/${dir}/env.dist`] : [];
  const host = !key ? ["runner/agents.json"]
    : [`runner/${dir}/env`, `runner/${dir}/agent.json`, ...(instance ? [`runner/${key}/env`, `runner/${key}/agent.json`] : [])];
  const read = await Promise.all([...author, ...host].map(f => readFile(ctx, rec.name, f)));
  const of = (f: string) => read[[...author, ...host].indexOf(f)]!;
  const back = href(rec.name);
  const what: Record<string, string> = {
    "agents.json": "Which agents and instances run here, and each one's name, tags, sandbox and limits.",
    "env": "Variables, one KEY=value per line. They hold secrets: whoever the runner admits can read them.",
    "agent.json": "An override of the author's processes, deadline, idle and depends; nothing else may be overridden.",
  };
  const record = all.find(r => r.name === `#${key}@${rec.name.slice(rec.name.indexOf("@") + 1)}`);
  const body = <>
    <PageHead icon={<Icon name="settings" />} title={key ? <code>{key}</code> : "Host inventory"} back={{ href: back, label: rec.name }}
      sub={<>On <code>{rec.name}</code>. Saving checks a file as the runner reads it, then applies it at once, as a reload does.</>} />
    {note?.warning ? <p class="warn" role="alert" data-runner-warning>{note.warning}</p> : null}
    {key ? <Card title="The author's files" icon="file">
      <p class="muted small">Shipped with the agent's code, read only: the host decides in its own files below.</p>
      {author.map(f => { const r = of(f); return <div data-file={f}><h3><code>{f}</code></h3>
        {"problem" in r ? <p class="warn">{r.problem}</p> : r.exists ? <pre>{r.content}</pre> : <Muted>absent</Muted>}</div>; })}
    </Card> : null}
    <Card title="The host's files" icon="settings">
      {host.map(f => { const r = of(f); const content = kept?.file === f ? kept.content : "problem" in r ? "" : r.content;
        return <form method="post" action="/runner-file" data-file={f}>
          <h3><code>{f}</code>{"problem" in r ? null : r.exists ? null : <> <Muted>absent</Muted></>}</h3>
          <p class="muted small">{what[f.slice(f.lastIndexOf("/") + 1)]}</p>
          {"problem" in r ? <p class="warn">{r.problem}</p> : <>
            <input type="hidden" name="name" value={rec.name} />
            <input type="hidden" name="key" value={key} />
            <input type="hidden" name="file" value={f} />
            <textarea name="content" rows="8" spellcheck="false" autocomplete="off">{"\n"}{content}</textarea>
            <Button icon="save">Save</Button></>}
        </form>; })}
    </Card>
    {key ? <Card title="Instances" icon="copy">
      {record ? <p><a href={recordHref(record, { return: settingsHref(rec.name, key) })}>Its record</a> holds the bus secret and configuration.</p> : null}
      {instance ? <p><a class="btn btn-danger" href={`/runner-instance-delete?name=${encodeURIComponent(rec.name)}&key=${encodeURIComponent(key)}`} data-delete><Icon name="trash" />Delete instance {key}…</a></p> : null}
      <form method="post" action="/runner-instance" class="inline-form">
        <input type="hidden" name="name" value={rec.name} /><input type="hidden" name="verb" value="create" />
        <label>New instance of <code>{dir}</code>: <input type="text" name="key" value={`${dir}/`} required /></label>
        <Button icon="plus">Create</Button>
      </form>
    </Card> : null}
  </>;
  return respond(ctx, { title: key || "Host inventory", section: "runners", signedIn: true, you: st.you, flash: note?.done }, body);
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
  if (verb === "create" && "answer" in o && note.done) return settingsPage(ctx, rec, key, { done: `Created ${key}: configure its env below.` });
  return runnerPage(ctx, rec, o, note);
}

/** What deleting an instance takes and leaves, and the key typed again to do it. */
async function confirmDelete(ctx: Ctx, rec: Rec, key: string, warning?: string): Promise<Response> {
  const st = await ctx.status();
  const body = <>
    <PageHead icon={<Icon name="trash" />} title={<>Delete <code>{key}</code></>} back={{ href: settingsHref(rec.name, key), label: key }} sub={<>On <code>{rec.name}</code></>} />
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
      <label>Type <code>{key}</code> to confirm: <input type="text" name="confirm" autocomplete="off" required /></label>
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
