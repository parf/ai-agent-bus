// One Slack Web API call: POST with the bot token as a bearer, a JSON body,
// bounded at 15 s (Plans/R1.2/slack/README.md#behaviour-pinned-by-the-review).
// Retry policy lives in the callers: a write is never retried except after a
// 429, a read retries once on 5xx.
export async function slackCall(token: string, api: string | undefined, method: string, params: Record<string, unknown> = {}): Promise<any> {
  const base = (api || process.env.SLACK_API_URL || "https://slack.com/api/").replace(/\/+$/, "");
  const response = await fetch(`${base}/${method}`, {
    method: "POST",
    headers: { authorization: `Bearer ${token}`, "content-type": "application/json; charset=utf-8" },
    body: JSON.stringify(params),
    signal: AbortSignal.timeout(15_000),
  });
  const text = await response.text();
  try {
    return JSON.parse(text);
  } catch {
    return { ok: false, error: `malformed answer (${response.status}): ${text.slice(0, 200)}` };
  }
}
