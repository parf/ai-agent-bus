package main

import (
	"html/template"
	"strconv"
	"strings"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A record has one form, and registering it and editing it are the same form
// with different things filled in. The two used to be separate markup and
// drifted apart field by field: the settings page offered a queue policy to
// kinds that hold no queue and no credential to the one kind that has one,
// while the registration page asked for neither Maintainers nor an agent's
// inbox policy. Every field a kind has is asked once, here.
// See Plans/MVP/web/forms.md#rules.

// Editing says which of the two this render is. FormKind, Field, Ticked and
// MayAssign are what the shared block asks instead of branching on it, so a
// control cannot end up reading the stored value on one page and the
// submitted value on the other.

// FormKind is the kind whose questions the form asks: the one being
// registered, or the one the record already is.
func (v adminView) FormKind() string {
	if v.Editing {
		return v.Record.Kind
	}
	return v.NewKind
}

// Submitted reports whether this render is a refused submission of this very
// form coming back. A refusal of some other form on the page — a replaced
// configuration, an ownership transfer — carries none of these fields, and
// reading its values would blank every control the caller did not touch.
func (v adminView) Submitted() bool {
	if v.Editing {
		return v.Form.Action == "save"
	}
	return v.Form.Action == "create"
}

// Field is what a control shows: what the caller submitted when this render
// is that submission coming back, and the stored value otherwise.
func (v adminView) Field(name, stored string) string {
	if v.Submitted() {
		return v.Form.Value(name)
	}
	return stored
}

// Ticked is Field for a checkbox. An unchecked box submits nothing, so an
// absent value in a submission that did come back is a real "off".
func (v adminView) Ticked(name string, stored bool) bool {
	if v.Submitted() {
		return v.Form.Checked(name)
	}
	return stored
}

// MayAssign is whether this caller may set the owner-only fields, Personal
// and Maintainers. Registering a record makes you its owner, so the question
// only has a No on the settings page.
func (v adminView) MayAssign() bool { return !v.Editing || v.Record.CanTransfer }

// ErrorID is the one element a refused field points at with aria-describedby,
// named for the form it belongs to so two forms on one page cannot claim it.
func (v adminView) ErrorID() string {
	if v.Editing {
		return "save-error"
	}
	return "create-error"
}

// recordFields is the whole field set, for every record kind. A kind's
// questions are chosen here and nowhere else.
const recordFields = `{{define "record-fields"}}{{$kind := .FormKind}}{{$err := .ErrorID}}<div class=form-grid>
{{if .Editing}}<input type=hidden name=name value="{{.Record.Name}}">
{{else}}<label class="form-field form-field-wide">Name <input id=create-name name=name required placeholder="name@realm" value="{{.Field "name" ""}}" aria-invalid="{{if .Form.Invalid "name"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "name"}}{{$err}}{{end}}"><small>The routing identity callers use. It cannot be changed afterwards.</small></label>
{{end}}
<label class="form-field form-field-wide">Description <input name=descr value="{{.Field "descr" .Record.Descr}}" placeholder="What this {{recordNoun $kind}} is for"><small>Shown first in the registry.</small></label>
{{if eq $kind "service"}}<label class=form-field>Address <input name=addr required value="{{.Field "addr" .Record.Addr}}" placeholder="host:port, a path, or a URL" aria-invalid="{{if .Form.Invalid "addr"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "addr"}}{{$err}}{{end}}"><small>Where the caller reaches it. Required: a service is not on this bus.</small></label>
<label class=form-field>Protocol <input name=protocol required value="{{.Field "protocol" .Record.Proto}}" placeholder="https, postgresql, smtp" aria-invalid="{{if .Form.Invalid "protocol"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "protocol"}}{{$err}}{{end}}"><small>How the caller speaks to it. A hint in the registry; the daemon neither implements nor checks it.</small></label>
<label class="form-field form-field-wide"><span class=field-heading>Secret <button type=button class=help-button popovertarget=form-secret-help aria-label="About the secret" data-tooltip="Opaque bytes, stored as sent. Only the allow list reads them back, and no page ever shows them again.">ⓘ</button></span><textarea name=secret rows=4 autocomplete=off spellcheck=false placeholder="PGPASSWORD=..."></textarea><small>{{if .Editing}}Leave empty to keep the stored credential. Anything here replaces it.{{else}}Optional. Leave empty to register without one.{{end}}</small></label>
{{end}}
{{if queuePolicy $kind}}<label class=form-field>Queue TTL <input name=ttl value="{{.Field "ttl" .Record.TTL}}" placeholder="default"><small>How long a message in its inbox is worth delivering.</small></label>
<label class=form-field>Queue capacity <input type=number min=0 name=bound value="{{.Field "bound" (digits .Record.Bound)}}" aria-invalid="{{if .Form.Invalid "bound"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "bound"}}{{$err}}{{end}}"><small>0 selects the default.</small></label>
<label class=form-field>Overflow <select name=overflow><option value=strict {{if ne (.Field "overflow" .Record.Full) "ring"}}selected{{end}}>Refuse</option><option value=ring {{if eq (.Field "overflow" .Record.Full) "ring"}}selected{{end}}>Drop oldest</option></select><small>What a full inbox does with the next message.</small></label>
{{end}}
{{if eq $kind "pubsub"}}{{if .Editing}}<input type=hidden name=edit_subs value=1>{{end}}<label class="form-field form-field-wide"><span class=field-heading>Deliver-To <button type=button class=help-button popovertarget=form-deliver-help aria-label="About the Deliver-To list" data-tooltip="Who receives a copy: one user, agent or @group per line. The allow list is who may publish.">ⓘ</button></span><textarea name=subs rows=5 placeholder="someone@realm&#10;agent@realm&#10;@group" aria-invalid="{{if .Form.Invalid "subs"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "subs"}}{{$err}}{{end}}">{{.Field "subs" (lines .Record.Subs)}}</textarea><small>One user, agent or group per line. Empty means a publication here reaches nobody.</small></label>
{{end}}
{{if .Editing}}<input type=hidden name=edit_allow value=1>{{end}}<label class="form-field form-field-wide"><span class=field-heading>Allow list <button type=button class=help-button popovertarget=form-access-help aria-label="About the allow list" data-tooltip="One identity per line. Empty is owner and Maintainers only; @owner adds the records the Owner directly owns; * shares with every admitted principal.">ⓘ</button></span><textarea name=allow rows=5 placeholder="agent@realm&#10;@group&#10;@owner&#10;*">{{.Field "allow" (lines .Record.Allow)}}</textarea><small>{{if eq $kind "pubsub"}}Who may publish here. Who receives is the Deliver-To list above.{{else if .Record.Personal}}One agent identity per line while Personal.{{else}}One identity, group, <code>@owner</code>, or <code>*</code> per line.{{end}}</small></label>
{{if eq $kind "agent"}}{{if .MayAssign}}<input type=hidden name=edit_personal value=1>{{end}}<fieldset class=form-field-wide><legend>Classification</legend><div class=choice-row><label><input type=checkbox name=personal {{if not .MayAssign}}disabled{{end}} {{if .Ticked "personal" .Record.Personal}}checked{{end}}> Personal</label><span class=muted>Groups this agent in the owner&rsquo;s Personal view; access is unchanged.</span></div><small>{{if .MayAssign}}A Personal agent may allow only other registered agents directly. Users, groups, <code>@owner</code>, <code>*</code>, this agent and Maintainers are refused, so clear Personal in this same form before adding any of them.{{else}}Shown for reference: only this record&rsquo;s Owner or a daemon administrator may change it.{{end}}</small></fieldset>
{{end}}
{{if .MayAssign}}<input type=hidden name=edit_sharing value=1>{{end}}<label class="form-field form-field-wide">Maintainers<textarea name=maintainers rows=5 {{if not .MayAssign}}disabled{{end}} aria-invalid="{{if .Form.Invalid "maintainers"}}true{{else}}false{{end}}" aria-describedby="{{if .Form.Invalid "maintainers"}}{{$err}}{{end}}">{{.Field "maintainers" (lines .Record.Maintainers)}}</textarea><small>{{if .MayAssign}}One user, group, agent or service per line; <code>@owner</code> is ACL-only.{{else}}Shown for reference: only this record&rsquo;s Owner or a daemon administrator may change it.{{end}}</small></label></div>
{{if .Form.Is .FormAction}}<p class=warn id="{{$err}}">{{.Form.Error}}</p>{{end}}{{end}}`

// recordFieldHelp is every explanation the field set points at, rendered once
// beside it rather than inside the grid.
const recordFieldHelp = `{{define "record-field-help"}}{{$kind := .FormKind}}
<div popover id=form-access-help class=context-help><h2>Allow list</h2><ul>{{if eq $kind "pubsub"}}<li>On a 📣 it is who may <strong>publish</strong>. It decides nothing about who receives.</li>{{end}}<li>Empty allows only the owner and assigned Maintainers.</li><li><code>@owner</code> adds records directly owned by this record&rsquo;s Owner. It is runtime ACL syntax, not an editable group.</li><li><code>*</code> shares with every admitted principal.</li>{{if eq $kind "agent"}}<li>A Personal agent may name only other registered agents directly; users, groups, <code>@owner</code>, <code>*</code>, itself and Maintainers are refused.</li>{{end}}</ul></div>
{{if eq $kind "pubsub"}}<div popover id=form-deliver-help class=context-help><h2>The Deliver-To list</h2><ul><li>Who receives a copy of every publication, in that name&rsquo;s own inbox.</li><li>Users and agents only: a service and a channel have no inbox a copy can land in.</li><li>A <code>@group</code> is expanded when the publish happens, so adding somebody to the group adds them to the delivery.</li><li>It is not the allow list. The allow list is who may publish here.</li></ul></div>{{end}}
{{if eq $kind "service"}}<div popover id=form-secret-help class=context-help><h2>The service secret</h2><ul><li>Opaque bytes. The daemon stores what it is sent and does not parse it; <code>KEY=value</code> is a convention between callers.</li><li>Whoever the allow list admits reads the bytes back with <code>agent-bus secret &lt;name&gt;</code>. Every listing and record answer carries only its digest.</li><li>This field is never filled in again, here or anywhere else &mdash; not after a refusal and not on the record&rsquo;s own page.</li></ul></div>{{end}}{{end}}`

// recordEdit is the settings form on a page of its own. The detail page links
// here rather than carrying the editor, so the form has one address, one
// shape and one place to come back to when a field is refused.
var recordEdit = template.Must(template.New("record-edit").Funcs(recordFormFuncs).Parse(
	shellTitle("records", `Edit {{.Record.Name}}`) + `
<p><a href="{{href .Record}}?name={{.Record.Name}}{{with .Return}}&amp;return={{urlquery .}}{{end}}">Back to {{.Record.Name}}</a></p>
<div class=page-title><h1>{{titleMark .Record.Kind}} Edit {{recordNoun .Record.Kind}} {{.Record.Name}}</h1></div>` + formErrorSummary + `
<form id=form-save class="editor-card task-card" method=post action=/service><input type=hidden name=action value=save>{{with .Return}}<input type=hidden name=return value="{{.}}">{{end}}` +
		`{{template "record-fields" .}}<div class=form-actions><button>Save settings</button></div></form>
{{template "record-field-help" .}}
<p><a class=danger href="/service-danger?name={{.Record.Name}}">Danger Zone</a></p>
` + recordFields + recordFieldHelp))

// FormAction names the verb this form submits, which is also what a refusal
// of it comes back as.
func (v adminView) FormAction() string {
	if v.Editing {
		return "save"
	}
	return "create"
}

// queuePolicy is the kinds that hold an inbox of their own, and so have a
// retention, a capacity and an overflow policy to declare. A 📣 keeps
// nothing — the copies are kept by the inboxes they land in — and a 📡 has
// no queue here at all.
// See docs/07-channels.md#the-two-channel-kinds.
func queuePolicy(kind string) bool {
	return kind == protocol.KindAgent || kind == protocol.KindQueue
}

// lines is a stored list as a textarea holds it, one per line.
func lines(list []string) string { return strings.Join(list, "\n") }

// digits is a stored number as a form control holds it: plain, ungrouped and
// never empty, because an empty capacity is not a number the save path can
// read back.
func digits(n int) string { return strconv.Itoa(n) }

var recordFormFuncs = template.FuncMap{
	"titleMark":   titleMark,
	"recordNoun":  recordNoun,
	"href":        detailPath,
	"queuePolicy": queuePolicy,
	"lines":       lines,
	"digits":      digits,
}
