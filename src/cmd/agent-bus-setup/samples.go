package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
)

// Sample data for a node somebody wants to look around: a few users, agents,
// services, queues, topics and groups from a galaxy far, far away (and from
// Spaceballs), each in its own realm. --samples adds them; --remove-samples
// takes away exactly these, and only while each still carries its sample
// description, so a real record that happens to share a name is never
// touched. The daemon never removes a user and retires a group by emptying
// it, so removal deactivates the users and empties the groups.
// See docs/09-setup.md#sample-data.

type sampleUser struct{ name, person string }
type sampleRecord struct {
	name, kind, owner, descr, addr, protocol string
	allow, subs, maintainers                 []string
	personal, inactive                       bool
	secret, config                           string
}
type sampleGroup struct {
	name, owner, descr string
	members            []string
}
type sampleSend struct{ to, body, topic string }

var sampleUsers = []sampleUser{
	{"luke@tatooine", "Luke Skywalker"},
	{"leia@alderaan", "Leia Organa"},
	{"han@corellia", "Han Solo"},
	{"yoda@dagobah", "Yoda"},
	{"vader@empire", "Darth Vader"},
	{"lone-starr@druidia", "Lone Starr"},
	{"vespa@druidia", "Princess Vespa"},
	{"dark-helmet@spaceball", "Lord Dark Helmet"},
	{"yogurt@vega", "Yogurt the Wise"},
}

var sampleRecords = []sampleRecord{
	{name: "#r2d2@naboo", kind: "agent", owner: "luke@tatooine", descr: "Astromech. Beeps; means it.", allow: []string{"@owner", "@rebels", "droid-inbox@tatooine"},
		maintainers: []string{"leia@alderaan"}, config: `{"hologram": "Help me, Obi-Wan Kenobi"}`},
	{name: "#c3po@tatooine", kind: "agent", owner: "luke@tatooine", descr: "Human-cyborg relations, fluent in over six million forms of communication", allow: []string{"*"}},
	{name: "#k2so@scarif", kind: "agent", owner: "leia@alderaan", descr: "Reprogrammed Imperial droid; will tell you the odds", allow: []string{"@rebels"}},
	{name: "#dot-matrix@druidia", kind: "agent", owner: "vespa@druidia", descr: "Lady-in-waiting droid with a virgin alarm", allow: []string{"@owner"}, personal: true},
	{name: "#barf@druidia", kind: "agent", owner: "lone-starr@druidia", descr: "Half man, half dog: his own best friend", allow: []string{"*"}},
	{name: "death-star@empire", kind: "service", owner: "vader@empire", descr: "Fully armed and operational battle station", addr: "death-star.empire:1138", protocol: "superlaser", allow: []string{"@empire"},
		maintainers: []string{"@empire"}, secret: "EXHAUST_PORT=two-meters-wide", config: `{"shield": "up", "superlaser": "charged"}`},
	{name: "death-star-ii@endor", kind: "service", owner: "vader@empire", descr: "Not yet operational; nobody tell the Emperor", addr: "death-star-ii.endor:1138", protocol: "superlaser", allow: []string{"@empire"}, inactive: true},
	{name: "jedi-archives@coruscant", kind: "service", owner: "yoda@dagobah", descr: "If an item does not appear in our records, it does not exist", addr: "https://archives.coruscant/api", protocol: "https", allow: []string{"@jedi"}},
	{name: "spaceball-one@spaceball", kind: "service", owner: "dark-helmet@spaceball", descr: "Goes plaid at ludicrous speed", addr: "spaceball-one:2112", protocol: "ludicrous-speed", allow: []string{"@spaceballs"}},
	{name: "schwartz@vega", kind: "service", owner: "yogurt@vega", descr: "May the Schwartz be with you", addr: "yogurt.vega:12345", protocol: "schwartz", allow: []string{"*"},
		secret: "COMBINATION=12345"},
	{name: "rebel-alerts@hoth", kind: "queue", owner: "leia@alderaan", descr: "Imperial walkers on the north ridge", allow: []string{"@rebels", "imperial-news@empire"}},
	{name: "jabba-debts@tatooine", kind: "queue", owner: "han@corellia", descr: "What Han owes, and to whom", allow: []string{"*"}},
	{name: "falcon-repairs@corellia", kind: "queue", owner: "han@corellia", descr: "Hyperdrive, again. Personal: Han's own", allow: []string{"@owner"}, personal: true},
	{name: "droid-inbox@tatooine", kind: "queue", owner: "luke@tatooine", descr: "Messages for the droids, routed on to R2", allow: []string{"*"}, subs: []string{"#r2d2@naboo"}},
	{name: "air-supply@druidia", kind: "queue", owner: "vespa@druidia", descr: "Perri-Air, fresh canned air from planet Druidia", allow: []string{"*"}},
	{name: "imperial-news@empire", kind: "pubsub", owner: "vader@empire", descr: "The Emperor is not as forgiving as I am", allow: []string{"@empire"}, subs: []string{"rebel-alerts@hoth"}},
	{name: "spaceballs-merch@spaceball", kind: "pubsub", owner: "yogurt@vega", descr: "Spaceballs the flamethrower, the lunchbox, the breakfast cereal", allow: []string{"*"}, subs: []string{"air-supply@druidia"}},
	{name: "jedi-journal@dagobah", kind: "pubsub", owner: "yoda@dagobah", descr: "Wisdom of the day. Personal: Yoda's own", allow: []string{"@owner"}, personal: true},
}

