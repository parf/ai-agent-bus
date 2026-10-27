import { Json } from "./ui/json.tsx";
import { Refusal, type Ctx } from "./ctx.ts";
import type { Rec } from "./ctx.ts";
import { h, Fragment } from "./jsx.ts";
import { Card } from "./ui/kit.tsx";

// Reformat whitespace only: JSON.parse/stringify would round large numbers,
// normalize escaped strings and lose duplicate keys, changing the CLI digest.
export function formatConfiguration(raw: string, pretty = false): string {
  JSON.parse(raw);
  const tokens = raw.match(/"(?:\\[\s\S]|[^"\\])*"|[^\s]/g) ?? [];
  if (!pretty) return tokens.join("");
  let depth = 0, out = "";
  const newline = () => "\n" + "  ".repeat(depth);
  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i]!;
    if (token === "{" || token === "[") {
      out += token; depth++;
      if (tokens[i + 1] !== "}" && tokens[i + 1] !== "]") out += newline();
    } else if (token === "}" || token === "]") {
      depth--;
      if (tokens[i - 1] !== "{" && tokens[i - 1] !== "[") out += newline();
      out += token;
    } else if (token === ",") out += token + newline();
    else if (token === ":") out += ": ";
    else out += token;
  }
  return out;
}

export async function configuration(ctx: Ctx, rec?: Rec): Promise<string | undefined> {
  if (!rec?.can_manage || !["agent", "service", "group"].includes(rec.kind)) return undefined;
  let raw: string;
  try { raw = await ctx.bus<string>("GET", "/config", { query: { name: rec.name }, raw: true }); } catch (e) {
    if (e instanceof Refusal && e.status === 403) return undefined;
    throw e;
  }
  return raw.trim() === "null" ? "" : formatConfiguration(raw, true);
}

/** Stored bytes only; the caller has already enforced management visibility. */
export function ConfigurationPreview({value,editHref,sha}:{value:string|undefined;editHref:string;sha?:string}) {
  if (!value) return null;
  return <Card title={<>Configuration{sha ? <code class="config-sha" title={sha}>sha {sha.slice(0,8)}</code> : null}</>} icon="settings" id="configuration" className="configuration-preview"
    actions={<><button type="button" class="btn btn-secondary btn-sm" data-action="expand-configuration" aria-expanded="false">Expand</button><a class="small" href={editHref}>Edit</a></>}>
    <pre data-configuration-json tabindex="0"><code><Json value={value} /></code></pre>
  </Card>;
}
