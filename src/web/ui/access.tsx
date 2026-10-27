import { h, Fragment } from "../jsx.ts";
import type { Rec } from "../ctx.ts";
import { Name, Muted } from "./kit.tsx";

export function AccessRows({ rec }: { rec: Rec }) {
  const names = (values?: string[]) => values === undefined ? <Muted>Not visible to you</Muted>
    : values.length ? values.map((name, i) => <>{i ? ", " : ""}<Name>{name}</Name></>) : <Muted>none</Muted>;
  return <dl class="pairs"><dt>Allow</dt><dd>{names(rec.allow)}</dd><dt>Maintainers</dt><dd>{names(rec.maintainers ?? [])}</dd></dl>;
}
