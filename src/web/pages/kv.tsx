// The key-value pages: every store the visitor may use (/kv), one record's
// store (/kv/record) and one value (/kv/value), with the forms that write
// them. The daemon decides who may; these pages only ask
// (docs/01-identity-and-authority.md#key-value-store).
import { h, Fragment } from "../jsx.ts";
import { Ctx, LocalProblem } from "../ctx.ts";
import { respond, flashRedirect } from "../ui/frame.tsx";
import { Icon, Help, PageHead, Card, KindIcon, Muted, Empty, Segmented, Button, recordHref } from "../ui/kit.tsx";
import { TextField, TextAreaField, FieldError, ErrorSummary, type FormState } from "../ui/forms.tsx";
import { formRefusal } from "../problem.tsx";
import { entity } from "../glyphs.ts";
import { number } from "../format.ts";
import { KV_KINDS, KV_TITLE, Preview, preview, storeHref, valueHref, type KVKind, type KVListing } from "../ui/kv.tsx";

const RECORD_KINDS = ["agent", "service", "queue", "pubsub", "resource", "group", "user"];
const HELP = ["A store of string, integer and JSON values on each registry record, durable across restarts.",
  "Its Owner, Maintainers and own Agent may read and write it; nobody else sees it.",
  "Each kind is its own namespace: a string and an integer may share a name."];

const isKind = (k: string): k is KVKind => (KV_KINDS as string[]).includes(k);
const kindWord = (k: string) => k === "pubsub" ? "PubSub" : `${entity(k)?.word ?? k}s`;
const size = (n: number) => n < 1024 ? `${n} B` : `${(n / 1024).toFixed(n < 10240 ? 1 : 0)} KiB`;

function listUrl(p: Record<string, string | undefined>): string {
  const q = new URLSearchParams(Object.entries(p).filter(([, v]) => v) as [string, string][]);
  return q.size ? `/kv?${q}` : "/kv";
}

// ------------------------------------------------------------------ /kv

async function kvPage(ctx: Ctx): Promise<Response> {
  const [st, all] = await Promise.all([ctx.status(), ctx.kvStores()]);
  const present = RECORD_KINDS.filter(k => all.some(r => r.kind === k));
  const q = ctx.q("q").trim(), kind = present.includes(ctx.q("kind")) ? ctx.q("kind") : "";
  const needle = q.toLowerCase();
  const rows = all.filter(r => (!kind || r.kind === kind) && (!needle || r.record.toLowerCase().includes(needle)));
  const clear = listUrl({});
  const body = <>
    <PageHead icon={<Icon name="database" />} title="KV" help={<Help id="kv-help" label="About KV" title="Key-value stores" items={HELP} />} />
    <form class="toolbar record-search" method="get" action="/kv">
      <div class="search"><Icon name="search" /><label for="kv-query" class="sr-only">Search stores</label>
        <input id="kv-query" type="search" name="q" value={q} placeholder="Search by record" /></div>
      {kind ? <input type="hidden" name="kind" value={kind} /> : null}
    </form>
    {present.length > 1 ? <div class="toolbar filters">
      <Segmented label="Kind" items={[{ href: listUrl({ q }), text: "All", current: !kind }, ...present.map(k => ({ href: listUrl({ q, kind: k }), text: kindWord(k), current: kind === k }))]} />
    </div> : null}
    {all.length === 0 ? <Empty icon="database" title="No values stored">A store appears here once a record you may use holds a value.</Empty>
      : rows.length === 0 ? <Empty icon="filter" title="No stores match these filters" action={<a class="btn" href={clear}>Clear filters</a>}>Change the search or the kind above, or clear filters.</Empty>
      : <>
        <div class="results-line"><span>Showing {number(rows.length)} of {number(all.length)} stores on records you may use.</span>{q || kind ? <a href={clear}>Clear filters</a> : null}</div>
        <div class="table-wrap"><div class="table-scroll"><table class="data stack record-table">
          <thead><tr><th>Record</th><th>Strings</th><th>Integers</th><th>JSON</th><th><span class="sr-only">Actions</span></th></tr></thead>
          <tbody>{rows.map(r => <tr>
            <td data-label="Record"><div class="rec-cell"><KindIcon kind={r.kind} /><a href={recordHref({ name: r.record, kind: r.kind })}><code>{r.record}</code></a></div></td>
            <td data-label="Strings">{number(r.string)}</td>
            <td data-label="Integers">{number(r.int)}</td>
            <td data-label="JSON">{number(r.json)}</td>
            <td data-label="Actions"><a class="btn btn-sm" href={storeHref(r.record)}>Open</a></td>
          </tr>)}</tbody>
        </table></div></div>
      </>}
    <Card title="Open a record's store" icon="folder-open">
      <form method="get" action="/kv/record" class="inline-form">
        <label for="kv-open" class="sr-only">Record name</label>
        <input id="kv-open" name="name" required placeholder="#agent@team, jobs@team, @group@team" spellcheck="false" autocomplete="off" />
        <button class="btn">Open</button>
      </form>
      <p class="muted small">Any record you own or maintain, or your own Agent's — including one whose store is still empty.</p>
    </Card>
  </>;
  return respond(ctx, { title: "KV", section: "kv", signedIn: true, you: st.you }, body);
}

