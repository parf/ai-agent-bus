// The page frame: head, sidebar, top bar, main and footer. Every page is one
// document built here; a signed-out page gets the frame without navigation.
import { h, Fragment, type Child, Html, render } from "../jsx.ts";
import type { Ctx, Identity } from "../ctx.ts";
import { STYLESHEETS, SCRIPTS, LIBS, type Local } from "../assets.ts";
import { Icon, Logo, Avatar } from "./kit.tsx";
import { generated } from "../format.ts";

export type Section = "overview" | "agents" | "services" | "queues" | "pubsub" | "users" | "groups" | "activity" | "diagnostics" | "account" | "";

export const NAV: { key: Section; label: string; href: string; icon: string; keys: string }[] = [
  { key: "overview", label: "Overview", href: "/", icon: "layout-dashboard", keys: "g o" },
  { key: "agents", label: "Agents", href: "/agents", icon: "bot", keys: "g a" },
  { key: "services", label: "Services", href: "/services", icon: "satellite-dish", keys: "g s" },
  { key: "queues", label: "Queues", href: "/queues", icon: "inbox", keys: "g q" },
  { key: "pubsub", label: "PubSub", href: "/pubsub", icon: "megaphone", keys: "g p" },
  { key: "users", label: "Users", href: "/users", icon: "user-round", keys: "g u" },
  { key: "groups", label: "Groups", href: "/groups", icon: "users", keys: "g g" },
  { key: "activity", label: "Activity", href: "/activity", icon: "activity", keys: "g t" },
  { key: "diagnostics", label: "Diagnostics", href: "/diagnostics", icon: "scan-search", keys: "g d" },
];

export type Assets = { css: Local; js: Local; hero: Local; favicon: Local };
let ASSETS: Assets;
export const setAssets = (a: Assets) => { ASSETS = a; };
export const assets = () => ASSETS;

export type PageOptions = {
  title: string;
  section?: Section;
  you?: string;               // signed in: the account name (may be empty when status failed)
  signedIn?: boolean;
  charts?: boolean;           // load uPlot
  bodyClass?: string;
  flash?: string;
};

const Brand = ({ id }: { id: Identity | null }) => <a class="brand" href="/" aria-label="agent-bus home">
  <Logo />
  <span class="brand-text">
    <strong>AgentBus</strong>
    {id ? <span class="brand-meta">
      {id.version ? <span class="build-tip" tabindex="0" title={id.build_info ? `Build daemon ${id.build_info}` : undefined}>v{id.version}</span> : <span>v unavailable</span>}
      <span class="host">@ {id.host || "host unavailable"}</span>
    </span> : <span class="brand-meta">node unavailable</span>}
  </span>
</a>;

const Sidebar = ({ section, id }: { section: Section; id: Identity | null }) => <aside class="sidebar" aria-label="Sections">
  <Brand id={id} />
  <nav class="side-nav" aria-label="sections">
    {NAV.map(n => <a href={n.href} class="side-link" aria-current={n.key === section ? "page" : undefined} data-keys={n.keys}>
      <Icon name={n.icon} /><span class="side-label">{n.label}</span>
    </a>)}
  </nav>
  <div class="side-foot">
    <button type="button" class="side-collapse" data-action="collapse" aria-label="Collapse the sidebar"><Icon name="panel-left" /><span class="side-label">Collapse</span></button>
  </div>
</aside>;

const Topbar = ({ you, section }: { you: string; section: Section }) => <header class="topbar" aria-label="Site header">
  <button type="button" class="icon-btn menu-btn" data-action="drawer" aria-label="Open navigation"><Icon name="menu" /></button>
  <button type="button" class="search-btn" data-action="palette" aria-label="Search records, users and pages">
    <Icon name="search" /><span>Jump to…</span><kbd>⌘K</kbd>
  </button>
  <div class="topbar-right">
    <button type="button" class="icon-btn theme-btn" data-action="theme" aria-label="Switch theme"><Icon name="sun" className="when-dark" /><Icon name="moon" className="when-light" /></button>
    <form method="post" action="/signout" class="who">
      <a class="account-link" href="/account" aria-current={section === "account" ? "page" : undefined}>
        <Avatar name={you || "?"} size="sm" /><code>{you || "account"}</code>
      </a>
      <button type="submit" class="icon-btn" aria-label="Sign out" data-tooltip="Sign out"><Icon name="log-out" /></button>
    </form>
  </div>
</header>;

