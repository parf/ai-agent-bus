// A record's key-value store, for its Owner, its Maintainers and its own
// Agent: the daemon refuses anybody else, and then the card is absent
// (docs/01-identity-and-authority.md#key-value-store).
import { h, Fragment } from "../jsx.ts";
import { Ctx } from "../ctx.ts";
import { Refusal } from "../daemon.ts";
import { Card, Muted } from "./kit.tsx";
import { number } from "../format.ts";

export type KVKind = "string" | "int" | "json";
export const KV_KINDS: KVKind[] = ["string", "int", "json"];
export const KV_TITLE: Record<KVKind, string> = { string: "Strings", int: "Integers", json: "JSON" };

export type KVEntry = { name: string; size: number; preview?: unknown; preview_base64?: string };
export type KVListing = { record: string; kind: string; values: Record<KVKind, KVEntry[] | null> };
export type KVStoreRow = { record: string; kind: string; string: number; int: number; json: number };

/** The record's store, or null when the visitor may not use it. */
export async function recordKV(ctx: Ctx, record: string): Promise<KVListing | null> {
  return ctx.kvList(record).catch(e => { if (e instanceof Refusal) return null; throw e; });
}

export const storeHref = (record: string) => `/kv/record?name=${encodeURIComponent(record)}`;
export const valueHref = (record: string, kind: KVKind, name: string) =>
  `/kv/value?${new URLSearchParams({ record, kind, name })}`;

/** A value as a listing shows it: the int itself, text as text, other bytes as base64. */
export function preview(kind: KVKind, e: KVEntry): string {
  if (kind === "int") return String(e.preview ?? "");
  if (e.preview_base64 !== undefined) return `base64:${e.preview_base64}`;
  return typeof e.preview === "string" ? e.preview : JSON.stringify(e.preview ?? "");
}

/** Whether a listed value is shown only in part. */
export const cut = (kind: KVKind, e: KVEntry) =>
  kind !== "int" && e.preview_base64 === undefined && e.size > new TextEncoder().encode(preview(kind, e)).length;

export const Preview = ({ kind, e }: { kind: KVKind; e: KVEntry }) =>
  <code class="kv-preview">{preview(kind, e).slice(0, 120)}{cut(kind, e) || preview(kind, e).length > 120 ? "…" : ""}</code>;

const SHOWN = 5;

/** The overview on a record's page: how much of each kind, the first names, and the way in. */
export const KVCard = ({ kv }: { kv: KVListing }) => {
  const total = KV_KINDS.reduce((n, k) => n + (kv.values[k]?.length ?? 0), 0);
  return <Card title="Key-value store" icon="database" id="kv" actions={<a class="btn btn-sm" href={storeHref(kv.record)}>Open store</a>}>
    {total === 0 ? <p class="muted">No values yet. <a href={storeHref(kv.record)}>Add one</a>.</p> : <>
      <p class="kv-counts">{KV_KINDS.map(k => <span><strong>{number(kv.values[k]?.length ?? 0)}</strong> {KV_TITLE[k].toLowerCase()}</span>)}</p>
      <table class="data stack kv-table"><thead><tr><th>Name</th><th>Kind</th><th>Value</th></tr></thead>
        <tbody>{KV_KINDS.flatMap(k => (kv.values[k] ?? []).slice(0, SHOWN).map(e => <tr>
          <td data-label="Name"><a href={valueHref(kv.record, k, e.name)}><code>{e.name}</code></a></td>
          <td data-label="Kind"><Muted>{k}</Muted></td>
          <td data-label="Value"><Preview kind={k} e={e} /></td>
        </tr>))}</tbody>
      </table>
      {KV_KINDS.some(k => (kv.values[k]?.length ?? 0) > SHOWN) ? <p class="muted small"><a href={storeHref(kv.record)}>All {number(total)} values</a></p> : null}
    </>}
    <p class="muted small">Stored and durable. The record's Owner, Maintainers and own Agent may use it.</p>
  </Card>;
};
