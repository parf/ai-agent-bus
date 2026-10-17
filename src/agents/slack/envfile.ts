// Dotenv parsing for the record's bus secret, in the agent's own shape:
// no library, because the libraries expand $VAR and a secret is what is
// written (Plans/R1.2/slack/README.md#file-formats owns the common rules).
//
//	# a comment             KEY=value  # a comment after it
//	export KEY=value        KEY='literal, # and $ included'
//	KEY="with \n \" \\ escapes"         KEY=
//
// One value per line; a quoted value ends on its own line. Nothing expands.

export type EnvValue = { key: string; value: string };

const name = /^[A-Za-z_][A-Za-z0-9_]*$/;

/** Parse dotenv bytes, naming the source and line of anything unreadable. */
export function parseEnvFile(source: string, bytes: Uint8Array): Record<string, string> {
  const out: Record<string, string> = {};
  const text = new TextDecoder().decode(bytes);
  for (const parsed of parseEnvList(source, text)) out[parsed.key] = parsed.value;
  return out;
}

export function parseEnvList(source: string, text: string): EnvValue[] {
  const out: EnvValue[] = [];
  const lines = text.split("\n");
  for (let i = 0; i < lines.length; i++) {
    const where = `${source}:${i + 1}`;
    let line = lines[i]!.replace(/\r$/, "").trim();
    if (line === "" || line.startsWith("#")) continue;
    if (line.startsWith("export ")) line = line.slice("export ".length).trim();
    const at = line.indexOf("=");
    if (at < 0) throw new Error(`${where}: no = in "${line}"; a variable with nothing after it is KEY=`);
    const key = line.slice(0, at).trim();
    if (!name.test(key)) throw new Error(`${where}: "${key}" is not a variable name`);
    out.push({ key, value: value(where, key, line.slice(at + 1).replace(/^[\t ]+/, "")) });
  }
  return out;
}

/** One value as written: single quotes literal, double quotes with \n \t \" \\ escapes, an unquoted one up to a # after whitespace. */
function value(where: string, key: string, raw: string): string {
  if (raw.startsWith("'")) {
    const end = raw.indexOf("'", 1);
    if (end < 0) throw new Error(`${where}: ${key}: the ' is never closed`);
    return raw.slice(1, end);
  }
  if (raw.startsWith('"')) {
    let out = "";
    for (let i = 1; i < raw.length && raw[i] !== '"'; i++) {
      if (raw[i] === "\\" && i + 1 < raw.length) {
        i++;
        const c = raw[i];
        if (c === "n") out += "\n";
        else if (c === "t") out += "\t";
        else if (c === '"' || c === "\\") out += c;
        else out += "\\";
        continue;
      }
      out += raw[i];
    }
    if (!raw.endsWith('"') || raw.length < 2) throw new Error(`${where}: ${key}: the " is never closed`);
    return out;
  }
  const cut = raw.indexOf(" #");
  return (cut < 0 ? raw : raw.slice(0, cut)).trimEnd();
}
