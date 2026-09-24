// Every component on one page, served only with AGENT_BUS_WEB_DEV=1: what the
// owner approves the look on, and where a style change is seen everywhere at once.
import { h, Fragment } from "../jsx.ts";
import type { Ctx } from "../ctx.ts";
import { respond } from "../ui/frame.tsx";
import { Icon, Help, PageHead, Card, Name, Muted, KindIcon, KindPill, Pill, StatePill, Badge, Empty, Tabs, Segmented, Pager, Button, LinkButton, Avatar, Facts, Figure } from "../ui/kit.tsx";
import { TextField, LinesField, SecretField, SelectField, CheckField, ErrorSummary, FieldError } from "../ui/forms.tsx";
import { Ribbon, Spark, DayChart, type Slot } from "../ui/charts.tsx";
import { ENTITY } from "../glyphs.ts";

const TOKENS = ["surface-0", "surface-1", "surface-2", "surface-3", "surface-4", "text-1", "text-2", "text-3", "accent", "accent-2", "indigo", "sky", "ok", "warn", "bad", "info"];

function slots(): Slot[] {
  const start = Date.UTC(2026, 8, 23, 4, 0);
  return Array.from({ length: 144 }, (_, i) => {
    const w = Math.max(0, Math.round(6 * Math.sin(i / 12) + 4 * Math.sin(i / 5) + (i % 37 === 0 ? 12 : 0)));
    return { at: new Date(start + i * 600_000).toISOString(), in: w, out: Math.max(0, w - (i % 3)), dropped: i % 50 === 7 ? 2 : 0, expired: i % 61 === 3 ? 1 : 0, refused: i % 40 === 11 ? 3 : 0 };
  });
}

