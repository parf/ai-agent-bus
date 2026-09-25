// People pages: Users, a user, the profile editor, Groups, a group, its
// editor, Account, and their POST handlers (docs/web-face/people.md).
import { h, Fragment, type Child } from "../jsx.ts";
import { Ctx, NotFound, LocalProblem, Refusal, SignInRequired, type Rec, type UserRow, type Status } from "../ctx.ts";
import { respond, flashRedirect } from "../ui/frame.tsx";
import { Icon, Help, PageHead, Card, Name, Muted, KindIcon, KindPill, Pill, StatePill, Badge, Empty, Tabs, Segmented, Pager, Button, LinkButton, Avatar, recordHref } from "../ui/kit.tsx";
import { TextField, LinesField, SecretField, CheckField, ErrorSummary, FieldError, keep, terms, lines, type FormState } from "../ui/forms.tsx";
import { redirect, returnTo, json, local } from "../http.ts";
import { number, relative, minute, validTime } from "../format.ts";
import { formRefusal, lineRefusal, notYours } from "../problem.tsx";
import { authority, entityLabel, entity, DAEMON_OWNER, MAINTAINER } from "../glyphs.ts";
import { noun } from "./records.tsx";

const PAGE = 25;
const RETURNS = ["/users", "/diagnostics"];
const userReturn = (v: string) => returnTo(v, RETURNS, "/users");
const lc = (s: string) => s.toLowerCase();

/** Directory, records, and the visitor: what every user page loads. */
async function load(ctx: Ctx) {
  const [st, users, records] = await Promise.all([ctx.status(), ctx.users(), ctx.records()]);
  return { st, users, records };
}

function findRow(users: UserRow[], name: string) {
  return users.find(u => u.name === name) ?? users.find(u => lc(u.name) === lc(name));
}

function lastUsed(u: UserRow, records: Rec[]) {
  return records.find(r => r.name === u.name)?.last_used;
}

const Time = ({ iso, inactive }: { iso?: string; inactive?: boolean }) => {
  const d = validTime(iso);
  if (!d) return inactive ? <span class="muted" title="Not reported while inactive">—</span> : <span class="muted">never</span>;
  return <time datetime={d.toISOString()} title={minute(iso)}>{relative(iso)}</time>;
};

/** One pill: who this person is on the node. */
const IdentityPill = ({ u }: { u: UserRow }) => u.daemon_owner
  ? <span class="pill tone-accent"><Icon name={DAEMON_OWNER.icon} />{DAEMON_OWNER.word}</span>
  : u.administrator ? <span class="pill tone-info"><Icon name="shield" />Daemon administrator</span>
  : <span class="pill"><Icon name="kind:user" />User</span>;

// ------------------------------------------------------------------ /users

async function usersPage(ctx: Ctx): Promise<Response> {
  if (ctx.q("kind") === "other") return redirect("/diagnostics#leftovers");
  const { st, users, records } = await load(ctx);
  const q = ctx.q("q"), state = ["inactive", "all"].includes(ctx.q("state")) ? ctx.q("state") : "active";
  const people = users.filter(u => u.kind === "user");
  const needle = lc(q);
  const searched = people.filter(u => !needle || [u.name, u.person_name, u.email, u.github_user, u.github_company, u.github_location, u.github_twitter_username].filter(Boolean).join(" ").toLowerCase().includes(needle));
  const inState = (u: UserRow, s: string) => s === "all" || (s === "inactive" ? u.status === "inactive" : u.status !== "inactive");
  let rows = searched.filter(u => inState(u, state)).sort((a, b) => a.name.localeCompare(b.name));
  const matched = rows.length, pages = Math.max(1, Math.ceil(matched / PAGE));
  const pageNo = Math.min(pages, Math.max(1, Math.floor(Number(ctx.q("page"))) || 1));
  rows = rows.slice((pageNo - 1) * PAGE, pageNo * PAGE);
  const url = (p: Record<string, string | undefined>) => { const s = new URLSearchParams(); for (const [k, v] of Object.entries(p)) if (v) s.set(k, v); return s.size ? `/users?${s}` : "/users"; };
  const here = url({ q, state: state !== "active" ? state : undefined, page: pageNo > 1 ? String(pageNo) : undefined });
  const admin = !!st.administrator;
  const agents = (name: string) => records.filter(r => r.kind === "agent" && r.owner === name).length;
  const body = <>
    <PageHead icon={<Icon name="kind:user" />} title="Users"
      help={<Help label="About Users" title="Users" items={["A User is a person on this node; every record belongs to one.", "Active users call the bus; an inactive user's calls are refused and their names are struck through.", "Last used is when the user's own credential last reached the daemon.", "Counts are computed here from what the daemon lets you see."]} />}>
      {admin ? <LinkButton href="/users/new" tone="primary" icon="user-plus">Register user</LinkButton> : null}
    </PageHead>
    <Tabs label="Directory views" items={[{ href: "/users", text: "All", count: people.filter(u => inState(u, state)).length, current: true }, ...(admin ? [{ href: "/users/new", text: "Register user", className: "register", icon: "plus" }] : [])]} />
    <form class="toolbar" method="get" action="/users">
      <div class="search"><Icon name="search" /><label for="record-query" class="sr-only">Search users</label>
        <input id="record-query" type="search" name="q" value={q} placeholder="Search by name, identity, email or GitHub login" /></div>
      {state !== "active" ? <input type="hidden" name="state" value={state} /> : null}
      <Segmented label="Status" items={[["active", "Active"], ["inactive", "Inactive"], ["all", "All states"]].map(([v, t]) => ({
        href: url({ q, state: v === "active" ? undefined : v }), text: <>{t}<span class="count">{searched.filter(u => inState(u, v!)).length}</span></>, current: state === v }))} />
    </form>
    {!matched ? (q ? <Empty icon="search-x" title="No user matches this search" action={<a class="btn" href="/users">Clear the search</a>}>A User is a person on this node; every record belongs to one.</Empty>
      : state === "inactive" ? <Empty icon="kind:user" title="No inactive users">A User is a person on this node; every record belongs to one.</Empty>
      : <Empty icon="kind:user" title="No users yet" action={admin ? <LinkButton href="/users/new" tone="primary" icon="user-plus">Register a user</LinkButton> : undefined}>A User is a person on this node; every record belongs to one.</Empty>)
      : <>
        <div class="results-line"><span>Showing {(pageNo - 1) * PAGE + 1}–{Math.min(pageNo * PAGE, matched)} of {number(matched)} matching users.</span></div>
        <div class="table-wrap"><table class="data stack">
          <thead><tr><th>User</th><th>Authority</th><th>Contact</th><th class="num">Agents</th><th>Last used</th></tr></thead>
          <tbody>{rows.map(u => {
            const off = u.status === "inactive";
            const nameEl = <a href={`/user?${new URLSearchParams({ name: u.name, return: here })}`}><code>{u.name}</code></a>;
            return <tr class={off ? "inactive-record" : ""}>
              <td data-label="User"><div class="user-cell"><Avatar photo={u.photo_png} name={u.person_name || u.name} />
                <div class="who-text">
                  {u.person_name ? (off ? <s><strong>{u.person_name}</strong></s> : <strong>{u.person_name}</strong>) : null}
                  <span>{u.daemon_owner ? <Icon name={DAEMON_OWNER.icon} label="Daemon owner" className="kind-ic kind-owner" /> : null} {off ? <s>{nameEl}</s> : nameEl}{off ? <> <Badge>INACTIVE</Badge></> : null}</span>
                  {u.github_company ? <span class="muted small">{u.github_company}</span> : null}
                </div></div></td>
              <td data-label="Authority">{u.daemon_owner ? <span class="authority"><Icon name={DAEMON_OWNER.icon} className="kind-ic kind-owner" />{DAEMON_OWNER.word}</span> : authority(u.daemon_owner, u.administrator)}</td>
              <td data-label="Contact">{u.email || u.github_user ? <>{u.email ? <div><a href={`mailto:${u.email}`}>{u.email}</a></div> : null}{u.github_user ? <div class="muted small">GitHub <code>{u.github_user}</code></div> : null}</> : <Muted>—</Muted>}</td>
              <td class="num" data-label="Agents">{agents(u.name) ? number(agents(u.name)) : <Muted>0</Muted>}</td>
              <td data-label="Last used"><Time iso={lastUsed(u, records)} inactive={off} /></td>
            </tr>;
          })}</tbody>
        </table></div>
        <Pager label="Directory pages" prev={pageNo > 1 ? url({ q, state: state !== "active" ? state : undefined, page: String(pageNo - 1) }) : undefined}
          next={pageNo < pages ? url({ q, state: state !== "active" ? state : undefined, page: String(pageNo + 1) }) : undefined}>Page {pageNo} of {pages}</Pager>
      </>}
  </>;
  return respond(ctx, { title: "Users", section: "kind:group", signedIn: true, you: st.you }, body);
}

