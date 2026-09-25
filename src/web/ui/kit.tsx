// The component kit. Views compose these; the stylesheet styles these class
// names and nothing else, so a new page looks like the old ones for free.
import { h, Fragment, type Child, Html, raw } from "../jsx.ts";
import { entity, identity as identityMark, kindGlyph } from "../glyphs.ts";
import { number } from "../format.ts";

/** A Lucide icon, drawn by the pinned library; the box is sized before it renders. */
export const Icon = ({ name, label, className }: { name: string; label?: string; className?: string }) => {
  // A kind is drawn with its shared glyph, the one the CLI prints.
  const g = kindGlyph(name);
  if (g) return label
    ? <span class={`glyph ${className ?? ""}`} role="img" aria-label={label}>{g.glyph}</span>
    : <span class={`glyph ${className ?? ""}`} aria-hidden="true">{g.glyph}</span>;
  return label
    ? <i class={`ic ${className ?? ""}`} data-lucide={name} role="img" aria-label={label}></i>
    : <i class={`ic ${className ?? ""}`} data-lucide={name} aria-hidden="true"></i>;
};

export const KindIcon = ({ kind, owner }: { kind: string; owner?: boolean }) => {
  const e = identityMark(kind, !!owner);
  return e ? <Icon name={e.icon} label={e.word} className={`kind-ic kind-${owner ? "owner" : kind}`} /> : <></>;
};

/** Kind pill: icon and the display word, e.g. "Agent". */
export const KindPill = ({ kind, owner }: { kind: string; owner?: boolean }) => {
  const e = identityMark(kind, !!owner);
  return <span class={`pill kind-pill kind-${owner ? "owner" : kind}`}>{e ? <Icon name={e.icon} /> : null}{e ? e.word : kind}</span>;
};

export const Pill = ({ tone, children, dot }: { tone?: string; dot?: boolean; children?: Child }) =>
  <span class={`pill ${tone ? "tone-" + tone : ""}`}>{dot ? <span class="dot" aria-hidden="true"></span> : null}{children}</span>;

export const StatePill = ({ inactive }: { inactive?: boolean }) =>
  inactive ? <Pill tone="muted" dot>Inactive</Pill> : <Pill tone="ok" dot>Active</Pill>;

export const Badge = ({ children, tone }: { children?: Child; tone?: string }) => <span class={`badge ${tone ? "tone-" + tone : ""}`}>{children}</span>;

export const Name = ({ children, copy }: { children?: Child; copy?: boolean }) =>
  copy ? <span class="name-copy"><code class="name">{children}</code><button type="button" class="copy-btn" data-copy={String(children)} aria-label="Copy name"><Icon name="copy" /></button></span>
       : <code class="name">{children}</code>;

export const Muted = ({ children }: { children?: Child }) => <span class="muted">{children}</span>;

/** Zero is a muted dash, as the node strip shows it. */
export const Figure = ({ n }: { n?: number }) => (n ?? 0) === 0 ? <span class="muted">—</span> : <>{number(n)}</>;

let helpSeq = 0;
/** ⓘ button: tooltip on hover or focus, a popover on click. */
export const Help = ({ id, label, tip, title, items }: { id?: string; label: string; tip?: string; title?: string; items?: Child[] }) => {
  const pid = id ?? `help-${++helpSeq}`;
  return <>
    <button type="button" class="help-button" popovertarget={pid} aria-label={label} data-tooltip={tip ?? label}><Icon name="circle-help" /></button>
    <div popover id={pid} class="context-help">
      {title ? <h2>{title}</h2> : null}
      {items?.length ? <ul>{items.map(i => <li>{i}</li>)}</ul> : null}
    </div>
  </>;
};

export const PageHead = ({ icon, title, help, children, sub, back }: {
  icon?: Child; title: Child; help?: Html; sub?: Child; back?: { href: string; label: string }; children?: Child;
}) => <header class="page-head">
  {back ? <a class="back-link" href={back.href}><Icon name="arrow-left" />{back.label}</a> : null}
  <div class="page-title">
    <h1>{icon ? <span class="title-mark" aria-hidden="true">{icon}</span> : null}<span class="title-text">{title}</span></h1>
    {help ?? null}
    {children ? <div class="page-actions">{children}</div> : null}
  </div>
  {sub ? <div class="page-sub">{sub}</div> : null}
</header>;

export const Card = ({ title, icon, children, className, id, actions, tone }: {
  title?: Child; icon?: string; children?: Child; className?: string; id?: string; actions?: Child; tone?: string;
}) => <section class={`card ${tone ? "card-" + tone : ""} ${className ?? ""}`} id={id}>
  {title ? <header class="card-head"><h2>{icon ? <Icon name={icon} /> : null}{title}</h2>{actions ? <div class="card-actions">{actions}</div> : null}</header> : null}
  <div class="card-body">{children}</div>
