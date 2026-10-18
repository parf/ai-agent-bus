// One Telegram Bot API call: POST with a JSON body to
// `<endpoint>/bot<token>/<method>` (https://core.telegram.org/bots/api#making-requests).
// The endpoint comes from the bus secret or Telegram's own default — never
// the process environment, which is the runner's (Plans/R1.0-Release/telegram/README.md,
// slack-review-S.1 P1 is the same mistake not made twice). The token travels
// in the URL path, so no error ever echoes the URL or the body: an answer is
// named by its method and status alone.
//
// A Telegram answer is an object with a boolean `ok`; anything else, JSON or
// not, is malformed and says only its status. `ok:false` is an answer, not a
// failure of the transport; its `description` names it to the caller.

/** The endpoint a call goes to: the secret's URL, or Telegram's own. Nothing else. */
export function endpoint(api: string | undefined): string {
  return ((api && api.trim()) || "https://api.telegram.org").replace(/\/+$/, "");
}

export type TgAnswer = {
  ok: boolean;
  result?: any;
  description?: string;
  parameters?: { retry_after?: number };
  error?: string;
};

/** A Telegram answer is `{ok: …}`; anything else is malformed, status only. */
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

/** One Bot API call. Transport failures name the method, never the URL: the
 *  token is in the URL path, and an error message travels to the caller. */
export async function telegramWeb(token: string, api: string | undefined, method: string, params: Record<string, unknown> = {}): Promise<TgAnswer> {
  let response: Response;
  try {
    response = await fetch(`${endpoint(api)}/bot${token}/${method}`, {
      method: "POST",
      headers: { "content-type": "application/json; charset=utf-8" },
      body: JSON.stringify(params),
      signal: AbortSignal.timeout(15_000),
    });
  } catch (e) {
    throw new Error(`Telegram ${method} did not answer: ${e instanceof Error ? e.name : "network failure"}`);
  }
  return shape(await response.text(), response.status);
}
