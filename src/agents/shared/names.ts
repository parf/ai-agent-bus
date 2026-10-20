// Same component grammar and 64-byte canonical limit as protocol.ParseName.
// Keep the shared Go/TypeScript name vectors in the gate when changing this.
const part = /^[a-z0-9][a-z0-9._-]*$/;
const local = /^[a-z0-9][a-z0-9._+-]*(?:@[a-z0-9][a-z0-9._+-]*)*$/;
// Go strings.TrimSpace accepts these characters; JS trim also accepts BOM,
// which ParseName refuses as non-ASCII, so use the Go whitespace set explicitly.
const whitespace = "\\t\\n\\v\\f\\r \\u0085\\u00a0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000";
const surroundingWhitespace = new RegExp(`^[${whitespace}]+|[${whitespace}]+$`, "g");
const trim = (value: string): string => value.replace(surroundingWhitespace, "");

export function parseBusName(raw: unknown): string | undefined {
  if (typeof raw !== "string") return;
  let value = trim(raw);
  if (/[^\x00-\x7f]/.test(value)) return;
  const agent = value.startsWith("#");
  if (agent) {
    value = value.slice(1);
    // The agent prefix must touch the first component.
    if (value !== value.replace(/^[\t\n\v\f\r ]+/, "")) return;
  }
  value = value.toLowerCase();
  const at = value.lastIndexOf("@");
  let name = trim(at < 0 ? value : value.slice(0, at));
  const realm = at < 0 ? undefined : trim(value.slice(at + 1));
  if (realm !== undefined && !part.test(realm)) return;

  const slash = name.indexOf("/");
  let template: string | undefined;
  if (slash >= 0) {
    template = trim(name.slice(0, slash));
    name = trim(name.slice(slash + 1));
    if (!part.test(template)) return;
  }
  if (!local.test(name)) return;
  const canonical = (agent ? "#" : "") + (template === undefined ? "" : template + "/") + name + (realm === undefined ? "" : "@" + realm);
  return canonical.length <= 64 ? canonical : undefined;
}

export const busName = { test: (raw: unknown): boolean => parseBusName(raw) !== undefined };
export const agentName = { test: (raw: unknown): boolean => parseBusName(raw)?.startsWith("#") === true };
export const userName = {
  test: (raw: unknown): boolean => {
    const name = parseBusName(raw);
    return name !== undefined && !name.startsWith("#");
  },
};
