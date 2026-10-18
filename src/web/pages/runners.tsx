// /runners and /runner: the runners the visitor may see, each one's status
// asked of the runner itself, and its agents linked to their own pages
// (Plans/R1.0-Release/runner/web-face.md#goal). The face is a client of the
// runner and nothing more: every observation is the runner's answer, every
// action a call the runner's ACL decides, and no record is edited here.
import { h, Fragment } from "../jsx.ts";
import { Ctx, LocalProblem, NotFound, Refusal, Unreachable, type CallResult, type Rec } from "../ctx.ts";
import { respond } from "../ui/frame.tsx";
import { Icon, Help, PageHead, Card, Facts, Pill, Empty, Muted, Button, KindIcon, DocumentationLink, recordHref } from "../ui/kit.tsx";
import { redirect } from "../http.ts";
import { number, relative } from "../format.ts";

/** A runner is `#runner/<instance>@<host>` (Plans/R1.0-Release/DECISIONS.md#recorded-decisions, RD84); a tag never makes one. */
export const isRunner = (r: { name: string; kind: string }) => r.kind === "agent" && /^#runner\/[^/@]+@[^/@]+$/.test(r.name);

/** How long each call waits for the runner, in ms; tests shorten them. */
export const waits = { status: 10_000, chooser: 3_000, reload: 30_000 };

type AgentRow = { name: string; enabled: boolean; state: string; reason?: string; processes?: { running: number; min: number; max: number } };
type Answer = { runner?: string; version?: string; error?: string; agents?: AgentRow[] };

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
        ["Asked", (o as { at: Date }).at.toISOString().replace("T", " ").slice(0, 19) + " UTC"],
        ...(answer.error ? [["Load error", <span class="warn" data-load-error>{answer.error}</span>] as [string, any]] : []),
      ]} /> : <p class="warn" data-runner-problem>{(o as { problem: string }).problem}</p>}
      <p><a href={recordHref(rec, { return: back })}>The runner's own record</a> holds its description, access list and Maintainers.</p>
    </Card>
    <Card title="Agents" icon="kind:agent" id="agents">
      {!answer ? <Muted>Not known: the runner did not answer, so its agents are not listed rather than shown as none.</Muted>
        : agents.length === 0 ? <Muted>The runner serves no agents.</Muted>
        : <div class="table-wrap"><div class="table-scroll"><table class="data stack record-table">
          <thead><tr><th>Agent</th><th>State</th><th>Processes</th><th>Inbox</th><th>Last used</th><th>Reason</th></tr></thead>
          <tbody>{agents.map(a => {
            const r = visible.get(a.name);
            return <tr data-agent={a.name}>
              <td data-label="Agent"><div class="rec-cell"><KindIcon kind="agent" />{r ? <a href={recordHref(r, { return: back })}><code>{a.name}</code></a> : <code>{a.name}</code>}</div>
                {r?.descr ? <div class="muted small">{r.descr}</div> : !r ? <div class="muted small">its record is not visible to you</div> : null}
                {tagsOf(r).length ? <div class="small">{tagsOf(r).map(t => <><Pill>{t}</Pill> </>)}</div> : null}</td>
              <td data-label="State"><Pill tone={STATE_TONE[a.state] ?? ""}>{a.state}</Pill>{!a.enabled && a.state !== "disabled" ? <div class="muted small">not enabled</div> : null}</td>
              <td data-label="Processes">{a.processes ? <span title={`min ${a.processes.min}, max ${a.processes.max}`}>{number(a.processes.running)} of {number(a.processes.max)}</span> : <Muted>—</Muted>}</td>
              <td data-label="Inbox" data-inbox>{r ? <>{number(r.queued)} queued · {number(r.readers)} reading{r.at_bound ? <> <Pill tone="warn">full</Pill></> : null}</> : <Muted>—</Muted>}</td>
              <td data-label="Last used">{r?.last_used ? <span title={r.last_used}>{relative(r.last_used)}</span> : <Muted>—</Muted>}</td>
              <td data-label="Reason">{a.reason ? <span data-reason>{a.reason}</span> : <Muted>—</Muted>}</td>
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
      <p class="muted small" data-unavailable>Starting and stopping one agent is not built yet: the runner has no per-agent control, so the face offers none.</p>
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

export const handlers = { runners: runnersPage, runner, postReload };
