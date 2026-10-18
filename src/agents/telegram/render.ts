// What a call answers: either it worked (a text line for the text form and
// the fields for the JSON form), or it did not (an error), in the shape
// Slack's answers take (Plans/R1.2/slack/README.md § JSON answers, carried
// over by the telegram plan's forms row).
export type Result = { ok: true; text: string; fields: Record<string, unknown> } | { ok: false; error: string };

export const ok = (text: string, fields: Record<string, unknown> = {}): Result => ({ ok: true, text, fields });
export const fail = (error: string): Result => ({ ok: false, error });

/** One boundary: the text form gets the text line (or `error: why`), the
 *  JSON form gets `{"ok":true, …fields}` or `{"ok":false,"error":…}`. */
export function render(json: boolean, r: Result): string {
  return json
    ? JSON.stringify(r.ok ? { ok: true, ...r.fields } : { ok: false, error: r.error })
    : r.ok ? r.text : `error: ${r.error}`;
}
