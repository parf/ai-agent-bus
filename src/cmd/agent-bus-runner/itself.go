package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/cgroup"
	"github.com/parf/ai-agent-bus/internal/ports"
)

// Agents that serve themselves, wave M.9
// (Plans/R1.0-Release/TODO.md#runner-waves): `serves: itself` in agent.json.
// The runner claims the agent's token as for any agent, then keeps a fixed
// number of its processes running — each one reading its own inbox with that
// token through a client library — and restarts one that exits, with backoff.
// Unlike a one-message script, such a process is handed its token: serving
// itself is what it was written to do. It is not handed the bus secret, which
// it reads itself, as its own record's.

// The restart backoff: a process that ran for stable long is restarted at
// once the next time; one that keeps dying waits longer, up to backoffMost.
var (
	backoffFirst = time.Second
	backoffMost  = 30 * time.Second
	stable       = time.Minute
)

// stopWait is how long a stopping process has to finish before its cgroup is
// killed whole, unless agent.json gives a deadline.
const stopWait = 10 * time.Second

// socketCheck is how often a kept sandboxed process's bus socket is looked at.
var socketCheck = 2 * time.Second

// keeper keeps one self-serving agent's processes running.
type keeper struct {
	r      *runner
	s      *served
	token  string
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	running map[*exec.Cmd]keptProcess // successful starts and their actual launch jails
	groups  map[*cgroup.Group]bool    // groups actually used, retained for retirement cleanup
}

// keep starts an agent's processes and returns its keeper.
func (r *runner) keep(s *served, token string) *keeper {
	ctx, cancel := context.WithCancel(r.ctx)
	k := &keeper{r: r, s: s, token: token, ctx: ctx, cancel: cancel, running: map[*exec.Cmd]keptProcess{}, groups: map[*cgroup.Group]bool{}}
	n := s.in.Span.Max
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		k.wg.Add(1)
		go k.loop()
	}
	fmt.Fprintf(r.say, "%s: keeping %s, %d process(es), sandbox %s\n", r.name, s.name, n, s.box.Name())
	return k
}

// A live process retains only its actual jail and current start time.
type keptProcess struct {
	jail      *jail
	startedAt time.Time
}

// snapshot counts live processes and their earliest known start.
func (k *keeper) snapshot() (int, *time.Time) {
	k.mu.Lock()
	defer k.mu.Unlock()
	var earliest time.Time
	for _, process := range k.running {
		if !process.startedAt.IsZero() && (earliest.IsZero() || process.startedAt.Before(earliest)) {
			earliest = process.startedAt
		}
	}
	if earliest.IsZero() {
		return len(k.running), nil
	}
	return len(k.running), &earliest
}

func (k *keeper) loop() {
	defer k.wg.Done()
	wait := backoffFirst
	for k.ctx.Err() == nil {
		began := time.Now()
		cmd, j, err := k.command()
		var sock *boundSocket
		if err == nil {
			sock = k.socketToWatch()
			k.mu.Lock()
			if k.ctx.Err() != nil {
				err = k.ctx.Err()
			} else {
				err = cmd.Start()
				if err == nil {
					k.running[cmd] = keptProcess{jail: j, startedAt: time.Now().UTC()}
					if j.group != nil {
						k.groups[j.group] = true
					}
				}
			}
			k.mu.Unlock()
			// The filter's pipe is the child's now, or nobody's.
			for _, f := range cmd.ExtraFiles {
				f.Close()
			}
		}
		if err == nil {
			var exited chan struct{}
			if sock != nil {
				exited = make(chan struct{})
				go k.watchSocket(cmd, j, sock, exited)
			}
			err = cmd.Wait()
			if exited != nil {
				close(exited)
			}
			k.mu.Lock()
			delete(k.running, cmd)
			k.mu.Unlock()
		}
		if cmd != nil {
			for _, out := range []io.Writer{cmd.Stdout, cmd.Stderr} {
				if filtered, ok := out.(*stderrLine); ok {
					filtered.flush()
				}
			}
		}
		k.r.recordExit(k.s, k, cmd)
		if reason, unconfigured := configurationExit(cmd, err); unconfigured && k.ctx.Err() == nil {
			k.r.notConfigured(k.s, k, reason)
			return
		}
		if j != nil && k.ctx.Err() == nil {
			k.r.checkDisk(k.s, j)
		}
		if k.ctx.Err() != nil {
			return
		}
		if time.Since(began) >= stable {
			wait = backoffFirst
		}
		fmt.Fprintf(k.r.say, "%s: %s exited (%v); restarting in %s\n", k.r.name, k.s.name, err, wait)
		select {
		case <-time.After(wait):
		case <-k.ctx.Done():
			return
		}
		if wait *= 2; wait > backoffMost {
			wait = backoffMost
		}
	}
}

