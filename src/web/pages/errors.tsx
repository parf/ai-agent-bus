// A mistyped address is a 404, never a front page; a wrong method is a framed 405.
import { h } from "../jsx.ts";
import type { Ctx } from "../ctx.ts";
import { respond } from "../ui/frame.tsx";
import { Icon } from "../ui/kit.tsx";
import { html } from "../http.ts";

async function framed(ctx: Ctx, status: number, title: string, text: string) {
  const you = ctx.signedIn ? await ctx.you() : "";
  const body = <section class="problem">
    <div class="problem-mark" aria-hidden="true"><Icon name="compass" /></div>
    <p class="problem-code">{status}</p>
    <h1>{title}</h1>
    <p class="advice">{text}</p>
    <p><a class="btn" href="/">{ctx.signedIn ? "Back to Overview" : "Back to the start"}</a></p>
  </section>;
  return respond(ctx, { title, signedIn: ctx.signedIn, you }, body, status);
}

export const notFoundPage = (ctx: Ctx) => framed(ctx, 404, "No such page", "Nothing on this dashboard answers at that address. The sections are in the sidebar; a record is reached from its list.");
export const methodPage = (ctx: Ctx) => framed(ctx, 405, "Not answered this way", "This address exists, but not for that kind of request. Open it as a page, or use the form that sends it.");
