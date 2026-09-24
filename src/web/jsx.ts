// A small JSX runtime for server-rendered HTML. Every string is escaped by
// default; markup reaches the page only as Html, which only h() and raw() make.
// There is no style attribute: the CSP allows no inline style, so a view that
// writes one fails at render time rather than in a browser nobody watches.

export class Html {
  constructor(readonly html: string) {}
  toString() { return this.html; }
}

export type Child = Html | string | number | bigint | boolean | null | undefined | Child[];
type Props = Record<string, unknown> & { children?: Child };
type Component = (props: any) => Html;

const ESC: Record<string, string> = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };
export const escape = (s: string) => s.replace(/[&<>"']/g, c => ESC[c]!);

const VOID = new Set(["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source", "track", "wbr"]);
const ALIAS: Record<string, string> = { className: "class", htmlFor: "for" };
const NAME = /^[a-zA-Z_:][-a-zA-Z0-9_:.]*$/;

export function render(c: Child): string {
  if (c == null || c === false || c === true) return "";
  if (c instanceof Html) return c.html;
  if (Array.isArray(c)) return c.map(render).join("");
  return escape(String(c));
}

function attrs(props: Props | null): string {
  let out = "";
  for (const [k, v] of Object.entries(props ?? {})) {
    if (k === "children" || v == null || v === false) continue;
    if (k === "style") throw new Error("the style attribute is not allowed: the CSP forbids inline style");
    if (k.startsWith("on") && k.length > 2 && k[2] === k[2]!.toUpperCase()) throw new Error(`inline handler ${k} is not allowed`);
    const name = ALIAS[k] ?? k;
    if (!NAME.test(name)) throw new Error(`bad attribute name ${name}`);
    out += v === true ? ` ${name}` : ` ${name}="${escape(String(v))}"`;
  }
  return out;
}

export function h(tag: string | Component, props: Props | null, ...children: Child[]): Html {
  if (typeof tag === "function") return tag({ ...(props ?? {}), children: children.length === 1 ? children[0] : children });
  if (!NAME.test(tag)) throw new Error(`bad tag ${tag}`);
  if (VOID.has(tag)) return new Html(`<${tag}${attrs(props)}>`);
  return new Html(`<${tag}${attrs(props)}>${render(children)}</${tag}>`);
}

export function Fragment(props: { children?: Child }): Html {
  return new Html(render(props.children));
}

/** Markup the caller built and vouches for: fixed SVG, never data. */
export const raw = (s: string) => new Html(s);

declare global {
  namespace JSX {
    type Element = Html;
    interface IntrinsicElements { [tag: string]: Record<string, unknown> }
    interface ElementChildrenAttribute { children: {} }
  }
}