const Footer = ({ id, at }: { id: Identity | null; at: Date }) => <footer class="site-footer" aria-label="Node and build information">
  {id ? <>
    <span>Owner <code>{id.owner || "unavailable"}</code></span>
    <span>Uptime {id.up || "unavailable"}</span>
  </> : <span>Node information unavailable</span>}
  <span>Generated {generated(at)}</span>
</footer>;

function head(o: PageOptions) {
  const a = ASSETS;
  return <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width,initial-scale=1" />
    <meta name="color-scheme" content="dark light" />
    <title>{o.title} · agent-bus</title>
    <link rel="icon" href={a.favicon.path} type="image/svg+xml" />
    {STYLESHEETS.filter(s => o.charts || s !== LIBS.uplotCss).map(s => <link rel="stylesheet" href={s.url} integrity={s.integrity} crossorigin="anonymous" />)}
    <link rel="stylesheet" href={a.css.path} />
    {SCRIPTS.filter(s => o.charts || s !== LIBS.uplot).map(s => <script defer src={s.url} integrity={s.integrity} crossorigin="anonymous"></script>)}
    <script defer src={a.js.path}></script>
  </head>;
}

/** The theme the visitor chose; anything but these two follows the system. */
export function theme(ctx: Ctx): "dark" | "light" | undefined {
  const t = ctx.cookies["ab_theme"];
  return t === "dark" || t === "light" ? t : undefined;
}

/** Success messages a 303 may carry to the next page, by key; nothing else is ever shown. */
export const FLASH: Record<string, string> = {
  registered: "Registered.", saved: "Settings saved.", configured: "Configuration replaced.", deactivated: "Deactivated.",
  reactivated: "Reactivated.", transferred: "Ownership transferred.", removed: "Registration removed.", unsubscribed: "Your inbox is off the list.",
  "recipient-removed": "Recipient removed.", "profile-saved": "Profile saved.", "user-created": "User registered.",
  "user-deactivated": "User deactivated.", "user-reactivated": "User reactivated.", "credential-removed": "Credential removed.",
  "group-saved": "Group saved.", "group-registered": "Group registered.", "github-refreshed": "GitHub profile refreshed.",
  "email-saved": "Email saved.",
};

export async function respond(ctx: Ctx, o: PageOptions, body: Child, status = 200): Promise<Response> {
  const key = ctx.cookies["ab_flash"];
  if (key) ctx.setCookies.push(`ab_flash=; Path=/; HttpOnly; SameSite=Strict; Max-Age=0${ctx.tls ? "; Secure" : ""}`);
  const flash = o.flash ?? (key ? FLASH[key] : undefined);
  const res = new Response(await page(ctx, { ...o, flash }, body), { status, headers: { "Content-Type": "text/html; charset=utf-8" } });
  for (const c of ctx.setCookies) res.headers.append("Set-Cookie", c);
  return res;
}

/** 303 to a page that shows one allowlisted success message once. */
export function flashRedirect(ctx: Ctx, to: string, key: keyof typeof FLASH): Response {
  const res = new Response(null, { status: 303, headers: { Location: to } });
  res.headers.append("Set-Cookie", `ab_flash=${key}; Path=/; HttpOnly; SameSite=Strict; Max-Age=60${ctx.tls ? "; Secure" : ""}`);
  return res;
}

export async function page(ctx: Ctx, o: PageOptions, body: Child): Promise<string> {
  const id = await ctx.identity();
  const signedIn = o.signedIn ?? false;
  const doc = <html lang="en" data-theme={theme(ctx)} data-sidebar={ctx.cookies["ab_sidebar"] === "collapsed" ? "collapsed" : undefined}>
    {head(o)}
    <body class={`${signedIn ? "app" : "bare"} ${o.bodyClass ?? ""}`}>
      <a class="skip-link" href="#main">Skip to main content</a>
      {signedIn ? <Sidebar section={o.section ?? ""} id={id} /> : null}
      <div class="shell">
        {signedIn ? <Topbar you={o.you ?? ""} section={o.section ?? ""} /> : <header class="topbar bare-top" aria-label="Site header"><Brand id={id} />
          <button type="button" class="icon-btn theme-btn" data-action="theme" aria-label="Switch theme"><Icon name="sun" className="when-dark" /><Icon name="moon" className="when-light" /></button></header>}
        <main id="main" tabindex="-1">
          {o.flash ? <div class="toast" role="status" data-toast>{<Icon name="circle-check" />}<span>{o.flash}</span></div> : null}
          {body}
        </main>
        <Footer id={id} at={ctx.started} />
      </div>
    </body>
  </html>;
  return "<!doctype html>" + render(doc);
}

export { Html };