// command is one process of the agent, confined as a script is, with the bus
// socket mounted in and its own identity in its environment.
func (k *keeper) command() (*exec.Cmd, *jail, error) {
	s := k.s
	launch := k.r.launchSettings(s)
	if _, err := k.r.workPath(launch.key); err != nil {
		return nil, nil, err
	}
	j := launch.jail
	if j.group != nil {
		if err := j.group.Prepare(); err != nil {
			return nil, nil, fmt.Errorf("its cgroup: %w", err)
		}
	}
	env, err := configuration(context.Background(), s.bus, s.name, launch.env)
	if err != nil {
		return nil, nil, err
	}
	job := ports.Job{
		Argv: []string{"/bin/sh", "-c", "exec " + launch.cmd},
		Work: s.work, Read: []string{s.dir}, Net: launch.net, TmpSize: j.tmp, Seccomp: len(j.seccomp) > 0,
	}
	addr := k.r.addr
	if strings.HasPrefix(addr, "/") {
		job.Bind = []string{addr}
	}
	argv := launch.box.Wrap(job)
	if j.files > 0 {
		argv = append([]string{"prlimit", fmt.Sprintf("--nofile=%d:%d", j.files, j.files), "--"}, argv...)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = s.work
	attr := &syscall.SysProcAttr{Setpgid: true}
	if j.group != nil {
		attr.UseCgroupFD, attr.CgroupFD = true, j.group.FD()
	}
	cmd.SysProcAttr = attr
	base := []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=" + s.work, "LANG=C.UTF-8"}
	cmd.Env = mergeEnv(base, env, []string{
		"AGENT_BUS_NAME=" + s.name, "AGENT_BUS_TOKEN=" + k.token, "AGENT_BUS_ADDR=" + addr, "AGENT_BUS_WORK=" + s.work})
	private := append(append([]string{}, launch.secretEnv...), "AGENT_BUS_TOKEN="+k.token)
	cmd.Stdout = secretStderr(prefixed{k.r.say, s.name + ": "}, private)
	cmd.Stderr = secretStderr(prefixed{k.r.say, s.name + ": "}, private)
	if len(j.seccomp) > 0 {
		rd, wr, err := os.Pipe()
		if err != nil {
			return nil, nil, err
		}
		_, werr := wr.Write(j.seccomp)
		wr.Close()
		if werr != nil {
			rd.Close()
			return nil, nil, werr
		}
		cmd.ExtraFiles = []*os.File{rd}
	}
	return cmd, j, nil
}

// stop ends the agent: no more restarts, SIGTERM to its own processes — not
// to bubblewrap, which would take them down without their drain — then, after
// the deadline, the whole cgroup killed. It returns when every loop is done.
func (k *keeper) stop() {
	k.cancel()
	wait := k.deadline()
	k.signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { k.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(wait):
		k.signal(syscall.SIGKILL)
		<-done
	}
	fmt.Fprintf(k.r.say, "%s: %s stopped\n", k.r.name, k.s.name)
}

// deadline is how long a process asked to stop has before it is killed.
func (k *keeper) deadline() time.Duration {
	if d, err := time.ParseDuration(k.r.launchSettings(k.s).agent.Deadline); err == nil && d > 0 {
		return d
	}
	return stopWait
}

// A sandbox mounts the bus socket as a file, and the mount keeps the inode it
// found. A daemon that restarts makes a new socket at the same path, which a
// running process cannot see: it would retry the dead one forever (J30). So a
// kept sandboxed process is restarted when the socket at that path is replaced.
// The socket's directory is not mounted instead: it also holds the account
// sockets, and the runner account's would hand the agent the runner's identity.
type boundSocket struct {
	path     string
	dev, ino uint64
}

func socketIdentity(path string) (dev, ino uint64, ok bool) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil || st.Mode&syscall.S_IFMT != syscall.S_IFSOCK {
		return 0, 0, false
	}
	return uint64(st.Dev), st.Ino, true
}

