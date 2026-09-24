// Form fields and refusal recovery (docs/web-face/shell.md#form-recovery).
// A field is marked invalid only when the face can tell which one without
// guessing from prose; every other field says aria-invalid="false".
import { h, Fragment, type Child } from "../jsx.ts";
import { Icon, Help } from "./kit.tsx";

export type FormError = { message: string; field?: string; line?: number; status: number };
export type FormState = { values: Record<string, string>; error?: FormError };

/** Split a list field into its terms; empty lines drop. */
export const terms = (v: string) => v.split(/\s+/).map(s => s.trim()).filter(Boolean);
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

type FieldProps = {
  name: string; label: Child; st: FormState; errId: string; value?: string; placeholder?: string; required?: boolean;
  hint?: Child; help?: { label: string; tip?: string; title?: string; items?: Child[] }; disabled?: boolean; type?: string;
  wide?: boolean; id?: string; autocomplete?: string; min?: string;
};

const invalid = (st: FormState, name: string) => st.error?.field === name;
const aria = (st: FormState, name: string, errId: string) => invalid(st, name)
  ? { "aria-invalid": "true", "aria-describedby": errId } : { "aria-invalid": "false" };

function Label({ p, forId }: { p: FieldProps; forId: string }) {
  return <label for={forId}>{p.label}{p.required ? <span class="req" aria-hidden="true">*</span> : null}
    {p.help ? <Help label={p.help.label} tip={p.help.tip} title={p.help.title} items={p.help.items} /> : null}</label>;
}

export function TextField(p: FieldProps) {
  const id = p.id ?? `f-${p.name}`;
  const v = p.value ?? p.st.values[p.name] ?? "";
  return <div class={`field ${p.wide ? "wide" : ""}`}>
    <Label p={p} forId={id} />
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

/** A secret or configuration: never prefilled, never returned after a refusal. */
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
    </label>
  </div>;
}

export const FieldError = ({ id, error }: { id: string; error?: FormError }) =>
  error ? <p class="warn" id={id}>{error.message}</p> : <></>;

/** The values a refused form keeps: an explicit allowlist, never a secret. */
export function keep(form: URLSearchParams | undefined, names: string[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const n of names) { const v = form?.get(n); if (v != null) out[n] = v; }
  return out;
}
