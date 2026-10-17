// Adapted from motdotla/dotenv; copyright Scott Motte, BSD-2-Clause (NOTICE.md).
// Official motdotla/dotenv parseRegex grammar, without evaluation. The shared
// conformance table lives in internal/envfile/testdata/common.json.
export type EnvValue = { key: string; value: string };
const LINE = /(?:^|^)\s*(?:export\s+)?([\w.-]+)(?:\s*=\s*?|:\s+?)(\s*'(?:\\'|[^'])*'|\s*"(?:\\"|[^"])*"|\s*`(?:\\`|[^`])*`|[^#\r\n]+)?\s*(?:#.*)?(?:$|$)/mg;

export function parseEnvFile(source: string, bytes: Uint8Array): Record<string, string> {
  return Object.fromEntries(parseEnvList(source, new TextDecoder().decode(bytes)).map(v => [v.key, v.value]));
}

export function parseEnvList(source: string, input: string): EnvValue[] {
  const text = input.replace(/\r\n?/g, "\n");
  const nul = text.indexOf("\0");
  if (nul >= 0) throw new Error(`${source}: line ${text.slice(0, nul).split(/[\n\u2028\u2029]/g).length}: NUL cannot be delivered in an environment`);
  const out: EnvValue[] = [];
  let end = 0;
  const check = (start: number, finish: number) => {
    let line = text.slice(0, start).split(/[\n\u2028\u2029]/g).length;
    for (const raw of text.slice(start, finish).split(/[\n\u2028\u2029]/g)) {
      const s = raw.trim();
      if (s && !s.startsWith("#")) throw new Error(`${source}: line ${line}: expected a dotenv assignment`);
      line++;
    }
  };
  for (const match of text.matchAll(LINE)) {
    check(end, match.index!);
    let value = (match[2] || "").trim();
    const quote = value[0];
    value = value.replace(/^(['"`])([\s\S]*)\1$/mg, "$2");
    if (quote === '"') value = value.replace(/\\n/g, "\n").replace(/\\r/g, "\r");
    out.push({ key: match[1]!, value });
    end = match.index! + match[0].length;
  }
  check(end, text.length);
  return out;
}
