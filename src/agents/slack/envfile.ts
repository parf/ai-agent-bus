// Adapted from motdotla/dotenv; copyright Scott Motte, BSD-2-Clause (licenses/NOTICE.md).
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
  // Counted forward once: re-splitting the text before every match was quadratic.
  let line = 1, counted = 0;
  const check = (start: number, finish: number) => {
    for (; counted < start; counted++) if (LINE_END.test(text[counted]!)) line++;
    let at = line;
    for (const raw of text.slice(start, finish).split(/[\n\u2028\u2029]/g)) {
      const s = raw.trim();
      if (s && !s.startsWith("#")) throw new Error(`${source}: line ${at}: expected a dotenv assignment`);
      at++;
    }
  };
  for (const match of text.matchAll(LINE)) {
    check(end, match.index!);
    let value = (match[2] || "").trim();
    const quote = value[0];
    // Nothing is expanded, so every reference is an unknown one (internal/envfile).
    if (quote !== "'" && quote !== "`" && value.includes("$") && REFERENCE.test(value)) {
      check(end, match.index!);
      const key = text.indexOf(match[1]!, match.index!);
      throw new Error(`${source}: line ${lines(text.slice(0, key)) + 1}: a reference ($NAME, \${…} or $(…)) is never expanded; put the value in '…' to keep a $`);
    }
    value = unquote(value);
    if (quote === '"') value = value.replace(/\\n/g, "\n").replace(/\\r/g, "\r");
    out.push({ key: match[1]!, value });
    end = match.index! + match[0].length;
  }
  check(end, text.length);
  return out;
}

const LINE_END = /[\n\u2028\u2029]/;
const REFERENCE = /\$[A-Za-z_{(]/;
const lines = (s: string) => { let n = 0; for (const c of s) if (LINE_END.test(c)) n++; return n; };
const DOLLAR = /[\n\r\u2028\u2029]/;

// What value.replace(/^(['"`])([\s\S]*)\1$/mg, "$2") does, in linear time:
// a pair opening at a line start closes at the last quote of its kind that
// ends a line. The regex backtracked to it from every opening.
function unquote(s: string): string {
  const last: Record<string, number> = {};
  for (let j = 0; j < s.length; j++) {
    const q = s[j]!;
    if ((q === "'" || q === '"' || q === "`") && (j + 1 === s.length || DOLLAR.test(s[j + 1]!))) last[q] = j;
  }
  let out = "", copied = 0;
  for (let i = 0; i < s.length; i++) {
    if (i > 0 && !DOLLAR.test(s[i - 1]!)) continue;
    const q = s[i]!;
    if (q !== "'" && q !== '"' && q !== "`") continue;
    const j = last[q] ?? -1;
    if (j <= i) continue;
    out += s.slice(copied, i) + s.slice(i + 1, j);
    copied = j + 1;
    i = j;
  }
  return out + s.slice(copied);
}
