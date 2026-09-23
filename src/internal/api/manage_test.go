package api

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestOwnerControlThroughAPI(t *testing.T) {
	bus := core.New()
	s, token := serverFor(t, bus, "admin@h")
	call := func(who, path, body string, want int) string {
		t.Helper()
		method := "POST"
		if body == "" {
			method = "GET"
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(HeaderToken, token(who))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: got %d %s, want %d", who, path, w.Code, w.Body.String(), want)
		}
		return w.Body.String()
	}
	// The people exist before they act: a credential is issued to somebody,
	// and a name nobody created cannot be handed one
	// (docs/02-access.md#getting-a-token).
	for _, who := range []string{"alice@h", "bob@h", "maint@h"} {
		if _, err := bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	call("alice@h", "/register", `{"kind":"agent","name":"#svc@h","allow":["alice@h"]}`, 200)
	call("admin@h", "/manage", `{"kind":"agent","name":"#svc@h","descr":"node owner may manage every service"}`, 200)
	call("bob@h", "/manage", `{"kind":"agent","name":"#svc@h","owner":"bob@h"}`, 403)
	// A Group is an ordinary record: alice may make one of her own, and may
	// not edit a group someone else owns.
	call("alice@h", "/group", `{"kind":"agent","name":"@alices","members":["alice@h"]}`, 200)
	call("admin@h", "/group", `{"kind":"agent","name":"@ops","members":["maint@h"]}`, 200)
	call("alice@h", "/group", `{"kind":"agent","name":"@ops","members":["alice@h"]}`, 403)
	managed := call("alice@h", "/manage", `{"kind":"agent","name":"#svc@h","maintainers":"@ops","bound":2,"ttl":"1h","overflow":"strict","descr":"owned"}`, 200)
	if !strings.Contains(managed, `"maintainers":["@ops"]`) || strings.Contains(managed, `"maintainers":"@ops"`) {
		t.Fatalf("legacy management input was not answered with the array spelling: %s", managed)
	}
	call("maint@h", "/manage", `{"kind":"agent","name":"#svc@h","descr":"maintained"}`, 200)
	call("maint@h", "/configure", `{"kind":"agent","name":"#svc@h","config":{"secret":"kept"}}`, 200)
	call("maint@h", "/manage", `{"kind":"agent","name":"#svc@h","owner":"maint@h"}`, 403)
	call("maint@h", "/manage", `{"kind":"agent","name":"#svc@h","maintainers":""}`, 403)
	// The removal verb is gone, so this is no longer the 409 that said the
	// group was in use: there is nothing to be in use for.
	call("admin@h", "/group", `{"kind":"agent","name":"@ops","remove":true}`, 400)
	call("alice@h", "/send", `{"to":"#svc@h","body":"preserved"}`, 200)
	call("alice@h", "/manage", `{"kind":"agent","name":"#svc@h","status":"inactive"}`, 200)
	// Inactive is no such record: it is only in the one read-only view.
	call("alice@h", "/lookup?name=%23svc@h", "", 404)
	inactiveOne := func() protocol.Record {
		t.Helper()
		var list []protocol.Record
		if err := json.Unmarshal([]byte(call("alice@h", "/inactive", "", 200)), &list); err != nil {
			t.Fatal(err)
		}
		for _, r := range list {
			if r.Name == "#svc@h" {
				return r
			}
		}
		t.Fatalf("the inactive record is not in its owner's inactive view: %+v", list)
		return protocol.Record{}
	}
	if omitted := inactiveOne(); !reflect.DeepEqual(omitted.Maintainers, protocol.MaintainerList{"@ops"}) || omitted.Status != protocol.StatusInactive {
		t.Fatalf("a status edit changed more than the status: %+v", omitted)
	}
	call("alice@h", "/send", `{"to":"#svc@h","body":"refused"}`, 404)
	// The agent itself is inactive, so it is refused as a caller.
	call("#svc@h", "/consume?wait=0s", "", 403)
	// Its name stays reserved: nobody registers over it, its owner included.
	call("alice@h", "/register", `{"kind":"agent","name":"#svc@h","maintainers":"","allow":["alice@h"]}`, 412)
	if rec := inactiveOne(); !reflect.DeepEqual(rec.Maintainers, protocol.MaintainerList{"@ops"}) || rec.Queued != 1 || !rec.CanManage || !rec.CanTransfer {
		t.Fatalf("a refused registration changed the inactive record: %+v", rec)
	}
	// Every operation naming it but its reactivation is refused, removal too.
	call("alice@h", "/unregister", `{"kind":"agent","name":"#svc@h"}`, 404)
	call("alice@h", "/manage", `{"kind":"agent","name":"#svc@h","descr":"while inactive"}`, 404)
	// Restoration includes policy, flat membership, configuration and backlog.
	snapshot, err := json.Marshal(bus.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	restored := core.New()
	// Decoded into a zero value: decoding over a populated snapshot would
	// merge each record with whichever one sat at that index before.
	var snap ports.Snapshot
	if err := json.Unmarshal(snapshot, &snap); err != nil {
		t.Fatal(err)
	}
	restored.Restore(snap)
	s = New(restored, s.tokens, "admin@h")
	call("#svc@h", "/consume?wait=0s", "", 403)
	call("maint@h", "/manage", `{"kind":"agent","name":"#svc@h","status":"active"}`, 200)
	if got := call("#svc@h", "/consume?wait=0s", "", 200); !strings.Contains(got, "preserved") {
		t.Fatal(got)
	}
	if got := call("#svc@h", "/config?name=%23svc@h", "", 200); !strings.Contains(got, "kept") {
		t.Fatal(got)
	}
	call("admin@h", "/group", `{"kind":"agent","name":"@ops","members":[]}`, 200)
	call("maint@h", "/manage", `{"kind":"agent","name":"#svc@h","descr":"revoked"}`, 403)
	call("alice@h", "/manage", `{"kind":"agent","name":"#svc@h","owner":"missing@h"}`, 400)
	call("alice@h", "/manage", `{"kind":"agent","name":"#svc@h","owner":"bob@h","allow":["bob@h"],"maintainers":""}`, 200)
	afterTransfer := core.New()
	afterTransfer.Restore(restored.Snapshot())
	s = New(afterTransfer, s.tokens, "admin@h")
	call("alice@h", "/manage", `{"kind":"agent","name":"#svc@h","status":"inactive"}`, 403)
	call("alice@h", "/token", `{"kind":"agent","name":"#svc@h"}`, 403)
	call("bob@h", "/token", `{"kind":"agent","name":"#svc@h"}`, 200)
	call("bob@h", "/configure", `{"kind":"agent","name":"#svc@h","config":{"new":true}}`, 200)
	call("bob@h", "/unregister", `{"kind":"agent","name":"#svc@h"}`, 200)
	// A removed name is not reserved for whoever last owned it: the previous
	// owner takes it as readily as anyone, and then owns it
	// (Plans/R1.2/README.md#removed-names defers protecting it).
	call("alice@h", "/register", `{"kind":"agent","name":"#svc@h"}`, 200)
	call("bob@h", "/register", `{"kind":"agent","name":"#svc@h"}`, 403)
	call("alice@h", "/manage", `{"kind":"agent","name":"alice@h","owner":"bob@h"}`, 403)
}

func TestRegistrationNeverSetsMaintainersInEitherWireShape(t *testing.T) {
	bus := core.New()
	s, token := serverFor(t, bus, "admin@h")
	if _, err := bus.SetUser("admin@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	if err := bus.SetGroup("admin@h", "@ops", []string{"alice@h"}); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"#legacy@h": `{"kind":"agent","name":"#legacy@h","maintainers":"@ops"}`,
		"#array@h":  `{"kind":"agent","name":"#array@h","maintainers":["@ops"]}`,
	} {
		if code, answer := post(t, s, token, "alice@h", "/register", body); code != 200 {
			t.Fatalf("register %s answered %d: %s", name, code, answer)
		}
		record, ok := bus.Lookup("alice@h", name)
		if !ok || len(record.Maintainers) != 0 {
			t.Fatalf("registration wrote Maintainers for %s: %+v", name, record)
		}
	}
}
