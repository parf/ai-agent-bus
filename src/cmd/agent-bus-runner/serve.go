package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/parf/ai-agent-bus/internal/agentmeta"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/cgroup"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/sandbox"
	"github.com/parf/ai-agent-bus/internal/script"
)

// Serving the installed agents, waves M.2 and M.10
// (Plans/R1.0-Release/TODO.md#runner-waves). Each agent is its
// person's: the runner creates it, keeps the token the creating register
// answered, and reads the agent's own inbox and secret as the agent, so a busy
// agent never holds an idle one's message and nothing is forwarded. It answers
// as itself.

// maxOutput bounds what one run may answer with: the answer is held in the
// runner, outside the child's limits, so it is cut and said rather than
// buffered whole.
const maxOutput = 1 << 20

// served is one agent the runner is serving now.
type served struct {
	readDone chan struct{} // reader has stopped consuming before a mode replacement

	in        installed
	name      string
	cmd       string // the command line, ./ resolved to the agent's directory
	work      string
	bus       *client // as the agent, with its own token
	env       []string
	secretEnv []string // bus Secret and private host-file entries only
	slots     chan struct{}
	busy      atomic.Int32
	box       ports.Sandbox // bwrap unless the host said sandbox off
	jail      *jail         // its cgroup and limits; read under r.mu
	kept      *keeper       // an agent that serves itself: its processes
	dir       string        // the agent's checkout, the one directory it reads
	net       bool          // its agent.json asked for the network
	cancel    context.CancelFunc
}

// launchSettings is one start's view of the fields reload replaces. The other
// served fields (name, work, bus and dir) are fixed at adoption.
type launchSettings struct {
	key       string
	agent     agentFile
	cmd       string
	env       []string
	secretEnv []string
	box       ports.Sandbox
	net       bool
	jail      *jail
}

func (r *runner) launchSettings(s *served) launchSettings {
	r.mu.Lock()
	defer r.mu.Unlock()
	return launchSettings{key: s.in.Key, agent: s.in.Agent, cmd: s.cmd, env: s.env, secretEnv: s.secretEnv, box: s.box, net: s.net, jail: s.jail}
}

// serveable says why an agent cannot be served by this wave, or "".
func serveable(in installed) string {
	switch in.Agent.Form {
	case "", "args", "json":
	default:
		return fmt.Sprintf("the %s form is not served yet (M.3)", in.Agent.Form)
	}
	// An agent that serves itself is kept at a fixed count: its processes
	// are not started per message, so there is nothing to grow or shrink by.
	if in.Agent.Serves == "itself" && in.Span.Min != in.Span.Max && !(in.Span.Min == 0 && in.Span.Max == 1) {
		return fmt.Sprintf("an agent that serves itself takes a fixed count of processes, not %d..%d", in.Span.Min, in.Span.Max)
	}
	return ""
}

// environment is the agent's layered env, or why it cannot be had: a
// template run as itself, or an instance still missing what env.dist requires.
func (r *runner) environment(in installed, as *client) ([]string, string) {
	env, _, why := r.environmentWithSecrets(in, as)
	return env, why
}

// Keep provenance separate from the merged environment: defaults and
// Configuration are public settings, while Secret and host files are private.
func (r *runner) environmentWithSecrets(in installed, as *client) ([]string, []string, string) {
	dir := filepath.Join(r.agents, in.Dir)
	dist, err := readEnv(filepath.Join(dir, "env.dist"))
	if err != nil {
		return nil, nil, err.Error()
	}
	if in.Agent.Serves == "itself" {
		dist = nil
	} // env.dist describes keys only for self-serving agents.
	// An agent that serves itself reads its own secret, as its record's; it
	// is not copied into its environment.
	var secret []envVar
	if in.Agent.Serves != "itself" {
		var err error
		if secret, err = busSecret(as, r.agentName(in)); err != nil {
			return nil, nil, "its bus secret: " + err.Error()
		}
	}
	agentWide, err := readEnv(filepath.Join(r.host, in.Dir, "env"))
	if err != nil {
		return nil, nil, err.Error()
	}
	var own []envVar
	_, instance, isInstance := strings.Cut(in.Key, "/")
	if isInstance {
		if own, err = readEnv(filepath.Join(r.host, in.Dir, instance, "env")); err != nil {
			return nil, nil, err.Error()
		}
	}
	env, missing := layered(dist, secret, agentWide, own)
	if len(missing) > 0 {

		return nil, nil, "needs " + strings.Join(missing, ", ")
	}
	env, err = configuration(context.Background(), as, r.agentName(in), env)
	if err != nil {
		return nil, nil, "its configuration: " + err.Error()
	}
	for _, warning := range agentmeta.Manifest(in.Agent).ConfigWarnings(env) {
		fmt.Fprintf(r.say, "%s: warning: %s\n", r.agentName(in), warning)
	}
	private, _ := layered(nil, secret, agentWide, own)
	warnShortSecrets(r.say, r.agentName(in), private)
	return env, private, ""
}

