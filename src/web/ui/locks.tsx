// A record's locks, for its Owner, its Maintainers and its own Agent: the
// daemon refuses anybody else, and then the card is absent
// (docs/01-identity-and-authority.md#shared-locks).
import { h, Fragment } from "../jsx.ts";
import { Ctx, Refusal } from "../ctx.ts";
import { Card, Pill, detailPath } from "./kit.tsx";
import { flashRedirect } from "./frame.tsx";
import { left } from "../format.ts";

type Locks = Record<string, { holder: string; expires: string }>;

/** The record's locks, or null when the caller may not use them. */
export async function recordLocks(ctx: Ctx, record: string): Promise<Locks | null> {
  return ctx.holders(record).then(h => h?.locks ?? {}).catch(e => { if (e instanceof Refusal) return null; throw e; });
}

export const LocksCard = ({ record, locks, you }: { record: string; locks: Locks; you: string }) => <Card title="Locks" icon="lock" id="locks">
  {Object.keys(locks).length === 0 ? <p class="muted">No locks held.</p> : (
    <table class="data stack"><thead><tr><th>Name</th><th>Holder</th><th>Time left</th><th><span class="sr-only">Actions</span></th></tr></thead>
      <tbody>{Object.entries(locks).sort(([a], [b]) => a < b ? -1 : 1).map(([ln, l]) => {
        const mine = l.holder === you;
        return <tr>
          <td data-label="Name"><code>{ln}</code></td>
          <td data-label="Holder"><code>{l.holder}</code>{mine ? <> <Pill tone="accent">you</Pill></> : null}</td>
          <td data-label="Time left">{left(new Date(l.expires).getTime() - Date.now())}</td>
          <td data-label="Actions"><LockActions record={record} name={ln} holder={l.holder} you={you} /></td>
        </tr>;
      })}</tbody>
    </table>
  )}
  <p class="muted small">Memory only: a restart releases every lock. The record's Owner, Maintainers and own Agent may use them.</p>
</Card>;

/** Release for one's own lock; for another's, Force release behind a confirmation. */
export const LockActions = ({ record, name, holder, you, back }: { record: string; name: string; holder: string; you: string; back?: string }) => {
  const hidden = <><input type="hidden" name="record" value={record} /><input type="hidden" name="name" value={name} />{back ? <input type="hidden" name="return" value={back} /> : null}</>;
  return holder === you ? <form method="post" action="/release-lock" class="inline-form">
    {hidden}<button class="btn btn-sm">Release</button>
  </form> : <details class="inline-confirm"><summary class="btn btn-sm btn-danger">Force release</summary>
    <form method="post" action="/release-lock" class="inline-form">
      {hidden}<input type="hidden" name="force" value="1" />
      <p class="small">Releases <code>{holder}</code>'s lock. The audit log records it.</p>
      <button class="btn btn-sm btn-danger">Confirm force release of <code>{name}</code></button>
    </form>
  </details>;
};

/** POST /release-lock: back to the record's page, or the Locks page it came from, the daemon deciding. */
export async function postReleaseLock(ctx: Ctx): Promise<Response> {
  const record = ctx.f("record"), name = ctx.f("name"), force = ctx.f("force") === "1";
  // A refusal is the daemon's to say, on the error page like any other.
  await ctx.bus("POST", force ? "/release-force" : "/release", { body: { record, name } });
  const back = ctx.f("return") === "/locks" ? "/locks" : record.startsWith("@") ? `/group?name=${encodeURIComponent(record)}#locks` : await ctx.lookup(record).then(r => r ? `${detailPath(r.kind)}?name=${encodeURIComponent(record)}#locks` : "/").catch(() => "/");
  return flashRedirect(ctx, back, force ? "lock-force-released" : "lock-released");
}