var sampleGroups = []sampleGroup{
	{name: "@rebels", owner: "leia@alderaan", descr: "The Rebel Alliance", members: []string{"luke@tatooine", "leia@alderaan", "han@corellia"}},
	{name: "@jedi", owner: "yoda@dagobah", descr: "Do, or do not. There is no try.", members: []string{"yoda@dagobah", "luke@tatooine"}},
	{name: "@empire", owner: "vader@empire", descr: "The Galactic Empire", members: []string{"vader@empire"}},
	{name: "@spaceballs", owner: "dark-helmet@spaceball", descr: "Planet Spaceball's finest", members: []string{"dark-helmet@spaceball"}},
}

var sampleSends = []sampleSend{
	{"rebel-alerts@hoth", "Imperial probe droid spotted near Echo Base", "alert"},
	{"rebel-alerts@hoth", "Shields are up; evacuate the transports", "alert"},
	{"jabba-debts@tatooine", "Jabba wants his money, Solo", "debt"},
	{"air-supply@druidia", "One can of Perri-Air, extra fresh", "order"},
	{"imperial-news@empire", "The Death Star is fully armed and operational", "news"},
	{"spaceballs-merch@spaceball", "New: Spaceballs the toilet paper", "merch"},
	{"#c3po@tatooine", "Sir, the possibility of navigating an asteroid field is 3,720 to 1", "odds"},
}

// ownerClient speaks to the daemon on its own account's socket, which
// answers as the daemon Owner: setup runs as root and may open it. With
// AGENT_BUS_ADDR set it uses that account socket instead — a development
// daemon's, whose account socket is its Owner's.
type ownerClient struct{ http, shared http.Client }

// ownerSocket is where the samples are written, and whether root is needed.
func ownerSocket() (string, bool) {
	if s := os.Getenv("AGENT_BUS_ADDR"); s != "" {
		return s, false
	}
	return api.UserSocket(api.SystemRuntimeDir, svcAccount), true
}

func newOwnerClient() *ownerClient {
	sock, _ := ownerSocket()
	over := func(path string) http.Client {
		return http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", path)
			}}}
	}
	// An account socket always answers as its account; a call made with a
	// token goes to the shared socket beside it, which reads the token.
	return &ownerClient{http: over(sock), shared: over(filepath.Join(filepath.Dir(sock), "bus.sock"))}
}

func (c *ownerClient) call(method, path string, body any) (int, []byte, error) {
	return c.callAs("", method, path, body)
}

// callAs is a call carrying a token, for the one thing only a record's own
// credential may do: take an agent's messages.
func (c *ownerClient) callAs(token, method, path string, body any) (int, []byte, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "http://bus"+path, r)
	if err != nil {
		return 0, nil, err
	}
	if token != "" {
		req.Header.Set("X-Agent-Bus-Token", token)
	}
	client := &c.http
	if token != "" {
		client = &c.shared
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	return res.StatusCode, out, nil
}

// must is a call whose failure stops the samples, with the daemon's reason.
func (c *ownerClient) must(what, method, path string, body any, ok ...int) error {
	code, out, err := c.call(method, path, body)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	for _, want := range append(ok, 200) {
		if code == want {
			return nil
		}
	}
	return fmt.Errorf("%s: %d %s", what, code, strings.TrimSpace(string(out)))
}

