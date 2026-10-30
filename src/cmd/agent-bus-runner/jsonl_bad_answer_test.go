package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type badAnswerLog struct {
	sync.Mutex
	text strings.Builder
}

func (l *badAnswerLog) Write(p []byte) (int, error) {
	l.Lock()
	defer l.Unlock()
	return l.text.Write(p)
}
func (l *badAnswerLog) String() string {
	l.Lock()
	defer l.Unlock()
	return l.text.String()
}

func TestJsonlBadAnswerIsLoggedAndDroppedWhileChildKeepsServing(t *testing.T) {
	for _, tc := range []struct{ name, line string }{
		{"unknown_id", `{"message_id":"not-in-flight","body":"must-not-answer"}`},
		{"unparseable", `this is not JSON`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The second valid answer is a FIFO stdout barrier: answers() has
			// handled the bad line before this reply reaches us. No quiet-time
			// probe is needed to show that the first request is still unanswered.
			script := fmt.Sprintf(`read -r first
printf '%%s\n' '%s'
printf ready > ready
read -r second
printf '{"message_id":"%%s","body":"second"}\n' "$(idof "$second")"
read -r third
printf '{"message_id":"%%s","body":"first"}\n' "$(idof "$first")"
printf '{"message_id":"%%s","body":"third"}\n' "$(idof "$third")"
while read -r extra; do :; done
`, tc.line)
			log := &badAnswerLog{}
			st, dir := poolFixtureOutput(t, script, 3, span{Min: 1, Max: 1}, "", log)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			type result struct {
				body string
				err  error
			}
			first := make(chan result, 1)
			go func() { body, err := st.do(ctx, message("first")); first <- result{body, err} }()
			await(t, filepath.Join(dir, "ready"), "ready", 2*time.Second)
			if body, err := st.do(ctx, message("second")); err != nil || body != "second" {
				t.Fatalf("child did not keep serving after bad line: body=%q err=%v", body, err)
			}
			select {
			case got := <-first:
				t.Fatalf("bad line answered or terminated the pending request: %+v", got)
			default:
			}
			if got := strings.Count(log.String(), "an answer line names no message in flight; dropped"); got != 1 {
				t.Fatalf("bad line logged %d times, want 1: %s", got, log.String())
			}
			if body, err := st.do(ctx, message("third")); err != nil || body != "third" {
				t.Fatalf("release answer: body=%q err=%v", body, err)
			}
			select {
			case got := <-first:
				if got.err != nil || got.body != "first" {
					t.Fatalf("pending answer: %+v", got)
				}
			case <-ctx.Done():
				t.Fatal("pending request was not answered after release")
			}
			starts, err := os.ReadFile(filepath.Join(dir, "starts"))
			if err != nil || string(starts) != "s" {
				t.Fatalf("child restarted: starts=%q err=%v", starts, err)
			}
		})
	}
}
