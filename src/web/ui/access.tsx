import { h, Fragment } from "../jsx.ts";
import type { Rec } from "../ctx.ts";
import { Name } from "./kit.tsx";

export function AccessRows({ rec }: { rec: Rec }) {
  const allow = rec.allow ?? [], maintainers = rec.maintainers ?? [];
  if (!allow.length && !maintainers.length) return null;
  const names = (values: string[]) => values.map((name, i) => <>{i ? ", " : ""}<Name>{name}</Name></>);
  return <dl class="pairs">{allow.length ? <><dt>Allow</dt><dd>{names(allow)}</dd></> : null}
    {maintainers.length ? <><dt>Maintainers</dt><dd>{names(maintainers)}</dd></> : null}</dl>;
}