func configuration(parent context.Context, as *client, name string, env []string) ([]string, error) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	out, code, err := as.do(ctx, "GET", "/config", url.Values{"name": {name}}, nil)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, fmt.Errorf("read refused (%d)", code)
	}
	return script.ConfigEnv(env, out)
}

// busSecret is the agent's own secret, asked as the agent, in env-file form;
// none while it has none.
func busSecret(as *client, name string) ([]envVar, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, code, err := as.do(ctx, "GET", "/secret", url.Values{"name": {name}}, nil)
	switch {
	case err != nil:
		return nil, err
	case code == http.StatusNotFound:
		return nil, nil
	case code >= 400:
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	return parseEnv("bus secret", out)
}

// command resolves a command line written relative to the agent's checkout.
func command(dir, line string) string {
	if strings.HasPrefix(line, "./") {
		return "'" + strings.ReplaceAll(filepath.Join(dir, line[2:]), "'", `'\''`) + "'"
	}
	return line
}

// serve brings what is served in line with what is installed: new agents are
// claimed and read; ones gone or refused are stopped, and what is sent to them
// waits in their own inboxes. It holds r.mu.
func (r *runner) serve(ctx context.Context) {
	want := map[string]bool{}
	socketChecked, socketWhy := false, ""
	// Two installed agents that come to one name would be one served agent
	// with the other's command: both are refused, by name.
	named := map[string][]string{}
	byKey := map[string]string{}
	for _, in := range r.installed {
		if in.wanted() {
			n := r.agentName(in)
			named[n] = append(named[n], in.Key)
			byKey[in.Key] = n
		}
	}
	// A renamed bus record still uses the same cgroup key. Retire its old
	// name before a replacement can take that key, not in the final sweep.
	for name, s := range r.served {
		stop := byKey[s.in.Key] != "" && byKey[s.in.Key] != name
		for key := range byKey {
			if byKey[s.in.Key] == "" && key != s.in.Key && (strings.HasPrefix(key, s.in.Key+"/") || strings.HasPrefix(s.in.Key, key+"/")) {
				stop = true
				break
			}
		}
		if stop {
			r.stop(name, s)
		}
	}
	// Preflight once before work-overlap decisions: an unconfigured template
	// is installed but cannot run alongside (or conflict with) its instance.
	type prepared struct {
		name      string
		as        *client
		env       []string
		secretEnv []string
		box       ports.Sandbox
	}
	ready := make(map[string]prepared)
	for i := range r.installed {
		in := &r.installed[i]
		if !in.wanted() {
			continue
		}
		if _, err := r.workPath(in.Key); err != nil {
			in.Refused = err.Error()
			continue
		}
		name := r.agentName(*in)
		box, why := r.sandboxFor(*in)
		if why != "" {
			in.Refused = why
			continue
		}
		if in.Agent.Serves == "itself" && box.Name() != "off" {
			if !socketChecked {
				socketWhy = r.keptSocket(ctx)
				socketChecked = true
			}
			if socketWhy != "" {
				in.Refused = socketWhy
				continue
			}
		}
		if keys := named[name]; len(keys) > 1 {
			in.Refused = fmt.Sprintf("%s is also the name of %s", name, strings.Join(others(keys, in.Key), ", "))
			continue
		}
		if why := serveable(*in); why != "" {
			in.Refused = why
			continue
		}
		// A served agent is still this runner's only while its kept token
		// answers as it and its person still owns it: a rotation or a
		// transfer stops it here and sends it back through claim, which
		// re-reads the kept token or says why not (review ri-M10 P2-1, P3-3).
		if s, ok := r.served[name]; ok {
			if s.in.Key != in.Key {
				// The same bus name with another key is another cgroup. Finish
				// the old identity before this reload adopts its replacement.
				r.stop(name, s)
				r.waitRetired(s.in.Key)
			} else if why := r.stillServes(s.bus, name); why != "" {
				fmt.Fprintf(r.say, "%s: %s is stopped: %s\n", r.name, name, why)
				r.stop(name, s)
			}
		}
		r.waitRetired(in.Key)
		as := r.bus
		if s, ok := r.served[name]; ok {
			as = s.bus
		} else {
			tok, err := r.claim(*in, name)
			if err != nil {
				in.Refused = err.Error()
				fmt.Fprintf(r.say, "%s: %s is refused: %v\n", r.name, in.Key, err)
				continue
			}
			as = r.bus.as(tok)
		}
		env, private, why := r.environmentWithSecrets(*in, as)
		if why != "" {
			in.Refused = why
			continue
		}
		ready[in.Key] = prepared{name: name, as: as, env: env, secretEnv: private, box: box}
	}
	conflicts := make(map[string]string)
	for _, in := range r.installed {
		if _, enabled := ready[in.Key]; !enabled {
			continue
		}
		for parent := in.Key; strings.Contains(parent, "/"); {
			parent = parent[:strings.LastIndex(parent, "/")]
			if _, enabled := ready[parent]; enabled {
				if conflicts[in.Key] == "" {
					conflicts[in.Key] = parent
				}
				if conflicts[parent] == "" {
					conflicts[parent] = in.Key
				}
			}
		}
	}
	// Previously served entries that failed preflight must retire before a
	// runnable overlapping replacement can bind its work directory.
	for name, s := range r.served {
		if _, ok := ready[s.in.Key]; !ok || conflicts[s.in.Key] != "" {
			r.stop(name, s)
		}
	}
	for i := range r.installed {
		in := &r.installed[i]
		p, ok := ready[in.Key]
		if !ok {
			continue
		}
		if other := conflicts[in.Key]; other != "" {
			in.Refused = "its work directory overlaps enabled agent " + other
			continue
		}
		name, as, env, box := p.name, p.as, p.env, p.box
		// The inbox reader and kept process are different serving modes.
		// Retire the old mode before adopting the replacement.
		if s := r.served[name]; s != nil && (s.in.Agent.Serves == "itself") != (in.Agent.Serves == "itself") {
			r.stop(name, s)
		}
		r.waitRetired(in.Key)
		var old *jail
		if s, ok := r.served[name]; ok {
			old = s.jail
		}
		j, why := r.jailFor(*in, box, old)
		if why != "" {
			in.Refused = why
			continue
		}
		if s, ok := r.served[name]; ok {
			// A one-message form reads its env at each start: the next run
			// gets the new one (Plans/R1.0-Release/runner.md#stop-and-reload).
			// A kept process cannot be told a new env, so a change restarts
			// it (№Q161), as does a changed installed package digest.
			cmd := command(filepath.Join(r.agents, in.Dir), in.Agent.Command)
			changed := s.kept != nil && (in.Digest != s.in.Digest || cmd != s.cmd || strings.Join(env, "\x00") != strings.Join(s.env, "\x00") || strings.Join(p.secretEnv, "\x00") != strings.Join(s.secretEnv, "\x00"))
			s.in, s.env, s.secretEnv, s.box, s.net, s.jail = *in, env, p.secretEnv, box, wantsNet(*in), j
			s.cmd = cmd
			if changed {
				old := s.kept
				s.kept = nil
				r.retire(in.Key, old, func() {
					if r.served[name] == s && s.kept == nil {
						s.kept = r.keep(s, s.bus.token)
					}
				})
			}
			want[name] = true
			continue
		}
		s, err := r.adopt(ctx, *in, name, as, env, p.secretEnv, box, j)
		if err != nil {
			in.Refused = err.Error()
			fmt.Fprintf(r.say, "%s: %s is refused: %v\n", r.name, in.Key, err)
			continue
		}
		r.served[name] = s
		want[name] = true
	}
	for name, s := range r.served {
		if !want[name] {
			r.stop(name, s)
			fmt.Fprintf(r.say, "%s: no longer serving %s\n", r.name, name)
		}
	}
}

// stop ends serving an agent: its reader stops and its cgroup is given up
// once nothing runs in it. Messages wait in its inbox. It holds r.mu.
func (r *runner) stop(name string, s *served) {
	s.cancel()
	if s.readDone != nil {
		<-s.readDone
	}
	delete(r.served, name)
	if k := s.kept; k != nil {
		// Its processes are stopped off the lock, then its cgroup given up.
		s.kept = nil
		current := s.jail
		r.retire(s.in.Key, k, func() {
			for g := range k.groups {
				g.Remove()
			}
			if current.group != nil && !k.groups[current.group] {
				current.group.Remove()
			}
		})
		return
	}
	r.waitRetired(s.in.Key)
	if s.jail.group != nil {
		s.jail.group.Remove() // stays while a process of it still runs
	}
}

// waitRetired keeps a cgroup key out of use until its old keeper and cleanup
// finish. reloadMu keeps installed stable while mu is released for the wait.
func (r *runner) waitRetired(key string) {
	var waiting []chan struct{}
	for old, done := range r.retiring {
		if old == key || strings.HasPrefix(old, key+"/") || strings.HasPrefix(key, old+"/") {
			waiting = append(waiting, done)
		}
	}
	if len(waiting) != 0 {
		r.mu.Unlock()
		for _, done := range waiting {
			<-done
		}
		r.mu.Lock()
	}
}

// retire starts off-lock stopping; finish and the completion signal run under mu.
func (r *runner) retire(key string, k *keeper, finish func()) {
	if r.retiring == nil {
		r.retiring = make(map[string]chan struct{})
	}
	done := make(chan struct{})
	r.retiring[key] = done
	go func() {
		k.stop()
		r.mu.Lock()
		defer r.mu.Unlock()
		finish()
		delete(r.retiring, key)
		close(done)
	}()
}

// stopKept stops every kept agent, all at once and bounded each by its own
// deadline: the runner itself is stopping.
func (r *runner) stopKept() {
	r.mu.Lock()
	var ks []*keeper
	var retiring []chan struct{}
	for _, done := range r.retiring {
		retiring = append(retiring, done)
	}
	for _, s := range r.served {
		if s.kept != nil {
			ks = append(ks, s.kept)
		}
	}
	r.mu.Unlock()
	var wg sync.WaitGroup
	for _, k := range ks {
		wg.Add(1)
		go func(k *keeper) { defer wg.Done(); k.stop() }(k)
	}
	wg.Wait()
	for _, done := range retiring {
		<-done
	}
}

// stillServes says why a token no longer lets this runner serve name, or "":
// it must answer as name, and name must still be the runner's person's.
func (r *runner) stillServes(as *client, name string) string {
	if who, err := as.whoami(); err != nil || who != name {
		return fmt.Sprintf("its kept token no longer answers as %s", name)
	}
	return r.ownedHere(as, name)
}

// ownedHere says why name is not the runner's person's, or "".
func (r *runner) ownedHere(as *client, name string) string {
	owner, err := as.ownerOf(name)
	if err != nil {
		return fmt.Sprintf("whose %s is: %v", name, err)
	}
	if owner != r.person {
		return fmt.Sprintf("%s is %s's now, not %s's: its own person's runner serves it", name, owner, r.person)
	}
	return ""
}

// tokenFile is where the runner keeps an agent's token, in its private home.
func (r *runner) tokenFile(name string) string {
	return filepath.Join(r.host, "tokens", url.PathEscape(name))
}

// claim is the agent's token: the one kept since the runner created it, or the
// one the creating register answers now. A name that exists with no token kept
// here is an agent the runner was never handed, and is refused by name, never
// taken: its Owner hands it over once (docs/02-access.md#getting-a-token).
func (r *runner) claim(in installed, name string) (string, error) {
	file := r.tokenFile(name)
	handOver := fmt.Sprintf("its Owner hands it over once: agent-bus-token '%s' into %s", name, file)
	if b, err := os.ReadFile(file); err == nil {
		tok := strings.TrimSpace(string(b))
		if who, err := r.bus.as(tok).whoami(); err != nil || who != name {
			return "", fmt.Errorf("the token kept for %s is not its any more (%v); %s", name, err, handOver)
		}
		if why := r.ownedHere(r.bus.as(tok), name); why != "" {
			return "", errors.New(why)
		}
		if err := refreshManagedTags(r.bus.as(tok), in, name); err != nil {
			return "", err
		}
		return tok, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	// An agent that serves itself is never registered by the runner
	// (№Q151): its Owner registers it, and hands its token over once.
	if in.Agent.Serves == "itself" {
		return "", fmt.Errorf("%s serves itself, so the runner does not register it; its Owner registers it and %s", name, strings.TrimPrefix(handOver, "its Owner "))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tags, err := protocol.NormalizeTags(append(append(append([]string(nil), in.Agent.Tags...), in.Host.Tags...), "runner"))
	if err != nil {
		return "", err
	}
	out, code, err := r.bus.doWith(ctx, "POST", "/register", nil, protocol.Record{
		Name: name, Kind: protocol.KindAgent, Descr: in.Agent.Description, Roles: in.Agent.Roles,
		Tags: tags,
	}, map[string]string{"If-None-Match": "*"})
	switch {
	case err != nil:
		return "", fmt.Errorf("register: %w", err)
	case code == http.StatusPreconditionFailed:
		return "", fmt.Errorf("%s exists and its token is not kept here; %s", name, handOver)
	case code >= 400:
		return "", fmt.Errorf("register: %s", strings.TrimSpace(string(out)))
	}
	var created struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(out, &created); err != nil || created.Token == "" {
		return "", fmt.Errorf("%s was created, but no token came back; %s", name, handOver)
	}
	if err := keep(file, created.Token); err != nil {
		return "", fmt.Errorf("%s was created, but its token could not be kept (%v); %s", name, err, handOver)
	}
	return created.Token, nil
}

// keep writes a token readable by the runner alone, whole or not at all.
func keep(file, token string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	tmp := file + ".new"
	if err := os.WriteFile(tmp, []byte(token+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// others is keys without key.
func others(keys []string, key string) []string {
	var out []string
	for _, k := range keys {
		if k != key {
			out = append(out, k)
		}
	}
	return out
}

// keptSocket checks the actual listener without a credential. Only the shared
// authenticated listener's 401 proves that binding it adds no account identity.
// Its answer is cached by the reload, never rechecked during a kept process.
func (r *runner) keptSocket(ctx context.Context) string {
	addr := r.addr
	if addr == "" {
		addr = api.ClientSocket()
	}
	if api.IsTCP(addr) {
		return ""
	}
	probe, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, code, err := r.bus.as("").do(probe, "GET", "/status", nil, nil)
	if err == nil && code == http.StatusUnauthorized {
		return ""
	}
	return "sandboxed agents that serve themselves require a shared authenticated socket, not an account socket or an unverified listener"
}

// workPath refuses symlinks anywhere inside the work tree before it is bound.
func (r *runner) workPath(key string) (string, error) {
	root := filepath.Join(r.host, "work")
	work := filepath.Join(root, key)
	for p := work; ; p = filepath.Dir(p) {
		fi, err := os.Lstat(p)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if err == nil && fi.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("its work directory contains a symlink: %s", p)
		}
		if p == root || p == filepath.Dir(p) {
			break
		}
	}
	return work, nil
}

// adopt starts reading for an agent the runner holds the token of.
func (r *runner) adopt(ctx context.Context, in installed, name string, as *client, env, private []string, box ports.Sandbox, j *jail) (*served, error) {
	work, err := r.workPath(in.Key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(work, 0o700); err != nil {
		return nil, err
	}
	max := in.Span.Max
	if max < 1 {
		max = 1
	}
	rctx, cancel := context.WithCancel(ctx)
	s := &served{in: in, name: name, work: work, bus: as, env: env, secretEnv: private,
		box: box, jail: j, dir: filepath.Join(r.agents, in.Dir), net: wantsNet(in),
		cmd:   command(filepath.Join(r.agents, in.Dir), in.Agent.Command),
		slots: make(chan struct{}, max), cancel: cancel}
	if in.Agent.Serves == "itself" {
		cancel()
		s.kept = r.keep(s, as.token)
		return s, nil
	}
	s.readDone = make(chan struct{})
	go func() { defer close(s.readDone); r.read(rctx, s) }()
	fmt.Fprintf(r.say, "%s: serving %s, %d at a time, sandbox %s\n", r.name, name, max, box.Name())
	return s, nil
}

// read takes this agent's messages off its own inbox, one per free process:
// the slot is taken before the read, so nothing is taken that no process is
// free to run.
func (r *runner) read(ctx context.Context, s *served) {
	const poll = 55 * time.Second
	for {
		select {
		case s.slots <- struct{}{}:
		case <-ctx.Done():
			return
		}
		e, got, err := s.bus.next(ctx, poll)
		if ctx.Err() != nil {
			// Taken just as serving stopped: it is off the bus, so it is run
			// and answered rather than lost.
			if err == nil && got {
				go func() {
					defer func() { <-s.slots }()
					r.runOne(s, e)
				}()
				return
			}
			<-s.slots
			return
		}
		if err != nil || !got {
			<-s.slots
			if err != nil {
				fmt.Fprintf(r.say, "%s: reading for %s: %v; reading again\n", r.name, s.name, err)
				select {
				case <-time.After(2 * time.Second):
				case <-ctx.Done():
					return
				}
			}
			continue
		}
		go func() {
			defer func() { <-s.slots }()
			r.runOne(s, e)
		}()
	}
}

// runOne serves one message: the script once, then its answer or done;
// only a script still running after two seconds gets an ack —
// all as the agent that was addressed.
func (r *runner) runOne(s *served, e protocol.Envelope) {
	if e.Receipt != "" {
		return
	}
	if e.TooLate(time.Now()) {
		fmt.Fprintf(r.say, "%s: %s arrived after its caller gave up; not run\n", s.name, e.ID)
		return
	}
	s.busy.Add(1)
	defer s.busy.Add(-1)

	launch := r.launchSettings(s)
	if (e.Ask == protocol.AskMCP || agentmeta.IsAsk(e.Ask)) && launch.agent.Help != "native" {
		body := `{"tools":[]}`
		if e.Ask != protocol.AskMCP {
			var err error
			body, err = agentmeta.Manifest(launch.agent).Answer(s.dir, e.Ask, e.Roles, false, s.name)
			if err != nil {
				body = "error: " + err.Error()
			}
		}
		r.answerAs(s.bus, e, body)
		return
	}
	if _, err := r.workPath(launch.key); err != nil {
		fmt.Fprintf(r.say, "%s: %v; not run\n", s.name, err)
		return
	}
	ctx := context.Background()
	if d, err := time.ParseDuration(launch.agent.Deadline); err == nil && d > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	if !e.Deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, e.Deadline)
		defer cancel()
	}
	ackDone := make(chan struct{})
	ackTimer := time.AfterFunc(2*time.Second, func() {
		r.receipt(ctx, s.bus, e, protocol.ReceiptAck)
		close(ackDone)
	})
	// Join a receipt already in flight before answering, and stop the timer
	// on every early return. No receipt can arrive after the answer.
	finishAck := sync.OnceFunc(func() {
		if !ackTimer.Stop() {
			<-ackDone
		}
	})
	defer finishAck()

	env, configErr := configuration(ctx, s.bus, s.name, launch.env)
	if configErr != nil {
		fmt.Fprintf(r.say, "%s: configuration unavailable: %v; not run\n", s.name, configErr)
		finishAck()
		r.answerAs(s.bus, e, "error: configuration unavailable")
		return
	}
	j := launch.jail
	if j.group != nil {
		if err := j.group.Prepare(); err != nil {
			fmt.Fprintf(r.say, "%s: its cgroup: %v; not run\n", s.name, err)
			return
		}
	}
	argv := launch.box.Wrap(ports.Job{
		Argv: script.Argv(launch.cmd, launch.agent.Form == "args", e.Body, e.Roles),
		Work: s.work, Read: []string{s.dir}, Net: launch.net, TmpSize: j.tmp, Seccomp: len(j.seccomp) > 0,
	})
	// Open files are a limit of the process, not of the cgroup: prlimit sets
	// it and then is the sandbox, so every process of the agent inherits it.
	if j.files > 0 {
		argv = append([]string{"prlimit", fmt.Sprintf("--nofile=%d:%d", j.files, j.files), "--"}, argv...)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	// Cloned straight into the agent's cgroup: no process of it ever runs
	// outside its limits, not even for the moment a move would take.
	if j.group != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: j.group.FD()}
	}
	// The filter is read by bubblewrap from its fd 3, a pipe written whole
	// before the start: a few hundred bytes, well inside a pipe's buffer.
	if len(j.seccomp) > 0 {
		rd, wr, err := os.Pipe()
		if err != nil {
			fmt.Fprintf(r.say, "%s: no pipe for the system-call filter: %v; %s not run\n", s.name, err, e.ID)
			return
		}
		_, werr := wr.Write(j.seccomp)
		wr.Close()
		if werr != nil {
			rd.Close()
			fmt.Fprintf(r.say, "%s: the system-call filter was not written: %v; %s not run\n", s.name, werr, e.ID)
			return
		}
		defer rd.Close()
		cmd.ExtraFiles = []*os.File{rd}
	}
	cmd.Dir = s.work
	if launch.agent.Form != "args" {
		cmd.Stdin = bytes.NewReader(append(mustJSON(script.Input(e)), '\n'))
	}
	// A sandboxed child starts from nothing of the runner's: what it is told
	// is the agent's layers and the message, over a bare PATH and HOME.
	base := script.ChildEnv(os.Environ())
	if launch.box.Name() != "off" {
		base = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=" + s.work, "LANG=C.UTF-8"}
	}
	cmd.Env = mergeEnv(base, env, []string{
		"AGENT_BUS_MESSAGE_ID=" + e.ID, "AGENT_BUS_FROM=" + e.From, "AGENT_BUS_TO=" + s.name,
		"AGENT_BUS_TOPIC=" + e.Topic, "AGENT_BUS_TAG=" + e.Tag, "AGENT_BUS_WORK=" + s.work,
		"AGENT_BUS_ASK=" + e.Ask})
	out := &bounded{max: maxOutput}
	cmd.Stdout = out
	cmd.Stderr = secretStderr(prefixed{r.say, s.name + ": "}, launch.secretEnv)
	var err error
	if launch.box.Name() == "off" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
		if deadline, ok := ctx.Deadline(); ok {
			// A pipe kept by an escaped descendant must eventually close, but
			// valid background output may continue until the caller's deadline.
			cmd.WaitDelay = max(time.Until(deadline), time.Millisecond)
		}
		if err = cmd.Start(); err == nil {
			// CommandContext stops watching when the leader exits; its children
			// may still hold the output pipes while Wait is draining them.
			killDone := make(chan struct{})
			stopKill := context.AfterFunc(ctx, func() { _ = cmd.Cancel(); close(killDone) })
			err = cmd.Wait()
			if !stopKill() {
				<-killDone
			}
			if err == nil {
				err = ctx.Err()
			}
		}
	} else {
		err = cmd.Run()
	}
	cmd.Stderr.(*stderrLine).flush()
	finishAck()
	if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
		r.recordExit(s, nil, cmd)
	}
	if reason, unconfigured := configurationExit(cmd, err); unconfigured {
		r.notConfigured(s, nil, reason)
		r.answerAs(s.bus, e, fmt.Sprintf("error: %s is not configured: %s", s.name, reason))
		return
	}
	r.checkDisk(s, j)
	if err != nil {
		fmt.Fprintf(r.say, "%s: %s exited badly for %s: %v\n", s.name, launch.agent.Command, e.ID, err)
		return
	}
	answer := strings.TrimRight(out.buf.String(), "\n")
	if out.cut {
		answer += fmt.Sprintf("\n[agent-bus-runner: the answer was cut at %d bytes]", maxOutput)
		fmt.Fprintf(r.say, "%s: the answer to %s was cut at %d bytes\n", s.name, e.ID, maxOutput)
	}
	if answer == "" {
		r.receipt(ctx, s.bus, e, protocol.ReceiptDone)
		return
	}
	r.answerAs(s.bus, e, answer)
}

// sandboxFor is the agent's sandbox: bubblewrap unless agents.json turned it
// off for that agent, and a refusal, never a quiet run unconfined, on a host
// that cannot give one (Plans/R1.0-Release/runner.md#sandboxing).
func (r *runner) sandboxFor(in installed) (ports.Sandbox, string) {
	switch in.Host.Sandbox {
	case "off":
		return sandbox.Off{}, ""
	case "", "on":
	default:
		return nil, fmt.Sprintf("agents.json: sandbox %q is on or off", in.Host.Sandbox)
	}
	r.probe.Do(func() { r.noSandbox = sandbox.ProbeBwrap() })
	if r.noSandbox != nil {
		return nil, "the sandbox is on and this host cannot give one: " + r.noSandbox.Error()
	}
	return sandbox.Bwrap{}, ""
}

// jail is how one agent's processes are confined beyond the sandbox's
// namespaces: its cgroup with the agent's limits, its open files, and the
// size of its /tmp. With the sandbox off it is empty, said at start.
type jail struct {
	processes int // resolved pids.max, for status without reading the host
	group     *cgroup.Group
	files     int
	tmp       int64
	disk      int64  // the work directory's soft limit, in bytes
	seccomp   []byte // the system-call filter bubblewrap loads
}

// The built limits, for an agent the host gave none: the same as the
// dedicated runner's sandbox (docs/06-dedicated-runner.md#sandboxing).
var builtLimits = struct {
	memory    int64
	processes int
	files     int
	disk      int64
}{512 << 20, 64, 1024, 1 << 30}

// limits is one agent's, worked out: its cgroup's, its open files, and its
// work directory's soft limit (Q167).
type agentLimits struct {
	cgroup cgroup.Limits
	files  int
	disk   int64
}

// limitsFor is an agent's limits, the host's over the built ones, or why they
// cannot be: a malformed one, or a `needs` above them.
func limitsFor(in installed) (agentLimits, string) {
	al := agentLimits{cgroup: cgroup.Limits{Memory: builtLimits.memory, Processes: builtLimits.processes},
		files: builtLimits.files, disk: builtLimits.disk}
	l := &al.cgroup
	if h := in.Host.Limits; h != nil {
		if h.Memory != "" {
			n, err := cgroup.ParseBytes(h.Memory)
			if err != nil {
				return al, "agents.json: limits.memory: " + err.Error()
			}
			l.Memory = n
		}
		if h.CPU != "" {
			n, err := cgroup.ParsePercent(h.CPU)
			if err != nil {
				return al, "agents.json: limits.cpu: " + err.Error()
			}
			l.CPU = n
		}
		if h.Processes > 0 {
			l.Processes = h.Processes
		}
		if h.Files > 0 {
			al.files = h.Files
		}
		if h.IOWeight != 0 {
			if h.IOWeight < 1 || h.IOWeight > 10000 {
				return al, fmt.Sprintf("agents.json: limits.io_weight %d is not 1..10000", h.IOWeight)
			}
			l.IOWeight = h.IOWeight
		}
		if h.Disk != "" {
			n, err := cgroup.ParseBytes(h.Disk)
			if err != nil {
				return al, "agents.json: limits.disk: " + err.Error()
			}
			al.disk = n
		}
	}
	if n := in.Agent.Needs; n != nil && n.Memory != "" {
		need, err := cgroup.ParseBytes(n.Memory)
		if err != nil {
			return al, "agent.json: needs.memory: " + err.Error()
		}
		if need > l.Memory {
			return al, fmt.Sprintf("it needs %s of memory and this host gives it %d bytes (limits.memory)", n.Memory, l.Memory)
		}
	}
	return al, ""
}

// checkDisk stops serving an agent whose run left its work directory over its
// soft limit: its messages wait in its inbox, and status says why by name
// until a reload finds the directory back under the limit (Q167).
func (r *runner) checkDisk(s *served, j *jail) {
	why := overDisk(s.work, j.disk)
	if why == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.served[s.name] != s {
		return
	}
	if s.kept != nil {
		s.kept.cancel()
	}
	r.stop(s.name, s)
	for i := range r.installed {
		if r.agentName(r.installed[i]) == s.name {
			r.installed[i].Refused = why
		}
	}
	fmt.Fprintf(r.say, "%s: %s is stopped: %s\n", r.name, s.name, why)
}

// workSize is what a work directory holds, the sizes of its files added up.
func workSize(dir string) int64 {
	var n int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

// overDisk says why an agent whose work directory has outgrown its soft
// limit is not served, or "".
func overDisk(work string, disk int64) string {
	if disk <= 0 {
		return ""
	}
	if n := workSize(work); n > disk {
		return fmt.Sprintf("its work directory holds %d bytes, over its disk limit of %d (limits.disk); empty %s and reload", n, disk, work)
	}
	return ""
}

// jailFor is the agent's confinement, its cgroup made or updated, or why it
// cannot have one: with the sandbox on, a host that cannot give it a cgroup
// refuses the agent rather than running it without limits.
func (r *runner) jailFor(in installed, box ports.Sandbox, old *jail) (*jail, string) {
	if box.Name() == "off" {
		return &jail{}, ""
	}
	al, why := limitsFor(in)
	if why != "" {
		return nil, why
	}
	l := al.cgroup
	if why := overDisk(filepath.Join(r.host, "work", in.Key), al.disk); why != "" {
		return nil, why
	}
	r.cgOnce.Do(func() { r.cg, r.cgErr = cgroup.Delegated() })
	if r.cgErr != nil {
		return nil, "the sandbox is on and this host cannot give the agent a cgroup: " + r.cgErr.Error()
	}
	r.filterOne.Do(func() { r.filter, r.filterErr = sandbox.Filter() })
	if r.filterErr != nil {
		return nil, "the sandbox is on and this host cannot filter its system calls: " + r.filterErr.Error()
	}
	var g *cgroup.Group
	if old != nil && old.group != nil {
		if err := old.group.Set(l); err != nil {
			return nil, "its cgroup: " + err.Error()
		}
		g = old.group
	} else {
		var err error
		if g, err = r.cg.Agent(in.Key, l); err != nil {
			return nil, "its cgroup: " + err.Error()
		}
	}
	return &jail{group: g, processes: l.Processes, files: al.files, tmp: l.Memory, disk: al.disk, seccomp: r.filter}, ""
}

// wantsNet is whether agent.json asked for the network; which hosts is M.7's.
func wantsNet(in installed) bool { return in.Agent.Needs != nil && len(in.Agent.Needs.Network) > 0 }

// mergeEnv is the child's environment, each later list winning a name an
// earlier one set: the runner's own, then the agent's layers, then what the
// runner tells it about the message. A child's getenv takes the first of a
// duplicated name, so a plain append would let the inherited PATH beat the
// agent's, and an env file's AGENT_BUS_FROM beat the real sender.
func mergeEnv(lists ...[]string) []string {
	at, out := map[string]int{}, []string{}
	for _, list := range lists {
		for _, kv := range list {
			name, _, _ := strings.Cut(kv, "=")
			if i, seen := at[name]; seen {
				out[i] = kv
				continue
			}
			at[name] = len(out)
			out = append(out, kv)
		}
	}
	return out
}

// route is where e's receipts and answer go.
func route(e protocol.Envelope) (string, string, string) {
	if e.ReplyTo != nil {
		return e.ReplyTo.Name, e.ReplyTo.Topic, e.ReplyTo.Tag
	}
	return e.From, e.Topic, e.Tag
}

func (r *runner) receipt(ctx context.Context, as *client, e protocol.Envelope, kind string) {
	back, topic, tag := route(e)
	if err := as.postContext(ctx, "/send", protocol.Envelope{To: back, Topic: topic, Tag: tag, Receipt: kind, Re: e.ID}, nil); err != nil {
		fmt.Fprintf(r.say, "%s: could not %s %s: %v\n", r.name, kind, e.ID, err)
	}
}

func (r *runner) answerAs(as *client, e protocol.Envelope, body string) {
	back, topic, tag := route(e)
	if err := as.post("/send", protocol.Envelope{To: back, Topic: topic, Tag: tag, Body: body}, nil); err != nil {
		fmt.Fprintf(r.say, "%s: could not answer %s: %v\n", r.name, e.ID, err)
	}
}

// bounded keeps the first max bytes written and says whether more came.
type bounded struct {
	buf bytes.Buffer
	max int
	cut bool
}

func (b *bounded) Write(p []byte) (int, error) {
	if room := b.max - b.buf.Len(); room < len(p) {
		b.cut = true
		if room > 0 {
			b.buf.Write(p[:room])
		}
		return len(p), nil
	}
	return b.buf.Write(p)
}

// prefixed writes each line a child says on stderr into the runner's log
// under its agent's name.
type prefixed struct {
	w      interface{ Write([]byte) (int, error) }
	prefix string
}

var logMu sync.Mutex

func (p prefixed) Write(b []byte) (int, error) {
	logMu.Lock()
	defer logMu.Unlock()
	for _, line := range strings.SplitAfter(string(b), "\n") {
		if line != "" {
			fmt.Fprint(p.w, p.prefix+line)
		}
	}
	return len(b), nil
}