// ------------------------------------------------------------------ profile form

const PROFILE_KEEP = ["name", "person_name", "email", "github_user", "company", "location", "twitter", "return"];

function ProfileFields({ st, row, create }: { st: FormState; row?: UserRow; create: boolean }) {
  const v = (n: string, from?: string) => st.values[n] ?? from ?? "";
  return <div class="form-grid">
    {create ? <TextField name="name" label="Identity" st={st} errId="profile-error" required placeholder="user@realm" wide
      hint="The principal name used by AgentBus. It cannot be changed afterwards." />
      : <input type="hidden" name="name" value={row!.name} />}
    <TextField name="person_name" label="Person name" st={st} errId="profile-error" value={v("person_name", row?.person_name)} hint="The name shown to people." />
    <TextField name="email" label="Email" type="email" st={st} errId="profile-error" value={v("email", row?.email)} autocomplete="email" />
    <TextField name="github_user" label="GitHub login" st={st} errId="profile-error" value={v("github_user", row?.github_user)} hint="Setting or changing it attempts to import public values." />
    <TextField name="company" label="Company" st={st} errId="profile-error" value={v("company", row?.github_company)} />
    <TextField name="location" label="Location" st={st} errId="profile-error" value={v("location", row?.github_location)} />
    <TextField name="twitter" label="Twitter/X" st={st} errId="profile-error" value={v("twitter", row?.github_twitter_username)} placeholder="handle" />
  </div>;
}

async function registerUserPage(ctx: Ctx, st: FormState = { values: {} }, status = 200): Promise<Response> {
  const s = await ctx.status();
  if (!s.administrator) throw notYours("only a daemon Administrator can register a user");
  const body = <>
    <PageHead back={{ href: "/users", label: "Back to directory" }} icon={<Icon name="user-plus" />} title="Add user" />
    <ErrorSummary id="create" error={st.error} />
    <div class="detail-grid">
      <form id="form-create" class="card form-card" method="post" action="/user"><div class="card-body">
        <h2 class="section-title"><Icon name="id-card" />Profile</h2>
        <input type="hidden" name="action" value="create" /><input type="hidden" name="return" value="/users" />
        <ProfileFields st={st} create />
        <FieldError id="profile-error" error={st.error} />
        <div class="actions"><Button tone="primary" icon="check">Save profile</Button><a class="btn btn-ghost" href="/users">Cancel</a></div>
      </div></form>
      <Card title="SSH access" icon="key-round" actions={<Help label="About SSH access" title="SSH access" items={[<>On the host: <code>agent-bus-admin user add &lt;user@realm&gt; &lt;key.pub&gt;</code></>, "The key lets the person fetch their own token with ssh agent-busd@<node> token."]} />}>
        <p class="muted">Public keys are added on the host after the profile is saved.</p>
      </Card>
    </div>
  </>;
  return respond(ctx, { title: "Add user", section: "kind:group", signedIn: true, you: s.you }, body, status);
}

// ------------------------------------------------------------------ /user

function ownedRecords(row: UserRow, records: Rec[]): Rec[] {
  const names = new Set([...(row.services ?? []), ...records.filter(r => r.owner === row.name && r.kind !== "group" && r.name !== row.name).map(r => r.name)]);
  return records.filter(r => names.has(r.name) && r.name !== row.name).sort((a, b) => a.name.localeCompare(b.name));
}

const OwnedList = ({ recs }: { recs: Rec[] }) => recs.length
  ? <ul class="subs-list">{recs.map(r => <li><a class="rec-link" href={r.kind === "group" ? `/group?name=${encodeURIComponent(r.name)}` : recordHref(r)}><KindIcon kind={r.kind} /><code>{r.name}</code></a>
      <span>{r.status === "inactive" ? <Badge>INACTIVE</Badge> : null} <KindPill kind={r.kind} /></span></li>)}</ul>
  : <p class="muted">None</p>;

