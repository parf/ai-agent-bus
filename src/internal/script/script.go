// Package script is what runs a script agent's child, shared by the
// foreground runner (agent-bus start) and the managed runner
// (agent-bus-runner): its command line, the roles it is told, and the
// environment it may inherit. See docs/06-runner-role.md#script-agents.
package script

import (
	"strings"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Argv is the child's command line: the script, with the body as its
// one argument for the args form, preceded by the sender's roles. The child's
// own shell unsets every AB_ROLE_* it started with and then exports only the
// roles the daemon worked out, because a sandboxed child starts from the
// user manager's environment, which the runner can neither see nor strip
// (Plans/R1.0-Release/roles.md#roles-are-for-agents).
func Argv(script string, bodyAsArg bool, body string, roles []string) []string {
	prelude := rolePrelude(roles)
	if bodyAsArg {
		return []string{"sh", "-c", prelude + script + ` "$@"`, "sh", body}
	}
	return []string{"sh", "-c", prelude + script}
}

// rolePrelude clears inherited roles and sets the held ones. A role name is
// lowercase letters, digits and _, so upper-cased it is a shell name already;
// anything else is skipped rather than quoted into the command.
func rolePrelude(roles []string) string {
	var b strings.Builder
	// export -p is the shell's own list, so this needs no program a
	// confined child may not have. Its words include pieces of values, so a
	// word is only a candidate: one that is not a shell name is skipped, and
	// unsetting an AB_ROLE_* that was never set costs nothing. Globbing is off
	// while it runs, and the loop's variable is AB_ROLE_ itself, cleared with
	// the rest, so no variable of the script's is touched.
	b.WriteString(`set -f; for AB_ROLE_ in $(export -p); do case ${AB_ROLE_%%=*} in AB_ROLE_*[!A-Za-z0-9_]*) ;; AB_ROLE_?*) unset "${AB_ROLE_%%=*}";; esac; done; unset AB_ROLE_; set +f; `)
	for _, kv := range RoleEnv(roles) {
		b.WriteString("export " + kv + "; ")
	}
	return b.String()
}

// RoleEnv is AB_ROLE_<NAME>=1 for each role held, the name upper-cased.
func RoleEnv(roles []string) []string {
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		if !protocol.ValidRole(r) {
			continue
		}
		out = append(out, "AB_ROLE_"+strings.ToUpper(r)+"=1")
	}
	return out
}

// ChildEnv is env with every AB_ROLE_* and the token taken out: a role in
// the runner's own environment would be one nobody holds, and a script never
// holds the agent's credential (docs/06-runner-role.md#script-agents).
func ChildEnv(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, "AB_ROLE_") && !strings.HasPrefix(kv, "AGENT_BUS_TOKEN=") {
			out = append(out, kv)
		}
	}
	return out
}
