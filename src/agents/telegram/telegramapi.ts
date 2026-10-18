// One Telegram Bot API call (Plans/R1.0-Release/telegram/README.md § methods
// and the forms row): POST with a JSON body to
// `<endpoint>/bot<token>/<method>`. The endpoint comes from the bus secret
// or Telegram's own default — never the process environment, which is the
// runner's. The token travels in the URL path, so no error ever echoes the
// URL or the body: an answer is named by its method and status alone.
//
// Every call runs inside one absolute deadline, the caller's: each attempt
// is cut to what is left of it (at most 15 s), a 429 waits
// `parameters.retry_after` (at most 30 s) and retries once if the wait and
// the retry still fit the deadline, a write is never retried otherwise, and
// a read retries once on 5xx (README § what carries over, the upstream
// errors row). A Telegram answer is an object with a boolean `ok`;
// `ok:false` is an answer, not a failure of the transport — its
// `description` names it to the caller.

export type TgAnswer = {
  ok: boolean;
  result?: any;
  description?: string;
  parameters?: { retry_after?: number };
  error?: string;
};

/** The endpoint a call goes to: the secret's URL, or Telegram's own. Nothing else. */
export function endpoint(api: string | undefined): string {
  return ((api && api.trim()) || "https://api.telegram.org").replace(/\/+$/, "");
}

/** The caller stopped waiting: nothing more is begun on their behalf. */
export class DeadlinePassed extends Error {
  constructor() {
    super("the caller's deadline passed");
    this.name = "DeadlinePassed";
  }
}

const perAttemptMs = 15_000;
const maxRetryAfterS = 30;

/** A Telegram answer is `{ok: …}`; anything else, JSON or not, is malformed
 *  and says only its status. */
export function shape(text: string, status: number): TgAnswer {
  let parsed: any;
  try {
    parsed = JSON.parse(text);
  } catch {
    parsed = undefined;
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed) || typeof parsed.ok !== "boolean") {
    return { ok: false, error: `malformed answer (${status})` };
  }
  if (parsed.ok === false && typeof parsed.description !== "string") parsed.description = `failed (${status})`;
  return parsed;
}

/** One bounded attempt; transport failures throw. The body is never echoed. */
async function attempt(token: string, api: string | undefined, method: string, params: Record<string, unknown>, deadline: number): Promise<TgAnswer & { status: number }> {
  const left = deadline - Date.now();
  if (left <= 0) throw new DeadlinePassed();
  const response = await fetch(`${endpoint(api)}/bot${token}/${method}`, {
    method: "POST",
    headers: { "content-type": "application/json; charset=utf-8" },
    body: JSON.stringify(params),
    signal: AbortSignal.timeout(Math.min(left, perAttemptMs)),
  });
  return { status: response.status, ...shape(await response.text(), response.status) };
}

/** The pinned HTTP policy, inside the caller's deadline. A write is one
 *  attempt, plus the single 429 retry; a read retries once on 5xx. */
export async function telegramWeb(token: string, api: string | undefined, kind: "read" | "write", method: string, params: Record<string, unknown> = {}, deadline = Date.now() + 60_000): Promise<TgAnswer> {
  let first: TgAnswer & { status: number };
  try {
    first = await attempt(token, api, method, params, deadline);
  } catch (e) {
    throw e instanceof DeadlinePassed ? e : new Error(`Telegram ${method} did not answer`);
  }
  if (first.status === 429) {
    // The wait and a second attempt must both fit the caller's deadline:
    // a retry the caller will never hear of is a write done for nobody.
    const retryAfterS = Number(first.parameters?.retry_after ?? 1);
    const waitMs = (Number.isFinite(retryAfterS) && retryAfterS >= 0 ? Math.min(retryAfterS, maxRetryAfterS) : 1) * 1000;
    if (Date.now() + waitMs >= deadline) return { ok: false, error: "ratelimited" };
    await Bun.sleep(waitMs);
    let second: TgAnswer & { status: number };
    try {
      second = await attempt(token, api, method, params, deadline);
    } catch (e) {
      throw e instanceof DeadlinePassed ? e : new Error(`Telegram ${method} did not answer`);
    }
    return second.status === 429 ? { ok: false, error: "ratelimited" } : second;
  }
  if (first.status >= 500 && kind === "read") {
    try {
      return await attempt(token, api, method, params, deadline);
    } catch {
      return first; // the read already failed once; its 5xx is the answer
    }
  }
  return first;
}
