package ports

// Sandbox confines a script agent's process. It **wraps** a command line
// rather than running one, so the runner keeps the pipes, the environment
// and the exit code it already had, and a sandbox that confines nothing is
// the identity function. See docs/06-dedicated-runner.md#sandboxing.
type Sandbox interface {
	// Name is what the runner tells the operator it got, because an
	// unsandboxed agent must never be a silence.
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
	// Secret is `K=V` too, but a value is never put on the command line,
	// where any process of the host could read it: the sandbox names the key
	// and the child's environment, which the caller sets, carries the value.
	Secret []string
	// Net says the agent asked for a network. Off by default: a script
	// that does not say it needs one does not get one.
	Net bool
	// TmpSize bounds the private /tmp, in bytes; zero leaves it the
	// backend's own. Its pages count against the agent's memory as well.
	TmpSize int64
	// Seccomp says the system-call filter is open on the child's fd 3.
	Seccomp bool
	// Bind are paths mounted read-write at the same place, such as the bus
	// socket an agent that serves itself talks to.
	Bind []string
	// Over are files mounted read-only over a path the child sees, such as a
	// shipped settings file with the host's override merged in. A sandbox
	// that cannot mount them must be refused one, as the runner does for "off".
	Over []Overlay
}

// Overlay is the file Src seen at Dst.
type Overlay struct{ Src, Dst string }
