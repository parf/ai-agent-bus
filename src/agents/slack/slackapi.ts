// One Slack Web API call: POST with the bot token as a bearer, a JSON body,
// bounded at 15 s (Plans/R1.2/slack/README.md#behaviour-pinned-by-the-review).
// The endpoint comes from the bus secret or the official default — never the
// process environment, which is the runner's and would receive the secret's
// bearer had anything else pointed it somewhere (slack-review-S.1 P1). Retry
// policy lives in the callers: a write is never retried except after a 429,
// a read retries once on 5xx.

/** The endpoint a call goes to: the secret's URL, or Slack's own. Nothing else. */
export function endpoint(api: string | undefined): string {
  return ((api && api.trim()) || "https://slack.com/api/").replace(/\/+$/, "");
}

export type Attempt = { status: number; retryAfterMs: number; parsed: any };

/** One bounded attempt; transport failures throw. The body is never echoed. */
async function attempt(token: string, api: string | undefined, method: string, params: Record<string, unknown>, ms: number): Promise<Attempt> {
  const response = await fetch(`${endpoint(api)}/${method}`, {
    method: "POST",
    headers: { authorization: `Bearer ${token}`, "content-type": "application/json; charset=utf-8" },
    body: JSON.stringify(params),
    signal: AbortSignal.timeout(ms),
  });
  const retryAfter = Number(response.headers.get("retry-after"));
  const retryAfterMs = Number.isFinite(retryAfter) && retryAfter >= 0 ? Math.min(retryAfter, 30) * 1000 : 1000;
  const text = await response.text();
  let parsed: any;
  try {
    parsed = JSON.parse(text);
  } catch {
    parsed = { ok: false, error: `malformed answer (${response.status})` };
  }
  if (parsed && typeof parsed === "object" && typeof parsed.error === "string") {
    parsed.error = scrub(parsed.error, token);
  }
  return { status: response.status, retryAfterMs, parsed };
}

/** The pinned HTTP policy. A write is tried once, except after a 429 Slack did
 *  not carry out: wait Retry-After (at most 30 s) and try once more; a second
 *  429 answers `ratelimited`. A read retries once on 5xx. Nothing else retries:
 *  an uncertain write is never replayed. */
export async function slackWeb(token: string, api: string | undefined, kind: "write" | "read", method: string, params: Record<string, unknown> = {}, boundMs = 15_000): Promise<any> {
  let first: Attempt;
  try {
    first = await attempt(token, api, method, params, boundMs);
  } catch (e) {
    throw new Error(`Slack ${method} did not answer: ${e instanceof Error ? e.name : "network failure"}`);
  }
  if (first.status === 429) {
    await Bun.sleep(first.retryAfterMs);
    try {
      const second = await attempt(token, api, method, params, boundMs);
      if (second.status !== 429) return second.parsed;
    } catch (e) {
      throw new Error(`Slack ${method} did not answer: ${e instanceof Error ? e.name : "network failure"}`);
    }
    return { ok: false, error: "ratelimited" };
  }
  if (first.status >= 500 && kind === "read") {
    try {
      return (await attempt(token, api, method, params, boundMs)).parsed;
    } catch (e) {
      throw new Error(`Slack ${method} did not answer: ${e instanceof Error ? e.name : "network failure"}`);
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