// ------------------------------------------------------------------ /kv/record

const HOWS: [string, string][] = [["set", "Set: write it"], ["add", "Add: only if absent"], ["replace", "Replace: only if present"]];

function AddForm({ record, st }: { record: string; st: FormState }) {
  const v = (n: string, d = "") => st.values[n] ?? d;
  return <form id="form-kv-add" method="post" action="/kv-set" class="form-grid">
    <input type="hidden" name="record" value={record} /><input type="hidden" name="return" value="store" />
    <div class="field"><label for="f-kind">Kind</label>
      <select id="f-kind" name="kind">{KV_KINDS.map(k => <option value={k} selected={v("kind", "string") === k}>{KV_TITLE[k]}</option>)}</select></div>
    <TextField name="name" label="Name" st={st} errId="kv-error" required placeholder="progress" />
    <div class="field"><label for="f-how">Mode</label>
      <select id="f-how" name="how">{HOWS.map(([k, t]) => <option value={k} selected={v("how", "set") === k}>{t}</option>)}</select></div>
    <TextAreaField name="value" label="Value" st={st} errId="kv-error" rows={4} placeholder='text, 42, or {"cursor": 1200}' hint="A JSON value is an object; an integer is a whole number." />
    <FieldError id="kv-error" error={st.error} />
    <div class="actions"><Button tone="primary" icon="plus">Add value</Button></div>
  </form>;
}

const DeleteValue = ({ record, kind, name, back }: { record: string; kind: KVKind; name: string; back: string }) =>
  <details class="inline-confirm"><summary class="btn btn-sm btn-danger">Delete</summary>
    <form method="post" action="/kv-delete" class="inline-form">
      <input type="hidden" name="record" value={record} /><input type="hidden" name="kind" value={kind} />
      <input type="hidden" name="name" value={name} /><input type="hidden" name="return" value={back} />
      <button class="btn btn-sm btn-danger">Confirm delete of <code>{name}</code></button>
    </form>
  </details>;

async function storePage(ctx: Ctx, st: FormState = { values: {} }, status = 200): Promise<Response> {
  const record = ctx.q("name") || ctx.f("record");
  if (!record) throw new LocalProblem(400, "Name the record whose store to open.");
  // A record the visitor may not use is the daemon's refusal, on the problem page.
  const [you, kv] = await Promise.all([ctx.you(), ctx.kvList(record)]);
  const body = <>
    <PageHead back={{ href: "/kv", label: "Back to KV" }} icon={<Icon name="database" />} title={<><KindIcon kind={kv.kind} /> <code>{kv.record}</code></>}
      sub={<a href={recordHref({ name: kv.record, kind: kv.kind })}>Open the record</a>}
      help={<Help id="kv-store-help" label="About this store" title="Key-value store" items={HELP} />} />
    <ErrorSummary id="kv-add" error={st.error} />
    {KV_KINDS.every(k => !(kv.values[k] ?? []).length) ? <Empty icon="database" title="No values yet">Add the first one below.</Empty> : null}
    {KV_KINDS.filter(k => (kv.values[k] ?? []).length).map(k => {
      const entries = kv.values[k] ?? [];
      return <Card title={KV_TITLE[k]} icon={k === "int" ? "hash" : k === "json" ? "braces" : "type"} id={`kv-${k}`} actions={<span class="count">{entries.length}</span>}>
        <table class="data stack kv-table">
          <thead><tr><th>Name</th>{k === "int" ? null : <th>Size</th>}<th>Value</th><th><span class="sr-only">Actions</span></th></tr></thead>
          <tbody>{entries.map(e => <tr>
            <td data-label="Name"><code>{e.name}</code></td>
            {k === "int" ? null : <td data-label="Size">{size(e.size)}</td>}
            <td data-label="Value"><Preview kind={k} e={e} /></td>
            <td data-label="Actions" class="row-actions">
              {e.preview_base64 !== undefined ? <Muted>binary; set through the CLI or API</Muted> : <a class="btn btn-sm" href={valueHref(kv.record, k, e.name)}>Edit</a>}
              <DeleteValue record={kv.record} kind={k} name={e.name} back="store" />
            </td>
          </tr>)}</tbody>
        </table>
      </Card>;
    })}
    <Card title="Add value" icon="plus" id="kv-add"><AddForm record={kv.record} st={st} /></Card>
  </>;
  return respond(ctx, { title: `KV ${kv.record}`, section: "kv", signedIn: true, you }, body, status);
}

// ------------------------------------------------------------------ /kv/value

