package ports

// Sandbox confines a script service's process. It **wraps** a command line
// rather than running one, so the runner keeps the pipes, the environment
// and the exit code it already had, and a sandbox that confines nothing is
// the identity function. See docs/08-runner-role.md#sandboxing.
type Sandbox interface {
	// Name is what the runner tells the operator it got, because an
	// unsandboxed service must never be a silence.
	Name() string
	// Wrap returns the command line to run instead of the job's own.
	Wrap(Job) []string
}

// Job is one run of a script: what to run, where it may write, what it must
// still be able to read, and what it should find in its environment.
type Job struct {
	Argv []string
	// Work is the one directory the child may write to, and its working
	// directory.
	Work string
	// Read are paths it must keep — the script's own above all, which a
	// private /tmp would otherwise hide.
	Read []string
	// Env is `K=V`, as exec.Cmd takes it. It is stated rather than inherited
	// because a transient unit starts from the user manager's environment
	// and not from the runner's.
	Env []string
	// Net says the service asked for a network. Off by default: a script
	// that does not say it needs one does not get one.
	Net bool
}
