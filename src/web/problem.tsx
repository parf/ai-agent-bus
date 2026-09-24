// Whole-page failures, one template. The advice sentences are the spec's own
// (docs/web-face/shell.md#problem-page); the daemon's message is shown where
// the table says so, and a transport error's text never is.
import { h, Fragment } from "./jsx.ts";
import { Ctx, Refusal, Unreachable, SignInRequired, LocalProblem, ConditionsChanged, NotFound } from "./ctx.ts";
import { respond } from "./ui/frame.tsx";
import { Icon } from "./ui/kit.tsx";
import { html, referer } from "./http.ts";
import { signInPage } from "./pages/landing.tsx";

type Problem = { status: number; title: string; message?: string; advice: string; back?: { href: string; label: string } };

export const SUSPENDED = "user access is suspended";

export function classify(e: unknown): Problem {
  if (e instanceof Unreachable) return { status: 502, title: "The bus is not answering", advice: "The dashboard is running; the daemon behind it is not reachable, so there is nothing to show and nothing was changed. It comes back on its own when the daemon does." };
  if (e instanceof LocalProblem) return { status: e.status, title: "That request was not understood", message: e.detail, advice: e.saved ? "Part of it was saved, as the sentence above says. Return to the page to finish it." : "Nothing was sent to the daemon. Return to the page and use the action shown there." };
  if (e instanceof ConditionsChanged) return { status: 409, title: "The conditions changed", message: e.detail, advice: "The daemon was re-read before the action. Review the current record and confirm again if the action still applies.", back: { href: e.back, label: "Review the current Danger Zone" } };
  if (e instanceof NotFound) return notFound();
  if (e instanceof Refusal) {
    const m = e.detail;
    switch (e.status) {
      case 403: return m.startsWith(SUSPENDED)
        ? { status: 403, title: "Your access is suspended", message: m, advice: "You are signed in, and every call is refused until your user is active again — signing in again would change nothing. An administrator of this node is who can lift it." }
        : { status: 403, title: "Not yours to see", message: m, advice: "You are signed in, and refused for lack of permission rather than for want of a credential — signing in again would change nothing. Its owner, or a maintainer of it, is who can grant this." };
      case 404: return notFound();
      case 503: return { status: 503, title: "The bus is busy", message: m, advice: "The daemon is there and briefly cannot answer. Nothing was changed; the same request is worth making again." };
      case 500: return { status: 500, title: "Something went wrong in the daemon", message: m, advice: "This is a fault, not a rule: repeating it is unlikely to help, and the daemon's log on this node is where it is recorded." };
      default: return { status: e.status, title: "That was refused", message: m, advice: "Nothing was changed. The reason above is the daemon's own." };
    }
  }
  console.error("web: unexpected", e);
  return { status: 500, title: "Something went wrong in the dashboard", advice: "This is a fault in the web face, not a daemon refusal; its log on this node records it." };
}

const notFound = (): Problem => ({ status: 404, title: "No such name", advice: "Either nothing is registered under that name or it is not one you may see. Those are deliberately the same answer, so this does not tell you which." });

/** Refused for the visitor: 403 "Not yours to see" with the face's reason. */
export const notYours = (detail: string) => new Refusal(403, detail);

export async function problemPage(ctx: Ctx, e: unknown): Promise<Response> {
  if (e instanceof SignInRequired) return signInPage(ctx, e.message, e.status);
  if (e instanceof Refusal && e.status === 401) return signInPage(ctx, "that session has ended — sign in to carry on", 401);
  const p = classify(e);
  let back = p.back;
  if (!back) {
    if (ctx.method === "GET" && p.status !== 404) back = { href: ctx.uri, label: "Try again" };
    else if (ctx.method !== "GET") { const r = referer(ctx.req); if (r && r !== "/") back = { href: r, label: "Back to the page" }; }
    if (p.status === 404) back = { href: "/", label: "Back to Overview" };
  }
  const you = ctx.signedIn ? await ctx.you() : "";
  const body = <section class="problem">
    <div class="problem-mark" aria-hidden="true"><Icon name="triangle-alert" /></div>
    <p class="problem-code">{p.status}</p>
    <h1>{p.title}</h1>
    {p.message ? <p class="warn">{p.message}</p> : null}
    <p class="advice">{p.advice}</p>
    {back ? <p><a class="btn" href={back.href}>{back.label}</a></p> : null}
  </section>;
  return respond(ctx, { title: p.title, signedIn: ctx.signedIn, you }, body, p.status);
}

/** One section failed and the rest of the page is true. */
export function sectionProblem(e: unknown): string {
  if (e instanceof Refusal) return e.detail || `refused (${e.status})`;
  return "the daemon did not answer";
}

/** A refused form: preserve re-renders the form with the code; anything else is a problem page. */
export function formRefusal(e: unknown): { status: number; message: string; preserve: boolean } | undefined {
  if (!(e instanceof Refusal)) return undefined;
  return { status: e.status, message: e.detail, preserve: [400, 404, 409, 412, 429].includes(e.status) };
}

/** The 1-based line of a submitted list field that the daemon's message names, if any. */
export function lineRefusal(message: string, lines: string[]): number {
  const words = new Set(message.split(/[\s:,;"'()]+/).filter(Boolean));
  for (let i = 0; i < lines.length; i++) {
    const l = lines[i]!.trim();
    if (l && (words.has(l) || message.includes(` ${l}:`) || message.endsWith(` ${l}`))) return i + 1;
  }
  return 0;
}