async function userPage(ctx: Ctx, stateOverride?: { st: FormState; status: number; section: string }): Promise<Response> {
  const name = ctx.q("name") || ctx.f("name");
  const { st, users, records } = await load(ctx);
  if (!name) { if (st.administrator) return registerUserPage(ctx); throw new NotFound(); }
  const row = findRow(users, name);
  if (!row) throw new NotFound();
  const ret = userReturn(ctx.q("return") || ctx.f("return"));
  const err = stateOverride?.st.error;
  if (row.kind !== "user") {
    const own = records.filter(r => r.owner === row.name || r.name === row.name);
    const body = <>
      <PageHead back={{ href: ret, label: "Back to directory" }} icon={<Icon name="id-card" />} title={<Name>{row.name}</Name>}
        sub={<><span class="pill">{row.kind === "record" ? "Self-owned record" : "Credential only"}</span><Muted>No user lifecycle state</Muted></>}
        help={<Help label="About credential removal" title="Credential removal" items={["Removing a credential stops it and every browser session made with it.", "No user or record is removed.", "Only an Administrator removes one, and only while no profile or other-owned record backs it."]} />} />
      <Card title={row.kind === "record" ? "Registered name" : "Credential only"} icon={row.kind === "record" ? "box" : "key-round"}>
        {row.kind === "record" ? <>
          <p>A self-owned record exists. {own[0] ? <a href={recordHref(own[0])}>Inspect its registration and queues</a> : null}</p>
          <OwnedList recs={own} />
        </> : <p>No registered record remains for this credential.</p>}
        {row.can_remove ? <form method="get" action="/credential-remove" class="actions"><input type="hidden" name="name" value={row.name} /><input type="hidden" name="return" value={ret} />
          <Button tone="danger" icon="key-round">Review credential removal…</Button></form> : null}
      </Card>
    </>;
    return respond(ctx, { title: row.name, section: "kind:group", signedIn: true, you: st.you }, body);
  }
  const off = row.status === "inactive";
  const canAccess = !!row.can_activate && !row.daemon_owner;
  const selfEmail = !!row.can_set_email && !row.can_edit && row.name === st.you;
  const profile: [string, Child][] = [];
  if (row.person_name) profile.push(["Person name", row.person_name]);
  if (row.email) profile.push(["Email", <a href={`mailto:${row.email}`}>{row.email}</a>]);
  if (row.github_user) profile.push(["GitHub login", <a href={`https://github.com/${encodeURIComponent(row.github_user)}`}>@{row.github_user}</a>]);
  if (row.github_company) profile.push(["Company", row.github_company]);
  if (row.github_location) profile.push(["Location", row.github_location]);
  if (row.github_twitter_username) profile.push(["Twitter/X", <a href={`https://x.com/${encodeURIComponent(row.github_twitter_username)}`}>@{row.github_twitter_username}</a>]);
  const emailSt: FormState = stateOverride?.section === "email" ? stateOverride.st : { values: {} };
  const body = <>
    <a class="back-link" href={ret}><Icon name="arrow-left" />Back to directory</a>
    <header class="page-head person-head">
      <Avatar photo={row.photo_png} name={row.person_name || row.name} size="lg" />
      <div>
        <h1>{row.person_name || row.name}</h1>
        <div class="person-sub"><Name copy>{row.name}</Name><IdentityPill u={row} /><StatePill inactive={off} /></div>
      </div>
      {row.can_edit ? <div class="page-actions"><LinkButton href={`/user/edit?${new URLSearchParams({ name: row.name, return: ret })}`} icon="pencil">Edit profile</LinkButton></div> : null}
    </header>
    <ErrorSummary id={stateOverride?.section ?? "save"} error={err} />
    <div class="detail-grid">
      <div>
        <Card title="Profile" icon="id-card" id="profile-edit">
          {profile.length ? <dl class="pairs">{profile.map(([k, v]) => <><dt>{k}</dt><dd>{v}</dd></>)}</dl> : <p class="muted">No profile details yet.</p>}
          {row.can_edit && row.github_user ? <form id="form-refresh-github" method="post" action="/user" class="actions">
            <input type="hidden" name="name" value={row.name} /><input type="hidden" name="return" value={ret} />
            <Button name="action" value="refresh-github" tone="ghost btn-sm" icon="refresh-cw">Refresh from GitHub</Button></form> : null}
          {!row.can_edit && !selfEmail ? <p class="muted small">Trusted profile fields are edited by a daemon administrator.</p> : null}
          {selfEmail ? <form id="form-email" method="post" action="/user" class="actions">
            <input type="hidden" name="name" value={row.name} /><input type="hidden" name="return" value={ret} />
            <TextField name="email" label="Your email" type="email" st={emailSt} errId="email-error" value={emailSt.values.email ?? row.email ?? ""} autocomplete="email" />
            <FieldError id="email-error" error={emailSt.error} />
            <Button name="action" value="email" icon="mail">Save email</Button></form> : null}
        </Card>
        <Card title="Owned records" icon="boxes"><OwnedList recs={ownedRecords(row, records)} /></Card>
      </div>
      <div>
        <Card title="Groups" icon="kind:group">
          {row.groups?.length ? <div class="chip-list">{row.groups.map(g => <a href={`/group?name=${encodeURIComponent(g)}`}><Icon name="kind:group" />{g}</a>)}</div> : <p class="muted">No memberships</p>}
        </Card>
        <Card title="Access" icon="power" actions={<Help label="About access" title="Access" items={["An active user's calls reach the bus; an inactive user's are refused.", "Deactivating keeps their records, queued work and tokens, and stops nothing already running.", "Only the daemon Owner reactivates an Administrator."]} />}>
          <p>Current: <StatePill inactive={off} /></p>
          {canAccess ? (off
            ? <form method="post" action="/user" class="actions"><input type="hidden" name="name" value={row.name} /><input type="hidden" name="return" value={`/user?${new URLSearchParams({ name: row.name, return: ret })}`} />
                <Button name="action" value="active" tone="primary" icon="power">Reactivate</Button></form>
            : <form method="get" action="/user-deactivate" class="actions"><input type="hidden" name="name" value={row.name} /><input type="hidden" name="return" value={ret} />
                <Button tone="ghost" icon="power">Deactivate…</Button></form>) : null}
        </Card>
      </div>
    </div>
  </>;
  return respond(ctx, { title: row.name, section: "kind:group", signedIn: true, you: st.you }, body, stateOverride?.status ?? 200);
}

