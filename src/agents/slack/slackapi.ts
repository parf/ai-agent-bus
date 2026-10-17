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

export async function slackCall(token: string, api: string | undefined, method: string, params: Record<string, unknown> = {}): Promise<any> {
  let response: Response;
  try {
    response = await fetch(`${endpoint(api)}/${method}`, {
      method: "POST",
      headers: { authorization: `Bearer ${token}`, "content-type": "application/json; charset=utf-8" },
      body: JSON.stringify(params),
      signal: AbortSignal.timeout(15_000),
    });
  } catch (e) {
    // Transport failure only: no upstream text exists to leak.
    throw new Error(`Slack ${method} did not answer: ${e instanceof Error ? e.name : "network failure"}`);
  }
  const text = await response.text();
  let parsed: any;
  try {
    parsed = JSON.parse(text);
  } catch {
    // The body is never echoed: a hostile or broken endpoint may reflect the
    // request — Authorization included — and an answer is bus-visible.
    return { ok: false, error: `malformed answer (${response.status})` };
  }
  if (parsed && typeof parsed === "object" && typeof parsed.error === "string") {
    parsed.error = scrub(parsed.error, token);
  }
  return parsed;
}

/** Remove the agent's own credential bytes from any string that reaches a caller. */
export function scrub(text: string, ...secrets: (string | undefined)[]): string {
  let out = text;
  for (const s of secrets) if (s && s.length >= 8) out = out.split(s).join("[redacted]");
  return out;
}
