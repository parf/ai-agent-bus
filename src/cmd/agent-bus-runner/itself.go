package main

import (
	"context"
	"fmt"
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

// keeper keeps one self-serving agent's processes running.
type keeper struct {
	r      *runner
	s      *served
	token  string
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	running map[*exec.Cmd]*jail    // successful starts and their actual launch jails
	groups  map[*cgroup.Group]bool // groups actually used, retained for retirement cleanup
}

// keep starts an agent's processes and returns its keeper.
func (r *runner) keep(s *served, token string) *keeper {
	ctx, cancel := context.WithCancel(r.ctx)
	k := &keeper{r: r, s: s, token: token, ctx: ctx, cancel: cancel, running: map[*exec.Cmd]*jail{}, groups: map[*cgroup.Group]bool{}}
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

// count is how many of its processes are running now.
func (k *keeper) count() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.running)
}

func (k *keeper) loop() {
	defer k.wg.Done()
	wait := backoffFirst
	for k.ctx.Err() == nil {
		began := time.Now()
		cmd, j, err := k.command()
		if err == nil {
			k.mu.Lock()
			if k.ctx.Err() != nil {
				err = k.ctx.Err()
			} else {
				err = cmd.Start()
				if err == nil {
					k.running[cmd] = j
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
			err = cmd.Wait()
			k.mu.Lock()
			delete(k.running, cmd)
			k.mu.Unlock()
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
	j, env := launch.jail, launch.env
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
	cmd.Stdout = prefixed{k.r.say, s.name + ": "}
	cmd.Stderr = prefixed{k.r.say, s.name + ": "}
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
	wait := stopWait
	launch := k.r.launchSettings(k.s)
	if d, err := time.ParseDuration(launch.agent.Deadline); err == nil && d > 0 {
		wait = d
	}
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

// signal sends sig to the agent's own processes: those in its cgroup that are
// not bubblewrap, or, with the sandbox off, each process group it started.
func (k *keeper) signal(sig syscall.Signal) {
	k.mu.Lock()
	defer k.mu.Unlock()
	seen := make(map[*cgroup.Group]bool)
	for cmd, j := range k.running {
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
		os.WriteFile(filepath.Join(g.Path(), "cgroup.kill"), []byte("1"), 0o644)
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