async function editUserPage(ctx: Ctx, st: FormState = { values: {} }, status = 200): Promise<Response> {
  const name = ctx.q("name") || ctx.f("name");
  const { st: s, users } = await load(ctx);
  const row = findRow(users, name);
  if (!row) throw new NotFound();
  if (row.kind !== "user" || !row.can_edit) throw notYours("that profile cannot be edited by you");
  const ret = userReturn(st.values.return ?? ctx.q("return"));
  const back = `/user?${new URLSearchParams({ name: row.name, return: ret })}`;
  const body = <>
    <a class="back-link" href={back}><Icon name="arrow-left" />Back to {row.name}</a>
    <header class="page-head person-head"><Avatar photo={row.photo_png} name={row.person_name || row.name} size="lg" /><h1>Edit <Name>{row.name}</Name></h1></header>
    <ErrorSummary id="save" error={st.error} />
    <form id="form-save" class="card form-card" method="post" action="/user"><div class="card-body">
      <input type="hidden" name="action" value="save" /><input type="hidden" name="return" value={ret} />
      <ProfileFields st={st} row={row} create={false} />
      <FieldError id="profile-error" error={st.error} />
      <div class="actions"><Button tone="primary" icon="check">Save profile</Button><a class="btn btn-ghost" href={back}>Cancel</a></div>
    </div></form>
  </>;
  return respond(ctx, { title: `Edit ${row.name}`, section: "kind:group", signedIn: true, you: s.you }, body, status);
}

async function deactivateUserPage(ctx: Ctx): Promise<Response> {
  const { st, users } = await load(ctx);
  const row = findRow(users, ctx.q("name"));
  if (!row) throw new NotFound();
  if (row.kind !== "user" || row.daemon_owner || !row.can_activate || row.status === "inactive") throw notYours("that user cannot be deactivated by you in their current state");
  const ret = userReturn(ctx.q("return"));
  const back = `/user?${new URLSearchParams({ name: row.name, return: ret })}`;
  const body = <>
    <PageHead back={{ href: back, label: "Back to user" }} icon={<Icon name="triangle-alert" />} title="Confirm deactivation" />
    <Card className="confirm-card" tone="danger" title={<>Deactivate <code>{row.name}</code>?</>} icon="power">
      <ul><li>Their bus access stops, and the records they own become inactive.</li><li>Queued work and tokens are kept; running processes are not stopped.</li>
        <li>{row.administrator ? "Only the daemon Owner can reactivate this Administrator later." : "An authorized Administrator or the daemon Owner can reactivate this user later."}</li></ul>
      <form method="post" action="/user" class="actions"><input type="hidden" name="name" value={row.name} /><input type="hidden" name="return" value={back} />
        <Button name="action" value="inactive" tone="danger" icon="power">Deactivate user</Button><a class="btn btn-ghost" href={back}>Cancel</a></form>
    </Card>
  </>;
  return respond(ctx, { title: `Confirm deactivation · ${row.name}`, section: "kind:group", signedIn: true, you: st.you }, body);
}

async function credentialRemovePage(ctx: Ctx): Promise<Response> {
  const { st, users } = await load(ctx);
  const row = findRow(users, ctx.q("name"));
  if (!row) throw new NotFound();
  if (row.kind === "user" || !row.can_remove) throw notYours("that credential cannot be removed by you");
  const ret = userReturn(ctx.q("return"));
  const back = `/user?${new URLSearchParams({ name: row.name, return: ret })}`;
  const body = <>
    <PageHead back={{ href: back, label: "Back to identity" }} icon={<Icon name="triangle-alert" />} title="Confirm credential removal" />
    <Card className="confirm-card" tone="danger" title={<>Remove the credential for <code>{row.name}</code>?</>} icon="key-round">
      <ul><li>Its current and previous credentials stop authenticating.</li><li>Every browser session made with it ends.</li><li>No user or record is removed.</li></ul>
      <form method="post" action="/user" class="actions"><input type="hidden" name="name" value={row.name} /><input type="hidden" name="return" value={ret} />
        <Button name="action" value="remove-credential" tone="danger" icon="trash-2">Remove credential</Button><a class="btn btn-ghost" href={back}>Cancel</a></form>
    </Card>
  </>;
  return respond(ctx, { title: `Confirm credential removal · ${row.name}`, section: "kind:group", signedIn: true, you: st.you }, body);
}

async function postUser(ctx: Ctx): Promise<Response> {
  const action = ctx.f("action"), name = ctx.f("name").trim();
  const ret = (() => { const r = local(ctx.f("return")); return r.startsWith("/user?") ? r : userReturn(r); })();
  const s = await ctx.status();
  const profile = () => ({
    name, person_name: ctx.f("person_name"), email: ctx.f("email"), github_user: ctx.f("github_user"),
    github_company: ctx.f("company"), github_location: ctx.f("location"), github_twitter_username: ctx.f("twitter"), profile_details_set: true,
  });
  const field = (status: number, msg: string, create: boolean) => create && status === 412 ? "name" : /email/i.test(msg) ? "email" : create && /name/i.test(msg) ? "name" : undefined;
  switch (action) {
    case "create": {
      let made: UserRow;
      try { made = await ctx.bus<UserRow>("POST", "/user", { body: { ...profile(), create: true } }); }
      catch (e) {
        const r = formRefusal(e);
        if (!r?.preserve || !s.administrator) throw e;
        return registerUserPage(ctx, { values: keep(ctx.form, PROFILE_KEEP), error: { status: r.status, message: r.message, field: field(r.status, r.message, true) } }, r.status);
      }
      return flashRedirect(ctx, `/user?name=${encodeURIComponent(made?.name ?? lc(name))}`, "user-created");
    }
    case "save": {
      try { await ctx.bus("POST", "/user", { body: profile() }); }
      catch (e) {
        const r = formRefusal(e);
        if (!r?.preserve) throw e;
        return editUserPage(ctx, { values: keep(ctx.form, PROFILE_KEEP), error: { status: r.status, message: r.message, field: field(r.status, r.message, false) } }, r.status);
      }
      return flashRedirect(ctx, ret, "profile-saved");
    }
    case "email": {
      try { await ctx.bus("POST", "/profile", { body: { email: ctx.f("email") } }); }
      catch (e) {
        const r = formRefusal(e);
        if (!r?.preserve) throw e;
        return userPage(ctx, { st: { values: { email: ctx.f("email") }, error: { status: r.status, message: r.message, field: "email" } }, status: r.status, section: "email" });
      }
      return flashRedirect(ctx, `/user?name=${encodeURIComponent(name)}`, "email-saved");
    }
    case "inactive": case "active":
      await ctx.bus("POST", "/user/state", { body: { name, status: action } });
      return flashRedirect(ctx, ret, action === "inactive" ? "user-deactivated" : "user-reactivated");
    case "remove-credential":
      await ctx.bus("POST", "/identity/remove", { body: { name } });
      return flashRedirect(ctx, ret, "credential-removed");
    case "refresh-github": {
      try { await ctx.bus("POST", "/user/github-refresh", { body: { name } }); }
      catch (e) {
        const r = formRefusal(e);
        if (!r?.preserve) throw e;
        return userPage(ctx, { st: { values: {}, error: { status: r.status, message: r.message } }, status: r.status, section: "refresh-github" });
      }
      return flashRedirect(ctx, `/user?${new URLSearchParams({ name, return: userReturn(ctx.f("return")) })}`, "github-refreshed");
    }
  }
  throw new LocalProblem(400, "That user action is not available.");
}

