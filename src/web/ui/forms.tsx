// Form fields and refusal recovery (src/web/web-face/shell.md#form-recovery).
// A field is marked invalid only when the face can tell which one without
// guessing from prose; every other field says aria-invalid="false".
import { h, Fragment, type Child } from "../jsx.ts";
import { FIELD_DOCUMENTATION } from "./documentation.tsx";
import { normalizeTags, recordTags } from "../tags.ts";
import { Icon, Help } from "./kit.tsx";
import { MAINTAINER } from "../glyphs.ts";

export type FormError = { message: string; field?: string; line?: number; status: number };
export type FormState = { values: Record<string, string>; error?: FormError };

/** Split a list field into its terms; empty lines drop. */
export function terms(v: string): string[] {
  const out: string[] = []; let start = 0, depth = 0;
  for (let i = 0; i < v.length; i++) {
    const c = v[i];
    if (c === "(") depth++;
    else if (c === ")") { if (depth > 0) depth--; }
    else if (depth === 0 && /[\s,]/.test(c)) {
      const term = v.slice(start, i).trim(); if (term) out.push(term); start = i + 1;
    }
  }
  const term = v.slice(start).trim(); if (term) out.push(term);
  return out;
}
/** The submitted lines, as typed, for line attribution. */
export const lines = (v: string) => v.replace(/\r\n/g, "\n").split("\n");

export function ErrorSummary({ id, error }: { id: string; error?: FormError }) {
  if (!error) return <></>;
  return <section class="form-error" role="alert" aria-labelledby="form-error-title">
    <Icon name="octagon-alert" />
    <div>
      <h2 id="form-error-title">Check this form</h2>
      <p>{error.message}</p>
      <p><a href={`#form-${id}`}>Review the submitted fields</a></p>
    </div>
  </section>;
}

export function TagsField({ st, errId, rec, disabled }: { st: FormState; errId: string; rec?: { tags?: string[]; personal?: boolean }; disabled?: boolean }) {
  const value = st.values.tags != null ? normalizeTags(st.values.tags) : rec ? recordTags(rec).filter(t => t !== "no-tags") : [];
  return <div class="field wide"><label for="f-tags">Tags<Help label="About tags" tip={FIELD_DOCUMENTATION.tags!.tip} doc={FIELD_DOCUMENTATION.tags}/></label>
    <input id="f-tags" name="tags" data-tags type="text" value={value.join(", ")} disabled={disabled} autocomplete="off" spellcheck="false" placeholder="claude, build notes"
      aria-invalid={st.error?.field === "tags" ? "true" : "false"} aria-describedby={st.error?.field === "tags" ? errId : undefined} />
    <p class="hint">Comma-separated. Letters, underscores and spaces; stored lowercase. Empty means no-tags. Only the Owner changes the personal tag.</p>
  </div>;
}

type FieldProps = {
  name: string; label: Child; st: FormState; errId: string; value?: string; placeholder?: string; required?: boolean;
  hint?: Child; help?: { label: string; tip?: string; title?: string; items?: Child[] }; disabled?: boolean; type?: string;
  wide?: boolean; id?: string; autocomplete?: string; min?: string;
};

const invalid = (st: FormState, name: string) => st.error?.field === name;
const aria = (st: FormState, name: string, errId: string) => invalid(st, name)
  ? { "aria-invalid": "true", "aria-describedby": errId } : { "aria-invalid": "false" };

function Label({ p, forId }: { p: FieldProps; forId: string }) {
  const doc=FIELD_DOCUMENTATION[p.name];
  return <label for={forId}>{p.label}{p.required ? <span class="req" aria-hidden="true">*</span> : null}
    {p.help || doc ? <Help label={p.help?.label ?? `About ${p.name}`} tip={p.help?.tip ?? doc?.tip} title={p.help?.title} items={p.help?.items} doc={doc} /> : null}</label>;
}

export function TextField(p: FieldProps & { besideLabel?: Child }) {
  const id = p.id ?? `f-${p.name}`;
  const v = p.value ?? p.st.values[p.name] ?? "";
  return <div class={`field ${p.wide ? "wide" : ""}`}>
    {p.besideLabel ? <div class="field-heading"><Label p={p} forId={id} />{p.besideLabel}</div> : <Label p={p} forId={id} />}
    <input id={id} name={p.name} type={p.type ?? "text"} value={v} placeholder={p.placeholder} required={p.required} disabled={p.disabled}
      autocomplete={p.autocomplete ?? "off"} spellcheck="false" min={p.min} {...aria(p.st, p.name, p.errId)} />
    {p.hint ? <p class="hint">{p.hint}</p> : null}
  </div>;
}

/** A textarea whose lines are numbered, so a "Line N:" refusal points at a visible line. */
export function LinesField(p: FieldProps & { rows?: number }) {
  const id = p.id ?? `f-${p.name}`;
  const v = p.value ?? p.st.values[p.name] ?? "";
  const bad = invalid(p.st, p.name) ? p.st.error?.line : undefined;
  return <div class={`field ${p.wide === false ? "" : "wide"}`}>
    <Label p={p} forId={id} />
    <div class="lines-wrap">
      <div class="gutter" aria-hidden="true"></div>
      <textarea id={id} name={p.name} class="lines" rows={String(p.rows ?? 5)} placeholder={p.placeholder} disabled={p.disabled}
        spellcheck="false" autocomplete="off" data-bad-line={bad ? String(bad) : undefined} {...aria(p.st, p.name, p.errId)}>{v}</textarea>
    </div>
    {p.hint ? <p class="hint">{p.hint}</p> : null}
  </div>;
}