// socketToWatch is the bus socket a process about to start gets mounted, or
// nil: no socket path, or no sandbox, whose process opens the path afresh.
// It is read just before the start, so a socket replaced in between costs one
// extra restart. A path with no socket at the start is not watched: the mount
// itself fails and the loop retries with backoff until the daemon is back.
// A device and inode reused by a new socket, as tmpfs can, would go unseen.
func (k *keeper) socketToWatch() *boundSocket {
	addr := k.r.addr
	if !strings.HasPrefix(addr, "/") || k.r.launchSettings(k.s).box.Name() == "off" {
		return nil
	}
	dev, ino, ok := socketIdentity(addr)
	if !ok {
		return nil
	}
	return &boundSocket{path: addr, dev: dev, ino: ino}
}

// watchSocket stops one process, as stop does, once another socket is at the
// path it was given; the loop then starts it again with the live one. A path
// with no socket yet is a daemon still starting, and is waited out.
func (k *keeper) watchSocket(cmd *exec.Cmd, j *jail, sock *boundSocket, exited <-chan struct{}) {
	tick := time.NewTicker(socketCheck)
	defer tick.Stop()
	for {
		select {
		case <-exited:
			return
		case <-k.ctx.Done():
			return
		case <-tick.C:
		}
		dev, ino, ok := socketIdentity(sock.path)
		if !ok || (dev == sock.dev && ino == sock.ino) {
			continue
		}
		fmt.Fprintf(k.r.say, "%s: %s: the bus socket was replaced; restarting it\n", k.r.name, k.s.name)
		k.signalOne(cmd, j, syscall.SIGTERM)
		select {
		case <-exited:
		case <-k.ctx.Done():
		case <-time.After(k.deadline()):
			k.signalOne(cmd, j, syscall.SIGKILL)
		}
		return
	}
}

// signalOne sends sig to one process and what it started, never to the rest
// of the agent's cgroup: its siblings share it, and one may already run on the
// new socket (J30b). As in stop, TERM skips bubblewrap so the agent can drain;
// KILL takes bubblewrap too, and its --die-with-parent ends the sandbox.
func (k *keeper) signalOne(cmd *exec.Cmd, j *jail, sig syscall.Signal) {
	if cmd.Process == nil {
		return
	}
	if j == nil || j.group == nil {
		syscall.Kill(-cmd.Process.Pid, sig)
		return
	}
	for _, pid := range descendants(cmd.Process.Pid) {
		if sig != syscall.SIGKILL {
			if comm, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); strings.TrimSpace(string(comm)) == "bwrap" {
				continue
			}
		}
		syscall.Kill(pid, sig)
	}
}

// descendants is root and every process below it, from /proc. A sandbox's own
// processes stay its descendants: bubblewrap's new session and namespaces
// change neither their parent nor what this namespace sees of them.
func descendants(root int) []int {
	entries, _ := os.ReadDir("/proc")
	children := map[int][]int{}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			continue
		}
		// pid (comm) state ppid ...: comm may hold spaces and parentheses.
		i := strings.LastIndexByte(string(stat), ')')
		fields := strings.Fields(string(stat[i+1:]))
		if i < 0 || len(fields) < 2 {
			continue
		}
		if ppid, err := strconv.Atoi(fields[1]); err == nil {
			children[ppid] = append(children[ppid], pid)
		}
	}
	out := []int{root}
	for n := 0; n < len(out); n++ {
		out = append(out, children[out[n]]...)
	}
	return out
}

// signal sends sig to the agent's own processes: those in its cgroup that are
// not bubblewrap, or, with the sandbox off, each process group it started.
func (k *keeper) signal(sig syscall.Signal) {
	k.mu.Lock()
	defer k.mu.Unlock()
	seen := make(map[*cgroup.Group]bool)
	for cmd, process := range k.running {
		j := process.jail
		if j.group != nil {
			if !seen[j.group] {
				signalGroup(j.group, sig)
				seen[j.group] = true
			}
		} else {
			syscall.Kill(-cmd.Process.Pid, sig)
		}
	}
}

func signalGroup(g *cgroup.Group, sig syscall.Signal) {
	if sig == syscall.SIGKILL {
		_ = g.Kill()
		return
	}
	b, _ := os.ReadFile(filepath.Join(g.Path(), "cgroup.procs"))
	for _, f := range strings.Fields(string(b)) {
		pid, err := strconv.Atoi(f)
		if err != nil {
			continue
		}
		if comm, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); strings.TrimSpace(string(comm)) == "bwrap" {
			continue
		}
		syscall.Kill(pid, sig)
	}
}