func (c *ownerClient) lookup(name string) (map[string]any, bool) {
	code, out, err := c.call("GET", "/lookup?name="+urlEscape(name), nil)
	if err != nil || code != 200 {
		return nil, false
	}
	var r map[string]any
	return r, json.Unmarshal(out, &r) == nil
}

// inactiveRecord finds name among the inactive records the Owner may see.
func (c *ownerClient) inactiveRecord(name string) (map[string]any, bool) {
	code, out, err := c.call("GET", "/inactive", nil)
	if err != nil || code != 200 {
		return nil, false
	}
	var list []map[string]any
	if json.Unmarshal(out, &list) != nil {
		return nil, false
	}
	for _, rec := range list {
		if rec["name"] == name {
			return rec, true
		}
	}
	return nil, false
}

func urlEscape(s string) string { return strings.NewReplacer("#", "%23", "@", "%40", "/", "%2F").Replace(s) }

// addSamples puts the sample node in place. Anything already there is left
// as it is, so running it twice changes nothing the second time.
func addSamples() error {
	c := newOwnerClient()
	var kept []string
	people := c.people()
	for _, u := range sampleUsers {
		if person, exists := people[u.name]; exists {
			if person != u.person {
				kept = append(kept, u.name)
				continue
			}
			// A sample user deactivated by --remove-samples comes back.
			_ = c.must("reactivate "+u.name, "POST", "/user/state", map[string]any{"name": u.name, "status": "active"}, 400, 403, 409)
			continue
		}
		if err := c.must("user "+u.name, "POST", "/user", map[string]any{"name": u.name, "person_name": u.person, "create": true}); err != nil {
			return err
		}
	}
	for _, g := range sampleGroups {
		// A group of that name that is somebody's own is left as it is.
		if rec, exists := c.lookup(g.name); exists && rec["descr"] != g.descr && rec["descr"] != nil {
			kept = append(kept, g.name)
			continue
		}
		if err := c.must("group "+g.name, "POST", "/group", map[string]any{"Name": g.name, "Members": g.members}); err != nil {
			return err
		}
		if err := c.must("describe "+g.name, "POST", "/manage", map[string]any{"name": g.name, "descr": g.descr}); err != nil {
			return err
		}
		if err := c.must("give "+g.name+" to "+g.owner, "POST", "/manage", map[string]any{"name": g.name, "owner": g.owner}); err != nil {
			return err
		}
	}
	fresh := map[string]bool{}
	for _, r := range sampleRecords {
		rec, exists := c.lookup(r.name)
		if !exists && r.inactive {
			// An inactive record is answered to nobody; a rerun finds it here.
			if rec, exists = c.inactiveRecord(r.name); exists && rec["descr"] == r.descr {
				continue // already made and deactivated
			}
		}
		if exists && rec["descr"] != r.descr {
			kept = append(kept, r.name) // a real record under a sample's name
			continue
		}
		if !exists {
			fresh[r.name] = true
			reg := map[string]any{"name": r.name, "kind": r.kind, "descr": r.descr, "allow": r.allow, "subs": r.subs}
			if r.kind == "service" {
				reg["addr"], reg["protocol"] = r.addr, r.protocol
			}
			if err := c.must("register "+r.name, "POST", "/register", reg); err != nil {
				return err
			}
		}
		change := map[string]any{"name": r.name, "owner": r.owner}
		if err := c.must("give "+r.name+" to "+r.owner, "POST", "/manage", change); err != nil {
			return err
		}
		if r.personal {
			if err := c.must("make "+r.name+" Personal", "POST", "/manage", map[string]any{"name": r.name, "personal": true}); err != nil {
				return err
			}
		}
		if len(r.maintainers) > 0 {
			if err := c.must("maintainers of "+r.name, "POST", "/manage", map[string]any{"name": r.name, "maintainers": r.maintainers}); err != nil {
				return err
			}
		}
		if r.secret != "" && fresh[r.name] {
			if err := c.must("secret of "+r.name, "POST", "/secret", map[string]any{"name": r.name, "secret": r.secret}); err != nil {
				return err
			}
		}
		if r.config != "" && fresh[r.name] {
			if err := c.must("configuration of "+r.name, "POST", "/configure", map[string]any{"Name": r.name, "Config": json.RawMessage(r.config)}); err != nil {
				return err
			}
		}
	}
	// A little traffic, so the charts have something to draw — once, when
	// its record is new, not again on every run.
	for _, s := range sampleSends {
		if fresh[s.to] {
			_ = c.must("send to "+s.to, "POST", "/send", map[string]any{"to": s.to, "body": s.body, "topic": s.topic})
		}
	}
	// One inactive record, to show what an inactive one looks like.
	for _, r := range sampleRecords {
		if r.inactive && fresh[r.name] {
			if err := c.must("deactivate "+r.name, "POST", "/manage", map[string]any{"name": r.name, "status": "inactive"}); err != nil {
				return err
			}
		}
	}
	fmt.Printf("sample data added: %d users, %d records, %d groups, in realms tatooine, alderaan, corellia, dagobah, empire, naboo, scarif, coruscant, hoth, druidia, spaceball and vega\n",
		len(sampleUsers), len(sampleRecords), len(sampleGroups))
	if len(kept) > 0 {
		fmt.Printf("left alone, since they are not samples: %s\n", strings.Join(kept, ", "))
	}
	return nil
}