/** Free text on several lines, kept after a refusal: a public key, a note. */
export function TextAreaField(p: FieldProps & { rows?: number }) {
  const id = p.id ?? `f-${p.name}`;
  const v = p.value ?? p.st.values[p.name] ?? "";
  // The parser drops one newline after <textarea>: a value starting with one keeps it.
  return <div class={`field ${p.wide === false ? "" : "wide"}`}>
    <Label p={p} forId={id} />
    <textarea id={id} name={p.name} rows={String(p.rows ?? 3)} autocomplete="off" spellcheck="false" placeholder={p.placeholder}
      disabled={p.disabled} required={p.required} {...aria(p.st, p.name, p.errId)}>{"\n"}{v}</textarea>
    {p.hint ? <p class="hint">{p.hint}</p> : null}
  </div>;
}

/** Stored configuration is replaced only by a nonempty JSON submission. */
export function ConfigurationField({ st, errId, disabled, kind, stored }: { st: FormState; errId: string; disabled?: boolean; kind?: string; stored?: boolean }) {
  return <TextAreaField name="config" label="Configuration (JSON)" st={st} errId={errId} rows={4} disabled={disabled}
    placeholder='{"key": "value"}'
    help={{ label: "About the configuration", tip: "Keep secrets in Secret and config in Configuration. Its Owner and Maintainers see and edit it here.", title: "Configuration", items: ["JSON settings, not secrets. Displayed with indentation; saved compact without changing numbers or string escapes.", "Scripts receive AGENT_BUS_CONFIG at their next start. Keep credentials in Secret instead."] }}
    hint={stored ? "Edit the stored configuration. Leave empty to keep it." : "No configuration is stored. Anything here becomes it."} />;
}

/** Whether a private value exists, never what it is. */
export function privateHint(what: string, stored: boolean | undefined): string {
  return stored ? `A ${what} is stored and not shown. Leave empty to keep it; anything here replaces it.` : `No ${what} is stored. Anything here becomes it.`;
}

/** Secret fields always start empty: a form never displays a stored Secret. */
export function SecretField(p: FieldProps & { rows?: number }) {
  const id = p.id ?? `f-${p.name}`;
  return <div class={`field ${p.wide === false ? "" : "wide"}`}>
    <Label p={p} forId={id} />
    <textarea id={id} name={p.name} rows={String(p.rows ?? 4)} autocomplete="off" spellcheck="false" placeholder={p.placeholder}
      disabled={p.disabled} required={p.required} {...aria(p.st, p.name, p.errId)}></textarea>
    {p.hint ? <p class="hint">{p.hint}</p> : null}
  </div>;
}

export function SelectField(p: FieldProps & { options: [string, string][] }) {
  const id = p.id ?? `f-${p.name}`;
  const v = p.value ?? p.st.values[p.name] ?? "";
  return <div class={`field ${p.wide ? "wide" : ""}`}>
    <Label p={p} forId={id} />
    <select id={id} name={p.name} disabled={p.disabled} {...aria(p.st, p.name, p.errId)}>
      {p.options.map(([val, text]) => <option value={val} selected={val === v}>{text}</option>)}
    </select>
    {p.hint ? <p class="hint">{p.hint}</p> : null}
  </div>;
}

export function CheckField(p: FieldProps & { checked: boolean }) {
  const id = p.id ?? `f-${p.name}`;
  return <div class={`field ${p.wide ? "wide" : ""}`}>
    <label class="check" for={id}>
      <input id={id} type="checkbox" name={p.name} value="on" checked={p.checked} disabled={p.disabled} {...aria(p.st, p.name, p.errId)} />
      <span>{p.label}{p.hint ? <span class="hint"> {p.hint}</span> : null}</span>
      {p.help ? <Help {...p.help} doc={FIELD_DOCUMENTATION[p.name]} /> : null}
    </label>
  </div>;
}

/** Shared registration and settings fields; only the Owner assigns these. */
export function MaintainersField({ st, errId, value, canAssign, hint }: { st: FormState; errId: string; value?: string; canAssign: boolean; hint?: string }) {
  return <>
    {canAssign ? <input type="hidden" name="edit_sharing" value="1" /> : null}
    <LinesField name="maintainers" label={<><Icon name={MAINTAINER.icon} />Maintainers</>} st={st} errId={errId}
      value={value} disabled={!canAssign} placeholder={"user@realm\n@group\n#agent@realm"}
      hint={hint ?? (canAssign ? "One user, group or agent per line; @owner is ACL-only. Maintainers manage this record, except transfer." : "Only the Owner or daemon Owner may change Maintainers.")} />
  </>;
}

export function PersonalField({ st, errId, checked, canAssign, hint }: { st: FormState; errId: string; checked: boolean; canAssign: boolean; hint: string }) {
  return <>
    {canAssign ? <input type="hidden" name="edit_personal" value="1" /> : null}
    <CheckField name="personal" label="Personal" st={st} errId={errId} checked={checked} disabled={!canAssign}
      help={{ label: "About Personal records", tip: hint }} />
  </>;
}

export const FieldError = ({ id, error }: { id: string; error?: FormError }) =>
  error ? <p class="warn" id={id}>{error.message}</p> : <></>;

/** The values a refused form keeps: an explicit allowlist, never a secret. */
export function keep(form: URLSearchParams | undefined, names: string[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const n of names) { const v = form?.get(n); if (v != null) out[n] = v; }
  return out;
}