</section>;

export const Facts = ({ rows }: { rows: [Child, Child][] }) =>
  <dl class="facts">{rows.map(([k, v]) => <div class="fact"><dt>{k}</dt><dd>{v}</dd></div>)}</dl>;

export const Empty = ({ icon, title, children, action }: { icon?: string; title: Child; children?: Child; action?: Child }) =>
  <section class="empty">
    <div class="empty-mark" aria-hidden="true"><Icon name={icon ?? "sparkles"} /></div>
    <h2>{title}</h2>
    {children ? <p>{children}</p> : null}
    {action ?? null}
  </section>;

export const Tabs = ({ label, items }: { label: string; items: { href: string; text: Child; count?: number; current?: boolean; className?: string; icon?: string }[] }) =>
  <nav class="tabs" aria-label={label}>
    {items.map(t => <a href={t.href} class={`tab ${t.className ?? ""}`} aria-current={t.current ? "true" : undefined}>
      {t.icon ? <Icon name={t.icon} /> : null}{t.text}{t.count != null ? <span class="count">{number(t.count)}</span> : null}
    </a>)}
  </nav>;

export const Segmented = ({ label, items }: { label: string; items: { href: string; text: Child; current?: boolean }[] }) =>
  <div class="segmented" role="group" aria-label={label}>
    <span class="seg-label">{label}</span>
    {items.map(i => <a href={i.href} aria-current={i.current ? "true" : undefined}>{i.text}</a>)}
  </div>;

export const Pager = ({ label, prev, next, children }: { label: string; prev?: string; next?: string; children?: Child }) =>
  <nav class="pager" aria-label={label}>
    {prev ? <a href={prev} rel="prev"><Icon name="chevron-left" />Previous page</a> : <span></span>}
    <span class="pager-info">{children}</span>
    {next ? <a href={next} rel="next">Next page<Icon name="chevron-right" /></a> : <span></span>}
  </nav>;

export const Button = ({ children, tone, name, value, type, icon, formaction }: {
  children?: Child; tone?: string; name?: string; value?: string; type?: string; icon?: string; formaction?: string;
}) => <button type={type ?? "submit"} class={`btn ${tone ? "btn-" + tone : ""}`} name={name} value={value} formaction={formaction}>{icon ? <Icon name={icon} /> : null}{children}</button>;

export const LinkButton = ({ href, children, tone, icon }: { href: string; children?: Child; tone?: string; icon?: string }) =>
  <a href={href} class={`btn ${tone ? "btn-" + tone : ""}`}>{icon ? <Icon name={icon} /> : null}{children}</a>;

export const Avatar = ({ photo, name, size }: { photo?: string; name: string; size?: "lg" | "sm" }) =>
  photo ? <img class={`avatar ${size ?? ""}`} src={`data:image/png;base64,${photo}`} alt="" width={size === "lg" ? 72 : 32} height={size === "lg" ? 72 : 32} />
        : <span class={`avatar initial ${size ?? ""}`} aria-hidden="true">{(name.replace(/^[^a-zA-Z0-9]+/, "")[0] ?? "?").toUpperCase()}</span>;

/** The bus logo, fixed markup. */
export const Logo = () => raw(`<svg class="logo" viewBox="0 0 64 48" aria-hidden="true"><defs><linearGradient id="lg-bus" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#ff6b5b"/><stop offset="1" stop-color="#d42a22"/></linearGradient></defs><rect x="4" y="4" width="56" height="34" rx="7" fill="url(#lg-bus)"/><rect x="9" y="9" width="11" height="9" rx="2" fill="#dff3ff"/><rect x="23" y="9" width="11" height="9" rx="2" fill="#dff3ff"/><rect x="37" y="9" width="11" height="9" rx="2" fill="#dff3ff"/><rect x="51" y="9" width="5" height="15" rx="1.5" fill="#dff3ff"/><rect x="9" y="22" width="39" height="3" rx="1.5" fill="#ffd6cf" opacity=".7"/><circle cx="17" cy="39" r="5.5" fill="#1b1e27" stroke="#c9ced8" stroke-width="2"/><circle cx="47" cy="39" r="5.5" fill="#1b1e27" stroke="#c9ced8" stroke-width="2"/></svg>`);

/** The per-kind detail path. */
export function detailPath(kind: string): string {
  switch (kind) {
    case "agent": return "/agent";
    case "queue": case "user": return "/queue";
    case "pubsub": return "/pubsub/topic";
    case "group": return "/group";
    default: return "/service";
  }
}

export const recordHref = (r: { name: string; kind: string }, extra?: Record<string, string>) =>
  `${detailPath(r.kind)}?${new URLSearchParams({ name: r.name, ...(extra ?? {}) })}`;

export { entity };