// ------------------------------------------------------------------ groups

type Groups = Record<string, string[] | null>;

/** May this visitor change a group's membership? The daemon decides again. */
function mayEdit(name: string, rec: Rec | undefined, st: Status): boolean {
  if (name === "@administrators") return !!st.daemon_owner;
  return !!st.administrator || !!rec?.can_manage;
}

async function groupsPage(ctx: Ctx): Promise<Response> {
  const [st, groups, ls] = await Promise.all([ctx.status(), ctx.groups(), ctx.ls()]);
  const personal = ctx.q("personal") === "1";
  const recs = new Map(ls.filter(r => r.kind === "group").map(r => [r.name, r]));
  const all = Object.keys(groups).sort();
  const personalNames = all.filter(n => recs.get(n)?.personal);
  const names = personal ? personalNames : all.filter(n => !recs.get(n)?.personal);
  const body = <>
    <PageHead icon={<Icon name="kind:group" />} title="Groups"
      help={<Help label="About Groups" tip="Named sets of users, agents and other groups, used in allow lists and as Maintainers." title="Groups"
        items={["A group is a named set of users, agents and nested groups.", "@owner is ACL syntax for a record's Owner, not a group.", "Anyone may register a group and becomes its Owner.", "Its Owner, its Maintainers and the daemon Administrators change its membership.", "You see a group's details when you are in it or manage it."]} />}>
      <LinkButton href={personal ? "/groups/new?personal=1" : "/groups/new"} tone="primary" icon="plus">{personal ? "Register Personal group" : "Register group"}</LinkButton>
    </PageHead>
    <Tabs label="Group views" items={[{ href: "/groups", text: "All", count: all.length - personalNames.length, current: !personal },
      { href: "/groups?personal=1", text: "Personal", count: personalNames.length, current: personal, className: "personal-view", icon: "lock" }]} />
    {names.length ? <div class="table-wrap"><table class="data stack">
      <caption>{plural(names.length, "group")}</caption>
      <thead><tr><th>Group</th>{personal ? null : <th>Owner</th>}<th>Maintainers</th><th>Members</th></tr></thead>
      <tbody>{names.map(n => {
        const r = recs.get(n), members = groups[n];
        const visible = !!st.administrator || !!r;
        return <tr>
          <td data-label="Group"><a class="rec-link" href={`/group?name=${encodeURIComponent(n)}`}><KindIcon kind="group" /><code>{n}</code></a>{n === "@administrators" ? <> <Pill tone="accent">protected</Pill></> : null}
            {r?.descr ? <div class="muted small">{r.descr}</div> : null}</td>
          {personal ? null : <td data-label="Owner">{r ? <code>{r.owner}</code> : <Muted>Not visible to you</Muted>}</td>}
          <td data-label="Maintainers">{r ? (r.maintainers?.length ? <code>{r.maintainers.join(", ")}</code> : <Muted>None</Muted>) : <Muted>Not visible to you</Muted>}</td>
          <td data-label="Members">{visible && members ? (members.length ? <ul class="members">{members.map(m => <li><code>{m}</code></li>)}</ul> : <Muted>No members</Muted>) : <Muted>Not visible to you</Muted>}</td>
        </tr>;
      })}</tbody></table></div> : <Empty icon={personal ? "lock" : "kind:group"} title={personal ? "No Personal groups yet" : "No groups registered"}>{personal ? <>A Personal group is named for its Owner, <code>@{st.you}/…</code>, and lists only the Owner and the Owner's own agents.</> : undefined}</Empty>}
  </>;
  return respond(ctx, { title: personal ? "Groups · Personal" : "Groups", section: "groups", signedIn: true, you: st.you, personal }, body);
}
const plural = (n: number, w: string) => `${number(n)} ${w}${n === 1 ? "" : "s"}`;

/** Which visible records use a group, and how: directly, as Maintainers, or through an outer group. */
export function usedBy(name: string, records: Rec[], groups: Groups): { rec: Rec; uses: string[] }[] {
  const outers = new Map<string, string[]>();
  const seen = new Set<string>([name]);
  const queue = [name];
  while (queue.length) {
    const g = queue.shift()!;
    for (const [outer, members] of Object.entries(groups)) if ((members ?? []).includes(g) && !seen.has(outer)) { seen.add(outer); outers.set(outer, [...(outers.get(g) ?? []), outer]); queue.push(outer); }
  }
  const out: { rec: Rec; uses: string[] }[] = [];
  for (const r of records) {
    if (r.name === name) continue;
    const uses: string[] = [];
    const inList = r.kind === "group" ? "member" : "ACL";
    if ((r.allow ?? []).includes(name)) uses.push(inList);
    if ((r.maintainers ?? []).includes(name)) uses.push("Maintainers");
    for (const [outer] of outers) {
      if ((r.allow ?? []).includes(outer)) uses.push(`${inList} via ${outer}`);
      if ((r.maintainers ?? []).includes(outer)) uses.push(`Maintainers via ${outer}`);
    }
    if (uses.length) out.push({ rec: r, uses });
  }
  return out.sort((a, b) => a.rec.name.localeCompare(b.rec.name));
}

async function groupPage(ctx: Ctx): Promise<Response> {
  const name = ctx.q("name");
  const [st, groups, ls] = await Promise.all([ctx.status(), ctx.groups(), ctx.ls()]);
  const key = name in groups ? name : Object.keys(groups).find(k => lc(k) === lc(name));
  if (!key) throw new NotFound();
  const rec = ls.find(r => r.kind === "group" && r.name === key);
  const members = groups[key];
  const editable = mayEdit(key, rec, st);
  const protectedGroup = key === "@administrators";
  const used = usedBy(key, ls, groups);
  const body = <>
    <PageHead back={{ href: rec?.personal ? "/groups?personal=1" : "/groups", label: "Back to Groups" }} icon={<Icon name="kind:group" />} title={<Name copy>{key}</Name>}
      sub={<div class="meta-row">
        {protectedGroup ? <Pill tone="accent"><Icon name="shield" />protected</Pill> : null}
        {rec ? <>
          <span class="owner-chip"><Icon name="kind:user" /><span>Owner</span><code>{rec.owner}</code></span>
          <span class="pill"><Icon name={MAINTAINER.icon} />Maintainers: {protectedGroup ? "none — its Owner follows daemon ownership" : rec.maintainers?.length ? rec.maintainers.join(", ") : "none"}</span>
          {rec.personal ? <Pill tone="warn"><Icon name="lock" />Personal</Pill> : null}
        </> : <Muted>Owner and Maintainers are not visible to you.</Muted>}
      </div>}
      help={protectedGroup ? <Help label="About @administrators" title="@administrators" items={["Direct users only, never empty; the daemon Owner is always a member.", "Adding a name creates a User profile for it.", "Nesting it inside another group grants no authority."]} /> : undefined}>
      {editable ? <LinkButton href={`/group/edit?name=${encodeURIComponent(key)}`} icon="pencil" >Edit group</LinkButton> : null}
    </PageHead>
    {rec?.descr ? <p class="descr lead-descr group-description">{rec.descr}</p> : null}
    {protectedGroup ? <div class="grid-2">
      <Card title="What membership grants" icon="shield-check"><ul class="static-list"><li><strong>Ordinary users</strong>: register, edit, activate and deactivate them.</li><li><strong>Ordinary groups</strong>: change any group's membership.</li><li><strong>Membership lists</strong>: see every group's members.</li></ul></Card>
      <Card title="What it does not grant" icon="shield-x"><ul class="static-list"><li><strong>The Owner, or each other</strong>: only the daemon Owner changes an Administrator.</li><li><strong>Services, Queues and PubSub</strong>: records stay their owners'.</li><li><strong>This group</strong>: only the daemon Owner changes it.</li></ul></Card>
    </div> : null}
    <div class="detail-grid">
      <div>
        <Card title="Members" icon="kind:group" id="members-edit" actions={members ? <span class="count">{members.length}</span> : undefined}>
          {members ? (members.length ? <ul class="members">{members.map(m => <li><code>{m}</code></li>)}</ul> : <p class="muted">No members</p>) : <p class="muted">Membership is not visible to you.</p>}
          <p class="muted small">{protectedGroup ? "Only the daemon owner changes this protected group." : "Its Owner, its Maintainers and the daemon Administrators change this group's membership."}</p>
        </Card>
        <Card title="Used by visible records" icon="link">
          {used.length ? <table class="data stack"><thead><tr><th>Record</th><th>Kind</th><th>Uses this group</th></tr></thead>
            <tbody>{used.map(u => <tr><td data-label="Record"><a class="rec-link" href={u.rec.kind === "group" ? `/group?name=${encodeURIComponent(u.rec.name)}` : recordHref(u.rec)}><KindIcon kind={u.rec.kind} /><code>{u.rec.name}</code></a></td>
              <td data-label="Kind"><KindPill kind={u.rec.kind} /></td><td data-label="Uses">{u.uses.join(" · ")}</td></tr>)}</tbody></table>
            : <p class="muted">No caller-visible record refers to this group.</p>}
        </Card>
      </div>
      <div>
        {rec?.can_manage ? <a class="danger-link" href={`/service-danger?name=${encodeURIComponent(key)}`}><span><Icon name="flame" /> Danger Zone</span><span class="small">configuration{rec.can_transfer && !protectedGroup && !/^@[^/]+\//.test(key) ? ", transfer" : ""}</span></a> : null}
      </div>
    </div>
  </>;
  return respond(ctx, { title: `Group ${key}`, section: "groups", signedIn: true, you: st.you, personal: !!rec?.personal }, body);
}

const GROUP_KEEP = ["name", "members", "new", "descr", "maintainers", "personal"];

async function groupFormPage(ctx: Ctx, create: boolean, st: FormState = { values: {} }, status = 200): Promise<Response> {
  const [s, groups, ls] = await Promise.all([ctx.status(), ctx.groups(), ctx.ls()]);
  const name = create ? "" : (ctx.q("name") || ctx.f("name"));
  let rec: Rec | undefined, members: string[] | null = [];
  if (!create) {
    const key = name in groups ? name : Object.keys(groups).find(k => lc(k) === lc(name));
    if (!key) throw new NotFound();
    rec = ls.find(r => r.kind === "group" && r.name === key);
    if (!mayEdit(key, rec, s)) throw notYours("that group's membership cannot be changed by you");
    members = groups[key] ?? [];
  }
  const protectedGroup = name === "@administrators";
  const assign = create || (!protectedGroup && !!rec?.can_transfer);
  const v = (n: string, from: string) => st.values[n] ?? from;
  const personal = st.values.personal != null || st.values.new != null ? st.values.personal === "on" : create ? ctx.q("personal") === "1" : !!rec?.personal;
  const title = create ? "Register group" : `Edit ${name}`;
  const back = create ? "/groups" : `/group?name=${encodeURIComponent(name)}`;
  const body = <>
    <PageHead back={{ href: back, label: create ? "Back to Groups" : `Back to ${name}` }} icon={<Icon name="kind:group" />} title={create ? "Register group" : <>Edit <Name>{name}</Name></>} />
    <ErrorSummary id="save" error={st.error} />
    <form id="form-save" class="card form-card" method="post" action="/groups"><div class="card-body">
      <input type="hidden" name="action" value="save" />
      {create ? <input type="hidden" name="new" value="1" /> : <input type="hidden" name="name" value={name} />}
      <div class="form-grid">
        {create ? <TextField name="name" label="Name" st={st} errId="group-error" required placeholder={`@${s.you}/team or @operators`} wide
          hint={<>One name for the set. It cannot be changed afterwards. You become its Owner. A Personal group is named <code>@{s.you}/…</code>.{st.values.name === "@owner" ? " @owner is reserved for ACLs and cannot be a group." : ""}</>} /> : null}
        <TextField name="descr" label="Description" st={st} errId="group-error" value={v("descr", rec?.descr ?? "")} placeholder="What this group is for" wide
          disabled={!create && !rec} hint={!create && !rec ? "This group's record is not visible to you, so its description is left as it is." : "Shown beside the group's name."} />
        <LinesField name="members" label="Members" st={st} errId="group-error" rows={8} value={v("members", (members ?? []).join("\n"))} placeholder={"user@realm\n#agent@realm\n@nested-group"}
          hint="One user, #agent or nested group per line; @owner is reserved for ACLs." />
        <fieldset class="wide"><legend>Classification</legend>
          {assign ? <input type="hidden" name="edit_personal" value="1" /> : null}
          <CheckField name="personal" label="Personal" st={st} errId="group-error" checked={personal} disabled={!assign}
            hint={protectedGroup ? "The protected group is never Personal." : assign ? "Puts this group in its Owner's Personal view. A Personal group's members and Maintainers may name only its Owner and the Owner's own agents." : "Shown for reference: only this group's Owner or the daemon Owner may change it."} />
        </fieldset>
        {!create ? <>
          {assign ? <input type="hidden" name="edit_sharing" value="1" /> : null}
          <LinesField name="maintainers" label={<><Icon name={MAINTAINER.icon} />Maintainers</>} st={st} errId="group-error" disabled={!assign}
            value={v("maintainers", (rec?.maintainers ?? []).join("\n"))} placeholder={"user@realm\n@group"}
            hint={protectedGroup ? "The protected group has no Maintainers." : "One user, group or agent per line. They change this group's members as its Owner does."} />
        </> : null}
        <SecretField name="secret" label="Secret" st={st} errId="group-error" placeholder="TOKEN=..." disabled={!create && !rec?.can_manage}
          hint={create ? "Optional. Every member reads it back with agent-bus secret." : rec?.can_manage ? "Leave empty to keep the stored secret. Anything here replaces it." : "Only this group's Owner and Maintainers write its secret."} />
      </div>
      <FieldError id="group-error" error={st.error} />
      <div class="actions"><Button tone="primary" icon="check">{create ? "Register group" : "Save group"}</Button><a class="btn btn-ghost" href={back}>Cancel</a></div>
    </div></form>
  </>;
  return respond(ctx, { title, section: "groups", signedIn: true, you: s.you, personal: create ? ctx.q("personal") === "1" || st.values.personal === "on" : !!rec?.personal }, body, status);
}

async function postGroups(ctx: Ctx): Promise<Response> {
  if (ctx.f("action") !== "save") throw new LocalProblem(400, "That group action is not available.");
  const s = await ctx.status();
  const create = ctx.f("new") === "1";
  const name = ctx.f("name").trim();
  const members = terms(ctx.f("members")).sort();
  const personal = ctx.f("personal") === "on";
  const values = keep(ctx.form, GROUP_KEEP);
  const back = (message: string, status: number, field = create ? "name" : "members", line?: number) =>
    groupFormPage(ctx, create, { values, error: { message, status, field, line } }, status);
  const groups = await ctx.groups();
  const exists = Object.keys(groups).some(k => lc(k) === lc(name));
  if (create && exists) return back("that name is already registered", 409, "name");
  if (create && personal && !lc(name).startsWith(`@${lc(s.you)}/`)) return back(`A Personal group is named for its owner: call it @${s.you}/<name>.`, 400, "name");
  const lineHit = (msg: string) => {
    for (const f of ["members", ...(ctx.form!.has("edit_sharing") ? ["maintainers"] : [])]) {
      const n = lineRefusal(msg, lines(ctx.f(f)));
      if (n) return { field: f, line: n, message: msg.startsWith("Line ") ? msg : `Line ${n}: ${msg}` };
    }
    return undefined;
  };
  const refused = async (e: unknown) => {
    const r = formRefusal(e);
    if (!r) throw e;
    const hit = lineHit(r.message);
    if (!r.preserve && !hit) throw e;
    const field = hit?.field ?? (create || /personal group is named/.test(r.message) ? "name" : "members");
    if (!create) { ctx.forget("/groups"); ctx.forget("/ls"); }
    return back(hit?.message ?? r.message, r.status, field, hit?.line);
  };
  let stored = name;
  if (!create && exists && name !== "@administrators") {
    const change: Record<string, unknown> = { name, allow: members };
    if (ctx.form!.has("descr")) change.descr = ctx.f("descr");
    if (ctx.form!.has("edit_sharing")) change.maintainers = terms(ctx.f("maintainers"));
    if (ctx.form!.has("edit_personal")) change.personal = personal;
    try { await ctx.bus("POST", "/manage", { body: change }); } catch (e) { return refused(e); }
  } else {
    try { const g = await ctx.bus<{ name?: string; Name?: string }>("POST", "/group", { body: { Name: name, Members: members } }); stored = g?.name ?? g?.Name ?? lc(name); }
    catch (e) { return refused(e); }
    const second: Record<string, unknown> = { name: stored };
    if (ctx.f("descr") && (create || name === "@administrators")) second.descr = ctx.f("descr");
    if (create && personal) second.personal = true;
    if (Object.keys(second).length > 1) {
      try { await ctx.bus("POST", "/manage", { body: second }); }
      catch (e) {
        const reason = e instanceof Refusal ? e.detail : "the daemon did not answer";
        throw new LocalProblem(502, `The group ${stored} was registered, and its description or classification was not stored: ${reason} Change them on its settings page.`, true);
      }
    }
  }
  const secret = ctx.f("secret");
  if (secret) {
    try { await ctx.bus("POST", "/secret", { body: { name: stored, secret: secret.replace(/\r\n/g, "\n") } }); }
    catch (e) {
      const reason = e instanceof Refusal ? e.detail : "the daemon did not answer";
      throw new LocalProblem(502, `The group ${stored} was saved, and its secret was not stored: ${reason} Set it with: agent-bus secret ${stored} '...'`, true);
    }
  }
  return flashRedirect(ctx, `/group?name=${encodeURIComponent(stored)}`, create ? "group-registered" : "group-saved");
}

// ------------------------------------------------------------------ account

type Held = { name: string; kind?: string; owner?: string; fingerprint?: string; issued?: string; used?: string };

async function accountPage(ctx: Ctx): Promise<Response> {
  const [st, users, ls, inactive] = await Promise.all([ctx.status(), ctx.users(), ctx.ls(), ctx.inactive().catch(() => [] as Rec[])]);
  const names = await ctx.get<Held[]>("/names").then(n => ({ ok: n ?? [] }), e => ({ err: e instanceof Refusal ? e.detail : "the daemon did not answer" }));
  const me = users.find(u => u.kind === "user" && u.name === st.you);
  const own = ls.find(r => r.name === st.you);
  const owned = [...ls, ...inactive.filter(i => !ls.some(r => r.name === i.name))].filter(r => r.owner === st.you && r.name !== st.you).sort((a, b) => a.name.localeCompare(b.name));
  const creds = "ok" in names ? names.ok : [];
  const otherOwner = creds.some(c => c.owner && c.owner !== st.you);
  const body = <>
    <PageHead icon={<Icon name="id-card" />} title="Account"
      help={<Help label="About Account" title="Account" items={["Every fact here is the daemon's own answer about you.", "A fingerprint identifies a credential without revealing it.", "Rotation keeps the replaced credential valid until the next rotation."]} />} />
    <div class="detail-grid">
      <div>
        <Card title="Identity" icon="fingerprint">
          {me ? <div class="person-head">
            <Avatar photo={me.photo_png} name={me.person_name || me.name} size="lg" />
            <div><h2>{me.person_name || me.name}</h2>
              <div class="person-sub"><Name copy>{me.name}</Name><IdentityPill u={me} /><StatePill inactive={me.status === "inactive"} /></div>
              {me.groups?.length ? <div class="chip-list">{me.groups.map(g => <a href={`/group?name=${encodeURIComponent(g)}`}><Icon name="kind:group" />{g}</a>)}</div> : null}
              <p><a href={`/user?name=${encodeURIComponent(me.name)}`}>Open your profile</a></p>
            </div>
          </div> : own ? <><p><KindPill kind={own.kind} /> <Name copy>{own.name}</Name></p><p class="muted">This identity has a registered record and no person profile visible here.</p></>
            : <><p><Name copy>{st.you}</Name></p><p class="muted">No person profile or caller-visible identity record was returned for this identity.</p></>}
        </Card>
        <Card title="Credentials" icon="key-round" id="credentials">
          {"err" in names ? <p class="warn">Credentials unavailable: {names.err}</p> : !creds.length ? <p class="muted">You hold no credential.</p>
            : <div class="table-scroll"><table class="data stack"><thead><tr><th>Name</th><th>Kind</th>{otherOwner ? <th>Owner</th> : null}<th>Fingerprint</th><th>Issued</th><th>Last used</th></tr></thead>
              <tbody>{creds.map(c => <tr>
                <td data-label="Name"><code>{c.name}</code></td>
                <td data-label="Kind">{c.kind === "person" ? entityLabel("user") : c.kind === "unregistered" ? "Unregistered" : c.kind ? entityLabel(c.kind) : "—"}</td>
                {otherOwner ? <td data-label="Owner">{c.owner && c.owner !== st.you ? <code>{c.owner}</code> : <Muted>—</Muted>}</td> : null}
                <td data-label="Fingerprint"><code>{c.fingerprint}</code></td>
                <td data-label="Issued">{validTime(c.issued) ? minute(c.issued) : <Muted>unavailable</Muted>}</td>
                <td data-label="Last used">{validTime(c.used) ? <time datetime={c.used} title={minute(c.used)}>{relative(c.used)}</time> : <Muted>not this run</Muted>}</td>
              </tr>)}</tbody></table></div>}
          <div class="card-note"><h3><Icon name="rotate-cw" />Rotate your identity credential</h3>
            <p><code>agent-bus-token {st.you} --rotate</code></p><p class="muted small">The replaced credential remains valid until the next rotation.</p></div>
        </Card>
      </div>
      <div>
        <Card title="Owned records" icon="boxes"><OwnedList recs={owned} /></Card>
      </div>
    </div>
  </>;
  return respond(ctx, { title: "Account", section: "account", signedIn: true, you: st.you }, body);
}

// ------------------------------------------------------------------ palette data

/** What the ⌘K palette offers: only names this visitor's own answers contain. */
async function paletteJson(ctx: Ctx): Promise<Response> {
  const site = ctx.req.headers.get("sec-fetch-site");
  if (site !== "same-origin" && site !== "none") return json({ error: "same-origin request required" }, 403);
  if (!ctx.signedIn) return json({ error: "sign in required" }, 401);
  try {
    const [records, groups, users] = await Promise.all([ctx.records(), ctx.groups(), ctx.users().catch(() => [] as UserRow[])]);
    const entries = [
      ...records.filter(r => r.kind !== "group" && r.kind !== "user").map(r => ({ title: r.name, hint: r.descr ?? noun(r.kind), href: recordHref(r), icon: entity(r.kind)?.icon ?? "box", glyph: entity(r.kind)?.glyph, group: "Records" })),
      ...users.filter(u => u.kind === "user").map(u => ({ title: u.name, hint: u.person_name ?? "", href: `/user?name=${encodeURIComponent(u.name)}`, icon: "kind:user", glyph: entity("user")!.glyph, group: "Users" })),
      ...Object.keys(groups).map(g => ({ title: g, hint: "", href: `/group?name=${encodeURIComponent(g)}`, icon: "kind:group", glyph: entity("group")!.glyph, group: "Groups" })),
    ];
    return json({ entries });
  } catch (e) {
    if (e instanceof Refusal) return json({ error: e.detail }, e.status);
    return json({ error: "the daemon did not answer" }, 502);
  }
}

export const handlers = {
  users: usersPage, newUser: (ctx: Ctx) => registerUserPage(ctx), user: (ctx: Ctx) => userPage(ctx), editUser: (ctx: Ctx) => editUserPage(ctx),
  deactivateUser: deactivateUserPage, credentialRemove: credentialRemovePage, postUser,
  groups: groupsPage, newGroup: (ctx: Ctx) => groupFormPage(ctx, true), group: groupPage, editGroup: (ctx: Ctx) => groupFormPage(ctx, false), postGroups,
  account: accountPage, palette: paletteJson,
};