// removeSamples takes the samples away again: records that still carry
// their sample description are unregistered, sample groups are emptied and
// sample users deactivated, since the daemon removes neither.
func removeSamples() error {
	c := newOwnerClient()
	removed, kept := 0, []string{}
	for _, r := range sampleRecords {
		rec, exists := c.lookup(r.name)
		woke := false
		if !exists && r.inactive {
			// An inactive record is answered to nobody; wake it to look.
			if c.must("reactivate "+r.name, "POST", "/manage", map[string]any{"name": r.name, "status": "active"}) == nil {
				rec, exists = c.lookup(r.name)
				woke = true
			}
		}
		if !exists {
			continue
		}
		if rec["descr"] != r.descr {
			if woke { // not a sample: put it back as it was
				_ = c.must("deactivate "+r.name, "POST", "/manage", map[string]any{"name": r.name, "status": "inactive"})
			}
			kept = append(kept, r.name)
			continue
		}
		// The daemon unregisters only an idle inbox: what the samples sent is
		// taken first, as the agent itself for an agent, as the Owner for a queue.
		c.drain(r.name, r.kind)
		if err := c.must("unregister "+r.name, "POST", "/unregister", map[string]any{"name": r.name}); err != nil {
			return err
		}
		removed++
	}
	for _, g := range sampleGroups {
		rec, exists := c.lookup(g.name)
		if !exists {
			continue
		}
		if rec["descr"] != g.descr {
			kept = append(kept, g.name)
			continue
		}
		if err := c.must("empty "+g.name, "POST", "/manage", map[string]any{"name": g.name, "allow": []string{}}); err != nil {
			return err
		}
	}
	for _, u := range sampleUsers {
		_ = c.must("deactivate "+u.name, "POST", "/user/state", map[string]any{"name": u.name, "status": "inactive"}, 400, 404, 409)
	}
	fmt.Printf("sample data removed: %d records unregistered, sample groups emptied, sample users deactivated (the daemon never removes a user)\n", removed)
	if len(kept) > 0 {
		fmt.Printf("left alone, since they no longer carry their sample description: %s\n", strings.Join(kept, ", "))
	}
	return nil
}

// drain takes every message a sample inbox holds, so it can be unregistered.
func (c *ownerClient) drain(name, kind string) {
	token, path := "", "/consume?wait=0s&inbox="+urlEscape(name)
	if kind == "agent" {
		code, out, err := c.call("POST", "/token", map[string]any{"name": name})
		var t struct{ Token string }
		if err != nil || code != 200 || json.Unmarshal(out, &t) != nil {
			return
		}
		token, path = t.Token, "/consume?wait=0s"
	}
	for i := 0; i < 10000; i++ {
		if code, _, err := c.callAs(token, "GET", path, nil); err != nil || code != 200 {
			return
		}
	}
}

// people is every user's person name, by user name.
func (c *ownerClient) people() map[string]string {
	out := map[string]string{}
	code, body, err := c.call("GET", "/users", nil)
	if err != nil || code != 200 {
		return out
	}
	var rows []struct {
		Name, Kind string
		Person     string `json:"person_name"`
	}
	if json.Unmarshal(body, &rows) == nil {
		for _, r := range rows {
			if r.Kind == "user" {
				out[r.Name] = r.Person
			}
		}
	}
	return out
}
