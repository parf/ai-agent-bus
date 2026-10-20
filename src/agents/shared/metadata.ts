// Bare/self-serving fallback for the runner's manifest-generated help contract.
// This reads authored declarations only, never runtime Secret/env values.
import { VERSION } from "../../clients/typescript/src/version.ts";
type Field = { help: string; type?: string | string[]; example?: unknown; default?: unknown };
type Arg = Partial<Field> & { name: string; optional?: boolean; rest?: boolean; json_only?: boolean };
type Method = Field & { args?: Arg[]; returns?: string; role?: string | string[]; destructive?: boolean; text?: string; example?: string };
export type Manifest = { description: string; version?: string; license?: string; author?: string; homepage?: string; methods?: Record<string, Method>; arguments?: Record<string, Field>; config?: Record<string, Field> };
export const asks = ["help", "help-config", "help-secret", "help-json"] as const;
export type HelpAsk = typeof asks[number];
const typed = <T extends Partial<Field>>(value: T): T => ({ ...value, type: value.type ?? "string" });
const kind = (type: string | string[] | undefined): string => Array.isArray(type) ? type.join("|") : type ?? "string";
export function secrets(source: string) {
  const out: { name: string; help: string; required: boolean }[] = [];
  let comments: string[] = [];
  for (const line of source.replaceAll("\r\n", "\n").split("\n")) {
    if (line.trim().startsWith("#")) { comments.push(line.trim().slice(1).trim()); continue; }
    const entry = /^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*?)\s*$/.exec(line);
    if (entry) {
      out.push({ name: entry[1]!, help: comments.join("\n"), required: entry[2] === "" || entry[2] === '""' || entry[2] === "''" });
    }
    comments = [];
  }
  return out;
}
export function helpData(manifest: Manifest, ask: HelpAsk, envDist: string, roles: string[] = []) {
  if (ask === "help-secret") return { secrets: secrets(envDist) };
  if (ask === "help-config") return { config: Object.fromEntries(Object.entries(manifest.config ?? {}).map(([name, field]) => [name, typed(field)])) };
  const methods = Object.fromEntries(Object.entries(manifest.methods ?? {}).filter(([, method]) => !method.role || roles.some(r => (Array.isArray(method.role) ? method.role.includes(r) : r === method.role) || r === "owner" || r === "maintainer"))
    .map(([name, method]) => [name, { ...typed(method), args: (method.args ?? []).map(a => typed({ ...(!a.help ? manifest.arguments?.[a.name] : {}), ...a })) }]));
  const result = { description: manifest.description, version: manifest.version ?? VERSION,
    ...(manifest.license ? { license: manifest.license } : {}), ...(manifest.author ? { author: manifest.author } : {}),
    ...(manifest.homepage ? { homepage: manifest.homepage } : {}), methods, asks: [...asks] };
  if (ask === "help-json") return { ...result,
    config: Object.fromEntries(Object.entries(manifest.config ?? {}).map(([name, field]) => [name, typed(field)])),
    secrets: secrets(envDist) };
  return result;
}
export function helpAnswer(manifest: Manifest, ask: HelpAsk, envDist: string, roles: string[] = [], json = false, name = process.env.AGENT_BUS_NAME || ""): string {
  const data = helpData(manifest, ask, envDist, roles);
  if (json || ask === "help-json") return JSON.stringify(data);
  const lines: string[] = [];
  if ("secrets" in data) {
    for (const s of data.secrets!) lines.push(`${s.name} (${s.required ? "required" : "optional"}): ${s.help}`);
    return lines.join("\n") || "No environment keys declared.";
  }
  if ("config" in data) {
    for (const name of Object.keys(data.config!).sort()) {
      const c = data.config![name]!;
      lines.push(`${name} (${kind(c.type)}): ${c.help}${Object.hasOwn(c, "example") ? `; example: ${JSON.stringify(c.example)}` : ""}${Object.hasOwn(c, "default") ? `; default: ${JSON.stringify(c.default)}` : ""}`);
    }
    return lines.join("\n") || "No Configuration keys declared.";
  }
  lines.push(data.description!);
  const shared = new Map<string, Arg>();
  const examples: string[] = [];
  const argument = (a: Arg, prefix = "") => `${prefix}${a.name} (${kind(a.type)}): ${a.help}${Object.hasOwn(a, "default") ? `; default: ${JSON.stringify(a.default)}` : ""}`;
  for (const [name, m] of Object.entries(data.methods!)) {
    if (name === "help") continue;
    const args = m.args.filter(a => !a.json_only);
    const usage = m.text || name + args.map(a => ` ${a.optional ? "[" : "<"}${a.name}${a.rest ? "..." : ""}${a.optional ? "]" : ">"}`).join("");
    lines.push((m.destructive ? "[destructive] " : "") + usage + `: ${m.help}` + (m.returns ? `; returns: ${m.returns}` : "") + (m.role ? `; role: ${kind(m.role)}` : ""));
    for (const a of args) {
      const authored = manifest.methods![name]!.args!.find(arg => arg.name === a.name)!;
      if (!authored.help) shared.set(a.name, a);
      else lines.push(argument(a, "  "));
    }
    if (m.example) examples.push(m.example);
  }
  if (shared.size) lines.push("", "Arguments", ...[...shared.keys()].sort().map(name => argument(shared.get(name)!)));
  if (examples.length) lines.push("", "Examples", ...examples);
  const footer = [`v${data.version}`];
  if (data.license) footer.push(data.license.replace("PolyForm-Noncommercial-1.0.0", "PolyForm-NC"));
  if (data.homepage) footer.push(data.homepage.replace(/^https?:\/\//, ""));
  const target = name ? "'" + name.replaceAll("'", "'\"'\"'") + "'" : "<name>";
  lines.push("", footer.join(" · "), `More: agent-bus help ${target} config | secret | json`);
  return lines.join("\n");
}
