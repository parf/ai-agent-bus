// The homepage for a visitor with no session, and sign-in in place of any
// refused page (docs/web-face/node.md#-signed-out-landing-and-sign-in).
import { h, Fragment } from "../jsx.ts";
import { Ctx, COOKIE, Refusal, Unreachable } from "../ctx.ts";
import { respond, assets } from "../ui/frame.tsx";
import { Icon, Help } from "../ui/kit.tsx";
import { html, redirect, local, referer, cookie } from "../http.ts";

const FEATURES = [
  { icon: "id-card", title: "Registry", text: "Who and what is on the bus: users, agents, queues, services and groups. Every record has an owner and a list of who may reach it." },
  { icon: "inbox", title: "Messages", text: "Queues hold what was sent until somebody reads it. Pub/sub copies one publication to everyone subscribed." },
  { icon: "bot", title: "MCP server", text: "An agent reaches the bus through MCP, so finding a peer and sending it a message are tools the model already knows how to call." },
  { icon: "layout-dashboard", title: "Dashboard", text: "This web face, once you are signed in: what is registered, what is waiting, and what has gone wrong." },
];

/** Where the form sends the visitor after sign-in: the page they asked for. */
function returnFor(ctx: Ctx): string {
  if (ctx.method === "GET") return ctx.path === "/" ? "" : ctx.uri;
  const r = local(ctx.f("return") || referer(ctx.req));
  return r === "/" ? "" : r;
}

export async function signInPage(ctx: Ctx, message = "", status = 200, ret = returnFor(ctx)): Promise<Response> {
  const hero = assets().hero.path;
  const body = <div class="landing">
    <div class="landing-backdrop" aria-hidden="true"><img src={hero} alt="" /></div>
    <section class="landing-hero">
      <div class="hero-copy">
        <p class="eyebrow"><Icon name="sparkles" />agent-bus</p>
        <h1>One bus for agents, bots and services</h1>
        <p class="lede">Connect AI and NON-AI agents, bots and services so they can find and message each other. One daemon gives you a registry, message queues, an MCP server, dashboard and much more…</p>
        <p class="hero-links">
          <a href="https://github.com/parf/ai-agent-bus" class="btn btn-ghost"><Icon name="code-xml" />GitHub — docs &amp; updates</a>
          <span class="by">by <a href="https://parf.dev/">Serg Parf</a></span>
        </p>
      </div>
      <section class="signin-card card card-glass" aria-labelledby="signin-title">
        <div class="page-title">
          <h2 id="signin-title"><Icon name="key-round" />Sign in</h2>
          <Help id="token-help" label="How to get a token" tip="A token is what every call carries. Run agent-bus-token <name> on the box, or ssh agent-busd@<node> token from anywhere your key reaches." title="Getting a token"
            items={[<>On this box: <code>agent-bus-token &lt;name&gt;</code></>, <>From anywhere your key reaches: <code>ssh agent-busd@&lt;node&gt; token</code></>, "Asking again returns the token you already have. It does not expire on its own."]} />
        </div>
        <form method="post" action="/signin" class="signin-form">
          {ret ? <input type="hidden" name="return" value={ret} /> : null}
          <label for="token">Token</label>
          <input id="token" name="token" type="password" autofocus autocomplete="current-password" spellcheck="false" placeholder="Paste your token" aria-invalid={message ? "true" : "false"} aria-describedby={message ? "signin-error" : undefined} />
          {message ? <p class="warn" id="signin-error" role="alert">{message}</p> : null}
          <button type="submit" class="btn btn-primary btn-block"><Icon name="log-in" />Sign in</button>
        </form>
      </section>
    </section>
    <figure class="hero-figure">
      <img src={hero} width="1536" height="1024" alt="A red double-decker named Agents Bus, carrying AI and non-AI riders: Claude, OpenAI, Slack, Telegram, Email and a shell" />
    </figure>
    <section class="features" aria-label="What agent-bus does">
      {FEATURES.map(f => <article class="feature card">
        <div class="feature-mark" aria-hidden="true"><Icon name={f.icon} /></div>
        <h3>{f.title}</h3>
        <p>{f.text}</p>
      </article>)}
    </section>
  </div>;
  return respond(ctx, { title: "Sign in", bodyClass: "landing-page" }, body, status);
}

export async function postSignIn(ctx: Ctx): Promise<Response> {
  const token = ctx.f("token").trim();
  const ret = local(ctx.f("return"));
  const keep = ret === "/" ? "" : ret;
  if (!token) return signInPage(ctx, "that credential was not accepted", 401, keep);
  try {
    const s = await ctx.daemon.call<{ session: string }>("POST", "/session", { cred: token });
    if (!s?.session) throw new Refusal(401, "no session");
    return redirect(ret, 303, [cookie(COOKIE, s.session, ctx.tls)]);
  } catch (e) {
    if (e instanceof Unreachable) return signInPage(ctx, "the bus is not answering, so nothing was checked — try again when it is back", 502, keep);
    return signInPage(ctx, "that credential was not accepted", 401, keep);
  }
}

export async function postSignOut(ctx: Ctx): Promise<Response> {
  if (ctx.session) await ctx.daemon.call("DELETE", "/session", { cred: ctx.session }).catch(() => {});
  return redirect("/", 303, [cookie(COOKIE, "", ctx.tls, 0)]);
}
