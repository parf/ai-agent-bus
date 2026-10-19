// Bare/self-serving fallback for the runner's manifest-generated help contract.
// This reads authored declarations only, never runtime Secret/env values.
import { VERSION } from "../../clients/typescript/src/version.ts";
type Field = { help: string; type?: string | string[]; example?: unknown; default?: unknown };
type Arg = Field & { name: string; optional?: boolean; rest?: boolean };
type Method = Field & { args?: Arg[]; returns?: string; role?: string | string[]; destructive?: boolean };
export type Manifest = { description: string; version?: string; license?: string; author?: string; homepage?: string; methods?: Record<string, Method>; config?: Record<string, Field> };
export const asks = ["help", "help-config", "help-secret", "help-json"] as const;
export type HelpAsk = typeof asks[number];
const typed = <T extends Field>(value: T): T => ({ ...value, type: value.type ?? "string" });
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
    .map(([name, method]) => [name, { ...typed(method), args: (method.args ?? []).map(typed) }]));
  const result = { description: manifest.description, version: manifest.version ?? VERSION,
    ...(manifest.license ? { license: manifest.license } : {}), ...(manifest.author ? { author: manifest.author } : {}),
    ...(manifest.homepage ? { homepage: manifest.homepage } : {}), methods, asks: [...asks] };
  if (ask === "help-json") return { ...result,
    config: Object.fromEntries(Object.entries(manifest.config ?? {}).map(([name, field]) => [name, typed(field)])),
    secrets: secrets(envDist) };
  return result;
}
export function helpAnswer(manifest: Manifest, ask: HelpAsk, envDist: string, roles: string[] = [], json = false): string {
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
  lines.push(data.description!, `Version: ${data.version}`);
  for (const key of ["license", "author", "homepage"] as const) if (data[key]) lines.push(`${key[0]!.toUpperCase() + key.slice(1)}: ${data[key]}`);
  lines.push("Asks: --help, --help-config, --help-secret, --help-json");
  for (const name of Object.keys(data.methods!).sort()) {
    const m = data.methods![name]!;
    lines.push(name + m.args.map(a => ` ${a.optional ? "[" : "<"}${a.name}${a.rest ? "..." : ""}${a.optional ? "]" : ">"}`).join("") + `: ${m.help}` + (m.returns ? `; returns: ${m.returns}` : "") + (m.role ? `; role: ${kind(m.role)}` : "") + (m.destructive ? "; destructive" : ""));
    for (const a of m.args) lines.push(`  ${a.name} (${kind(a.type)}): ${a.help}`);
  }
  return lines.join("\n");
}
