package core

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime/debug"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Stack overflow is fatal. Isolate each walk in a test subprocess so a
// recursive regression fails the parent's assertion, not the whole suite.
func TestGroupWalksUseBoundedStack(t *testing.T) {
	if walk := os.Getenv("AGENT_BUS_TEST_GROUP_WALK"); walk != "" {
		groupWalkSmallStack(t, walk)
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, walk := range []string{"acl", "directory", "publication", "roles"} {
		t.Run(walk, func(t *testing.T) {
			cmd := exec.Command(binary, "-test.run=^TestGroupWalksUseBoundedStack$", "-test.timeout=20s")
			cmd.Env = append(os.Environ(), "AGENT_BUS_TEST_GROUP_WALK="+walk)
			out, err := cmd.CombinedOutput()
			if err != nil {
				if len(out) > 1200 {
					out = out[:1200]
				}
				t.Fatalf("%s group walk failed with a bounded stack: %v\n%s", walk, err, out)
			}
		})
	}
}

func groupWalkSmallStack(t *testing.T, walk string) {
	b := New()
	b.users["owner@h"] = protocol.User{Name: "owner@h"}
	const depth = 1024
	name := func(i int) string { return fmt.Sprintf("@deep-%04d", i) }
	// Direct construction isolates traversal from mutation checks. A back edge
	// at the leaf proves termination; actual members prove positive results.
	for i := 0; i < depth; i++ {
		members := []string{name(i + 1)}
		if i == depth-1 {
			members = []string{name(0), "member@h", "#receiver@h"}
		}
		b.records[name(i)] = protocol.Record{Name: name(i), Kind: protocol.KindGroup, Owner: "owner@h", Allow: members}
	}
	b.records["member@h"] = protocol.Record{Name: "member@h", Kind: protocol.KindUser, Owner: "owner@h"}
	b.records["#receiver@h"] = protocol.Record{Name: "#receiver@h", Kind: protocol.KindAgent, Owner: "owner@h"}
	old := debug.SetMaxStack(128 << 10)
	defer debug.SetMaxStack(old)
	switch walk {
	case "acl":
		r := protocol.Record{Name: "inbox@h", Kind: protocol.KindQueue, Owner: "owner@h", Allow: []string{name(0)}}
		if !b.may("member@h", r) || b.may("stranger@h", r) {
			t.Fatal("ACL lost the deep member or granted a stranger through the cycle")
		}
	case "directory":
		ix := b.indexDirectory()
		if len(ix.groups["member@h"]) != depth || len(ix.groups["stranger@h"]) != 0 {
			t.Fatalf("directory membership counts: member=%d stranger=%d", len(ix.groups["member@h"]), len(ix.groups["stranger@h"]))
		}
	case "publication":
		r := protocol.Record{Name: "news@h", Owner: "owner@h", Subs: []string{"#first@h", name(0), "#receiver@h", "#last@h"}}
		want := []string{"#first@h", "member@h", "#receiver@h", "#last@h"}
		if got := b.deliverTo(r); !reflect.DeepEqual(got, want) {
			t.Fatalf("depth-first publication recipients = %v, want %v", got, want)
		}
	case "roles":
		grants := []protocol.RoleGrant{{Actor: name(0), Roles: []string{"read"}}}
		if !b.roleGroups("member@h", grants)[name(0)] || b.roleGroups("stranger@h", grants)[name(0)] {
			t.Fatal("assigned roles lost the deep member or granted a stranger through the cycle")
		}
	default:
		t.Fatalf("unknown walk %q", walk)
	}
}