export async function styleguide(ctx: Ctx): Promise<Response> {
  const s = slots();
  const err = { status: 404, message: "Line 2: no such name: group member nosuchuser", field: "members", line: 2 };
  const body = <>
    <PageHead icon={<Icon name="palette" />} title="Style guide" help={<Help label="About the style guide" tip="Every component, both themes: switch with the sun and moon above." />}
      sub="Development only. Every component the pages are built from, in the current theme.">
      <Button tone="ghost" type="button" icon="sun">Secondary</Button><LinkButton href="#" tone="primary" icon="plus">Primary action</LinkButton>
    </PageHead>
    <Card title="Colour tokens" icon="swatch-book"><div class="sg-swatches">{TOKENS.map(t => <div class={`sg-swatch sg-${t}`}><span></span><code>--{t}</code></div>)}</div></Card>
    <div class="grid-2">
      <Card title="Type" icon="type">
        <h1>Heading one</h1><h2>Heading two</h2><h3>Heading three</h3>
        <p>Body text reads calm and dense. <strong>Strong</strong>, <a href="#">a link</a>, <code>#claude/triage@dev</code>, <kbd>⌘K</kbd>.</p>
        <p class="muted">Muted supporting text.</p><p class="warn">A warning sentence, the daemon's own.</p>
      </Card>
      <Card title="Pills and badges" icon="tag">
        <div class="chips">{Object.keys(ENTITY).map(k => <KindPill kind={k} />)}<KindPill kind="user" owner /></div>
        <p class="chips"><StatePill /><StatePill inactive /><Pill tone="ok">ok</Pill><Pill tone="warn">warn</Pill><Pill tone="bad">bad</Pill><Pill tone="info">info</Pill><Pill tone="accent">accent</Pill><Badge>INACTIVE</Badge><Badge tone="warn">at capacity when observed</Badge></p>
        <p class="chips"><Name copy>jobs@dev</Name><Avatar name="Alice" /><Avatar name="Bob" size="sm" /><Muted>—</Muted> <Figure n={0} /> <Figure n={12345} /></p>
      </Card>
    </div>
    <Card title="Navigation" icon="compass">
      <Tabs label="Example tabs" items={[{ href: "#", text: "All", count: 12, current: true }, { href: "#", text: "My", count: 3 }, { href: "#", text: "Personal", count: 1, icon: "lock" }, { href: "#", text: "Register agent", className: "register", icon: "plus" }]} />
      <div class="toolbar"><Segmented label="Status" items={[{ href: "#", text: "All", current: true }, { href: "#", text: "Active" }, { href: "#", text: "Inactive" }]} />
        <a class="chip" href="#"><Icon name="bot" />Agents holding work</a></div>
      <Pager label="Example pages" prev="#" next="#">Page 2 of 5</Pager>
    </Card>
    <div class="node-strip">
      {[["Readers", "radio", 3], ["Queued", "layers", 128], ["Agents", "bot", 7], ["Services", "satellite-dish", 0]].map(([l, i, n]) =>
        <div class="node-fact tile"><span class="tile-label"><Icon name={String(i)} />{l}</span><strong class="tile-value"><Figure n={Number(n)} /></strong><Spark values={s.map(x => x.in)} label="trend" /></div>)}
    </div>
    <div class="attention-list">
      <article class="attention-item attention-red"><div class="attention-mark"><Icon name="octagon-alert" /></div><div class="attention-text"><h3>Queue at capacity when observed</h3><p><code>jobs@dev</code> · 5 held now · at capacity · when full: refuse</p></div><a class="attention-link" href="#">View record<Icon name="chevron-right" /></a></article>
      <article class="attention-item attention-orange"><div class="attention-mark"><Icon name="triangle-alert" /></div><div class="attention-text"><h3>Messages were lost from this inbox</h3><p><code>#summarizer@dev</code> · 2 dropped</p></div><a class="attention-link" href="#">View record<Icon name="chevron-right" /></a></article>
      <article class="attention-item attention-blue"><div class="attention-mark"><Icon name="info" /></div><div class="attention-text"><h3>Requests were refused</h3><p><code>acl</code> · 4 since this daemon started</p></div><a class="attention-link" href="#">View refusal reasons<Icon name="chevron-right" /></a></article>
    </div>
    <p></p>
    <Card title="Charts" icon="activity">
      <Ribbon slots={s} label="example ribbon" />
      <DayChart slots={s} id="sg-chart" />
    </Card>
    <div class="grid-2">
      <Card title="Facts" icon="gauge"><Facts rows={[["Readers", "0"], ["Held now", "5"], ["Oldest held", "4m31s"], ["Accepted", "1,204"]]} /></Card>
      <div class="table-wrap"><table class="data stack"><thead><tr><th>Name</th><th>Type</th><th class="num">Held</th></tr></thead>
        <tbody><tr><td data-label="Name"><a class="rec-link" href="#"><KindIcon kind="queue" /><code>jobs@dev</code></a></td><td data-label="Type"><KindPill kind="queue" /></td><td class="num" data-label="Held">5</td></tr>
          <tr class="personal-record"><td data-label="Name"><a class="rec-link" href="#"><KindIcon kind="agent" /><code>#mine@dev</code></a></td><td data-label="Type"><KindPill kind="agent" /></td><td class="num" data-label="Held">0</td></tr>
          <tr class="inactive-record"><td data-label="Name"><a class="rec-link" href="#"><KindIcon kind="service" /><code>old@dev</code></a> <Badge>INACTIVE</Badge></td><td data-label="Type"><KindPill kind="service" /></td><td class="num" data-label="Held">0</td></tr></tbody></table></div>
    </div>
    <ErrorSummary id="save" error={err} />
    <form id="form-save" class="card form-card" action="#"><div class="card-body">
      <div class="form-grid">
        <TextField name="name" label="Name" st={{ values: { name: "#agent@dev" } }} errId="sg-err" required hint="The routing identity callers use." help={{ label: "About the name", tip: "It cannot be changed afterwards." }} />
        <SelectField name="overflow" label="When full" st={{ values: {} }} errId="sg-err" options={[["strict", "Refuse new messages"], ["ring", "Drop the oldest"]]} />
        <LinesField name="members" label="Members" st={{ values: { members: "alice\nnosuchuser\nbob" }, error: err }} errId="sg-err" hint="One per line." />
        <SecretField name="secret" label="Secret" st={{ values: {} }} errId="sg-err" placeholder="TOKEN=..." hint="Never shown again." />
        <CheckField name="personal" label="Personal" st={{ values: {} }} errId="sg-err" checked hint="Puts this record in its Owner's Personal view." />
      </div>
      <FieldError id="sg-err" error={err} />
      <div class="actions"><Button tone="primary" type="button" icon="check">Save</Button><Button tone="danger" type="button" icon="trash-2">Remove</Button><Button type="button">Cancel</Button></div>
    </div></form>
    <Card title="Danger" tone="danger" icon="flame"><p>A red-rimmed card holds every irreversible action.</p></Card>
    <Empty icon="inbox" title="No queues yet" action={<LinkButton href="#" tone="primary" icon="plus">Register a queue</LinkButton>}>A queue holds what was sent until one reader takes it.</Empty>
    <section class="problem"><div class="problem-mark"><Icon name="triangle-alert" /></div><p class="problem-code">403</p><h1>Not yours to see</h1><p class="warn">only the owner or an assigned Maintainer can change this record's settings</p><p class="advice">You are signed in, and refused for lack of permission.</p></section>
  </>;
  return respond(ctx, { title: "Style guide", signedIn: true, you: "styleguide", charts: true, flash: "Toasts look like this." }, body);
}