async function valuePage(ctx: Ctx, st: FormState = { values: {} }, status = 200): Promise<Response> {
  const record = ctx.q("record") || ctx.f("record"), kind = ctx.q("kind") || ctx.f("kind"), name = ctx.q("name") || ctx.f("name");
  if (!record || !name || !isKind(kind)) throw new LocalProblem(400, "Name the record, the kind and the value to open.");
  const [you, got] = await Promise.all([ctx.you(), ctx.kvGet(record, kind, name)]);
  const binary = got.value_base64 !== undefined;
  const shown = st.values.value ?? (kind === "json" ? JSON.stringify(got.value, null, 2) : String(got.value ?? ""));
  const hidden = <><input type="hidden" name="record" value={record} /><input type="hidden" name="kind" value={kind} />
    <input type="hidden" name="name" value={name} /><input type="hidden" name="return" value="value" /></>;
  const body = <>
    <PageHead back={{ href: storeHref(record), label: `Back to ${record}` }} icon={<Icon name="database" />} title={<><code>{name}</code></>}
      sub={<><Muted>{KV_TITLE[kind]}</Muted> in <a href={storeHref(record)}><code>{record}</code></a></>} />
    <ErrorSummary id="kv-value" error={st.error} />
    <Card title="Value" icon="pencil" id="kv-value">
      {binary ? <>
        <p>These bytes are not text, shown as base64. Change them through the CLI or API.</p>
        <pre class="kv-value"><code>{got.value_base64}</code></pre>
      </> : <form id="form-kv-value" method="post" action="/kv-set">
        {hidden}<input type="hidden" name="how" value="replace" />
        {kind === "int"
          ? <TextField name="value" label="Integer" st={st} errId="kv-error" value={shown} autocomplete="off" />
          : <TextAreaField name="value" label={kind === "json" ? "JSON object" : "Text"} st={st} errId="kv-error" value={shown} rows={kind === "json" ? 14 : 6} />}
        <FieldError id="kv-error" error={st.error} />
        <div class="actions"><Button tone="primary" icon="check">Save</Button></div>
      </form>}
      {kind === "int" ? <form method="post" action="/kv-inc" class="actions">{hidden}
        <Button name="n" value="1" icon="plus">Add 1</Button><Button name="n" value="-1" icon="minus">Subtract 1</Button>
      </form> : null}
    </Card>
    <Card title="Delete" icon="trash-2"><DeleteValue record={record} kind={kind} name={name} back="store" /></Card>
  </>;
  return respond(ctx, { title: `${name} · ${record}`, section: "kv", signedIn: true, you }, body, status);
}

// ------------------------------------------------------------------ posts

/** A refusal the visitor can correct keeps the form, the value typed in. */
const correctable = (e: unknown) => { const r = formRefusal(e); return r && (r.preserve || r.status === 413) ? r : undefined; };

async function postSet(ctx: Ctx): Promise<Response> {
  const record = ctx.f("record"), kind = ctx.f("kind"), name = ctx.f("name").trim(), how = ctx.f("how") || "set", raw = ctx.f("value");
  if (!isKind(kind)) throw new LocalProblem(400, "A value is a string, an integer or JSON.");
  const again = (status: number, message: string) => {
    const st: FormState = { values: { kind, name, how, value: raw }, error: { status, message, field: "value" } };
    return ctx.f("return") === "value" ? valuePage(ctx, st, status) : storePage(ctx, st, status);
  };
  let value: unknown = raw;
  if (kind === "int") {
    // Sent as its digits: JSON.rawJSON keeps an integer past 2^53 exact.
    if (!/^\s*-?\d+\s*$/.test(raw)) return again(400, "An integer is a whole number, like 42 or -7.");
    value = (JSON as unknown as { rawJSON(s: string): unknown }).rawJSON(raw.trim());
  } else if (kind === "json") {
    try { value = JSON.parse(raw); } catch { return again(400, "That is not JSON."); }
  }
  try { await ctx.bus("POST", "/kv/set", { body: { record, kind, name, value, how } }); }
  catch (e) { const r = correctable(e); if (!r) throw e; return again(r.status, r.message); }
  // Saved is done: back to the store, where the value shows in its list. Only
  // a refusal stays on the form it came from.
  return flashRedirect(ctx, storeHref(record), "kv-saved");
}

async function postInc(ctx: Ctx): Promise<Response> {
  const record = ctx.f("record"), name = ctx.f("name"), n = ctx.f("n") === "-1" ? -1 : 1;
  await ctx.bus("POST", "/kv/inc", { body: { record, name, n } });
  return flashRedirect(ctx, valueHref(record, "int", name), "kv-saved");
}

async function postDelete(ctx: Ctx): Promise<Response> {
  const record = ctx.f("record"), kind = ctx.f("kind"), name = ctx.f("name");
  await ctx.bus("POST", "/kv/delete", { body: { record, kind, name } });
  return flashRedirect(ctx, storeHref(record), "kv-deleted");
}

export const handlers = { kv: kvPage, store: (ctx: Ctx) => storePage(ctx), value: (ctx: Ctx) => valuePage(ctx), postSet, postInc, postDelete };
export { preview };
