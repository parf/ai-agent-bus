package bus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func realFixture(t *testing.T) (string, map[string]string) {
	t.Helper()
	binary := os.Getenv("AB_CLIENT_FIXTURE")
	if binary == "" {
		binary = filepath.Join("..", "client-fixture")
	}
	cmd := exec.Command(binary)
	input, e := cmd.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	out, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	cmd.Stderr = os.Stderr
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { input.Close(); cmd.Wait() })
	var info struct {
		Addr   string            `json:"addr"`
		Tokens map[string]string `json:"tokens"`
	}
	if e = json.NewDecoder(out).Decode(&info); e != nil {
		t.Fatal(e)
	}
	return info.Addr, info.Tokens
}
func fixtureBus(t *testing.T, addr, token, name string) *Bus {
	t.Helper()
	b, e := Dial(Options{Addr: addr, Token: token, ExpectedName: name, Timeout: 2 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { b.Close() })
	return b
}
func TestRealDaemonRegistryPrivateTypedKVAndLocks(t *testing.T) {
	addr, tokens := realFixture(t)
	owner := fixtureBus(t, addr, tokens["owner@fixture"], "owner@fixture")
	worker := fixtureBus(t, addr, tokens["#worker@fixture"], "#worker@fixture")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r := owner.Record("#worker@fixture")
	if data, e := r.Lookup(ctx); e != nil || !bytes.Contains(data, []byte(`"#worker@fixture"`)) {
		t.Fatalf("lookup escaped name: %s %v", data, e)
	}
	if cfg, e := r.Config(ctx); e != nil || !bytes.Contains(cfg, []byte("fixture")) {
		t.Fatalf("config %s %v", cfg, e)
	}
	sec, e := r.Secret(ctx)
	if e != nil || !bytes.Contains(sec.Bytes(), []byte("dummy-fixture")) {
		t.Fatal(sec, e)
	}
	for _, format := range []string{"%s", "%v", "%#v", "%+v"} {
		if strings.Contains(fmt.Sprintf(format, sec), "dummy") {
			t.Fatalf("secret leaked via %s", format)
		}
	}
	j := owner.Record("jobs@fixture")
	if v, e := j.Config(ctx); e == nil || v != nil {
		t.Fatalf("queue private kind must refuse %s %v", v, e)
	}
	if _, e = j.Manage(ctx, map[string]any{"maintainers": []string{"#worker@fixture"}}); e != nil {
		t.Fatal(e)
	}
	for _, n := range []int64{-9223372036854775808, 9223372036854775807} {
		if e = j.KV.Set(ctx, KVInt, "large", n, KVSet); e != nil {
			t.Fatal(e)
		}
		v, e := j.KV.Get(ctx, KVInt, "large")
		if e != nil || v.Int != n {
			t.Fatal(v, e)
		}
	}
	if e = j.KV.Set(ctx, KVInt, "runs", int64(1), KVAdd); e != nil {
		t.Fatal(e)
	}
	if n, e := j.KV.Increment(ctx, "runs", 4); e != nil || n != 5 {
		t.Fatal(n, e)
	}
	data := []byte{0, 255, 10}
	if e = j.KV.Set(ctx, KVString, "bytes", data, KVSet); e != nil {
		t.Fatal(e)
	}
	v, e := j.KV.Get(ctx, KVString, "bytes")
	if e != nil || !bytes.Equal(v.Bytes, data) {
		t.Fatal(v, e)
	}
	if _, e = j.KV.Get(ctx, KVString, "absent"); !errors.Is(e, ErrKVNotFound) {
		t.Fatal(e)
	}
	if e = j.KV.Set(ctx, KVJSON, "state", json.RawMessage(`{"done":1}`), KVSet); e != nil {
		t.Fatal(e)
	}
	result, e := j.KV.JSON(ctx, "state", []KVOp{{Op: "inc", Key: "done", Value: json.RawMessage(`2`)}})
	if e != nil || len(result) != 1 || string(result[0].Value) != "3" {
		t.Fatal(result, e)
	}
	if listing, e := j.KV.List(ctx); e != nil || !bytes.Contains(listing, []byte("bytes")) {
		t.Fatal(string(listing), e)
	}
	if gone, e := j.KV.Delete(ctx, KVString, "bytes"); e != nil || !gone {
		t.Fatal(gone, e)
	}
	lease, e := j.Locks.TryAcquire(ctx, "build", time.Second)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = j.Locks.Acquire(ctx, "build", WithTTL(time.Second)); e == nil {
		t.Fatal("same-name retake accepted")
	}
	if e = lease.Extend(ctx, time.Second); e != nil {
		t.Fatal(e)
	}
	if holders, e := j.Locks.Holders(ctx); e != nil || !bytes.Contains(holders, []byte("build")) {
		t.Fatal(string(holders), e)
	}
	if holder, e := worker.Record("jobs@fixture").Locks.ForceRelease(ctx, "build"); e != nil || holder != "owner@fixture" {
		t.Fatal(holder, e)
	}
	name := "#created@fixture"
	if _, e = owner.Register(ctx, map[string]any{"name": name, "kind": "agent", "allow": []string{"*"}}); e != nil {
		t.Fatal(e)
	}
	created := owner.Record(name)
	if _, e = created.Manage(ctx, map[string]any{"desc": "changed"}); e != nil {
		t.Fatal(e)
	}
	if e = created.Unregister(ctx); e != nil {
		t.Fatal(e)
	}
}
func TestRealDaemonNestedServingAtCapacityAndQueueAuthority(t *testing.T) {
	addr, tokens := realFixture(t)
	worker := fixtureBus(t, addr, tokens["#worker@fixture"], "#worker@fixture")
	helper := fixtureBus(t, addr, tokens["#helper@fixture"], "#helper@fixture")
	caller := fixtureBus(t, addr, tokens["#caller@fixture"], "#caller@fixture")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hctx, hstop := context.WithCancel(ctx)
	wctx, wstop := context.WithCancel(ctx)
	hresult := make(chan error, 1)
	wresult := make(chan error, 1)
	go func() {
		hresult <- helper.Serve(hctx, func(_ context.Context, m Envelope) (*string, error) { s := "nested:" + m.Body; return &s, nil }, WithWorkers(2))
	}()
	go func() {
		wresult <- worker.Serve(wctx, func(ctx context.Context, m Envelope) (*string, error) {
			if m.Body == "done" {
				return Done, nil
			}
			out, e := worker.Call(ctx, "#helper@fixture", m.Body, WithTimeout(time.Second))
			if e != nil {
				return nil, e
			}
			s := out.Message.Body
			return &s, nil
		}, WithWorkers(2))
	}()
	outcomes := make(chan error, 2)
	for _, body := range []string{"one", "two"} {
		go func(body string) {
			out, e := caller.Call(ctx, "#worker@fixture", body, WithTimeout(2*time.Second))
			if e == nil && (!out.Answered || out.Message.Body != "nested:"+body || !out.Acked) {
				e = fmt.Errorf("bad outcome: %+v", out)
			}
			outcomes <- e
		}(body)
	}
	for range 2 {
		if e := <-outcomes; e != nil {
			t.Fatal(e)
		}
	}
	out, e := caller.Call(ctx, "#worker@fixture", "done", WithTimeout(time.Second))
	if e != nil || !out.Done {
		t.Fatal(out, e)
	}
	if _, e = caller.Publish(ctx, "jobs@fixture", "queued"); e != nil {
		t.Fatal(e)
	}
	m, e := caller.Consume(ctx, WithInbox("jobs@fixture"), WithWait(time.Second))
	if e != nil || m == nil || m.Body != "queued" {
		t.Fatal(m, e)
	}
	hstop()
	wstop()
	if e = <-hresult; e != nil {
		t.Fatal(e)
	}
	if e = <-wresult; e != nil {
		t.Fatal(e)
	}
}
