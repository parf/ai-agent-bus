// One Slack Web API call: POST with the bot token as a bearer and a form body
// (Plans/R1.2/done/slack/README.md#behaviour-pinned-by-the-review). A form is the
// one encoding every method takes — `chat.getPermalink` and other reads refuse
// a JSON body as invalid_arguments — and an object value travels JSON-encoded
// inside it, as Slack documents for `blocks` and its like. The endpoint
// comes from the bus secret or the official default — never the process
// environment, which is the runner's and would receive the secret's bearer
// had anything else pointed it somewhere (slack-review-S.1 P1).
//
// Every call runs inside one absolute deadline, the caller's: each attempt is
// cut to what is left of it (at most 15 s), a 429 waits only if the wait and
// the retry still fit, and no write is begun once it has passed
// (slack-review-S.3 P1). A write is never retried except after a 429, which
// Slack did not carry out; a read retries once on 5xx.

/** The endpoint a call goes to: the secret's URL, or Slack's own. Nothing else. */
export function endpoint(api: string | undefined): string {
  return ((api && api.trim()) || "https://slack.com/api/").replace(/\/+$/, "");
}

/** The caller stopped waiting: nothing more is begun on its behalf. */
export class DeadlinePassed extends Error {
  constructor() {
    super("the caller's deadline passed");
    this.name = "DeadlinePassed";
  }
}

export type Attempt = { status: number; retryAfterMs: number; parsed: any };

const perAttemptMs = 15_000;

/** A Slack answer is an object with a boolean `ok`; anything else, JSON or
 *  not, is malformed and says only its status (slack-review-S.1 P2). */
function shape(text: string, status: number): any {
  let parsed: any;
  try {
    parsed = JSON.parse(text);
  } catch {
    parsed = undefined;
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed) || typeof parsed.ok !== "boolean") {
    return { ok: false, error: `malformed answer (${status})` };
  }
  if (parsed.ok === false && typeof parsed.error !== "string") parsed.error = `failed (${status})`;
  // Slack says which scope a refused call lacked; the error says it too, so a
  // missing permission is fixed in one look at the app's settings.
  if (parsed.ok === false && parsed.error === "missing_scope" && typeof parsed.needed === "string" && parsed.needed !== "") {
    parsed.error = `missing_scope (needs ${parsed.needed})`;
  }
  return parsed;
}

/** One bounded attempt; transport failures throw. The body is never echoed. */
async function attempt(token: string, api: string | undefined, method: string, params: Record<string, unknown>, deadline: number): Promise<Attempt> {
  const left = deadline - Date.now();
  if (left <= 0) throw new DeadlinePassed();
  const response = await fetch(`${endpoint(api)}/${method}`, {
    method: "POST",
    headers: { authorization: `Bearer ${token}`, "content-type": "application/x-www-form-urlencoded; charset=utf-8" },
    body: formOf(params),
    signal: AbortSignal.timeout(Math.min(left, perAttemptMs)),
  });
  const retryAfter = Number(response.headers.get("retry-after"));
  const retryAfterMs = Number.isFinite(retryAfter) && retryAfter >= 0 ? Math.min(retryAfter, 30) * 1000 : 1000;
  const text = await response.text();
  return { status: response.status, retryAfterMs, parsed: shape(text, response.status) };
}

/** A call's parameters as a form: strings as they are, numbers and booleans
 *  spelled out, anything else JSON-encoded; an undefined one is left out. */
export function formOf(params: Record<string, unknown>): string {
  const form = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined) continue;
    form.set(k, typeof v === "string" ? v : typeof v === "number" || typeof v === "boolean" ? String(v) : JSON.stringify(v));
  }
  return form.toString();
}

function unanswered(method: string, e: unknown): Error {
  if (e instanceof DeadlinePassed) return e;
  return new Error(`Slack ${method} did not answer: ${e instanceof Error ? e.name : "network failure"}`);
}

/** The pinned HTTP policy, inside the caller's deadline. */
export async function slackWeb(token: string, api: string | undefined, kind: "write" | "read", method: string, params: Record<string, unknown> = {}, deadline = Date.now() + perAttemptMs): Promise<any> {
  let first: Attempt;
  try {
    first = await attempt(token, api, method, params, deadline);
  } catch (e) {
    throw unanswered(method, e);
  }
  if (first.status === 429) {
    // The wait and a second attempt must both fit the caller's deadline:
    // a retry the caller will never hear of is a write done for nobody.
    if (Date.now() + first.retryAfterMs >= deadline) return { ok: false, error: "ratelimited" };
    await Bun.sleep(first.retryAfterMs);
    let second: Attempt;
    try {
      second = await attempt(token, api, method, params, deadline);
    } catch (e) {
      throw unanswered(method, e);
    }
    return second.status === 429 ? { ok: false, error: "ratelimited" } : second.parsed;
  }
  if (first.status >= 500 && kind === "read") {
    try {
      return (await attempt(token, api, method, params, deadline)).parsed;
    } catch (e) {
      throw unanswered(method, e);
    }
  }
  return first.parsed;
}

/** Remove the agent's own credential bytes from any string that reaches a caller. */
export function scrub(text: string, ...secrets: (string | undefined)[]): string {
  let out = text;
  for (const s of secrets) if (s && s.length >= 8) out = out.split(s).join("[redacted]");
  return out;
}
