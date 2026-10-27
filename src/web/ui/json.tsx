import { h, Fragment, type Child } from "../jsx.ts";

/** Token spans contain only escaped text; their textContent is the input verbatim. */
export function Json({ value }: { value: string }) {
  try { JSON.parse(value); } catch { return <>{value}</>; }
  const parts: Child[] = [];
  const tokens = /"(?:\\[\s\S]|[^"\\])*"|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?|\b(?:true|false|null)\b|[{}\[\],:]/g;
  let at = 0;
  for (const match of value.matchAll(tokens)) {
    const token = match[0], start = match.index!;
    parts.push(value.slice(at, start));
    const kind = token[0] === '"' ? (/^\s*:/.test(value.slice(start + token.length)) ? "key" : "string")
      : /^[-\d]/.test(token) ? "number" : /^(true|false|null)$/.test(token) ? "literal" : "punctuation";
    parts.push(<span class={`json-${kind}`}>{token}</span>);
    at = start + token.length;
  }
  parts.push(value.slice(at));
  return <>{parts}</>;
}
